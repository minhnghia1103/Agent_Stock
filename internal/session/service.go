package session

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"agent_stock/internal/agent"
	"agent_stock/internal/provider"
	"agent_stock/internal/store"
	"agent_stock/internal/tools"
)

// Service orchestrates session persistence + agent loop turns.
type Service struct {
	sessions     store.SessionStore
	llm          provider.Provider
	tools        *tools.Registry
	systemPrompt string
	maxIter      int
}

func NewService(sessions store.SessionStore, llm provider.Provider, toolReg *tools.Registry, systemPrompt string, maxIter int) *Service {
	return &Service{
		sessions:     sessions,
		llm:          llm,
		tools:        toolReg,
		systemPrompt: systemPrompt,
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

// ChatTurn loads history, runs agent loop (tools), persists user+assistant.
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

	msgs := make([]provider.Message, 0, len(history)+2)
	prompt := s.systemPrompt
	if prompt == "" {
		prompt = "You are a helpful stock research assistant. Use tools when they improve accuracy."
	}
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
