// Package store persists users, sessions, the cluster inventory, the audit
// log and plugin state in SQLite (pure-Go driver, so the binary stays
// CGO-free).
package store

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
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

// conflictErr maps unique-key violations to ErrConflict with context.
func conflictErr(err error, format string, args ...any) error {
	switch constraintCode(err) {
	case sqlite3.SQLITE_CONSTRAINT_UNIQUE, sqlite3.SQLITE_CONSTRAINT_PRIMARYKEY:
		return fmt.Errorf("%s: %w", fmt.Sprintf(format, args...), ErrConflict)
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
