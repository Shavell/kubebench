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
	return s.CreateExternalSession(userID, ttl, "")
}

// CreateExternalSession is CreateSession for a sign-in through an OIDC
// provider, keeping its ID token to end the provider's session at sign-out
// (RP-initiated logout, §3.6).
func (s *Store) CreateExternalSession(userID int64, ttl time.Duration, idToken string) (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	token := hex.EncodeToString(raw)
	_, err := s.db.Exec(
		`INSERT INTO sessions (token, user_id, expires_at, id_token) VALUES (?, ?, ?, ?)`,
		token, userID, now().Add(ttl), idToken,
	)
	return token, err
}

// SessionIDToken is the ID token a session was signed in with; empty for
// local sign-ins and unknown tokens.
func (s *Store) SessionIDToken(token string) (string, error) {
	var idToken string
	err := s.db.QueryRow(`SELECT id_token FROM sessions WHERE token = ?`, token).Scan(&idToken)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return idToken, err
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
