package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"

	"agent_stock/internal/store"
)

// Store is a SQLite SessionStore + IdentityStore.
type Store struct {
	db *sql.DB
}

// Open opens (or creates) a SQLite DB at path and runs migrations.
func Open(path string, migrateSQL string) (*Store, error) {
	dir := filepath.Dir(path)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("mkdir database dir: %w", err)
		}
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	db.SetMaxOpenConns(1)
	db.SetConnMaxLifetime(0)

	if _, err := db.Exec(`PRAGMA foreign_keys = ON`); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("pragma foreign_keys: %w", err)
	}
	if _, err := db.Exec(migrateSQL); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	if err := migrateLegacy(db); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("migrate legacy: %w", err)
	}

	return &Store{db: db}, nil
}

func (s *Store) Close() error {
	if s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *Store) EnsureSession(ctx context.Context, sessionID string, owner store.SessionOwner) error {
	if owner.UserID == "" {
		return fmt.Errorf("user_id required")
	}
	if owner.AgentID == "" {
		owner.AgentID = "default"
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)

	var existingUser string
	err := s.db.QueryRowContext(ctx, `SELECT user_id FROM sessions WHERE id = ?`, sessionID).Scan(&existingUser)
	if err == nil {
		if existingUser != "" && existingUser != owner.UserID {
			return store.ErrForbidden
		}
		_, err = s.db.ExecContext(ctx, `
			UPDATE sessions
			SET updated_at = ?, tenant_id = CASE WHEN tenant_id = '' THEN ? ELSE tenant_id END,
			    user_id = CASE WHEN user_id = '' THEN ? ELSE user_id END,
			    agent_id = CASE WHEN agent_id = '' OR agent_id = 'default' THEN ? ELSE agent_id END
			WHERE id = ?
		`, now, owner.TenantID, owner.UserID, owner.AgentID, sessionID)
		if err != nil {
			return fmt.Errorf("touch session: %w", err)
		}
		return nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("lookup session: %w", err)
	}

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO sessions (id, tenant_id, user_id, agent_id, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, sessionID, owner.TenantID, owner.UserID, owner.AgentID, now, now)
	if err != nil {
		return fmt.Errorf("ensure session: %w", err)
	}
	return nil
}

func (s *Store) assertSessionOwner(ctx context.Context, sessionID string, owner store.SessionOwner) error {
	var userID string
	err := s.db.QueryRowContext(ctx, `SELECT user_id FROM sessions WHERE id = ?`, sessionID).Scan(&userID)
	if errors.Is(err, sql.ErrNoRows) {
		return store.ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("lookup session: %w", err)
	}
	if userID != owner.UserID {
		return store.ErrForbidden
	}
	return nil
}

func (s *Store) AppendMessages(ctx context.Context, sessionID string, owner store.SessionOwner, msgs ...store.Message) error {
	if len(msgs) == 0 {
		return nil
	}
	if err := s.EnsureSession(ctx, sessionID, owner); err != nil {
		return err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO messages (session_id, role, content, created_at)
		VALUES (?, ?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("prepare insert message: %w", err)
	}
	defer stmt.Close()

	now := time.Now().UTC()
	for _, m := range msgs {
		created := m.CreatedAt
		if created.IsZero() {
			created = now
		}
		if _, err := stmt.ExecContext(ctx, sessionID, m.Role, m.Content, created.UTC().Format(time.RFC3339Nano)); err != nil {
			return fmt.Errorf("insert message: %w", err)
		}
	}

	if _, err := tx.ExecContext(ctx, `UPDATE sessions SET updated_at = ? WHERE id = ?`, now.Format(time.RFC3339Nano), sessionID); err != nil {
		return fmt.Errorf("touch session: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

func (s *Store) ListMessages(ctx context.Context, sessionID string, owner store.SessionOwner) ([]store.Message, error) {
	if err := s.assertSessionOwner(ctx, sessionID, owner); err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, session_id, role, content, created_at
		FROM messages
		WHERE session_id = ?
		ORDER BY id ASC
	`, sessionID)
	if err != nil {
		return nil, fmt.Errorf("list messages: %w", err)
	}
	defer rows.Close()

	var out []store.Message
	for rows.Next() {
		var m store.Message
		var created string
		if err := rows.Scan(&m.ID, &m.SessionID, &m.Role, &m.Content, &created); err != nil {
			return nil, fmt.Errorf("scan message: %w", err)
		}
		t, err := time.Parse(time.RFC3339Nano, created)
		if err != nil {
			t, _ = time.Parse(time.RFC3339, created)
		}
		m.CreatedAt = t
		out = append(out, m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *Store) CountMessages(ctx context.Context, sessionID string, owner store.SessionOwner) (int, error) {
	if err := s.assertSessionOwner(ctx, sessionID, owner); err != nil {
		return 0, err
	}
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM messages WHERE session_id = ?`, sessionID).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("count messages: %w", err)
	}
	return n, nil
}
