package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"strings"

	"agent_stock/internal/store"
)

const (
	DefaultTenantID = "default"

	DefaultAliceID  = "user_alice"
	DefaultBobID    = "user_bob"
	DefaultAliceKey = "sk_alice_dev_change_me"
	DefaultBobKey   = "sk_bob_dev_change_me"
)

// Identity is the authenticated principal (from API key, never from body).
type Identity struct {
	TenantID string
	UserID   string
	Email    string
	Role     string
}

// CanChat reports whether role may run chat turns.
func (id Identity) CanChat() bool {
	return id.Role == store.RoleAdmin || id.Role == store.RoleOperator
}

// CanWriteAgents reports whether role may create agents.
func (id Identity) CanWriteAgents() bool {
	return id.Role == store.RoleAdmin || id.Role == store.RoleOperator
}

// HashAPIKey returns sha256 hex of the raw key.
func HashAPIKey(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func keyPrefix(raw string) string {
	if len(raw) <= 12 {
		return raw
	}
	return raw[:12] + "…"
}

// Authenticator resolves Bearer API keys.
type Authenticator struct {
	store store.IdentityStore
}

func NewAuthenticator(s store.IdentityStore) *Authenticator {
	return &Authenticator{store: s}
}

// AuthenticateRaw validates a raw API key string.
func (a *Authenticator) AuthenticateRaw(ctx context.Context, raw string) (*Identity, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, store.ErrNotFound
	}
	user, _, err := a.store.LookupAPIKey(ctx, HashAPIKey(raw))
	if err != nil {
		return nil, err
	}
	return &Identity{
		TenantID: user.TenantID,
		UserID:   user.ID,
		Email:    user.Email,
		Role:     user.Role,
	}, nil
}

// BootstrapDemo seeds default tenant + alice/bob API keys (idempotent).
func BootstrapDemo(ctx context.Context, idStore store.IdentityStore) (aliceKey, bobKey string, err error) {
	aliceKey = os.Getenv("AGENT_BOOTSTRAP_KEY_ALICE")
	if aliceKey == "" {
		aliceKey = DefaultAliceKey
	}
	bobKey = os.Getenv("AGENT_BOOTSTRAP_KEY_BOB")
	if bobKey == "" {
		bobKey = DefaultBobKey
	}

	users := []store.BootstrapUser{
		{ID: DefaultAliceID, Email: "alice@example.com", Role: store.RoleAdmin, KeyID: "key_alice", KeyName: "alice-dev"},
		{ID: DefaultBobID, Email: "bob@example.com", Role: store.RoleOperator, KeyID: "key_bob", KeyName: "bob-dev"},
	}
	hashes := map[string]string{
		DefaultAliceID: HashAPIKey(aliceKey),
		DefaultBobID:   HashAPIKey(bobKey),
	}
	prefixes := map[string]string{
		DefaultAliceID: keyPrefix(aliceKey),
		DefaultBobID:   keyPrefix(bobKey),
	}
	if err := idStore.SeedBootstrap(ctx, DefaultTenantID, "Default tenant", users, hashes, prefixes); err != nil {
		return "", "", fmt.Errorf("bootstrap: %w", err)
	}
	return aliceKey, bobKey, nil
}
