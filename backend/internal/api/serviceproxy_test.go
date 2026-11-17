package api

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"kubebench/internal/plugin"
	"kubebench/internal/store"
)

const webProxyPath = "/api/v1/namespaces/default/services/http:web:80/proxy"

// fakeServiceProxy stands in for the API server's service proxy: it records
// the requests it gets and answers like a small web app behind it.
type fakeServiceProxy struct {
	mu   sync.Mutex
	reqs []*http.Request
}

func (f *fakeServiceProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	f.reqs = append(f.reqs, r.Clone(context.Background()))
	f.mu.Unlock()
	switch strings.TrimPrefix(r.URL.Path, webProxyPath) {
	case "/":
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Add("Set-Cookie", "app=1; Path=/")
		w.Header().Add("Set-Cookie", "kubebench_session=planted; Path=/")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Clear-Site-Data", `"cookies"`)
		io.WriteString(w, `<a href="`+webProxyPath+`/page">page</a>`)
	case "/redir":
		http.Redirect(w, r, "/login", http.StatusFound)
	case "/form":
		io.WriteString(w, "got "+r.Method)
	default:
		http.NotFound(w, r)
	}
}

func (f *fakeServiceProxy) last() *http.Request {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.reqs) == 0 {
		return nil
	}
	return f.reqs[len(f.reqs)-1]
}

func (f *fakeServiceProxy) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.reqs)
}

// proxyEnv is a testEnv whose cluster has the Service default/web (port
// 80, named http) behind a fake API server.
func proxyEnv(t *testing.T, env map[string]string) (*testEnv, *fakeServiceProxy) {
	t.Helper()
	e := newTestEnv(t, env)
	upstream := &fakeServiceProxy{}
	srv := httptest.NewServer(upstream)
	t.Cleanup(srv.Close)
	e.apiServer = srv.URL
	_, err := e.cs.CoreV1().Services("default").Create(context.Background(), &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "default"},
		Spec:       corev1.ServiceSpec{Ports: []corev1.ServicePort{{Name: "http", Port: 80}}},
	}, metav1.CreateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	return e, upstream
}

func (e *testEnv) openProxy(user, target string) proxySessionView {
	e.t.Helper()
	rec := e.do(http.MethodPost, target, user, nil)
	expect(e.t, rec, http.StatusOK, "")
	return decode[proxySessionView](e.t, rec)
}

// browse sends a proxied request the way a browser frame does: with
// whatever cookies the browser holds, but never authorized by them.
func (e *testEnv) browse(method, url string, header http.Header) *httptest.ResponseRecorder {
	e.t.Helper()
	req := httptest.NewRequest(method, url, strings.NewReader("q=1"))
	for k, v := range header {
		req.Header[k] = v
	}
	return e.serve(req)
}

func (e *testEnv) proxyAudits() []store.AuditEntry {
	var rows []store.AuditEntry
	for _, row := range e.audit() {
		if strings.HasSuffix(row.Action, ".proxy") {
			rows = append(rows, row)
		}
	}
	return rows
}

func TestOpenProxyAccess(t *testing.T) {
	e, _ := proxyEnv(t, nil)
	// A viewer is refused before the cluster is asked, and that is audited.
	expect(t, e.do(http.MethodPost, "/api/proxy/services/default/web/80", "bob", nil), http.StatusForbidden, plugin.CodeForbiddenRole)
	rows := e.proxyAudits()
	if len(rows) != 1 || rows[0].Username != "bob" || rows[0].Status != http.StatusForbidden || rows[0].Action != "service.proxy" {
		t.Fatalf("refusal rows = %+v", rows)
	}
	expect(t, e.do(http.MethodPost, "/api/proxy/services/default/nope/80", "alice", nil), http.StatusNotFound, plugin.CodeKubernetes)
	expect(t, e.do(http.MethodPost, "/api/proxy/services/default/web/81", "alice", nil), http.StatusBadRequest, plugin.CodeBadRequest)
	expect(t, e.do(http.MethodPost, "/api/proxy/services/default/web/80?scheme=ftp", "alice", nil), http.StatusBadRequest, plugin.CodeBadRequest)
	expect(t, e.do(http.MethodPost, "/api/proxy/pods/default/web-1/http", "alice", nil), http.StatusBadRequest, plugin.CodeBadRequest)
	expect(t, e.do(http.MethodPost, "/api/proxy/services/default/web/80", "", nil), http.StatusUnauthorized, plugin.CodeUnauthenticated)

	s := e.openProxy("alice", "/api/proxy/services/default/web/http")
	if !s.Sandboxed || s.ReadOnly || s.Base != "/proxy/"+s.Token || s.Target.Port != "http" || s.Target.Scheme != "http" {
		t.Fatalf("session = %+v", s)
	}
	pod := e.openProxy("admin", "/api/proxy/pods/default/web-1/8080?scheme=https")
	if pod.Target.Kind != "pods" || pod.Target.Scheme != "https" {
		t.Fatalf("pod session = %+v", pod)
	}
	rows = e.proxyAudits()
	if len(rows) != 7 {
		t.Fatalf("audit rows = %d, want one per attempt: %+v", len(rows), rows)
	}
	// Newest first.
	if rows[0].Action != "pod.proxy" || rows[0].Target != "pod/default/web-1:8080" || rows[0].Status != http.StatusOK || !strings.Contains(rows[0].Detail, `"scheme":"https"`) {
		t.Fatalf("pod row = %+v", rows[0])
	}
	if rows[1].Target != "service/default/web:http" || rows[1].Username != "alice" {
		t.Fatalf("service row = %+v", rows[1])
	}
}

