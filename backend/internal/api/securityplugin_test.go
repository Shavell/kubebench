package api

import (
	"net/http"
	"net/url"
	"testing"

	"kubebench/internal/plugin"
	"kubebench/internal/plugins/security"
)

// The security plugin through the router: any role scans and reads
// reports without an audit row; accepting and un-accepting a risk is a
// write, refused for viewers, and audited with the finding as target.
func TestSecurityPluginGates(t *testing.T) {
	e := newTestEnv(t, nil, security.New())
	rows := len(e.audit())

	for _, user := range []string{"bob", "alice"} {
		expect(t, e.do(http.MethodPost, "/api/plugins/security/scan", user, map[string]bool{"includeSystem": false}), http.StatusOK, "")
		expect(t, e.do(http.MethodGet, "/api/plugins/security/report", user, nil), http.StatusOK, "")
		expect(t, e.do(http.MethodGet, "/api/plugins/security/export?format=csv", user, nil), http.StatusOK, "")
	}
	if len(e.audit()) != rows {
		t.Fatalf("read requests were audited: %+v", e.audit())
	}

	key := "default-namespace:Deployment:default:web"
	accept := map[string]string{"key": key, "reason": "batch worker"}
	expect(t, e.do(http.MethodPost, "/api/plugins/security/accept", "bob", accept), http.StatusForbidden, plugin.CodeForbiddenRole)
	if row := e.lastAudit(); row.Username != "bob" || row.Status != http.StatusForbidden || row.Target != "finding" {
		t.Fatalf("refused accept: audit row %+v", row)
	}
	expect(t, e.do(http.MethodPost, "/api/plugins/security/accept", "alice", accept), http.StatusOK, "")
	if row := e.lastAudit(); row.Username != "alice" || row.Status != http.StatusOK ||
		row.Target != "finding/"+key || row.Detail != `{"checkId":"default-namespace"}` {
		t.Fatalf("accept: audit row %+v", row)
	}

	rep := decode[security.Report](t, e.do(http.MethodGet, "/api/plugins/security/report", "bob", nil))
	accepted := false
	for _, f := range rep.Findings {
		if f.Key == key {
			accepted = f.Accepted != nil && f.Accepted.By == "alice"
		}
	}
	if !accepted {
		t.Fatalf("the viewer does not see the shared acceptance: %+v", rep.Findings)
	}

	path := "/api/plugins/security/accept/" + url.PathEscape(key)
	expect(t, e.do(http.MethodDelete, path, "bob", nil), http.StatusForbidden, plugin.CodeForbiddenRole)
	expect(t, e.do(http.MethodDelete, path, "alice", nil), http.StatusOK, "")
	if row := e.lastAudit(); row.Status != http.StatusOK || row.Target != "finding/"+key || row.Action != "DELETE /api/plugins/security/accept/{key}" {
		t.Fatalf("un-accept: audit row %+v", row)
	}
	if m := e.mutations(); len(m) != 0 {
		t.Fatalf("the plugin changed the cluster: %v", m)
	}
}
