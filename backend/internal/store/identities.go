package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// ErrUsernameTaken refuses an external sign-in whose username belongs to
// another account: a local user, or a user of another source or subject.
// External accounts never take over an existing one (§3.6).
var ErrUsernameTaken = errors.New("the username belongs to another account")

// ExternalAccount is a user of an identity provider as of one sign-in,
// with the access their groups grant (§3.6).
type ExternalAccount struct {
	Source      string // UserSourceOIDC | UserSourceLDAP
	Subject     string // stable ID at the source (users.external_id)
	Username    string
	DisplayName string
	Email       string
	Groups      []string
	// Role is the global role the groups grant (RoleAdmin or RoleUser).
	Role string
	// ClusterRoles maps cluster names to ClusterRoleViewer or
	// ClusterRoleOperator; names not in the inventory are skipped.
	ClusterRoles map[string]string
}

// ExternalUser is an external account as KubeBench last saw it.
type ExternalUser struct {
	User
	Groups     []string
	LastSignIn *time.Time
}

// SignInExternal creates or updates the account of an external user and
// replaces its global role and cluster assignments with the ones its
// groups grant: mappings win over manual edits for such users. Accounts
// are matched by source and subject; a username already used by another
// account fails with ErrUsernameTaken.
func (s *Store) SignInExternal(a ExternalAccount) (*User, error) {
	if !ValidExternalSource(a.Source) || a.Subject == "" || a.Username == "" {
		return nil, fmt.Errorf("invalid external account %s/%q", a.Source, a.Subject)
	}
	if a.Role != RoleAdmin && a.Role != RoleUser {
		return nil, fmt.Errorf("invalid role %q", a.Role)
	}
	for cluster, role := range a.ClusterRoles {
		if !ValidClusterRole(role) {
			return nil, fmt.Errorf("invalid cluster role %q for %q", role, cluster)
		}
	}
	groups, err := json.Marshal(nonNil(a.Groups))
	if err != nil {
		return nil, err
	}
	var id int64
	err = s.withTx(func(tx *sql.Tx) error {
		var owner int64
		err := tx.QueryRow(`SELECT id FROM users WHERE username = ?`, a.Username).Scan(&owner)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		err = tx.QueryRow(`SELECT id FROM users WHERE source = ? AND external_id = ?`, a.Source, a.Subject).Scan(&id)
		switch {
		case errors.Is(err, sql.ErrNoRows):
			if owner != 0 {
				return ErrUsernameTaken
			}
			// No password: the empty hash never matches, and Authenticate
			// refuses external users before comparing.
			res, err := tx.Exec(
				`INSERT INTO users (username, password_hash, role, source, external_id, display_name, email,
				                    external_groups, last_sign_in)
				 VALUES (?, '', ?, ?, ?, ?, ?, ?, ?)`,
				a.Username, a.Role, a.Source, a.Subject, a.DisplayName, a.Email, string(groups), now(),
			)
			if err != nil {
				return err
			}
			if id, err = res.LastInsertId(); err != nil {
				return err
			}
		case err != nil:
			return err
		default:
			if owner != 0 && owner != id {
				return ErrUsernameTaken
			}
			_, err := tx.Exec(
				`UPDATE users SET username = ?, role = ?, display_name = ?, email = ?, external_groups = ?,
				                  last_sign_in = ?, must_change_password = 0
				 WHERE id = ?`,
				a.Username, a.Role, a.DisplayName, a.Email, string(groups), now(), id,
			)
			if err != nil {
				return err
			}
		}
		return replaceAssignmentsByName(tx, id, a.ClusterRoles)
	})
	if err != nil {
		return nil, err
	}
	return s.GetUser(id)
}

// replaceAssignmentsByName sets a user's assignments to roles, keyed by
// cluster name; names without a cluster are skipped.
func replaceAssignmentsByName(tx *sql.Tx, userID int64, roles map[string]string) error {
	if _, err := tx.Exec(`DELETE FROM user_clusters WHERE user_id = ?`, userID); err != nil {
		return err
	}
	for cluster, role := range roles {
		_, err := tx.Exec(
			`INSERT INTO user_clusters (user_id, cluster_id, role)
			 SELECT ?, id, ? FROM clusters WHERE name = ?`,
			userID, role, cluster,
		)
		if err != nil {
			return err
		}
	}
	return nil
}

// ExternalUserByUsername returns an external user with the groups of
// their last sign-in; local users and unknown names yield ErrNotFound.
func (s *Store) ExternalUserByUsername(username string) (*ExternalUser, error) {
	var u ExternalUser
	var groups string
	err := s.db.QueryRow(
		`SELECT `+userColumns+`, external_groups, last_sign_in FROM users WHERE username = ? AND source != 'local'`,
		username,
	).Scan(append(userFields(&u.User), &groups, &u.LastSignIn)...)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(groups), &u.Groups); err != nil {
		return nil, fmt.Errorf("groups of user %q: %w", username, err)
	}
	return &u, nil
}

// UserByUsername returns any user by name.
func (s *Store) UserByUsername(username string) (*User, error) {
	return scanUser(s.db.QueryRow(`SELECT `+userColumns+` FROM users WHERE username = ?`, username))
}

// RevokeExternalSessions ends every session of the external user with this
// source and subject, e.g. after the provider reported the account
// disabled. It reports whether such a user exists.
func (s *Store) RevokeExternalSessions(source, subject string) (bool, error) {
	var id int64
	err := s.db.QueryRow(`SELECT id FROM users WHERE source = ? AND external_id = ?`, source, subject).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	_, err = s.db.Exec(`DELETE FROM sessions WHERE user_id = ?`, id)
	return true, err
}

func nonNil(list []string) []string {
	if list == nil {
		return []string{}
	}
	return list
}
