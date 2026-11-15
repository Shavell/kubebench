package store

import (
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
)

// TestUpgradeFromV2AddsSettings migrates a v0.5 pre-release (schema v2)
// database: users keep their rows and start with no seen version.
func TestUpgradeFromV2AddsSettings(t *testing.T) {
	path := filepath.Join(t.TempDir(), "v2.db")
	old, err := sql.Open("sqlite", path+"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)")
	if err != nil {
		t.Fatal(err)
	}
	tx, err := old.Begin()
	if err != nil {
		t.Fatal(err)
	}
	for _, step := range migrations[:2] {
		if err := step(tx); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := tx.Exec(`PRAGMA user_version = 2`); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(`INSERT INTO users (id, username, password_hash, role) VALUES (1, 'admin', 'h', 'admin')`); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	old.Close()

	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if v := schemaVersion(t, s); v != len(migrations) {
		t.Fatalf("user_version = %d, want %d", v, len(migrations))
	}
	if seen, err := s.LastSeenVersion(1); err != nil || seen != "" {
		t.Fatalf("LastSeenVersion = %q, %v; want empty", seen, err)
	}
	if _, set, err := s.AppSetting(SettingUpdateMode); err != nil || set {
		t.Fatalf("a migrated database has no update mode yet (set %v, err %v)", set, err)
	}
}

func TestAppSettings(t *testing.T) {
	s, _ := openTemp(t)
	if err := s.SetAppSetting(SettingUpdateMode, "off"); err != nil {
		t.Fatal(err)
	}
	if err := s.SetAppSetting(SettingUpdateMode, "prerelease"); err != nil {
		t.Fatal(err)
	}
	if v, set, err := s.AppSetting(SettingUpdateMode); err != nil || !set || v != "prerelease" {
		t.Fatalf("AppSetting = %q, %v, %v", v, set, err)
	}
}

func TestLastSeenVersion(t *testing.T) {
	s, _ := openTemp(t)
	id, err := s.CreateUser("alice", "correct horse", RoleUser, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SetLastSeenVersion(id, "0.5.0"); err != nil {
		t.Fatal(err)
	}
	if seen, err := s.LastSeenVersion(id); err != nil || seen != "0.5.0" {
		t.Fatalf("LastSeenVersion = %q, %v", seen, err)
	}
	if err := s.SetLastSeenVersion(id+1, "0.5.0"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown user: %v, want ErrNotFound", err)
	}
	if _, err := s.LastSeenVersion(id + 1); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown user: %v, want ErrNotFound", err)
	}
}
