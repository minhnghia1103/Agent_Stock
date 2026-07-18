package session

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"agent_stock/internal/provider"
	"agent_stock/internal/store"
)

// Service orchestrates session persistence + LLM chat turns.
type Service struct {
	sessions     store.SessionStore
	llm          provider.Provider
	systemPrompt string
}

func NewService(sessions store.SessionStore, llm provider.Provider, systemPrompt string) *Service {
	return &Service{
		sessions:     sessions,
		llm:          llm,
		systemPrompt: systemPrompt,
	}
}

// TurnResult is the outcome of one chat turn.
type TurnResult struct {
	SessionID    string
	Reply        string
	MessageCount int
	Model        string
	Usage        *provider.Usage
}

// ChatTurn loads history, calls the LLM, persists user+assistant messages.
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
	if s.systemPrompt != "" {
		msgs = append(msgs, provider.Message{Role: provider.RoleSystem, Content: s.systemPrompt})
	}
	for _, m := range history {
		msgs = append(msgs, provider.Message{Role: m.Role, Content: m.Content})
	}
	msgs = append(msgs, provider.Message{Role: provider.RoleUser, Content: message})

	resp, err := s.llm.Chat(ctx, provider.ChatRequest{Messages: msgs})
	if err != nil {
		return nil, fmt.Errorf("llm chat: %w", err)
	}
	reply := resp.Content
	if reply == "" {
		return nil, fmt.Errorf("llm returned empty content")
	}

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

	slog.Info("llm chat ok",
		"provider", s.llm.Name(),
		"model", resp.Model,
		"session_id", sessionID,
		"message_count", n,
	)

	return &TurnResult{
		SessionID:    sessionID,
		Reply:        reply,
		MessageCount: n,
		Model:        resp.Model,
		Usage:        resp.Usage,
	}, nil
}
