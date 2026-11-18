package store

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"time"
)

// Session tokens are 256-bit random values handed to the browser once.
// The database keeps only their SHA-256 digest (sessions.token_hash), so a
// copy of it holds no usable sessions. A plain digest, rather than a keyed
// one or a slow password hash, suffices: the tokens are uniformly random
// and far too long to guess, so nothing can be recovered by brute force,
// and a key would have to be stored next to the digests anyway.

// hashToken is the stored form of a session token.
func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// CreateSession issues a random 256-bit session token for the user and
// stores its digest.
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
		`INSERT INTO sessions (token_hash, user_id, expires_at, id_token) VALUES (?, ?, ?, ?)`,
		hashToken(token), userID, now().Add(ttl), idToken,
	)
	return token, err
}

// SessionIDToken is the ID token a session was signed in with; empty for
// local sign-ins and unknown tokens.
func (s *Store) SessionIDToken(token string) (string, error) {
	var idToken string
	err := s.db.QueryRow(`SELECT id_token FROM sessions WHERE token_hash = ?`, hashToken(token)).Scan(&idToken)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return idToken, err
}

// UserBySession resolves a live session token to its user; unknown and
// expired tokens yield ErrNotFound. The lookup goes by the token's digest,
// so its timing reveals nothing about the stored tokens.
func (s *Store) UserBySession(token string) (*User, error) {
	var userID int64
	var expires time.Time
	err := s.db.QueryRow(
		`SELECT user_id, expires_at FROM sessions WHERE token_hash = ?`, hashToken(token),
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

// DeleteSession ends the session of token (signing out).
func (s *Store) DeleteSession(token string) error {
	_, err := s.db.Exec(`DELETE FROM sessions WHERE token_hash = ?`, hashToken(token))
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

// hashSessionTokens replaces the raw tokens of sessions stored before
// schema v6 with their digests, so their users stay signed in.
func hashSessionTokens(tx *sql.Tx) error {
	rows, err := tx.Query(`SELECT token_hash FROM sessions`)
	if err != nil {
		return err
	}
	var tokens []string
	for rows.Next() {
		var token string
		if err := rows.Scan(&token); err != nil {
			rows.Close()
			return err
		}
		tokens = append(tokens, token)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	for _, token := range tokens {
		_, err := tx.Exec(`UPDATE sessions SET token_hash = ? WHERE token_hash = ?`, hashToken(token), token)
		if err != nil {
			return err
		}
	}
	return nil
}
