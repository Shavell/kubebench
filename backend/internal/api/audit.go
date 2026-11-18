package api

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"maps"
	"net"
	"net/http"
	"slices"
	"strings"
	"sync"
	"unicode/utf8"

	"kubebench/internal/plugin"
	"kubebench/internal/reason"
	"kubebench/internal/store"
)

// Sizes kept of an audit row's fields, in characters, so that no request
// can write rows of arbitrary size.
const (
	maxAuditAction  = 200
	maxAuditTarget  = 512
	maxAuditDetail  = 1000
	maxAuditErrBody = 2048 // bytes of an error response read for the detail
)

// targetParams are the query parameters that name an object, in the order
// they appear in an audit target ("<kind>/<ns>/<name>/<container>").
var targetParams = []string{"ns", "namespace", "name", "pod", "node", "container"}

// kindParamKinds are the path kinds of the core routes that act on the
// kind their kind query parameter names (/api/resource…, /api/workloads/…;
// §7.2, §7.3). Every other route acts on the kind its path names; there
// the parameter is ignored by the handler and must not rewrite the target.
var kindParamKinds = map[string]bool{"resource": true, "workload": true}

// auditTarget is how a route derives the default target of its rows.
type auditTarget struct {
	kind      string   // e.g. "deployment" for /api/deployments/scale
	wildcards []string // path wildcards, appended in order
	// kindParam makes the kind query parameter, when given, the kind.
	kindParam bool
}

// auditWriter records the outcome of one audited request (§3.4). It wraps
// the ResponseWriter to capture the status and the start of an error body,
// while keeping http.Flusher and http.Hijacker working for streaming and
// WebSocket handlers. The row is written when the handler returns, or at
// hijack time for connections that outlive the handler.
type auditWriter struct {
	http.ResponseWriter
	s      *Server
	c      *call
	action string
	target string
	detail string
	// more are the details of further rows of the same request, for a
	// record that does not fit one row (see call.setAuditParts).
	more []string

	status  int
	errBody bytes.Buffer
	once    sync.Once
}

func (s *Server) newAuditWriter(w http.ResponseWriter, r *http.Request, c *call, action string, t auditTarget) *auditWriter {
	return &auditWriter{
		ResponseWriter: w, s: s, c: c, action: action,
		target: defaultTarget(r, t), detail: defaultDetail(r),
	}
}

func (a *auditWriter) WriteHeader(code int) {
	if a.status == 0 && code >= http.StatusOK {
		a.status = code
	}
	a.ResponseWriter.WriteHeader(code)
}

func (a *auditWriter) Write(b []byte) (int, error) {
	if a.status == 0 {
		a.status = http.StatusOK
	}
	if a.status >= http.StatusBadRequest && a.errBody.Len() < maxAuditErrBody {
		a.errBody.Write(b[:min(len(b), maxAuditErrBody-a.errBody.Len())])
	}
	return a.ResponseWriter.Write(b)
}

func (a *auditWriter) Flush() {
	if a.status == 0 {
		a.status = http.StatusOK
	}
	_ = http.NewResponseController(a.ResponseWriter).Flush()
}

func (a *auditWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	conn, rw, err := http.NewResponseController(a.ResponseWriter).Hijack()
	if err == nil {
		a.status = http.StatusSwitchingProtocols
		a.finish()
	}
	return conn, rw, err
}

// Unwrap lets http.ResponseController reach the underlying writer.
func (a *auditWriter) Unwrap() http.ResponseWriter { return a.ResponseWriter }

// recordable refuses a request whose URL parameters do not fit into its
// row, before the handler runs: the log must show completely what was
// done, never a command cut off after its harmless start. Kubernetes names
// fit easily. a may be nil (a request that is not audited).
func (a *auditWriter) recordable() error {
	if a == nil {
		return nil
	}
	if utf8.RuneCountInString(a.target) > maxAuditTarget {
		return badRequest(reasonAuditTarget, reason.Params{"max": maxAuditTarget})
	}
	if utf8.RuneCountInString(a.detail) > maxAuditDetail {
		return badRequest(reasonAuditDetail, reason.Params{"max": maxAuditDetail})
	}
	return nil
}

// finish writes the audit row once.
func (a *auditWriter) finish() {
	a.once.Do(func() {
		status := a.status
		if status == 0 {
			status = http.StatusOK
		}
		detail := a.detail
		if status >= http.StatusBadRequest {
			if msg := errorDetail(a.errBody.Bytes()); msg != "" {
				detail = msg
			}
		}
		for _, d := range append([]string{detail}, a.more...) {
			a.s.writeAudit(store.AuditEntry{
				Username: a.c.user.Username,
				Cluster:  a.c.clusterName(),
				Action:   a.action,
				Target:   a.target,
				Status:   status,
				Detail:   d,
			})
		}
	})
}

