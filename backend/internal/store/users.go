package store

import (
	"database/sql"
	"errors"
	"fmt"
	"sync"

	"golang.org/x/crypto/bcrypt"

	"kubebench/internal/reason"
)

type User struct {
	ID                 int64  `json:"id"`
	Username           string `json:"username"`
	Role               string `json:"role"` // RoleAdmin | RoleUser
	ActiveClusterID    *int64 `json:"activeClusterId,omitempty"`
	MustChangePassword bool   `json:"mustChangePassword"`
	// Source is where the account comes from: UserSourceLocal (a password
	// kept here) or an external identity provider (§3.6).
	Source      string `json:"source"`
	DisplayName string `json:"displayName"`
	Email       string `json:"email"`
}

func (u *User) IsAdmin() bool { return u.Role == RoleAdmin }

// IsExternal reports whether the user signs in through an identity
// provider; such users have no local password.
func (u *User) IsExternal() bool { return u.Source != UserSourceLocal }

const userColumns = `id, username, role, active_cluster_id, must_change_password, source, display_name, email`

func userFields(u *User) []any {
	return []any{&u.ID, &u.Username, &u.Role, &u.ActiveClusterID, &u.MustChangePassword, &u.Source, &u.DisplayName, &u.Email}
}

func scanUser(row interface{ Scan(...any) error }) (*User, error) {
	var u User
	if err := row.Scan(userFields(&u)...); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &u, nil
}

func hashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(hash), err
}

// dummyHash is compared against when a username does not exist, so a
// failed login takes the same time either way (no user enumeration).
var dummyHash = sync.OnceValue(func() []byte {
	hash, _ := bcrypt.GenerateFromPassword([]byte("kubebench-timing-equalizer"), bcrypt.DefaultCost)
	return hash
})

func (s *Store) CountUsers() (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&n)
	return n, err
}

func (s *Store) CreateUser(username, password, role string, mustChangePassword bool) (int64, error) {
	hash, err := hashPassword(password)
	if err != nil {
		return 0, err
	}
	res, err := s.db.Exec(
		`INSERT INTO users (username, password_hash, role, must_change_password) VALUES (?, ?, ?, ?)`,
		username, hash, role, boolInt(mustChangePassword),
	)
	if err != nil {
		return 0, conflictErr(err, reasonUserExists, reason.Params{"name": username})
	}
	return res.LastInsertId()
}

func (s *Store) GetUser(id int64) (*User, error) {
	return scanUser(s.db.QueryRow(`SELECT `+userColumns+` FROM users WHERE id = ?`, id))
}

func (s *Store) ListUsers() ([]User, error) {
	rows, err := s.db.Query(`SELECT ` + userColumns + ` FROM users ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	users := []User{}
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		users = append(users, *u)
	}
	return users, rows.Err()
}

// Authenticate returns the local user when the password matches,
// ErrNotFound otherwise (unknown user, wrong password and an external
// user, who has no local password, are indistinguishable).
func (s *Store) Authenticate(username, password string) (*User, error) {
	var hash string
	var u User
	err := s.db.QueryRow(
		`SELECT `+userColumns+`, password_hash FROM users WHERE username = ?`, username,
	).Scan(append(userFields(&u), &hash)...)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && u.IsExternal()) {
		_ = bcrypt.CompareHashAndPassword(dummyHash(), []byte(password))
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

// CheckPassword reports whether password is the user's current password.
func (s *Store) CheckPassword(userID int64, password string) (bool, error) {
	var hash string
	err := s.db.QueryRow(`SELECT password_hash FROM users WHERE id = ?`, userID).Scan(&hash)
	if errors.Is(err, sql.ErrNoRows) {
		return false, ErrNotFound
	}
	if err != nil {
		return false, err
	}
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil, nil
}

// SetPassword replaces the user's password and revokes all of their
// sessions except keepToken (empty revokes every session).
func (s *Store) SetPassword(userID int64, password string, mustChangePassword bool, keepToken string) error {
	hash, err := hashPassword(password)
	if err != nil {
		return err
	}
	return s.withTx(func(tx *sql.Tx) error {
		res, err := tx.Exec(
			`UPDATE users SET password_hash = ?, must_change_password = ? WHERE id = ?`,
			hash, boolInt(mustChangePassword), userID,
		)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return ErrNotFound
		}
		_, err = tx.Exec(`DELETE FROM sessions WHERE user_id = ? AND token_hash != ?`, userID, hashToken(keepToken))
		return err
	})
}

// SetMustChangePassword flags (or clears) a forced password change.
func (s *Store) SetMustChangePassword(userID int64, must bool) error {
	_, err := s.db.Exec(`UPDATE users SET must_change_password = ? WHERE id = ?`, boolInt(must), userID)
	return err
}

// keepsAnAdmin is true unless the row being changed is the only local
// admin. Evaluated inside the UPDATE/DELETE statement, so concurrent
// demotions cannot both pass the check. Only local admins count: they are
// the break-glass accounts that work when the identity provider does not
// (§3.6), and an external admin loses the role whenever the groups of
// their next sign-in no longer grant it.
const keepsAnAdmin = `(role != 'admin' OR source != 'local' OR
  (SELECT COUNT(*) FROM users WHERE role = 'admin' AND source = 'local') > 1)`

// SetUserRole changes a user's global role; demoting the last local admin
// fails with ErrLastAdmin.
func (s *Store) SetUserRole(id int64, role string) error {
	if role != RoleAdmin && role != RoleUser {
		return fmt.Errorf("invalid role %q", role)
	}
	res, err := s.db.Exec(`UPDATE users SET role = ? WHERE id = ? AND (? = 'admin' OR `+keepsAnAdmin+`)`, role, id, role)
	if err != nil {
		return err
	}
	return s.guardedChange(res, id)
}

// DeleteUser removes a user with their sessions and assignments; deleting
// the last local admin fails with ErrLastAdmin.
func (s *Store) DeleteUser(id int64) error {
	res, err := s.db.Exec(`DELETE FROM users WHERE id = ? AND `+keepsAnAdmin, id)
	if err != nil {
		return err
	}
	return s.guardedChange(res, id)
}

// guardedChange explains a statement guarded by keepsAnAdmin that touched
// no row: either the user does not exist or it is the last local admin.
func (s *Store) guardedChange(res sql.Result, id int64) error {
	if n, err := res.RowsAffected(); err != nil || n > 0 {
		return err
	}
	if _, err := s.GetUser(id); err != nil {
		return err
	}
	return ErrLastAdmin
}

func (s *Store) SetActiveCluster(userID, clusterID int64) error {
	_, err := s.db.Exec(`UPDATE users SET active_cluster_id = ? WHERE id = ?`, clusterID, userID)
	return err
}
