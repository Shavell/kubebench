package store

import (
	"database/sql"
	"errors"
)

// Keys of app_settings, KubeBench's own server-wide settings (unlike
// plugin_kv, which belongs to plugins).
const (
	// SettingUpdateMode is the update check mode an admin chose (§7.11).
	SettingUpdateMode = "updates.mode"
)

// AppSetting returns a server-wide setting; set is false when it was never
// stored, so the caller uses its default.
func (s *Store) AppSetting(key string) (value string, set bool, err error) {
	err = s.db.QueryRow(`SELECT value FROM app_settings WHERE key = ?`, key).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	return value, err == nil, err
}

func (s *Store) SetAppSetting(key, value string) error {
	_, err := s.db.Exec(
		`INSERT INTO app_settings (key, value, updated_at) VALUES (?, ?, ?)
		 ON CONFLICT (key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at`,
		key, value, now(),
	)
	return err
}

// LastSeenVersion is the release whose What's new the user last closed;
// empty until then.
func (s *Store) LastSeenVersion(userID int64) (string, error) {
	var version string
	err := s.db.QueryRow(`SELECT last_seen_version FROM users WHERE id = ?`, userID).Scan(&version)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotFound
	}
	return version, err
}

func (s *Store) SetLastSeenVersion(userID int64, version string) error {
	res, err := s.db.Exec(`UPDATE users SET last_seen_version = ? WHERE id = ?`, version, userID)
	if err != nil {
		return err
	}
	if n, err := res.RowsAffected(); err != nil {
		return err
	} else if n == 0 {
		return ErrNotFound
	}
	return nil
}
