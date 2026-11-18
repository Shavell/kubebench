package api

import (
	"net/http"
	"strconv"
	"time"

	"kubebench/internal/plugin"
	"kubebench/internal/reason"
	"kubebench/internal/store"
)

// Page sizes of GET /api/audit.
const (
	defaultAuditPage = 100
	maxAuditPage     = 500
)

type auditView struct {
	ID      int64  `json:"id"`
	TS      string `json:"ts"`
	User    string `json:"user"`
	Cluster string `json:"cluster"`
	Action  string `json:"action"`
	Target  string `json:"target"`
	Status  int    `json:"status"`
	Detail  string `json:"detail"`
}

type auditPage struct {
	Items      []auditView `json:"items"`
	NextBefore *int64      `json:"nextBefore"`
}

// handleListAudit pages through the audit log, newest first (§3.4).
func (s *Server) handleListAudit(w http.ResponseWriter, r *http.Request, _ *call) {
	q := r.URL.Query()
	limit, err := intParam(q.Get("limit"), defaultAuditPage, 1)
	if err != nil {
		writeErr(w, err)
		return
	}
	before, err := intParam(q.Get("before"), 0, 1)
	if err != nil {
		writeErr(w, err)
		return
	}
	limit = min(limit, maxAuditPage)
	// One extra row tells whether another page exists.
	rows, err := s.store.ListAudit(store.AuditQuery{
		Limit: limit + 1, Before: int64(before),
		User: q.Get("user"), Cluster: q.Get("cluster"), Action: q.Get("action"),
	})
	if err != nil {
		writeErr(w, err)
		return
	}
	page := auditPage{Items: []auditView{}}
	if len(rows) > limit {
		rows = rows[:limit]
		next := rows[limit-1].ID
		page.NextBefore = &next
	}
	for _, e := range rows {
		page.Items = append(page.Items, auditView{
			ID: e.ID, TS: e.Time.UTC().Format(time.RFC3339Nano), User: e.Username, Cluster: e.Cluster,
			Action: e.Action, Target: e.Target, Status: e.Status, Detail: e.Detail,
		})
	}
	plugin.WriteJSON(w, page)
}

// intParam parses an optional integer query value of at least lowest.
func intParam(raw string, fallback, lowest int) (int, error) {
	if raw == "" {
		return fallback, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < lowest {
		return 0, badRequest(reasonInvalidNumber, reason.Params{"value": raw, "min": lowest})
	}
	return n, nil
}
