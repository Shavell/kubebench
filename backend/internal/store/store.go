// Package store persists users, sessions, the cluster inventory, the audit
// log and plugin state in SQLite (pure-Go driver, so the binary stays
// CGO-free).
package store

import (
	"database/sql"
	"errors"
	"io/fs"
	"log"
	"os"
	"time"

	"modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"

	"kubebench/internal/reason"
)

// Store is safe for concurrent use.
type Store struct{ db *sql.DB }

var (
	ErrNotFound = errors.New("not found")
	ErrConflict = errors.New("already exists")
	// ErrLastAdmin protects the invariant that at least one admin exists.
	ErrLastAdmin = errors.New("at least one admin must remain")
)

// Global user roles (users.role).
const (
	RoleAdmin = "admin"
	RoleUser  = "user"
)

// Per-cluster assignment roles (user_clusters.role). A global admin's
// effective role on every cluster is RoleAdmin.
const (
	ClusterRoleViewer   = "viewer"
	ClusterRoleOperator = "operator"
)

// Cluster sources.
const (
	SourceKubeconfig = "kubeconfig"
	SourceInCluster  = "incluster"
	SourceAddress    = "address" // bookmark only; cannot be activated
)

// ValidEnvironment reports whether env is a cluster environment label.
func ValidEnvironment(env string) bool {
	switch env {
	case "", "development", "staging", "production":
		return true
	}
	return false
}

// ValidClusterRole reports whether role can be assigned on a cluster.
func ValidClusterRole(role string) bool {
	return role == ClusterRoleViewer || role == ClusterRoleOperator
}

// Open opens (creating if needed) and migrates the database at path.
func Open(path string) (*Store, error) {
	if err := restrictFiles(path); err != nil {
		return nil, err
	}
	// foreign_keys makes deletes cascade to sessions/assignments;
	// _time_format=sqlite stores times as "YYYY-MM-DD HH:MM:SS.fff+00:00"
	// so UTC values compare correctly as text in SQL.
	dsn := path + "?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)" +
		"&_pragma=foreign_keys(1)&_time_format=sqlite"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1) // modernc sqlite prefers a single writer
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error { return s.db.Close() }

// dbFileMode keeps the database, which holds cluster credentials and
// session tokens, readable by the server's own account only. SQLite gives
// its -wal and -shm files the database file's mode.
const dbFileMode = 0o600

// restrictFiles creates the database file with dbFileMode, whatever the
// umask, and tightens the mode of an existing database and its -wal and
// -shm files (older versions created them world-readable). A file that
// cannot be changed, e.g. on a mount owned by another user, is logged
// rather than refused.
func restrictFiles(path string) error {
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, dbFileMode)
	if err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	for _, file := range []string{path, path + "-wal", path + "-shm"} {
		info, err := os.Stat(file)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err == nil && info.Mode().Perm()&^dbFileMode != 0 {
			err = os.Chmod(file, dbFileMode)
		}
		if err != nil {
			log.Printf("[store] cannot restrict the permissions of %s: %v", file, err)
		}
	}
	return nil
}

// now is the store's clock; every persisted time is UTC.
func now() time.Time { return time.Now().UTC() }

// withTx runs fn in a transaction, committing when it returns nil.
// Never call other Store methods from fn: the pool holds one connection.
func (s *Store) withTx(fn func(tx *sql.Tx) error) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

func constraintCode(err error) int {
	var sqlErr *sqlite.Error
	if errors.As(err, &sqlErr) {
		return sqlErr.Code()
	}
	return 0
}

// Reasons of unique-key conflicts (§3.3).
var (
	reasonUserExists    = reason.Define("users.exists", "user \"{{name}}\": already exists")
	reasonClusterExists = reason.Define("clusters.exists", "cluster \"{{name}}\": already exists")
)

// conflictErr maps unique-key violations to ErrConflict, explained by
// code with params.
func conflictErr(err error, code reason.Code, params reason.Params) error {
	switch constraintCode(err) {
	case sqlite3.SQLITE_CONSTRAINT_UNIQUE, sqlite3.SQLITE_CONSTRAINT_PRIMARYKEY:
		return reason.Wrap(ErrConflict, code, params)
	}
	return err
}

func isForeignKeyViolation(err error) bool {
	return constraintCode(err) == sqlite3.SQLITE_CONSTRAINT_FOREIGNKEY
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
