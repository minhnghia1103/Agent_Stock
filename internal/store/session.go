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

// SessionOwner scopes a chat session to a user/agent.
type SessionOwner struct {
	TenantID string
	UserID   string
	AgentID  string
}

// SessionStore persists chat sessions and messages (Repository).
// All reads/writes are ownership-checked (Phase 10A).
type SessionStore interface {
	EnsureSession(ctx context.Context, sessionID string, owner SessionOwner) error
	AppendMessages(ctx context.Context, sessionID string, owner SessionOwner, msgs ...Message) error
	ListMessages(ctx context.Context, sessionID string, owner SessionOwner) ([]Message, error)
	CountMessages(ctx context.Context, sessionID string, owner SessionOwner) (int, error)
	Close() error
}
