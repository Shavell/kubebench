package store

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

// GroupMapping grants the members of an identity provider group the
// global admin role or a role on one cluster (§3.6).
type GroupMapping struct {
	ID    int64
	Group string
	// Source limits the mapping to the groups of one provider
	// (UserSourceOIDC or UserSourceLDAP); "" applies to every provider.
	Source string
	// Role is RoleAdmin (global) or ClusterRoleOperator/ClusterRoleViewer
	// on Cluster.
	Role    string
	Cluster string // cluster name; "" for RoleAdmin
}

// maxGroupLen bounds a mapping's group (LDAP DNs can be long).
const maxGroupLen = 512

// settingMappingsSeeded records that the configured mappings were seeded,
// so that mappings an admin deleted later stay deleted.
const settingMappingsSeeded = "auth.mappings.seeded"

// Normalize trims the group and cluster names.
func (m GroupMapping) Normalize() GroupMapping {
	m.Group, m.Cluster = strings.TrimSpace(m.Group), strings.TrimSpace(m.Cluster)
	return m
}

// Validate checks a normalized mapping's fields.
func (m GroupMapping) Validate() error {
	switch {
	case m.Group == "":
		return errors.New("the group is required")
	case len(m.Group) > maxGroupLen:
		return fmt.Errorf("the group is at most %d characters", maxGroupLen)
	case m.Source != "" && !ValidExternalSource(m.Source):
		return fmt.Errorf("invalid source %q: use oidc, ldap or nothing for every provider", m.Source)
	case m.Role == RoleAdmin && m.Cluster != "":
		return errors.New("an admin mapping applies to every cluster: leave the cluster empty")
	case m.Role != RoleAdmin && !ValidClusterRole(m.Role):
		return fmt.Errorf("invalid role %q: use admin, operator or viewer", m.Role)
	case m.Role != RoleAdmin && m.Cluster == "":
		return errors.New("an operator or viewer mapping needs a cluster")
	}
	return nil
}

const mappingColumns = `id, group_name, source, role, cluster`

func scanMapping(row interface{ Scan(...any) error }) (*GroupMapping, error) {
	var m GroupMapping
	if err := row.Scan(&m.ID, &m.Group, &m.Source, &m.Role, &m.Cluster); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &m, nil
}

// ListGroupMappings returns every mapping, ordered by group.
func (s *Store) ListGroupMappings() ([]GroupMapping, error) {
	rows, err := s.db.Query(`SELECT ` + mappingColumns + ` FROM auth_group_mappings
	                         ORDER BY group_name COLLATE NOCASE, source, role, cluster`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []GroupMapping{}
	for rows.Next() {
		m, err := scanMapping(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *m)
	}
	return out, rows.Err()
}

func (s *Store) GetGroupMapping(id int64) (*GroupMapping, error) {
	return scanMapping(s.db.QueryRow(`SELECT `+mappingColumns+` FROM auth_group_mappings WHERE id = ?`, id))
}

// CreateGroupMapping stores a mapping; one that repeats the group (in any
// case), source and cluster of another fails with ErrConflict.
func (s *Store) CreateGroupMapping(m GroupMapping) (int64, error) {
	m = m.Normalize()
	if err := m.Validate(); err != nil {
		return 0, err
	}
	return insertMapping(s.db, m)
}

type execer interface {
	Exec(query string, args ...any) (sql.Result, error)
}

func insertMapping(db execer, m GroupMapping) (int64, error) {
	t := now()
	res, err := db.Exec(
		`INSERT INTO auth_group_mappings (group_name, source, role, cluster, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		m.Group, m.Source, m.Role, m.Cluster, t, t,
	)
	if err != nil {
		return 0, conflictErr(err, "group %q already has a mapping for this cluster", m.Group)
	}
	return res.LastInsertId()
}

// UpdateGroupMapping replaces the fields of mapping m.ID.
func (s *Store) UpdateGroupMapping(m GroupMapping) error {
	m = m.Normalize()
	if err := m.Validate(); err != nil {
		return err
	}
	err := affectedOne(s.db.Exec(
		`UPDATE auth_group_mappings SET group_name = ?, source = ?, role = ?, cluster = ?, updated_at = ?
		 WHERE id = ?`,
		m.Group, m.Source, m.Role, m.Cluster, now(), m.ID,
	))
	return conflictErr(err, "group %q already has a mapping for this cluster", m.Group)
}

func (s *Store) DeleteGroupMapping(id int64) error {
	return affectedOne(s.db.Exec(`DELETE FROM auth_group_mappings WHERE id = ?`, id))
}

// SeedGroupMappings stores the configured mappings once per database and
// reports whether it did: after the first time it does nothing, so that
// later edits by admins win. Mappings that already exist are skipped.
func (s *Store) SeedGroupMappings(mappings []GroupMapping) (bool, error) {
	for i, m := range mappings {
		mappings[i] = m.Normalize()
		if err := mappings[i].Validate(); err != nil {
			return false, fmt.Errorf("mapping of group %q: %w", m.Group, err)
		}
	}
	seeded := false
	err := s.withTx(func(tx *sql.Tx) error {
		var done int
		if err := tx.QueryRow(`SELECT COUNT(*) FROM app_settings WHERE key = ?`, settingMappingsSeeded).Scan(&done); err != nil {
			return err
		}
		if done > 0 {
			return nil
		}
		for _, m := range mappings {
			if _, err := insertMapping(tx, m); err != nil && !errors.Is(err, ErrConflict) {
				return err
			}
		}
		if _, err := tx.Exec(`INSERT INTO app_settings (key, value, updated_at) VALUES (?, 'true', ?)`,
			settingMappingsSeeded, now()); err != nil {
			return err
		}
		seeded = true
		return nil
	})
	return seeded, err
}
