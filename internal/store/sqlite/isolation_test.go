package sqlite_test

import (
	"errors"
	"path/filepath"
	"testing"

	"agent_stock/internal/auth"
	"agent_stock/internal/store"
	"agent_stock/internal/store/sqlite"
	"agent_stock/internal/tools"
	"agent_stock/internal/workspacepath"
)

func TestSessionIsolationBetweenUsers(t *testing.T) {
	path := filepath.Join(t.TempDir(), "t.db")
	db, err := sqlite.Open(path, sqlite.SchemaSQL())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	aliceKey, bobKey, err := auth.BootstrapDemo(t.Context(), db)
	if err != nil {
		t.Fatal(err)
	}
	_ = bobKey

	aAuth := auth.NewAuthenticator(db)
	alice, err := aAuth.AuthenticateRaw(t.Context(), aliceKey)
	if err != nil {
		t.Fatal(err)
	}
	bob, err := aAuth.AuthenticateRaw(t.Context(), auth.DefaultBobKey)
	if err != nil {
		t.Fatal(err)
	}

	ownerA := store.SessionOwner{TenantID: alice.TenantID, UserID: alice.UserID, AgentID: "default"}
	ownerB := store.SessionOwner{TenantID: bob.TenantID, UserID: bob.UserID, AgentID: "default"}

	sid := "sess-alice-1"
	if err := db.EnsureSession(t.Context(), sid, ownerA); err != nil {
		t.Fatal(err)
	}
	if err := db.AppendMessages(t.Context(), sid, ownerA, store.Message{Role: store.RoleUser, Content: "secret from alice"}); err != nil {
		t.Fatal(err)
	}

	_, err = db.ListMessages(t.Context(), sid, ownerB)
	if !errors.Is(err, store.ErrForbidden) {
		t.Fatalf("bob should be forbidden, got %v", err)
	}

	msgs, err := db.ListMessages(t.Context(), sid, ownerA)
	if err != nil || len(msgs) != 1 {
		t.Fatalf("alice read: %v %#v", err, msgs)
	}
}

func TestWorkspacePathsIsolated(t *testing.T) {
	base := t.TempDir()
	a, err := workspacepath.PrivateRoot(base, "user_alice", "default")
	if err != nil {
		t.Fatal(err)
	}
	b, err := workspacepath.PrivateRoot(base, "user_bob", "default")
	if err != nil {
		t.Fatal(err)
	}
	wa, err := tools.NewWorkspace(a)
	if err != nil {
		t.Fatal(err)
	}
	wb, err := tools.NewWorkspace(b)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := wa.Resolve("../user_bob/secret.txt"); err == nil {
		t.Fatal("expected path escape blocked")
	}
	if wa.Root() == wb.Root() {
		t.Fatal("roots must differ")
	}
}