func TestProxyForwardsAndIsolates(t *testing.T) {
	e, upstream := proxyEnv(t, nil)
	s := e.openProxy("alice", "/api/proxy/services/default/web/80")
	before := len(e.proxyAudits())

	rec := e.browse(http.MethodGet, s.Base+"/", http.Header{
		"Cookie":           {"kubebench_session=" + e.tokens["alice"] + "; app=1"},
		"Authorization":    {"Bearer browser-supplied"},
		"Impersonate-User": {"system:admin"},
		"Referer":          {"http://example.com" + s.Base + "/"},
		"Sec-Fetch-Mode":   {"navigate"},
		"Sec-Fetch-Dest":   {"iframe"},
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body)
	}
	up := upstream.last()
	if up.URL.Path != webProxyPath+"/" {
		t.Fatalf("upstream path = %q", up.URL.Path)
	}
	for _, h := range []string{"Authorization", "Impersonate-User", "Referer"} {
		if up.Header.Get(h) != "" {
			t.Errorf("%s reached the API server: %q", h, up.Header.Get(h))
		}
	}
	if got := up.Header.Get("Cookie"); got != "app=1" {
		t.Errorf("upstream Cookie = %q, want only the app's cookie", got)
	}
	h := rec.Header()
	if csp := h.Values("Content-Security-Policy"); len(csp) != 1 || !strings.HasPrefix(csp[0], "sandbox allow-scripts allow-forms allow-popups allow-downloads; frame-ancestors http://example.com") {
		t.Errorf("CSP = %q", csp)
	}
	if h.Get("X-Frame-Options") != "SAMEORIGIN" || h.Get("Clear-Site-Data") != "" {
		t.Errorf("headers = %v", h)
	}
	if got := h.Values("Set-Cookie"); len(got) != 1 || got[0] != "app=1; Path="+s.Base+"/" {
		t.Errorf("Set-Cookie = %q", got)
	}
	if got := rec.Body.String(); got != `<a href="`+s.Base+`/page">page</a>` {
		t.Errorf("body = %q", got)
	}

	rec = e.browse(http.MethodGet, s.Base+"/redir", nil)
	if rec.Code != http.StatusFound || rec.Header().Get("Location") != s.Base+"/login" {
		t.Errorf("redirect = %d %q", rec.Code, rec.Header().Get("Location"))
	}
	// The sandboxed page's own form posts carry "Origin: null".
	rec = e.browse(http.MethodPost, s.Base+"/form", http.Header{"Origin": {"null"}})
	if rec.Code != http.StatusOK || rec.Body.String() != "got POST" || rec.Header().Get("Access-Control-Allow-Origin") != "null" {
		t.Errorf("form post = %d %q %v", rec.Code, rec.Body, rec.Header())
	}
	rec = e.browse(http.MethodOptions, s.Base+"/form", http.Header{"Origin": {"null"}, "Access-Control-Request-Method": {"PUT"}, "Access-Control-Request-Headers": {"content-type"}})
	if rec.Code != http.StatusNoContent || !strings.Contains(rec.Header().Get("Access-Control-Allow-Methods"), "PUT") {
		t.Errorf("preflight = %d %v", rec.Code, rec.Header())
	}
	// Climbing out of the subresource is refused before the API server.
	n := upstream.count()
	if rec := e.browse(http.MethodGet, s.Base+"/a%2f..%2f..%2fsecrets", nil); rec.Code != http.StatusBadRequest {
		t.Errorf("encoded traversal = %d", rec.Code)
	}
	if upstream.count() != n {
		t.Error("a traversal reached the API server")
	}
	// One audit row per session, not per request.
	if got := len(e.proxyAudits()); got != before {
		t.Errorf("proxied requests wrote %d audit rows", got-before)
	}
	// The session reports where the frame navigated.
	got := decode[proxySessionView](t, e.do(http.MethodGet, "/api/proxy/sessions/"+s.Token, "alice", nil))
	if got.Location != "/" {
		t.Errorf("location = %q", got.Location)
	}
	expect(t, e.do(http.MethodGet, "/api/proxy/sessions/"+s.Token, "admin", nil), http.StatusNotFound, plugin.CodeNotFound)
	if rec := e.browse(http.MethodGet, "/proxy/"+s.Token, nil); rec.Code != http.StatusFound || rec.Header().Get("Location") != s.Base+"/" {
		t.Errorf("root = %d %q", rec.Code, rec.Header().Get("Location"))
	}
	// Closing the tab ends the session.
	expect(t, e.do(http.MethodDelete, "/api/proxy/sessions/"+s.Token, "alice", nil), http.StatusOK, "")
	if rec := e.browse(http.MethodGet, s.Base+"/", nil); rec.Code != http.StatusNotFound {
		t.Errorf("closed session = %d", rec.Code)
	}
}

