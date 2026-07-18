package store

import (
	"context"
	"time"
)

const (
	RoleUser      = "user"
	RoleAssistant = "assistant"
)

// Message is one turn message in a session.
type Message struct {
	ID        int64
	SessionID string
	Role      string
	Content   string
	CreatedAt time.Time
}

// SessionStore persists chat sessions and messages (Repository).
type SessionStore interface {
	EnsureSession(ctx context.Context, sessionID string) error
	AppendMessages(ctx context.Context, sessionID string, msgs ...Message) error
	ListMessages(ctx context.Context, sessionID string) ([]Message, error)
	CountMessages(ctx context.Context, sessionID string) (int, error)
	Close() error
}
