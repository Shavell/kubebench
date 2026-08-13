// Package store persists users, sessions and the cluster inventory in
// SQLite (pure-Go driver, so the binary stays CGO-free).
package store

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"golang.org/x/crypto/bcrypt"
	_ "modernc.org/sqlite"
)

type Store struct{ db *sql.DB }

type User struct {
	ID              int64  `json:"id"`
	Username        string `json:"username"`
	Role            string `json:"role"` // "admin" | "user"
	ActiveClusterID *int64 `json:"activeClusterId,omitempty"`
}

type Cluster struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	Source     string `json:"source"` // "kubeconfig" | "incluster" | "address"
	Kubeconfig string `json:"-"`      // full YAML content, never serialized
	Context    string `json:"context,omitempty"`
	Address    string `json:"address,omitempty"`
}

var ErrNotFound = errors.New("not found")

func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path+"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1) // modernc sqlite prefers a single writer
	s := &Store{db: db}
	return s, s.migrate()
}

func (s *Store) migrate() error {
	_, err := s.db.Exec(`
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
);`)
	return err
}

// --- users ---

func (s *Store) CountUsers() (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&n)
	return n, err
}

func (s *Store) CreateUser(username, password, role string) (int64, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return 0, err
	}
	res, err := s.db.Exec(
		`INSERT INTO users (username, password_hash, role) VALUES (?, ?, ?)`,
		username, string(hash), role,
	)
	if err != nil {
		return 0, fmt.Errorf("user %q: %w", username, err)
	}
	return res.LastInsertId()
}

func (s *Store) DeleteUser(id int64) error {
	_, err := s.db.Exec(`DELETE FROM users WHERE id = ?`, id)
	return err
}

func (s *Store) Authenticate(username, password string) (*User, error) {
	var u User
	var hash string
	err := s.db.QueryRow(
		`SELECT id, username, password_hash, role, active_cluster_id FROM users WHERE username = ?`,
		username,
	).Scan(&u.ID, &u.Username, &hash, &u.Role, &u.ActiveClusterID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) != nil {
		return nil, ErrNotFound
	}
	return &u, nil
}

func (s *Store) GetUser(id int64) (*User, error) {
	var u User
	err := s.db.QueryRow(
		`SELECT id, username, role, active_cluster_id FROM users WHERE id = ?`, id,
	).Scan(&u.ID, &u.Username, &u.Role, &u.ActiveClusterID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &u, err
}

func (s *Store) ListUsers() ([]User, error) {
	rows, err := s.db.Query(`SELECT id, username, role, active_cluster_id FROM users ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var users []User
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.Username, &u.Role, &u.ActiveClusterID); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

func (s *Store) SetActiveCluster(userID, clusterID int64) error {
	_, err := s.db.Exec(`UPDATE users SET active_cluster_id = ? WHERE id = ?`, clusterID, userID)
	return err
}

// --- sessions ---

func (s *Store) CreateSession(userID int64, ttl time.Duration) (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	token := hex.EncodeToString(raw)
	_, err := s.db.Exec(
		`INSERT INTO sessions (token, user_id, expires_at) VALUES (?, ?, ?)`,
		token, userID, time.Now().Add(ttl).UTC(),
	)
	return token, err
}

func (s *Store) UserBySession(token string) (*User, error) {
	var userID int64
	var expires time.Time
	err := s.db.QueryRow(
		`SELECT user_id, expires_at FROM sessions WHERE token = ?`, token,
	).Scan(&userID, &expires)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && time.Now().After(expires)) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return s.GetUser(userID)
}

func (s *Store) DeleteSession(token string) error {
	_, err := s.db.Exec(`DELETE FROM sessions WHERE token = ?`, token)
	return err
}

// --- clusters ---

func (s *Store) AddCluster(c Cluster) (int64, error) {
	res, err := s.db.Exec(
		`INSERT INTO clusters (name, source, kubeconfig, context, address) VALUES (?, ?, ?, ?, ?)`,
		c.Name, c.Source, c.Kubeconfig, c.Context, c.Address,
	)
	if err != nil {
		return 0, fmt.Errorf("cluster %q: %w", c.Name, err)
	}
	return res.LastInsertId()
}

func (s *Store) DeleteCluster(id int64) error {
	_, err := s.db.Exec(`DELETE FROM clusters WHERE id = ?`, id)
	return err
}

func (s *Store) GetCluster(id int64) (*Cluster, error) {
	var c Cluster
	var kubeconfig, context, address sql.NullString
	err := s.db.QueryRow(
		`SELECT id, name, source, kubeconfig, context, address FROM clusters WHERE id = ?`, id,
	).Scan(&c.ID, &c.Name, &c.Source, &kubeconfig, &context, &address)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	c.Kubeconfig, c.Context, c.Address = kubeconfig.String, context.String, address.String
	return &c, err
}

func (s *Store) ClusterByName(name string) (*Cluster, error) {
	var id int64
	err := s.db.QueryRow(`SELECT id FROM clusters WHERE name = ?`, name).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return s.GetCluster(id)
}

// ListClusters returns all clusters for admins, assigned ones otherwise.
func (s *Store) ListClusters(user *User) ([]Cluster, error) {
	query := `SELECT id, name, source, kubeconfig, context, address FROM clusters ORDER BY id`
	args := []any{}
	if user.Role != "admin" {
		query = `SELECT c.id, c.name, c.source, c.kubeconfig, c.context, c.address
		         FROM clusters c JOIN user_clusters uc ON uc.cluster_id = c.id
		         WHERE uc.user_id = ? ORDER BY c.id`
		args = append(args, user.ID)
	}
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var clusters []Cluster
	for rows.Next() {
		var c Cluster
		var kubeconfig, context, address sql.NullString
		if err := rows.Scan(&c.ID, &c.Name, &c.Source, &kubeconfig, &context, &address); err != nil {
			return nil, err
		}
		c.Kubeconfig, c.Context, c.Address = kubeconfig.String, context.String, address.String
		clusters = append(clusters, c)
	}
	return clusters, rows.Err()
}

func (s *Store) CanAccessCluster(user *User, clusterID int64) (bool, error) {
	if user.Role == "admin" {
		return true, nil
	}
	var n int
	err := s.db.QueryRow(
		`SELECT COUNT(*) FROM user_clusters WHERE user_id = ? AND cluster_id = ?`,
		user.ID, clusterID,
	).Scan(&n)
	return n > 0, err
}

func (s *Store) AssignClusters(userID int64, clusterIDs []int64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM user_clusters WHERE user_id = ?`, userID); err != nil {
		return err
	}
	for _, cid := range clusterIDs {
		if _, err := tx.Exec(
			`INSERT INTO user_clusters (user_id, cluster_id) VALUES (?, ?)`, userID, cid,
		); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) UserClusterIDs(userID int64) ([]int64, error) {
	rows, err := s.db.Query(`SELECT cluster_id FROM user_clusters WHERE user_id = ?`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}