// auditParts renders fields plus items (under key) as audit details of at
// most maxAuditDetail characters each, so that no item is cut off: one
// detail when everything fits, else one per share of items, each with
// "part":"<i>/<n>". A single item must fit one detail.
func auditParts(fields map[string]any, key string, items []string) []string {
	render := func(share []string, part string) string {
		row := maps.Clone(fields)
		row[key] = share
		if part != "" {
			row["part"] = part
		}
		return auditJSON(row)
	}
	var shares [][]string
	current := []string{} // no items: an empty list, not null
	for _, item := range items {
		next := append(slices.Clip(current), item)
		// Sized with the widest part label a bulk request can need.
		if len(current) > 0 && utf8.RuneCountInString(render(next, "999/999")) > maxAuditDetail {
			shares = append(shares, current)
			next = []string{item}
		}
		current = next
	}
	shares = append(shares, current)
	if len(shares) == 1 {
		return []string{render(shares[0], "")}
	}
	details := make([]string, len(shares))
	for i, share := range shares {
		details[i] = render(share, fmt.Sprintf("%d/%d", i+1, len(shares)))
	}
	return details
}

// writeAudit appends a row; a failure is logged but never fails the request.
func (s *Server) writeAudit(e store.AuditEntry) {
	e.Action = truncate(e.Action, maxAuditAction)
	e.Target = truncate(e.Target, maxAuditTarget)
	e.Detail = truncate(e.Detail, maxAuditDetail)
	if err := s.store.InsertAudit(e); err != nil {
		log.Printf("[audit] %s %s by %s: %v", e.Action, e.Target, e.Username, err)
	}
}

func (c *call) clusterName() string {
	if c.cluster == nil {
		return ""
	}
	return c.cluster.Name
}

// errorDetail renders an error response as "<code>: <message>".
func errorDetail(body []byte) string {
	var e plugin.ErrorBody
	if json.Unmarshal(body, &e) == nil && e.Error != "" {
		if e.Code == "" {
			return e.Error
		}
		return e.Code + ": " + e.Error
	}
	return strings.TrimSpace(string(body))
}

// defaultTarget builds "<kind>/<ns>/<name>"-style targets from the query
// and path, e.g. "deployment/default/web" for
// POST /api/deployments/scale?ns=default&name=web.
func defaultTarget(r *http.Request, t auditTarget) string {
	q := r.URL.Query()
	parts := []string{t.kind}
	if kind := q.Get("kind"); kind != "" && t.kindParam {
		parts[0] = strings.ToLower(kind)
	}
	for _, key := range targetParams {
		parts = append(parts, q.Get(key))
	}
	for _, name := range t.wildcards {
		parts = append(parts, r.PathValue(name))
	}
	return joinTarget(parts...)
}

// defaultDetail is the remaining query parameters as a JSON object, e.g.
// {"replicas":"3"}. Request bodies are never recorded: they may carry
// passwords or secrets.
func defaultDetail(r *http.Request) string {
	extra := map[string]string{}
	for key, values := range r.URL.Query() {
		if key == "kind" || key == "dryRun" || slices.Contains(targetParams, key) {
			continue
		}
		extra[key] = values[0]
	}
	if len(extra) == 0 {
		return ""
	}
	return auditJSON(extra)
}

// joinTarget joins the non-empty parts with "/".
func joinTarget(parts ...string) string {
	kept := parts[:0:0]
	for _, p := range parts {
		if p != "" {
			kept = append(kept, p)
		}
	}
	return strings.Join(kept, "/")
}

// targetKind derives the target kind from a route path: the first segment
// after /api/ (or /api/plugins/<id>/), made singular, so
// /api/deployments/scale gives "deployment" and a plugin's
// /releases/{ns}/{name}/rollback gives "release".
func targetKind(path string, p plugin.Plugin) string {
	prefix := "/api/"
	if p != nil {
		prefix = "/api/plugins/" + p.Manifest().ID + "/"
	}
	first, _, _ := strings.Cut(strings.TrimPrefix(path, prefix), "/")
	if strings.HasPrefix(first, "{") {
		return ""
	}
	switch {
	case strings.HasSuffix(first, "ies"):
		return strings.TrimSuffix(first, "ies") + "y"
	case strings.HasSuffix(first, "sses"):
		return strings.TrimSuffix(first, "es")
	case strings.HasSuffix(first, "s") && !strings.HasSuffix(first, "ss"):
		return strings.TrimSuffix(first, "s")
	}
	return first
}

// wildcardNames lists the {name} and {name...} wildcards of a path.
func wildcardNames(path string) []string {
	var names []string
	for _, seg := range strings.Split(path, "/") {
		if len(seg) > 2 && seg[0] == '{' && seg[len(seg)-1] == '}' {
			if name := strings.TrimSuffix(seg[1:len(seg)-1], "..."); name != "$" {
				names = append(names, name)
			}
		}
	}
	return names
}

func truncate(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	return string([]rune(s)[:n-1]) + "…"
}
