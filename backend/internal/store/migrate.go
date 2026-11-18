package store

import (
	"database/sql"
	"fmt"
	"time"
)

// migrations upgrade the schema one version at a time: step i produces
// schema version i+1, recorded in PRAGMA user_version inside the same
// transaction. Never edit a released step; append a new one.
var migrations = []func(tx *sql.Tx) error{
	migrateV1,
	migrateV2,
	migrateV3,
	migrateV4,
	migrateV5,
	migrateV6,
}

func (s *Store) migrate() error {
	latest := len(migrations)
	var version int
	if err := s.db.QueryRow(`PRAGMA user_version`).Scan(&version); err != nil {
		return fmt.Errorf("read schema version: %w", err)
	}
	if version > latest {
		return fmt.Errorf("database schema v%d is newer than this KubeBench supports (v%d); upgrade KubeBench",
			version, latest)
	}
	for v := version; v < latest; v++ {
		err := s.withTx(func(tx *sql.Tx) error {
			if err := migrations[v](tx); err != nil {
				return err
			}
			_, err := tx.Exec(fmt.Sprintf(`PRAGMA user_version = %d`, v+1))
			return err
		})
		if err != nil {
			return fmt.Errorf("migrate schema to v%d: %w", v+1, err)
		}
	}
	return nil
}

