package session

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"agent_stock/internal/agent"
	"agent_stock/internal/bootstrap"
	"agent_stock/internal/progress"
	"agent_stock/internal/provider"
	"agent_stock/internal/security"
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
	policy       *security.Policy
	guard        *security.InputGuard
}

func NewService(
	sessions store.SessionStore,
	llm provider.Provider,
	toolReg *tools.Registry,
	workspaceDir string,
	basePrompt string,
	maxIter int,
	pol *security.Policy,
	guard *security.InputGuard,
) *Service {
	return &Service{
		sessions:     sessions,
		llm:          llm,
		tools:        toolReg,
		workspaceDir: workspaceDir,
		basePrompt:   basePrompt,
		maxIter:      maxIter,
		policy:       pol,
		guard:        guard,
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

// ChatTurn runs a non-streaming chat turn.
func (s *Service) ChatTurn(ctx context.Context, sessionID, message string) (*TurnResult, error) {
	return s.ChatTurnWithEvents(ctx, sessionID, message, nil)
}

// ChatTurnWithEvents runs a chat turn and emits progress events (SSE / WS).
func (s *Service) ChatTurnWithEvents(ctx context.Context, sessionID, message string, emit progress.Emitter) (*TurnResult, error) {
	if message == "" {
		return nil, fmt.Errorf("message is required")
	}
	if s.llm == nil {
		return nil, fmt.Errorf("llm provider not configured")
	}
	if err := security.CheckUserMessage(s.policy, s.guard, message); err != nil {
		progress.Emit(emit, progress.EventError, map[string]any{"message": err.Error()})
		return nil, err
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

	files := bootstrap.Load(s.workspaceDir)
	prompt := bootstrap.BuildSystemPrompt(s.basePrompt, files)

	msgs := make([]provider.Message, 0, len(history)+2)
	msgs = append(msgs, provider.Message{Role: provider.RoleSystem, Content: prompt})
	for _, m := range history {
		msgs = append(msgs, provider.Message{Role: m.Role, Content: m.Content})
	}
	msgs = append(msgs, provider.Message{Role: provider.RoleUser, Content: message})

	progress.Emit(emit, progress.EventRunStarted, map[string]any{
		"session_id": sessionID,
	})

	loop := &agent.Loop{
		LLM:           s.llm,
		Tools:         s.tools,
		MaxIterations: s.maxIter,
		OnEvent:       emit,
	}
	run, err := loop.Run(ctx, msgs)
	if err != nil {
		return nil, err
	}

	reply := security.Redact(s.policy, run.Content)

	now := time.Now().UTC()
	if err := s.sessions.AppendMessages(ctx, sessionID,
		store.Message{Role: store.RoleUser, Content: message, CreatedAt: now},
		store.Message{Role: store.RoleAssistant, Content: reply, CreatedAt: now},
	); err != nil {
		return nil, err
	}

	n, err := s.sessions.CountMessages(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	result := &TurnResult{
		SessionID:    sessionID,
		Reply:        reply,
		MessageCount: n,
		Model:        run.Model,
		Usage:        run.Usage,
		Iterations:   run.Iterations,
		ToolCalls:    run.ToolCallsRan,
	}

	payload := map[string]any{
		"session_id":    result.SessionID,
		"reply":         result.Reply,
		"message_count": result.MessageCount,
		"model":         result.Model,
		"iterations":    result.Iterations,
		"tool_calls":    result.ToolCalls,
	}
	if result.Usage != nil {
		payload["usage"] = result.Usage
	}
	progress.Emit(emit, progress.EventRunCompleted, payload)

	slog.Info("agent turn ok",
		"provider", s.llm.Name(),
		"model", run.Model,
		"session_id", sessionID,
		"message_count", n,
		"iterations", run.Iterations,
		"tool_calls", run.ToolCallsRan,
		"bootstrap", bootstrap.PresentNames(files),
	)

	return result, nil
}
