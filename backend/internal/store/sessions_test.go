package store

import (
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"testing"
	"time"
)

// openAtVersion creates a database migrated up to schema version v only.
func openAtVersion(t *testing.T, path string, v int) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", path+"?_pragma=foreign_keys(1)&_time_format=sqlite")
	if err != nil {
		t.Fatal(err)
	}
	s := &Store{db: db}
	for i := 0; i < v; i++ {
		err := s.withTx(func(tx *sql.Tx) error { return migrations[i](tx) })
		if err != nil {
			t.Fatalf("migration %d: %v", i+1, err)
		}
	}
	if _, err := db.Exec(fmt.Sprintf(`PRAGMA user_version = %d`, v)); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestUpgradeToV6HashesExistingSessions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "v5.db")
	old := openAtVersion(t, path, 5)
	exec := func(query string, args ...any) {
		t.Helper()
		if _, err := old.Exec(query, args...); err != nil {
			t.Fatalf("%s: %v", query, err)
		}
	}
	live := time.Now().Add(time.Hour).UTC()
	exec(`INSERT INTO users (id, username, password_hash, role) VALUES (1, 'admin', 'h', 'admin'), (2, 'alice', 'h', 'user')`)
	exec(`INSERT INTO sessions (token, user_id, expires_at) VALUES ('admin-token', 1, ?), ('alice-token', 2, ?)`, live, live)
	old.Close()

	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if v := schemaVersion(t, s); v != len(migrations) {
		t.Fatalf("user_version = %d", v)
	}
	for token, want := range map[string]string{"admin-token": "admin", "alice-token": "alice"} {
		u, err := s.UserBySession(token)
		if err != nil || u.Username != want {
			t.Fatalf("session %q after the upgrade: user %+v, err %v", token, u, err)
		}
	}
	var raw int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM sessions WHERE token_hash IN ('admin-token', 'alice-token')`).Scan(&raw); err != nil || raw != 0 {
		t.Fatalf("%d raw tokens left in the database (err %v)", raw, err)
	}
}

func TestSessionsStoreOnlyDigests(t *testing.T) {
	s, _ := openTemp(t)
	uid, err := s.CreateUser("alice", "password1", RoleUser, false)
	if err != nil {
		t.Fatal(err)
	}
	token, err := s.CreateSession(uid, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	var stored string
	if err := s.db.QueryRow(`SELECT token_hash FROM sessions WHERE user_id = ?`, uid).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if stored == token || stored != hashToken(token) || len(stored) != 64 {
		t.Fatalf("stored %q for token %q, want its SHA-256 digest", stored, token)
	}
	// The digest itself is no credential.
	if _, err := s.UserBySession(stored); !errors.Is(err, ErrNotFound) {
		t.Fatalf("signing in with the stored digest: %v", err)
	}
	if got, err := s.UserBySession(token); err != nil || got.ID != uid {
		t.Fatalf("UserBySession = %+v, %v", got, err)
	}
	if err := s.DeleteSession(token); err != nil {
		t.Fatal(err)
	}
	if _, err := s.UserBySession(token); !errors.Is(err, ErrNotFound) {
		t.Fatalf("session survived DeleteSession: %v", err)
	}
}
