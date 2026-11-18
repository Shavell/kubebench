package store

import (
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// v1DDL is the schema exactly as v0.4 created it (no user_version, no
// foreign key enforcement, times in Go's time.String format).
const v1DDL = `
CREATE TABLE IF NOT EXISTS users (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  username TEXT NOT NULL UNIQUE,
  password_hash TEXT NOT NULL,
  role TEXT NOT NULL DEFAULT 'user',
  active_cluster_id INTEGER,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE IF NOT EXISTS sessions (
  token TEXT PRIMARY KEY,
  user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  expires_at TIMESTAMP NOT NULL
);
CREATE TABLE IF NOT EXISTS clusters (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  name TEXT NOT NULL UNIQUE,
  source TEXT NOT NULL,
  kubeconfig TEXT,
  context TEXT,
  address TEXT,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE IF NOT EXISTS user_clusters (
  user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  cluster_id INTEGER NOT NULL REFERENCES clusters(id) ON DELETE CASCADE,
  PRIMARY KEY (user_id, cluster_id)
);`

func openTemp(t *testing.T) (*Store, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "kubebench.db")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s, path
}

func schemaVersion(t *testing.T, s *Store) int {
	t.Helper()
	var v int
	if err := s.db.QueryRow(`PRAGMA user_version`).Scan(&v); err != nil {
		t.Fatal(err)
	}
	return v
}

func TestFreshDatabaseMigratesToLatest(t *testing.T) {
	s, path := openTemp(t)
	if v := schemaVersion(t, s); v != len(migrations) {
		t.Fatalf("user_version = %d, want %d", v, len(migrations))
	}
	for _, table := range []string{"users", "sessions", "clusters", "user_clusters", "audit_log", "plugin_settings", "plugin_kv"} {
		var n int
		if err := s.db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?`, table).Scan(&n); err != nil || n != 1 {
			t.Fatalf("table %s missing (err %v)", table, err)
		}
	}
	var fk int
	if err := s.db.QueryRow(`PRAGMA foreign_keys`).Scan(&fk); err != nil || fk != 1 {
		t.Fatalf("foreign_keys = %d (err %v)", fk, err)
	}
	s.Close()

	again, err := Open(path)
	if err != nil {
		t.Fatalf("reopening a migrated database: %v", err)
	}
	again.Close()
}

func TestNewerSchemaIsRefused(t *testing.T) {
	s, path := openTemp(t)
	if _, err := s.db.Exec(`PRAGMA user_version = 99`); err != nil {
		t.Fatal(err)
	}
	s.Close()
	if _, err := Open(path); err == nil || !strings.Contains(err.Error(), "newer") {
		t.Fatalf("expected a newer-schema error, got %v", err)
	}
}

func TestUpgradeFromV1KeepsData(t *testing.T) {
	path := filepath.Join(t.TempDir(), "v1.db")
	old, err := sql.Open("sqlite", path+"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)")
	if err != nil {
		t.Fatal(err)
	}
	expiry := time.Now().Add(time.Hour).UTC()
	exec := func(query string, args ...any) {
		t.Helper()
		if _, err := old.Exec(query, args...); err != nil {
			t.Fatalf("%s: %v", query, err)
		}
	}
	exec(v1DDL)
	exec(`INSERT INTO clusters (id, name, source, kubeconfig) VALUES (1, 'prod', 'kubeconfig', 'apiVersion: v1'), (2, 'dev', 'kubeconfig', '')`)
	exec(`INSERT INTO users (id, username, password_hash, role, active_cluster_id) VALUES
	      (1, 'admin', 'h', 'admin', NULL), (2, 'alice', 'h', 'user', 1), (3, 'bob', 'h', 'user', 99)`)
	// (77, 1) and the session of user 77 are orphans left by v0.4, which
	// deleted users without foreign key enforcement.
	exec(`INSERT INTO user_clusters (user_id, cluster_id) VALUES (2, 1), (2, 2), (3, 1), (77, 1)`)
	exec(`INSERT INTO sessions (token, user_id, expires_at) VALUES (?, 2, ?), ('orphan', 77, ?), ('garbled', 2, 'not a time')`,
		"alice-token", expiry, expiry)
	old.Close()

	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if v := schemaVersion(t, s); v != len(migrations) {
		t.Fatalf("user_version = %d", v)
	}

	users, err := s.ListUsers()
	if err != nil || len(users) != 3 {
		t.Fatalf("users = %+v (err %v)", users, err)
	}
	if users[1].ActiveClusterID == nil || *users[1].ActiveClusterID != 1 {
		t.Fatal("alice lost her active cluster")
	}
	if users[2].ActiveClusterID != nil {
		t.Fatal("bob's dangling active cluster was not cleared")
	}
	for _, u := range users {
		if u.MustChangePassword {
			t.Fatalf("%s unexpectedly must change password", u.Username)
		}
	}

	assignments, err := s.Assignments()
	if err != nil {
		t.Fatal(err)
	}
	if len(assignments) != 3 {
		t.Fatalf("assignments = %+v, want the 3 non-orphan rows", assignments)
	}
	for _, a := range assignments {
		if a.Role != ClusterRoleOperator {
			t.Fatalf("v0.4 assignment %+v must migrate to operator", a)
		}
	}

	cluster, err := s.GetCluster(1)
	if err != nil || cluster.ReadOnly || cluster.Environment != "" || cluster.Kubeconfig != "apiVersion: v1" {
		t.Fatalf("cluster = %+v (err %v)", cluster, err)
	}

	if u, err := s.UserBySession("alice-token"); err != nil || u.Username != "alice" {
		t.Fatalf("existing session lost: %v", err)
	}
	for _, token := range []string{"orphan", "garbled"} {
		var n int
		if err := s.db.QueryRow(`SELECT COUNT(*) FROM sessions WHERE token_hash IN (?, ?)`, token, hashToken(token)).Scan(&n); err != nil || n != 0 {
			t.Fatalf("session %q survived the migration", token)
		}
	}
	if n, err := s.PruneSessions(time.Now().Add(2 * time.Hour)); err != nil || n != 1 {
		t.Fatalf("pruning the re-encoded session: n=%d err=%v", n, err)
	}

	// New assignments default to the least privilege.
	if _, err := s.db.Exec(`INSERT INTO user_clusters (user_id, cluster_id) VALUES (3, 2)`); err != nil {
		t.Fatal(err)
	}
	if role, _ := s.ClusterRole(&users[2], 2); role != ClusterRoleViewer {
		t.Fatalf("default assignment role = %q", role)
	}
}
