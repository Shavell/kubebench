//go:build unix

package store

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

// The database holds cluster credentials and session tokens: whatever the
// umask, it and its WAL files are readable by the server's account only,
// and an existing world-readable database is tightened on open.
func TestDatabaseFilesAreOwnerOnly(t *testing.T) {
	defer syscall.Umask(syscall.Umask(0o022))
	path := filepath.Join(t.TempDir(), "kubebench.db")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	mustUser(t, s, "alice", RoleUser) // writes through the WAL
	expectModes(t, path)
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}

	if err := os.Chmod(path, 0o644); err != nil {
		t.Fatal(err)
	}
	s, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	mustUser(t, s, "bob", RoleUser)
	expectModes(t, path)
}

func expectModes(t *testing.T, path string) {
	t.Helper()
	for _, file := range []string{path, path + "-wal", path + "-shm"} {
		info, err := os.Stat(file)
		if err != nil {
			t.Fatal(err)
		}
		if mode := info.Mode().Perm(); mode != dbFileMode {
			t.Errorf("%s has mode %o, want %o", filepath.Base(file), mode, dbFileMode)
		}
	}
}
