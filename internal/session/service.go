package session

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"agent_stock/internal/store"
)

// Service orchestrates session persistence for chat turns.
type Service struct {
	sessions store.SessionStore
}

func NewService(sessions store.SessionStore) *Service {
	return &Service{sessions: sessions}
}

// TurnResult is the outcome of one stub chat turn (Phase 2: still echo, but persisted).
type TurnResult struct {
	SessionID    string
	Reply        string
	MessageCount int
}

// ChatTurn ensures session, persists user+assistant messages, returns stub reply.
func (s *Service) ChatTurn(ctx context.Context, sessionID, message string) (*TurnResult, error) {
	if message == "" {
		return nil, fmt.Errorf("message is required")
	}
	if sessionID == "" {
		sessionID = uuid.NewString()
	}

	if err := s.sessions.EnsureSession(ctx, sessionID); err != nil {
		return nil, err
	}

	reply := "echo: " + message
	now := time.Now().UTC()
	err := s.sessions.AppendMessages(ctx, sessionID,
		store.Message{Role: store.RoleUser, Content: message, CreatedAt: now},
		store.Message{Role: store.RoleAssistant, Content: reply, CreatedAt: now},
	)
	if err != nil {
		return nil, err
	}

	n, err := s.sessions.CountMessages(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	return &TurnResult{
		SessionID:    sessionID,
		Reply:        reply,
		MessageCount: n,
	}, nil
}
