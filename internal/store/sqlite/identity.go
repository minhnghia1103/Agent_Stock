package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"agent_stock/internal/store"
)

func (s *Store) LookupAPIKey(ctx context.Context, keyHash string) (*store.User, *store.APIKeyMeta, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT k.id, k.user_id, k.name, k.key_prefix, k.key_hash, k.created_at, k.revoked_at,
		       u.id, u.tenant_id, u.email, u.role, u.created_at
		FROM api_keys k
		JOIN users u ON u.id = k.user_id
		WHERE k.key_hash = ?
	`, keyHash)

	var key store.APIKeyMeta
	var user store.User
	var keyCreated, userCreated string
	var revoked sql.NullString
	err := row.Scan(
		&key.ID, &key.UserID, &key.Name, &key.KeyPrefix, &key.KeyHash, &keyCreated, &revoked,
		&user.ID, &user.TenantID, &user.Email, &user.Role, &userCreated,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil, store.ErrNotFound
	}
	if err != nil {
		return nil, nil, fmt.Errorf("lookup api key: %w", err)
	}
	if revoked.Valid && revoked.String != "" {
		return nil, nil, store.ErrForbidden
	}
	key.CreatedAt, _ = parseTime(keyCreated)
	user.CreatedAt, _ = parseTime(userCreated)
	return &user, &key, nil
}

// SeedBootstrap inserts tenant/users/keys. keyHashes values are already hashed.
func (s *Store) SeedBootstrap(ctx context.Context, tenantID, tenantName string, users []store.BootstrapUser, keyHashes map[string]string, keyPrefixes map[string]string) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := s.db.ExecContext(ctx, `
		INSERT INTO tenants (id, name, created_at) VALUES (?, ?, ?)
		ON CONFLICT(id) DO NOTHING
	`, tenantID, tenantName, now); err != nil {
		return fmt.Errorf("seed tenant: %w", err)
	}

	for _, u := range users {
		if _, err := s.db.ExecContext(ctx, `
			INSERT INTO users (id, tenant_id, email, role, created_at)
			VALUES (?, ?, ?, ?, ?)
			ON CONFLICT(id) DO UPDATE SET email = excluded.email, role = excluded.role
		`, u.ID, tenantID, u.Email, u.Role, now); err != nil {
			return fmt.Errorf("seed user: %w", err)
		}

		hash := keyHashes[u.ID]
		prefix := keyPrefixes[u.ID]
		if hash == "" {
			continue
		}
		keyID := u.KeyID
		if keyID == "" {
			keyID = uuid.NewString()
		}
		var existing string
		err := s.db.QueryRowContext(ctx, `SELECT id FROM api_keys WHERE key_hash = ?`, hash).Scan(&existing)
		if errors.Is(err, sql.ErrNoRows) {
			if _, err := s.db.ExecContext(ctx, `
				INSERT INTO api_keys (id, user_id, name, key_prefix, key_hash, created_at)
				VALUES (?, ?, ?, ?, ?, ?)
			`, keyID, u.ID, u.KeyName, prefix, hash, now); err != nil {
				return fmt.Errorf("seed api key: %w", err)
			}
		} else if err != nil {
			return err
		}

		if _, err := s.EnsureDefaultAgent(ctx, store.User{ID: u.ID, TenantID: tenantID, Email: u.Email, Role: u.Role}); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) ListAgents(ctx context.Context, ownerUserID string) ([]store.Agent, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, tenant_id, owner_user_id, name, slug, created_at
		FROM agents WHERE owner_user_id = ?
		ORDER BY slug ASC
	`, ownerUserID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []store.Agent
	for rows.Next() {
		var a store.Agent
		var created string
		if err := rows.Scan(&a.ID, &a.TenantID, &a.OwnerUserID, &a.Name, &a.Slug, &created); err != nil {
			return nil, err
		}
		a.CreatedAt, _ = parseTime(created)
		out = append(out, a)
	}
	return out, rows.Err()
}

func (s *Store) GetAgent(ctx context.Context, ownerUserID, agentIDOrSlug string) (*store.Agent, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, tenant_id, owner_user_id, name, slug, created_at
		FROM agents
		WHERE owner_user_id = ? AND (id = ? OR slug = ?)
	`, ownerUserID, agentIDOrSlug, agentIDOrSlug)
	var a store.Agent
	var created string
	err := row.Scan(&a.ID, &a.TenantID, &a.OwnerUserID, &a.Name, &a.Slug, &created)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, store.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	a.CreatedAt, _ = parseTime(created)
	return &a, nil
}

func (s *Store) CreateAgent(ctx context.Context, a store.Agent) (*store.Agent, error) {
	if a.ID == "" {
		a.ID = uuid.NewString()
	}
	if a.Slug == "" {
		a.Slug = "default"
	}
	now := time.Now().UTC()
	a.CreatedAt = now
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO agents (id, tenant_id, owner_user_id, name, slug, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, a.ID, a.TenantID, a.OwnerUserID, a.Name, a.Slug, now.Format(time.RFC3339Nano))
	if err != nil {
		return nil, fmt.Errorf("create agent: %w", err)
	}
	return &a, nil
}

func (s *Store) EnsureDefaultAgent(ctx context.Context, user store.User) (*store.Agent, error) {
	ag, err := s.GetAgent(ctx, user.ID, "default")
	if err == nil {
		return ag, nil
	}
	if !errors.Is(err, store.ErrNotFound) {
		return nil, err
	}
	return s.CreateAgent(ctx, store.Agent{
		TenantID:    user.TenantID,
		OwnerUserID: user.ID,
		Name:        "Default",
		Slug:        "default",
	})
}

func parseTime(s string) (time.Time, error) {
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		return time.Parse(time.RFC3339, s)
	}
	return t, nil
}
