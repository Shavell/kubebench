package svcproxy

import (
	"io"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"testing"
	"testing/iotest"
	"time"
)

var web = Target{Kind: KindService, Namespace: "shop", Name: "web", Port: "80", Scheme: "http"}

func TestTargetValidate(t *testing.T) {
	if err := web.Validate(); err != nil {
		t.Fatal(err)
	}
	named := web
	named.Port = "http-web"
	if err := named.Validate(); err != nil {
		t.Fatal(err)
	}
	for name, bad := range map[string]Target{
		"kind":      {Kind: "secrets", Namespace: "shop", Name: "web", Port: "80", Scheme: "http"},
		"namespace": {Kind: KindService, Namespace: "../x", Name: "web", Port: "80", Scheme: "http"},
		"name":      {Kind: KindService, Namespace: "shop", Name: "web/proxy", Port: "80", Scheme: "http"},
		"port":      {Kind: KindService, Namespace: "shop", Name: "web", Port: "70000", Scheme: "http"},
		"port name": {Kind: KindService, Namespace: "shop", Name: "web", Port: "a:b", Scheme: "http"},
		"scheme":    {Kind: KindService, Namespace: "shop", Name: "web", Port: "80", Scheme: "ftp"},
	} {
		if err := bad.Validate(); err == nil {
			t.Errorf("%s: %+v passed validation", name, bad)
		}
	}
	if got := web.APIPath(); got != "/api/v1/namespaces/shop/services/http:web:80/proxy" {
		t.Errorf("APIPath = %q", got)
	}
	if got := (Target{Kind: KindPod, Namespace: "shop", Name: "web-1", Port: "8080"}).AuditTarget(); got != "pod/shop/web-1:8080" {
		t.Errorf("AuditTarget = %q", got)
	}
}