func TestProxyOnAReadOnlyCluster(t *testing.T) {
	e, upstream := proxyEnv(t, nil)
	open := e.openProxy("alice", "/api/proxy/services/default/web/80")
	expect(t, e.do(http.MethodPatch, "/api/clusters/1", "admin", map[string]any{"readOnly": true}), http.StatusOK, "")

	// Opening stays allowed, for reading.
	s := e.openProxy("alice", "/api/proxy/services/default/web/80")
	if !s.ReadOnly {
		t.Fatalf("session = %+v, want readOnly", s)
	}
	if rec := e.browse(http.MethodGet, s.Base+"/form", nil); rec.Code != http.StatusOK {
		t.Fatalf("GET = %d", rec.Code)
	}
	n := upstream.count()
	// Both the new session and one opened before the flag was set.
	for _, base := range []string{s.Base, open.Base} {
		for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodDelete} {
			if rec := e.browse(method, base+"/form", nil); rec.Code != http.StatusForbidden {
				t.Errorf("%s on a read-only cluster = %d", method, rec.Code)
			}
		}
	}
	// A WebSocket upgrade is a GET, but it opens a two-way channel that
	// requests no method rule can see.
	upgrade := http.Header{"Connection": {"keep-alive, Upgrade"}, "Upgrade": {"websocket"}, "Sec-Websocket-Version": {"13"}, "Sec-Websocket-Key": {"dGhlIHNhbXBsZSBub25jZQ=="}}
	for _, header := range []http.Header{upgrade, {"Upgrade": {"websocket"}}} {
		if rec := e.browse(http.MethodGet, s.Base+"/form", header); rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), "WebSocket") {
			t.Errorf("upgrade %v on a read-only cluster = %d %q", header, rec.Code, rec.Body.String())
		}
	}
	if upstream.count() != n {
		t.Error("a refused request reached the API server")
	}
}

