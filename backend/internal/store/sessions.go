package store

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"time"
)

// CreateSession issues a random 256-bit session token for the user.
func (s *Store) CreateSession(userID int64, ttl time.Duration) (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	token := hex.EncodeToString(raw)
	_, err := s.db.Exec(
		`INSERT INTO sessions (token, user_id, expires_at) VALUES (?, ?, ?)`,
		token, userID, now().Add(ttl),
	)
	return token, err
}

// UserBySession resolves a live session token to its user; unknown and
// expired tokens yield ErrNotFound.
func (s *Store) UserBySession(token string) (*User, error) {
	var userID int64
	var expires time.Time
	err := s.db.QueryRow(
		`SELECT user_id, expires_at FROM sessions WHERE token = ?`, token,
	).Scan(&userID, &expires)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if !now().Before(expires) {
		return nil, ErrNotFound
	}
	return s.GetUser(userID)
}

func (s *Store) DeleteSession(token string) error {
	_, err := s.db.Exec(`DELETE FROM sessions WHERE token = ?`, token)
	return err
}

// PruneSessions deletes sessions that expired before t.
func (s *Store) PruneSessions(t time.Time) (int64, error) {
	res, err := s.db.Exec(`DELETE FROM sessions WHERE expires_at <= ?`, t.UTC())
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}
