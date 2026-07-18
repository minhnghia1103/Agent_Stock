package session

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"agent_stock/internal/agent"
	"agent_stock/internal/bootstrap"
	"agent_stock/internal/provider"
	"agent_stock/internal/store"
	"agent_stock/internal/tools"
)

// Service orchestrates session persistence + agent loop turns.
type Service struct {
	sessions     store.SessionStore
	llm          provider.Provider
	tools        *tools.Registry
	workspaceDir string
	basePrompt   string
	maxIter      int
}

func NewService(
	sessions store.SessionStore,
	llm provider.Provider,
	toolReg *tools.Registry,
	workspaceDir string,
	basePrompt string,
	maxIter int,
) *Service {
	return &Service{
		sessions:     sessions,
		llm:          llm,
		tools:        toolReg,
		workspaceDir: workspaceDir,
		basePrompt:   basePrompt,
		maxIter:      maxIter,
	}
}

// TurnResult is the outcome of one chat turn.
type TurnResult struct {
	SessionID    string
	Reply        string
	MessageCount int
	Model        string
	Usage        *provider.Usage
	Iterations   int
	ToolCalls    int
}

// ChatTurn loads history, builds system prompt from workspace markdown,
// runs agent loop, persists user+assistant.
func (s *Service) ChatTurn(ctx context.Context, sessionID, message string) (*TurnResult, error) {
	if message == "" {
		return nil, fmt.Errorf("message is required")
	}
	if s.llm == nil {
		return nil, fmt.Errorf("llm provider not configured")
	}
	if sessionID == "" {
		sessionID = uuid.NewString()
	}

	if err := s.sessions.EnsureSession(ctx, sessionID); err != nil {
		return nil, err
	}

	history, err := s.sessions.ListMessages(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	// Reload markdown each turn so edits apply without restart (Phase 5 Done).
	files := bootstrap.Load(s.workspaceDir)
	prompt := bootstrap.BuildSystemPrompt(s.basePrompt, files)

	msgs := make([]provider.Message, 0, len(history)+2)
	msgs = append(msgs, provider.Message{Role: provider.RoleSystem, Content: prompt})
	for _, m := range history {
		msgs = append(msgs, provider.Message{Role: m.Role, Content: m.Content})
	}
	msgs = append(msgs, provider.Message{Role: provider.RoleUser, Content: message})

	loop := &agent.Loop{
		LLM:           s.llm,
		Tools:         s.tools,
		MaxIterations: s.maxIter,
	}
	run, err := loop.Run(ctx, msgs)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	if err := s.sessions.AppendMessages(ctx, sessionID,
		store.Message{Role: store.RoleUser, Content: message, CreatedAt: now},
		store.Message{Role: store.RoleAssistant, Content: run.Content, CreatedAt: now},
	); err != nil {
		return nil, err
	}

	n, err := s.sessions.CountMessages(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	slog.Info("agent turn ok",
		"provider", s.llm.Name(),
		"model", run.Model,
		"session_id", sessionID,
		"message_count", n,
		"iterations", run.Iterations,
		"tool_calls", run.ToolCallsRan,
		"bootstrap", bootstrap.PresentNames(files),
	)

	return &TurnResult{
		SessionID:    sessionID,
		Reply:        run.Content,
		MessageCount: n,
		Model:        run.Model,
		Usage:        run.Usage,
		Iterations:   run.Iterations,
		ToolCalls:    run.ToolCallsRan,
	}, nil
}