func TestProxyEndsWithAccess(t *testing.T) {
	revocations := map[string]struct {
		revoke func(e *testEnv) error
		status int
	}{
		"logout": {func(e *testEnv) error { return e.store.DeleteSession(e.tokens["alice"]) }, http.StatusUnauthorized},
		"demoted": {func(e *testEnv) error {
			return e.store.SetAssignments(e.users["alice"], []store.Assignment{{ClusterID: e.cluster, Role: store.ClusterRoleViewer}})
		}, http.StatusForbidden},
		"unassigned":      {func(e *testEnv) error { return e.store.SetAssignments(e.users["alice"], nil) }, http.StatusForbidden},
		"cluster removed": {func(e *testEnv) error { return e.store.DeleteCluster(e.cluster) }, http.StatusNotFound},
	}
	for name, r := range revocations {
		t.Run(name, func(t *testing.T) {
			e, _ := proxyEnv(t, nil)
			s := e.openProxy("alice", "/api/proxy/services/default/web/80")
			if rec := e.browse(http.MethodGet, s.Base+"/form", nil); rec.Code != http.StatusOK {
				t.Fatalf("before: %d", rec.Code)
			}
			if err := r.revoke(e); err != nil {
				t.Fatal(err)
			}
			rec := e.browse(http.MethodGet, s.Base+"/form", nil)
			if rec.Code != r.status || !strings.HasPrefix(rec.Body.String(), "KubeBench service browser: ") {
				t.Fatalf("after: %d %q, want %d", rec.Code, rec.Body, r.status)
			}
			if !strings.Contains(rec.Header().Get("Content-Security-Policy"), "sandbox") {
				t.Error("an error page went out without the sandbox")
			}
		})
	}
}

func TestProxyOrigin(t *testing.T) {
	e, _ := proxyEnv(t, map[string]string{"KUBEBENCH_PROXY_ORIGIN": "http://proxy.test"})
	s := e.openProxy("alice", "/api/proxy/services/default/web/80")
	if s.Sandboxed || s.Base != "http://proxy.test/proxy/"+s.Token {
		t.Fatalf("session = %+v", s)
	}
	path := strings.TrimPrefix(s.Base, "http://proxy.test")

	// On the proxy origin: no sandbox, framed only by KubeBench.
	req := httptest.NewRequest(http.MethodGet, "http://proxy.test"+path+"/", nil)
	rec := e.serve(req)
	if rec.Code != http.StatusOK || rec.Header().Get("Content-Security-Policy") != "frame-ancestors http://example.com" {
		t.Fatalf("proxy origin = %d %v", rec.Code, rec.Header())
	}
	// Nothing but the proxy is served there, not even with a session.
	req = httptest.NewRequest(http.MethodGet, "http://proxy.test/api/info", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookie, Value: e.tokens["alice"]})
	if rec := e.serve(req); rec.Code != http.StatusNotFound {
		t.Fatalf("/api/info on the proxy origin = %d", rec.Code)
	}
	// KubeBench's own origin still serves the grant, sandboxed.
	rec = e.browse(http.MethodGet, path+"/", nil)
	if !strings.HasPrefix(rec.Header().Get("Content-Security-Policy"), "sandbox ") {
		t.Fatalf("main origin CSP = %q", rec.Header().Get("Content-Security-Policy"))
	}
}

func TestProxyOriginMustBeAnotherSite(t *testing.T) {
	// Cookies are scoped by host (and parent domain), not by port: a page
	// on such a proxy origin could plant a kubebench_session cookie that
	// KubeBench would read. KubeBench answers as http://example.com.
	for _, origin := range []string{"http://example.com:9877", "http://proxy.example.com", "http://www.example.com:8080"} {
		t.Run(origin, func(t *testing.T) {
			e, upstream := proxyEnv(t, map[string]string{"KUBEBENCH_PROXY_ORIGIN": origin})
			rec := e.do(http.MethodPost, "/api/proxy/services/default/web/80", "alice", nil)
			expect(t, rec, http.StatusServiceUnavailable, plugin.CodeCapabilityUnavailable)
			if !strings.Contains(rec.Body.String(), "KUBEBENCH_PROXY_ORIGIN") {
				t.Errorf("error = %s", rec.Body.String())
			}
			if upstream.count() != 0 {
				t.Error("the refused grant reached the API server")
			}
		})
	}
}

func TestAppOriginIsAPlainOrigin(t *testing.T) {
	e := newTestEnv(t, nil)
	for origin, want := range map[string]string{
		"https://kb.example:8443":         "https://kb.example:8443",
		"http://[::1]:9876":               "http://[::1]:9876",
		"http://kb.example; script-src *": "http://example.com", // falls back to the Host
		"null":                            "http://example.com",
	} {
		req := httptest.NewRequest(http.MethodPost, "/api/proxy/services/default/web/80", nil)
		req.Header.Set("Origin", origin)
		if got := e.srv.appOrigin(req); got != want {
			t.Errorf("Origin %q: %q, want %q", origin, got, want)
		}
	}
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Host = "kb.example;sandbox"
	if got := e.srv.appOrigin(req); got != "'self'" {
		t.Errorf("odd Host: %q", got)
	}
}
