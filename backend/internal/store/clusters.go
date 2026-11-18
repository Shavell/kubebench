package store

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"kubebench/internal/reason"
)

// Cluster is one inventory entry. The struct is comparable, which lets the
// runtime cache detect changed records.
type Cluster struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Source      string `json:"source"` // SourceKubeconfig | SourceInCluster | SourceAddress
	Kubeconfig  string `json:"-"`      // full YAML content, never serialized
	Context     string `json:"context,omitempty"`
	Address     string `json:"address,omitempty"`
	ReadOnly    bool   `json:"readOnly"`
	Environment string `json:"environment"` // "" | development | staging | production
	// Credentials are per-cluster cloud credentials (cloudauth.Credentials
	// as JSON, "" for none), never serialized.
	Credentials string `json:"-"`
}

// UserCluster is a cluster visible to a user with the user's effective
// role on it (RoleAdmin for global admins, else the assignment role).
type UserCluster struct {
	Cluster
	Role string
}

// Assignment grants a user a role on a cluster.
type Assignment struct {
	UserID      int64
	ClusterID   int64
	ClusterName string
	Role        string // ClusterRoleViewer | ClusterRoleOperator
}

const clusterColumns = `c.id, c.name, c.source, c.kubeconfig, c.context, c.address, c.read_only, c.environment, c.credentials`

func scanCluster(row interface{ Scan(...any) error }, extra ...any) (*Cluster, error) {
	var c Cluster
	var kubeconfig, context, address sql.NullString
	dest := append([]any{&c.ID, &c.Name, &c.Source, &kubeconfig, &context, &address, &c.ReadOnly, &c.Environment, &c.Credentials}, extra...)
	if err := row.Scan(dest...); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	c.Kubeconfig, c.Context, c.Address = kubeconfig.String, context.String, address.String
	return &c, nil
}

func (s *Store) AddCluster(c Cluster) (int64, error) {
	res, err := s.db.Exec(
		`INSERT INTO clusters (name, source, kubeconfig, context, address, read_only, environment, credentials)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		c.Name, c.Source, c.Kubeconfig, c.Context, c.Address, boolInt(c.ReadOnly), c.Environment, c.Credentials,
	)
	if err != nil {
		return 0, conflictErr(err, reasonClusterExists, reason.Params{"name": c.Name})
	}
	return res.LastInsertId()
}

func (s *Store) GetCluster(id int64) (*Cluster, error) {
	return scanCluster(s.db.QueryRow(`SELECT `+clusterColumns+` FROM clusters c WHERE c.id = ?`, id))
}

func (s *Store) ClusterByName(name string) (*Cluster, error) {
	return scanCluster(s.db.QueryRow(`SELECT `+clusterColumns+` FROM clusters c WHERE c.name = ?`, name))
}

// ListClusters returns every cluster for admins and the assigned ones
// otherwise, each with the user's effective role.
func (s *Store) ListClusters(user *User) ([]UserCluster, error) {
	query := `SELECT ` + clusterColumns + `, 'admin' FROM clusters c ORDER BY c.id`
	var args []any
	if !user.IsAdmin() {
		query = `SELECT ` + clusterColumns + `, uc.role FROM clusters c
		         JOIN user_clusters uc ON uc.cluster_id = c.id
		         WHERE uc.user_id = ? ORDER BY c.id`
		args = append(args, user.ID)
	}
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []UserCluster{}
	for rows.Next() {
		var role string
		c, err := scanCluster(rows, &role)
		if err != nil {
			return nil, err
		}
		out = append(out, UserCluster{Cluster: *c, Role: role})
	}
	return out, rows.Err()
}

// ClusterRole is the user's effective role on a cluster, "" without access.
func (s *Store) ClusterRole(user *User, clusterID int64) (string, error) {
	if user.IsAdmin() {
		return RoleAdmin, nil
	}
	var role string
	err := s.db.QueryRow(
		`SELECT role FROM user_clusters WHERE user_id = ? AND cluster_id = ?`, user.ID, clusterID,
	).Scan(&role)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return role, err
}

// ClusterUpdate holds the fields of a partial cluster update; nil fields
// are left unchanged.
type ClusterUpdate struct {
	Name        *string
	ReadOnly    *bool
	Environment *string
	Credentials *string // "" removes them
}

func (s *Store) UpdateCluster(id int64, u ClusterUpdate) error {
	var sets []string
	var args []any
	if u.Name != nil {
		sets, args = append(sets, "name = ?"), append(args, *u.Name)
	}
	if u.ReadOnly != nil {
		sets, args = append(sets, "read_only = ?"), append(args, boolInt(*u.ReadOnly))
	}
	if u.Environment != nil {
		sets, args = append(sets, "environment = ?"), append(args, *u.Environment)
	}
	if u.Credentials != nil {
		sets, args = append(sets, "credentials = ?"), append(args, *u.Credentials)
	}
	if len(sets) == 0 {
		_, err := s.GetCluster(id)
		return err
	}
	res, err := s.db.Exec(`UPDATE clusters SET `+strings.Join(sets, ", ")+` WHERE id = ?`, append(args, id)...)
	if err != nil {
		if u.Name != nil {
			return conflictErr(err, reasonClusterExists, reason.Params{"name": *u.Name})
		}
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteCluster removes a cluster and its assignments, and clears it as
// anyone's active cluster.
func (s *Store) DeleteCluster(id int64) error {
	return s.withTx(func(tx *sql.Tx) error {
		if _, err := tx.Exec(`UPDATE users SET active_cluster_id = NULL WHERE active_cluster_id = ?`, id); err != nil {
			return err
		}
		res, err := tx.Exec(`DELETE FROM clusters WHERE id = ?`, id)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return ErrNotFound
		}
		return nil
	})
}

// Assignments lists every assignment, ordered by user then cluster.
func (s *Store) Assignments() ([]Assignment, error) {
	rows, err := s.db.Query(`SELECT uc.user_id, uc.cluster_id, c.name, uc.role
	                         FROM user_clusters uc JOIN clusters c ON c.id = uc.cluster_id
	                         ORDER BY uc.user_id, c.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Assignment{}
	for rows.Next() {
		var a Assignment
		if err := rows.Scan(&a.UserID, &a.ClusterID, &a.ClusterName, &a.Role); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// SetAssignments replaces all of a user's assignments. Unknown users or
// clusters yield ErrNotFound; invalid roles are rejected.
func (s *Store) SetAssignments(userID int64, assignments []Assignment) error {
	for _, a := range assignments {
		if !ValidClusterRole(a.Role) {
			return fmt.Errorf("invalid cluster role %q", a.Role)
		}
	}
	return s.withTx(func(tx *sql.Tx) error {
		var exists int
		if err := tx.QueryRow(`SELECT COUNT(*) FROM users WHERE id = ?`, userID).Scan(&exists); err != nil {
			return err
		}
		if exists == 0 {
			return fmt.Errorf("user %d: %w", userID, ErrNotFound)
		}
		if _, err := tx.Exec(`DELETE FROM user_clusters WHERE user_id = ?`, userID); err != nil {
			return err
		}
		for _, a := range assignments {
			_, err := tx.Exec(
				`INSERT INTO user_clusters (user_id, cluster_id, role) VALUES (?, ?, ?)
				 ON CONFLICT (user_id, cluster_id) DO UPDATE SET role = excluded.role`,
				userID, a.ClusterID, a.Role,
			)
			if isForeignKeyViolation(err) {
				return fmt.Errorf("cluster %d: %w", a.ClusterID, ErrNotFound)
			}
			if err != nil {
				return err
			}
		}
		return nil
	})
}
