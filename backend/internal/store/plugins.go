package store

import (
	"database/sql"
	"errors"
	"fmt"
)

// PluginSettings returns the admin's enable/disable choices by plugin id.
// Ids without a row use their default (built-in on, external off).
func (s *Store) PluginSettings() (map[string]bool, error) {
	rows, err := s.db.Query(`SELECT id, enabled FROM plugin_settings`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var id string
		var enabled bool
		if err := rows.Scan(&id, &enabled); err != nil {
			return nil, err
		}
		out[id] = enabled
	}
	return out, rows.Err()
}

// PluginEnabled returns the stored choice for one plugin; set is false
// when the admin never chose.
func (s *Store) PluginEnabled(id string) (enabled, set bool, err error) {
	err = s.db.QueryRow(`SELECT enabled FROM plugin_settings WHERE id = ?`, id).Scan(&enabled)
	if errors.Is(err, sql.ErrNoRows) {
		return false, false, nil
	}
	return enabled, err == nil, err
}

func (s *Store) SetPluginEnabled(id string, enabled bool) error {
	_, err := s.db.Exec(
		`INSERT INTO plugin_settings (id, enabled, updated_at) VALUES (?, ?, ?)
		 ON CONFLICT (id) DO UPDATE SET enabled = excluded.enabled, updated_at = excluded.updated_at`,
		id, boolInt(enabled), now(),
	)
	return err
}

// KV limits keep one plugin from bloating the database.
const (
	maxKVKeyBytes   = 512
	maxKVValueBytes = 1 << 20
)

// PluginKV is the persistent key/value space of one plugin (plugin_kv).
// It satisfies plugin.KV.
type PluginKV struct {
	s        *Store
	pluginID string
}

func (s *Store) PluginKV(pluginID string) *PluginKV {
	return &PluginKV{s: s, pluginID: pluginID}
}

func (kv *PluginKV) Get(key string) (string, bool, error) {
	var value string
	err := kv.s.db.QueryRow(
		`SELECT value FROM plugin_kv WHERE plugin_id = ? AND key = ?`, kv.pluginID, key,
	).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	return value, err == nil, err
}

func (kv *PluginKV) Set(key, value string) error {
	if key == "" || len(key) > maxKVKeyBytes {
		return fmt.Errorf("plugin kv: key must be 1-%d bytes", maxKVKeyBytes)
	}
	if len(value) > maxKVValueBytes {
		return fmt.Errorf("plugin kv: value exceeds %d bytes", maxKVValueBytes)
	}
	_, err := kv.s.db.Exec(
		`INSERT INTO plugin_kv (plugin_id, key, value, updated_at) VALUES (?, ?, ?, ?)
		 ON CONFLICT (plugin_id, key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at`,
		kv.pluginID, key, value, now(),
	)
	return err
}

func (kv *PluginKV) Delete(key string) error {
	_, err := kv.s.db.Exec(`DELETE FROM plugin_kv WHERE plugin_id = ? AND key = ?`, kv.pluginID, key)
	return err
}

// List returns every entry whose key starts with prefix ("" lists all).
func (kv *PluginKV) List(prefix string) (map[string]string, error) {
	rows, err := kv.s.db.Query(
		`SELECT key, value FROM plugin_kv
		 WHERE plugin_id = ? AND substr(key, 1, length(?)) = ?`,
		kv.pluginID, prefix, prefix,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			return nil, err
		}
		out[key] = value
	}
	return out, rows.Err()
}
