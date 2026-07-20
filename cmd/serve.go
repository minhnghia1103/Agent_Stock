package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"agent_stock/internal/bootstrap"
	"agent_stock/internal/config"
	httpserver "agent_stock/internal/http"
	"agent_stock/internal/mcp"
	"agent_stock/internal/provider"
	"agent_stock/internal/security"
	"agent_stock/internal/session"
	"agent_stock/internal/skills"
	"agent_stock/internal/store/sqlite"
	"agent_stock/internal/tools"
)

func serveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "serve",
		Short: "Start the HTTP gateway",
		Run: func(cmd *cobra.Command, args []string) {
			runServe()
		},
	}
}

func runServe() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config: %v\n", err)
		os.Exit(1)
	}
	setupLogging()

	pol, err := security.Load(cfg.PolicyPath)
	if err != nil {
		slog.Error("failed to load policy", "error", err, "path", cfg.PolicyPath)
		os.Exit(1)
	}
	guard := security.NewInputGuard()

	sessionStore, err := sqlite.Open(cfg.DatabasePath, sqlite.SchemaSQL())
	if err != nil {
		slog.Error("failed to open database", "error", err, "path", cfg.DatabasePath)
		os.Exit(1)
	}
	defer func() {
		if err := sessionStore.Close(); err != nil {
			slog.Error("close database", "error", err)
		}
	}()

	llm, err := provider.NewFromConfig(provider.BuildConfig{
		Name:         cfg.LLMProvider,
		APIKey:       cfg.LLMAPIKey,
		BaseURL:      cfg.LLMBaseURL,
		DefaultModel: cfg.LLMModel,
		Timeout:      time.Duration(cfg.LLMTimeoutSec) * time.Second,
		MaxRetries:   cfg.LLMMaxRetries,
	})
	if err != nil {
		slog.Error("failed to init llm provider", "error", err)
		os.Exit(1)
	}

	ws, err := tools.NewWorkspace(cfg.WorkspacePath)
	if err != nil {
		slog.Error("failed to init workspace", "error", err, "path", cfg.WorkspacePath)
		os.Exit(1)
	}
	if err := bootstrap.SeedIfMissing(ws.Root()); err != nil {
		slog.Error("failed to seed workspace bootstrap", "error", err)
		os.Exit(1)
	}
	if err := skills.SeedIfMissing(ws.Root()); err != nil {
		slog.Error("failed to seed skills", "error", err)
		os.Exit(1)
	}
	bootFiles := bootstrap.Load(ws.Root())

	skillsReg := skills.NewRegistry(ws.Root(), "")
	if cfg.SkillsPath != "" {
		skillsReg = skills.NewRegistryWithDir(cfg.SkillsPath)
	}
	if err := skillsReg.Reload(); err != nil {
		slog.Error("failed to load skills", "error", err)
		os.Exit(1)
	}

	toolReg := tools.NewRegistry()
	toolReg.SetPolicy(pol)
	tools.RegisterBuiltins(toolReg, ws, pol)
	tools.RegisterSkillTools(toolReg, skillsReg)

	mcpMgr := mcp.NewManager(toolReg, cfg.MCPJSONPath)
	if err := mcpMgr.Start(context.Background()); err != nil {
		slog.Error("failed to start mcp manager", "error", err, "path", cfg.MCPJSONPath)
		os.Exit(1)
	}
	defer mcpMgr.Close()

	sessionSvc := session.NewService(sessionStore, llm, toolReg, skillsReg, ws.Root(), cfg.SystemPrompt, cfg.MaxToolIterations, pol, guard)
	srv := httpserver.New(cfg, Version, sessionSvc, pol, mcpMgr, skillsReg)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	httpServer := &http.Server{
		Addr:              cfg.Addr(),
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		skillNames := make([]string, 0)
		for _, sk := range skillsReg.List() {
			skillNames = append(skillNames, sk.Slug)
		}
		slog.Info("listening",
			"addr", cfg.Addr(),
			"database", cfg.DatabasePath,
			"workspace", ws.Root(),
			"policy", cfg.PolicyPath,
			"mcp_json", cfg.MCPJSONPath,
			"mcp_tools", mcpMgr.ToolNames(),
			"skills", skillNames,
			"bootstrap", bootstrap.PresentNames(bootFiles),
			"llm_provider", llm.Name(),
			"llm_model", llm.DefaultModel(),
			"tools", toolReg.Names(),
			"tools_enabled", provider.SupportsTools(llm),
			"injection_action", pol.PromptInjection.Action,
		)
		errCh <- httpServer.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		slog.Info("shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			slog.Error("shutdown error", "error", err)
			os.Exit(1)
		}
	case err := <-errCh:
		if err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}
}

func setupLogging() {
	level := slog.LevelInfo
	if verbose {
		level = slog.LevelDebug
	}
	if lvl := os.Getenv("AGENT_LOG_LEVEL"); lvl != "" {
		switch strings.ToLower(lvl) {
		case "debug":
			level = slog.LevelDebug
		case "info":
			level = slog.LevelInfo
		case "warn":
			level = slog.LevelWarn
		case "error":
			level = slog.LevelError
		default:
			fmt.Fprintf(os.Stderr, "warning: unknown AGENT_LOG_LEVEL=%q, using info\n", lvl)
		}
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: level})))
}