// migrateV1 is the v0.4 schema. Its statements are idempotent because v0.4
// databases carry these tables without a user_version.
func migrateV1(tx *sql.Tx) error {
	_, err := tx.Exec(`
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

// migrateV2 adds v0.5 roles, cluster safety flags, the audit log and
// plugin state (docs/design-v0.5.md §3, §4.6, §5).
func migrateV2(tx *sql.Tx) error {
	_, err := tx.Exec(`
-- New assignments default to the least privilege; v0.4 assignments granted
-- full access, so they become operators.
ALTER TABLE user_clusters ADD COLUMN role TEXT NOT NULL DEFAULT 'viewer'
  CHECK (role IN ('viewer', 'operator'));
UPDATE user_clusters SET role = 'operator';

ALTER TABLE clusters ADD COLUMN read_only INTEGER NOT NULL DEFAULT 0;
ALTER TABLE clusters ADD COLUMN environment TEXT NOT NULL DEFAULT '';
ALTER TABLE users ADD COLUMN must_change_password INTEGER NOT NULL DEFAULT 0;

-- v0.4 ran without foreign_keys: drop orphans before enforcement applies.
DELETE FROM sessions WHERE user_id NOT IN (SELECT id FROM users);
DELETE FROM user_clusters
 WHERE user_id NOT IN (SELECT id FROM users) OR cluster_id NOT IN (SELECT id FROM clusters);
UPDATE users SET active_cluster_id = NULL
 WHERE active_cluster_id IS NOT NULL AND active_cluster_id NOT IN (SELECT id FROM clusters);

CREATE INDEX idx_sessions_user ON sessions(user_id);
CREATE INDEX idx_sessions_expires ON sessions(expires_at);
CREATE INDEX idx_user_clusters_cluster ON user_clusters(cluster_id);

CREATE TABLE audit_log (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  ts TIMESTAMP NOT NULL,
  username TEXT NOT NULL,
  cluster TEXT NOT NULL DEFAULT '',
  action TEXT NOT NULL,
  target TEXT NOT NULL DEFAULT '',
  status INTEGER NOT NULL,
  detail TEXT NOT NULL DEFAULT ''
);
CREATE INDEX idx_audit_ts ON audit_log(ts);

CREATE TABLE plugin_settings (
  id TEXT PRIMARY KEY,
  enabled INTEGER NOT NULL,
  updated_at TIMESTAMP NOT NULL
);
CREATE TABLE plugin_kv (
  plugin_id TEXT NOT NULL,
  key TEXT NOT NULL,
  value TEXT NOT NULL,
  updated_at TIMESTAMP NOT NULL,
  PRIMARY KEY (plugin_id, key)
);`)
	if err != nil {
		return err
	}
	return reencodeSessionExpiry(tx)
}

// migrateV3 adds server-wide settings (the update check mode) and the
// release whose What's new each user has seen (docs/design-v0.5.md §7.11).
func migrateV3(tx *sql.Tx) error {
	_, err := tx.Exec(`
CREATE TABLE app_settings (
  key TEXT PRIMARY KEY,
  value TEXT NOT NULL,
  updated_at TIMESTAMP NOT NULL
);
ALTER TABLE users ADD COLUMN last_seen_version TEXT NOT NULL DEFAULT '';`)
	return err
}

// migrateV4 adds per-cluster cloud credentials (docs/design-v0.5.md
// §7.12), stored like the kubeconfig: JSON, never returned by the API.
func migrateV4(tx *sql.Tx) error {
	_, err := tx.Exec(`ALTER TABLE clusters ADD COLUMN credentials TEXT NOT NULL DEFAULT '';`)
	return err
}

// migrateV5 adds notes (docs/design-v0.5.md §7.13). The foreign keys
// carry the lifecycle: a deleted cluster takes its notes along, a deleted
// user their personal notes (owner_id), while shared notes stay with an
// unknown author (created_by/updated_by set to NULL).
func migrateV5(tx *sql.Tx) error {
	_, err := tx.Exec(`
CREATE TABLE notes (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  visibility TEXT NOT NULL CHECK (visibility IN ('personal', 'shared')),
  owner_id INTEGER REFERENCES users(id) ON DELETE CASCADE,
  cluster_id INTEGER REFERENCES clusters(id) ON DELETE CASCADE,
  obj_api_version TEXT NOT NULL DEFAULT '',
  obj_group TEXT NOT NULL DEFAULT '',
  obj_kind TEXT NOT NULL DEFAULT '',
  obj_namespace TEXT NOT NULL DEFAULT '',
  obj_name TEXT NOT NULL DEFAULT '',
  title TEXT NOT NULL,
  body TEXT NOT NULL DEFAULT '',
  pinned INTEGER NOT NULL DEFAULT 0,
  version INTEGER NOT NULL DEFAULT 1,
  created_by INTEGER REFERENCES users(id) ON DELETE SET NULL,
  created_at TIMESTAMP NOT NULL,
  updated_by INTEGER REFERENCES users(id) ON DELETE SET NULL,
  updated_at TIMESTAMP NOT NULL,
  -- Personal notes have an owner, shared notes none; shared notes and
  -- notes on an object belong to a cluster.
  CHECK ((visibility = 'personal') = (owner_id IS NOT NULL)),
  CHECK (visibility = 'personal' OR cluster_id IS NOT NULL),
  CHECK (obj_kind = '' OR cluster_id IS NOT NULL)
);
CREATE INDEX idx_notes_object ON notes(cluster_id, obj_kind, obj_namespace, obj_name);
CREATE INDEX idx_notes_owner ON notes(owner_id);
CREATE INDEX idx_notes_created_by ON notes(created_by);
CREATE INDEX idx_notes_updated_by ON notes(updated_by);

CREATE TABLE note_tags (
  note_id INTEGER NOT NULL REFERENCES notes(id) ON DELETE CASCADE,
  tag TEXT NOT NULL,
  PRIMARY KEY (note_id, tag)
);
CREATE INDEX idx_note_tags_tag ON note_tags(tag);`)
	return err
}

// migrateV6 adds external identities (docs/design-v0.5.md §3.6, v0.7):
// where a user comes from (local, oidc, ldap) and their identity at that
// source, the ID token an OIDC session ends at the identity provider with,
// and the group mappings that grant external users their roles.
func migrateV6(tx *sql.Tx) error {
	_, err := tx.Exec(`
ALTER TABLE users ADD COLUMN source TEXT NOT NULL DEFAULT 'local'
  CHECK (source IN ('local', 'oidc', 'ldap'));
-- The subject at the source (OIDC "sub", the directory entry's ID);
-- NULL for local users.
ALTER TABLE users ADD COLUMN external_id TEXT;
ALTER TABLE users ADD COLUMN display_name TEXT NOT NULL DEFAULT '';
ALTER TABLE users ADD COLUMN email TEXT NOT NULL DEFAULT '';
-- The groups of the last sign-in (JSON array), shown to admins.
ALTER TABLE users ADD COLUMN external_groups TEXT NOT NULL DEFAULT '[]';
ALTER TABLE users ADD COLUMN last_sign_in TIMESTAMP;
CREATE UNIQUE INDEX idx_users_external ON users(source, external_id) WHERE external_id IS NOT NULL;

ALTER TABLE sessions ADD COLUMN id_token TEXT NOT NULL DEFAULT '';

CREATE TABLE auth_group_mappings (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  group_name TEXT NOT NULL CHECK (group_name != ''),
  -- '' applies to the groups of every provider.
  source TEXT NOT NULL DEFAULT '' CHECK (source IN ('', 'oidc', 'ldap')),
  role TEXT NOT NULL CHECK (role IN ('admin', 'operator', 'viewer')),
  -- The cluster (by name) of an operator or viewer grant; '' for admin.
  cluster TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMP NOT NULL,
  updated_at TIMESTAMP NOT NULL,
  CHECK ((role = 'admin') = (cluster = ''))
);
CREATE UNIQUE INDEX idx_auth_group_mappings
  ON auth_group_mappings(group_name COLLATE NOCASE, source, cluster);`)
	return err
}

// reencodeSessionExpiry rewrites v0.4 session expiries (stored in Go's
// time.String format) in the canonical format so expired sessions can be
// pruned with a plain comparison. Unparseable rows are dropped; their users
// simply sign in again.
func reencodeSessionExpiry(tx *sql.Tx) error {
	rows, err := tx.Query(`SELECT token, expires_at FROM sessions`)
	if err != nil {
		return err
	}
	expiry := map[string]any{}
	for rows.Next() {
		var token string
		var value any
		if err := rows.Scan(&token, &value); err != nil {
			rows.Close()
			return err
		}
		expiry[token] = value
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	for token, value := range expiry {
		if t, ok := value.(time.Time); ok {
			_, err = tx.Exec(`UPDATE sessions SET expires_at = ? WHERE token = ?`, t.UTC(), token)
		} else {
			_, err = tx.Exec(`DELETE FROM sessions WHERE token = ?`, token)
		}
		if err != nil {
			return err
		}
	}
	return nil
}
