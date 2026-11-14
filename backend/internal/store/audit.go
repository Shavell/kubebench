package store

import (
	"strings"
	"time"
)

// AuditEntry is one audit_log row (docs/design-v0.5.md §3.4).
type AuditEntry struct {
	ID       int64
	Time     time.Time
	Username string
	Cluster  string
	Action   string // "<METHOD> <route pattern>" or an explicit name such as exec.session
	Target   string // "<kind>/<ns>/<name>"-style
	Status   int    // HTTP status; 0 for rows not tied to a response
	Detail   string // short JSON or error text
}

// AuditQuery pages through the log newest first.
type AuditQuery struct {
	Limit   int
	Before  int64  // only rows with a smaller id; 0 starts at the newest
	User    string // exact username
	Cluster string
	Action  string // case-insensitive substring
}

// InsertAudit appends a row; a zero Time means now.
func (s *Store) InsertAudit(e AuditEntry) error {
	if e.Time.IsZero() {
		e.Time = now()
	}
	_, err := s.db.Exec(
		`INSERT INTO audit_log (ts, username, cluster, action, target, status, detail)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		e.Time.UTC(), e.Username, e.Cluster, e.Action, e.Target, e.Status, e.Detail,
	)
	return err
}

func (s *Store) ListAudit(q AuditQuery) ([]AuditEntry, error) {
	var where []string
	var args []any
	if q.Before > 0 {
		where, args = append(where, "id < ?"), append(args, q.Before)
	}
	if q.User != "" {
		where, args = append(where, "username = ?"), append(args, q.User)
	}
	if q.Cluster != "" {
		where, args = append(where, "cluster = ?"), append(args, q.Cluster)
	}
	if q.Action != "" {
		where, args = append(where, `action LIKE ? ESCAPE '\'`), append(args, "%"+escapeLike(q.Action)+"%")
	}
	query := `SELECT id, ts, username, cluster, action, target, status, detail FROM audit_log`
	if len(where) > 0 {
		query += ` WHERE ` + strings.Join(where, " AND ")
	}
	query += ` ORDER BY id DESC LIMIT ?`
	rows, err := s.db.Query(query, append(args, q.Limit)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []AuditEntry{}
	for rows.Next() {
		var e AuditEntry
		if err := rows.Scan(&e.ID, &e.Time, &e.Username, &e.Cluster, &e.Action, &e.Target, &e.Status, &e.Detail); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// PruneAudit deletes rows written before t.
func (s *Store) PruneAudit(t time.Time) (int64, error) {
	res, err := s.db.Exec(`DELETE FROM audit_log WHERE ts < ?`, t.UTC())
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

var likeEscaper = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)

func escapeLike(s string) string { return likeEscaper.Replace(s) }