func TestCleanPath(t *testing.T) {
	for in, want := range map[string]string{"": "/", "/": "/", "/a/b.js": "/a/b.js", "x": "/x", "/a%20b": "/a%20b"} {
		if got, err := CleanPath(in); err != nil || got != want {
			t.Errorf("CleanPath(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, bad := range []string{"/../secrets", "/a/./b", "/%2e%2e/x", "/%2E%2e/x", "/a%2fb", "/a%5Cb", `/a\b`, "/%zz"} {
		if _, err := CleanPath(bad); err == nil {
			t.Errorf("CleanPath(%q) passed", bad)
		}
	}
}

type clock struct{ now time.Time }

func (c *clock) Now() time.Time { return c.now }

func TestGrantsExpireAndBelongToTheirSession(t *testing.T) {
	c := &clock{now: time.Date(2026, 11, 17, 12, 0, 0, 0, time.UTC)}
	gs := NewGrants(c.Now)
	g, err := gs.Issue(Grant{Session: "s1", Target: web})
	if err != nil || len(g.Token) < 40 {
		t.Fatalf("Issue = %+v, %v", g, err)
	}
	if _, ok := gs.Get(g.Token, "s2"); ok {
		t.Fatal("another session read the grant")
	}
	if gs.Revoke(g.Token, "s2") {
		t.Fatal("another session revoked the grant")
	}
	gs.RecordNavigation(g.Token, "/a?b=1")
	if got, ok := gs.Get(g.Token, "s1"); !ok || !slices.Equal(got.Navigations(), []Navigation{{Seq: 1, Location: "/a?b=1"}}) {
		t.Fatalf("Get = %+v, %v", got, ok)
	}
	// Used within the idle timeout, it lives on, up to its lifetime.
	for range 12 { // 12 × 59 min < MaxLifetime
		c.now = c.now.Add(IdleTimeout - time.Minute)
		if _, ok := gs.Use(g.Token); !ok {
			t.Fatalf("grant ended at %v while in use", c.now)
		}
	}
	c.now = c.now.Add(IdleTimeout - time.Minute) // past MaxLifetime
	if _, ok := gs.Use(g.Token); ok {
		t.Fatal("grant outlived MaxLifetime")
	}
	idle, _ := gs.Issue(Grant{Session: "s1", Target: web})
	c.now = c.now.Add(IdleTimeout)
	if _, ok := gs.Use(idle.Token); ok {
		t.Fatal("idle grant did not expire")
	}
	revoked, _ := gs.Issue(Grant{Session: "s1", Target: web})
	if !gs.Revoke(revoked.Token, "s1") {
		t.Fatal("Revoke failed")
	}
	if _, ok := gs.Use(revoked.Token); ok {
		t.Fatal("revoked grant still works")
	}
}

func TestGrantsPerSessionLimitEndsTheLeastRecentlyUsed(t *testing.T) {
	c := &clock{now: time.Date(2026, 11, 17, 12, 0, 0, 0, time.UTC)}
	gs := NewGrants(c.Now)
	var tokens []string
	for range PerSession {
		g, _ := gs.Issue(Grant{Session: "s1", Target: web})
		tokens = append(tokens, g.Token)
		c.now = c.now.Add(time.Second)
	}
	gs.Use(tokens[0]) // now the most recently used
	other, _ := gs.Issue(Grant{Session: "s2", Target: web})
	gs.Issue(Grant{Session: "s1", Target: web})
	if _, ok := gs.Use(tokens[1]); ok {
		t.Fatal("the least recently used grant survived the limit")
	}
	for _, token := range []string{tokens[0], tokens[2], other.Token} {
		if _, ok := gs.Use(token); !ok {
			t.Fatalf("grant %s ended", token)
		}
	}
}

func TestFilterRequestHeaders(t *testing.T) {
	h := http.Header{
		"Authorization":         {"Bearer stolen"},
		"Proxy-Authorization":   {"Basic x"},
		"Impersonate-User":      {"system:admin"},
		"Impersonate-Group":     {"system:masters"},
		"Impersonate-Extra-X":   {"y"},
		"X-Remote-User":         {"root"},
		"Referer":               {"http://kb/proxy/token/"},
		"Accept-Encoding":       {"br"},
		"Cookie":                {"app=1; kubebench_session=secret", "other=2"},
		"Accept":                {"text/html"},
		"X-Csrf-Token":          {"abc"},
		"Sec-Websocket-Key":     {"k"},
		"Sec-Websocket-Version": {"13"},
	}
	filterRequestHeaders(h, "kubebench_session")
	for _, gone := range []string{"Authorization", "Proxy-Authorization", "Impersonate-User", "Impersonate-Group", "Impersonate-Extra-X", "X-Remote-User", "Referer", "Accept-Encoding"} {
		if h.Get(gone) != "" {
			t.Errorf("%s was forwarded", gone)
		}
	}
	if got := h.Values("Cookie"); len(got) != 1 || got[0] != "app=1; other=2" {
		t.Errorf("Cookie = %q", got)
	}
	for _, kept := range []string{"Accept", "X-Csrf-Token", "Sec-Websocket-Key", "Sec-Websocket-Version"} {
		if h.Get(kept) == "" {
			t.Errorf("%s was dropped", kept)
		}
	}
	only := http.Header{"Cookie": {"kubebench_session=secret"}}
	filterCookies(only, "kubebench_session")
	if _, ok := only["Cookie"]; ok {
		t.Errorf("an empty Cookie header was left: %q", only)
	}
}

func TestStripFrameAncestors(t *testing.T) {
	h := http.Header{"Content-Security-Policy": {"default-src 'self'; frame-ancestors 'none'", "FRAME-ANCESTORS 'self'"}}
	stripFrameAncestors(h, "Content-Security-Policy")
	if got := h.Values("Content-Security-Policy"); len(got) != 1 || got[0] != "default-src 'self'" {
		t.Errorf("CSP = %q", got)
	}
}

func TestIsolationHeaders(t *testing.T) {
	h := http.Header{}
	setIsolationHeaders(h, true, "https://kb.example")
	if got := h.Get("Content-Security-Policy"); got != "sandbox "+SandboxFlags+"; frame-ancestors https://kb.example" {
		t.Errorf("sandboxed CSP = %q", got)
	}
	if strings.Contains(SandboxFlags, "allow-same-origin") || strings.Contains(SandboxFlags, "allow-top-navigation") {
		t.Fatal("the sandbox must keep the opaque origin and block top navigation")
	}
	if h.Get("X-Frame-Options") != "SAMEORIGIN" || h.Get("Referrer-Policy") != "no-referrer" || h.Get("Cross-Origin-Resource-Policy") == "" {
		t.Errorf("headers = %v", h)
	}
	h = http.Header{}
	setIsolationHeaders(h, false, "https://kb.example")
	if got := h.Get("Content-Security-Policy"); got != "frame-ancestors https://kb.example" || h.Get("X-Frame-Options") != "" {
		t.Errorf("proxy origin headers = %v", h)
	}
}

func newTestRewriter(apiServer string) *rewriter {
	u, _ := url.Parse(apiServer)
	return newRewriter("/proxy/tok", u, web)
}

func TestLocationRewrite(t *testing.T) {
	rw := newTestRewriter("https://10.0.0.1:6443")
	for in, want := range map[string]string{
		"/login?next=%2F":          "/proxy/tok/login?next=%2F",
		"page.html":                "page.html",
		"../up":                    "../up",
		"http://10.0.0.1:8080/p#x": "/proxy/tok/p#x",
		"https://example.com/x":    "https://example.com/x",
		"/api/v1/namespaces/shop/services/http:web:80/proxy/a":                     "/proxy/tok/a",
		"/api/v1/namespaces/shop/services/http:web:80/proxy":                       "/proxy/tok/",
		"https://other.example/api/v1/namespaces/shop/services/http:web:80/proxy/": "/proxy/tok/",
	} {
		if got := rw.location(in); got != want {
			t.Errorf("location(%q) = %q, want %q", in, got, want)
		}
	}
	prefixed := newTestRewriter("https://rancher.example/k8s/clusters/c-1")
	if got := prefixed.location("/k8s/clusters/c-1/api/v1/namespaces/shop/services/http:web:80/proxy/x"); got != "/proxy/tok/x" {
		t.Errorf("prefixed location = %q", got)
	}
}

func TestSetCookieRewrite(t *testing.T) {
	rw := newTestRewriter("https://10.0.0.1:6443")
	if _, ok := rw.setCookie("kubebench_session=evil; Path=/", "kubebench_session"); ok {
		t.Fatal("the session cookie's name passed")
	}
	got, ok := rw.setCookie("sid=1; Path=/app; Domain=example.com; HttpOnly; SameSite=Lax", "kubebench_session")
	if !ok || got != "sid=1; Path=/proxy/tok/app; HttpOnly; SameSite=Lax" {
		t.Errorf("setCookie = %q, %v", got, ok)
	}
	if got, _ := rw.setCookie("a=b", "kubebench_session"); got != "a=b" {
		t.Errorf("cookie without a path = %q", got)
	}
}

func TestBodyRewriteAcrossReads(t *testing.T) {
	rw := newTestRewriter("https://10.0.0.1:6443/gw")
	prefix := "/api/v1/namespaces/shop/services/http:web:80/proxy"
	in := `<a href="` + prefix + `/a">a</a><img src="/gw` + prefix + `/b.png">` + strings.Repeat("x/", 40000) + `<a href="` + prefix + `/">`
	want := `<a href="/proxy/tok/a">a</a><img src="/proxy/tok/b.png">` + strings.Repeat("x/", 40000) + `<a href="/proxy/tok/">`
	// One byte per read splits every pattern.
	body := rw.body(io.NopCloser(iotest.OneByteReader(strings.NewReader(in))))
	got, err := io.ReadAll(body)
	if err != nil || string(got) != want {
		t.Fatalf("rewritten body differs (err %v): %.200q", err, got)
	}
	got, _ = io.ReadAll(rw.body(io.NopCloser(strings.NewReader(in))))
	if string(got) != want {
		t.Fatal("rewritten body differs with large reads")
	}
}

func TestSameSite(t *testing.T) {
	for _, pair := range [][2]string{
		{"kb.example.com", "kb.example.com"},
		{"kb.example.com", "KB.Example.com."},
		{"kb.example.com", "proxy.example.com"},
		{"example.com", "proxy.example.com"},
		{"kb.corp.internal", "proxy.corp.internal"},
		{"10.0.0.5", "10.0.0.5"},
		{"[::1]", "::1"},
		{"localhost", "localhost"},
		{"localhost", "proxy.localhost"},
	} {
		if !SameSite(pair[0], pair[1]) {
			t.Errorf("SameSite(%q, %q) = false", pair[0], pair[1])
		}
	}
	for _, pair := range [][2]string{
		{"kb.example.com", "kubebench-proxy.example.net"},
		{"a.github.io", "b.github.io"}, // a public suffix
		{"10.0.0.5", "10.0.0.6"},
		{"kb.example.com", "10.0.0.5"},
	} {
		if SameSite(pair[0], pair[1]) {
			t.Errorf("SameSite(%q, %q) = true", pair[0], pair[1])
		}
	}
}

func TestIsUpgrade(t *testing.T) {
	for _, h := range []http.Header{
		{"Upgrade": {"websocket"}},
		{"Connection": {"keep-alive, Upgrade"}},
		{"Connection": {"close", "upgrade"}},
	} {
		if !isUpgrade(h) {
			t.Errorf("isUpgrade(%v) = false", h)
		}
	}
	if isUpgrade(http.Header{"Connection": {"keep-alive"}}) {
		t.Error("keep-alive is not an upgrade")
	}
}

func TestBodyRewriteOfAbsoluteURLsNamingTheAPIServer(t *testing.T) {
	rw := newTestRewriter("https://10.0.0.1:6443/gw")
	prefix := "/api/v1/namespaces/shop/services/http:web:80/proxy"
	in := `<a href="https://10.0.0.1:6443/gw` + prefix + `/a">` +
		`<a href="//10.0.0.1:6443` + prefix + `/b">` +
		`<a href="http://10.0.0.1` + prefix + `/c">` +
		`<a href="https://other.example` + prefix + `/d">`
	want := `<a href="/proxy/tok/a">` +
		`<a href="/proxy/tok/b">` +
		`<a href="/proxy/tok/c">` +
		`<a href="https://other.example/proxy/tok/d">`
	for name, src := range map[string]io.Reader{
		"one byte per read": iotest.OneByteReader(strings.NewReader(in)),
		"large reads":       strings.NewReader(in),
	} {
		got, err := io.ReadAll(rw.body(io.NopCloser(src)))
		if err != nil || string(got) != want {
			t.Errorf("%s: %q (err %v)", name, got, err)
		}
	}
}

func TestGrantsKeepTheLatestNavigations(t *testing.T) {
	gs := NewGrants(time.Now)
	g, _ := gs.Issue(Grant{Session: "s1", Target: web})
	for i := range MaxNavigations + 3 {
		gs.RecordNavigation(g.Token, "/p"+strconv.Itoa(i+1))
	}
	gs.RecordNavigation(g.Token, "/"+strings.Repeat("x", MaxLocation))
	got, _ := gs.Get(g.Token, "s1")
	navs := got.Navigations()
	if len(navs) != MaxNavigations || navs[0] != (Navigation{Seq: 5, Location: "/p5"}) {
		t.Fatalf("oldest kept = %+v of %d", navs[0], len(navs))
	}
	if last := navs[len(navs)-1]; last != (Navigation{Seq: MaxNavigations + 4, Location: ""}) {
		t.Errorf("an over-long location = %+v", last)
	}
	gs.RecordNavigation("unknown", "/x") // ignored
}
