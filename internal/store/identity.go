package store

import (
	"context"
	"errors"
	"time"
)

var (
	ErrForbidden = errors.New("forbidden")
	ErrNotFound  = errors.New("not found")
)

const (
	RoleAdmin    = "admin"
	RoleOperator = "operator"
	RoleViewer   = "viewer"
)

// User is a tenant-scoped account.
type User struct {
	ID        string
	TenantID  string
	Email     string
	Role      string
	CreatedAt time.Time
}

// APIKeyMeta is a stored API key (hash only).
type APIKeyMeta struct {
	ID        string
	UserID    string
	Name      string
	KeyPrefix string
	KeyHash   string
	CreatedAt time.Time
	RevokedAt *time.Time
}

// Agent is a user-owned agent instance.
type Agent struct {
	ID          string
	TenantID    string
	OwnerUserID string
	Name        string
	Slug        string
	CreatedAt   time.Time
}

// BootstrapUser seeds a demo account (plaintext key handled by auth package).
type BootstrapUser struct {
	ID      string
	Email   string
	Role    string
	KeyID   string
	KeyName string
}

// IdentityStore looks up users via API keys and manages agents.
type IdentityStore interface {
	LookupAPIKey(ctx context.Context, keyHash string) (*User, *APIKeyMeta, error)
	SeedBootstrap(ctx context.Context, tenantID, tenantName string, users []BootstrapUser, keyHashes, keyPrefixes map[string]string) error
	ListAgents(ctx context.Context, ownerUserID string) ([]Agent, error)
	GetAgent(ctx context.Context, ownerUserID, agentIDOrSlug string) (*Agent, error)
	CreateAgent(ctx context.Context, a Agent) (*Agent, error)
	EnsureDefaultAgent(ctx context.Context, user User) (*Agent, error)
}
