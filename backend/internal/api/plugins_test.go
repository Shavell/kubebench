package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"kubebench/internal/kube"
	"kubebench/internal/plugin"
)

func TestPluginRoutesAreGated(t *testing.T) {
	fake := newFakePlugin("fake")
	e := newTestEnv(t, nil, fake)

	// Built-in plugins start enabled; handlers get the caller and cluster.
	got := decode[map[string]string](t, e.do(http.MethodGet, "/api/plugins/fake/items", "bob", nil))
	if got["user"] != "bob" || got["cluster"] != "test" {
		t.Fatalf("plugin context = %v", got)
	}
	expect(t, e.do(http.MethodGet, "/api/plugins/fake/items", "", nil), http.StatusUnauthorized, plugin.CodeUnauthenticated)
	expect(t, e.do(http.MethodPost, "/api/plugins/fake/settings", "alice", nil), http.StatusForbidden, plugin.CodeAdminRequired)
	expect(t, e.do(http.MethodGet, "/api/plugins/fake/nope", "bob", nil), http.StatusNotFound, plugin.CodeNotFound)

	expect(t, e.do(http.MethodPut, "/api/plugins/fake", "admin", map[string]bool{"enabled": false}), http.StatusOK, "")
	expect(t, e.do(http.MethodGet, "/api/plugins/fake/items", "bob", nil), http.StatusNotFound, plugin.CodePluginDisabled)
	expect(t, e.do(http.MethodPut, "/api/plugins/fake", "admin", map[string]bool{"enabled": true}), http.StatusOK, "")
	expect(t, e.do(http.MethodGet, "/api/plugins/fake/items", "bob", nil), http.StatusOK, "")

	// Detect results are cached for a minute per cluster.
	fake.status.Store(&plugin.Status{Reason: "the CRDs are not installed"})
	expect(t, e.do(http.MethodGet, "/api/plugins/fake/items", "bob", nil), http.StatusOK, "")
	if n := fake.detects.Load(); n != 1 {
		t.Fatalf("Detect ran %d times within the cache TTL", n)
	}
	e.clock.Advance(detectTTL)
	rec := e.do(http.MethodGet, "/api/plugins/fake/items", "bob", nil)
	expect(t, rec, http.StatusServiceUnavailable, plugin.CodePluginUnavailable)
	if body := decode[plugin.ErrorBody](t, rec); body.Error != "the CRDs are not installed" {
		t.Fatalf("reason = %q", body.Error)
	}

	// Plugin routes keep the core gates.
	fake.status.Store(&plugin.Status{Available: true})
	e.clock.Advance(detectTTL)
	expect(t, e.do(http.MethodPatch, "/api/clusters/1", "admin", map[string]any{"readOnly": true}), http.StatusOK, "")
	expect(t, e.do(http.MethodPost, "/api/plugins/fake/items/a", "admin", nil), http.StatusForbidden, plugin.CodeClusterReadOnly)
	expect(t, e.do(http.MethodPost, "/api/plugins/fake/items/a?dryRun=true", "bob", nil), http.StatusOK, "")
	if err := e.store.SetMustChangePassword(e.users["bob"], true); err != nil {
		t.Fatal(err)
	}
	expect(t, e.do(http.MethodGet, "/api/plugins/fake/items", "bob", nil), http.StatusForbidden, plugin.CodePasswordChangeRequired)
}

func TestPluginKVAndExtraAudit(t *testing.T) {
	e := newTestEnv(t, nil, newFakePlugin("fake"), newFakePlugin("other"))
	expect(t, e.do(http.MethodPost, "/api/plugins/fake/items/a", "alice", nil), http.StatusOK, "")
	if items := decode[map[string]string](t, e.do(http.MethodGet, "/api/plugins/fake/items/stored", "bob", nil)); items["item/a"] != "created" {
		t.Fatalf("fake KV = %v", items)
	}
	if items := decode[map[string]string](t, e.do(http.MethodGet, "/api/plugins/other/items/stored", "bob", nil)); len(items) != 0 {
		t.Fatalf("KV leaked across plugins: %v", items)
	}

	expect(t, e.do(http.MethodPost, "/api/plugins/fake/settings", "admin", nil), http.StatusOK, "")
	rows := e.audit()
	if len(rows) != 3 {
		t.Fatalf("audit rows = %+v", rows)
	}
	// The handler's extra row lands before the router's row for the request.
	if rows[1].Action != "fake.settings" || rows[1].Target != "settings/global" || rows[1].Detail != `{"extra":true}` || rows[1].Cluster != "test" {
		t.Fatalf("extra row = %+v", rows[1])
	}
	if rows[0].Action != "POST /api/plugins/fake/settings" || rows[0].Status != http.StatusOK {
		t.Fatalf("router row = %+v", rows[0])
	}
}

func TestPluginCatalog(t *testing.T) {
	fake := newFakePlugin("fake")
	e := newTestEnv(t, nil, fake)
	expect(t, e.do(http.MethodPut, "/api/plugins/topology", "admin", map[string]bool{"enabled": false}), http.StatusOK, "")
	expect(t, e.do(http.MethodPut, "/api/plugins/Bad_ID", "admin", map[string]bool{"enabled": true}), http.StatusBadRequest, plugin.CodeBadRequest)
	expect(t, e.do(http.MethodPut, "/api/plugins/topology", "admin", map[string]any{}), http.StatusBadRequest, plugin.CodeBadRequest)

	catalog := decode[pluginCatalog](t, e.do(http.MethodGet, "/api/plugins", "bob", nil))
	if len(catalog.Plugins) != 1 {
		t.Fatalf("plugins = %+v", catalog.Plugins)
	}
	want := pluginView{
		ID: "fake", Name: "Fake", Version: "0.0.1", Description: "test plugin", Category: "tools",
		Builtin: true, Enabled: true, Status: plugin.Status{Available: true},
	}
	if !reflect.DeepEqual(catalog.Plugins[0], want) {
		t.Fatalf("plugin = %+v, want %+v", catalog.Plugins[0], want)
	}
	if s, ok := catalog.Settings["topology"]; !ok || s.Enabled || len(catalog.Settings) != 1 {
		t.Fatalf("settings = %+v", catalog.Settings)
	}

	// Without a usable cluster the status says why.
	if err := e.store.SetAssignments(e.users["bob"], nil); err != nil {
		t.Fatal(err)
	}
	catalog = decode[pluginCatalog](t, e.do(http.MethodGet, "/api/plugins", "bob", nil))
	if st := catalog.Plugins[0].Status; st.Available || st.Reason == "" {
		t.Fatalf("status without a cluster = %+v", st)
	}
}

func TestInvalidPluginRoutesPanic(t *testing.T) {
	e := newTestEnv(t, nil)
	for _, routes := range []func(plugin.Router){
		func(r plugin.Router) { r.Handle("GET", "/", plugin.AccessRead, nil) },
		func(r plugin.Router) { r.Handle("GET", "items", plugin.AccessRead, nil) },
		func(r plugin.Router) { r.Handle("TRACE", "/items", plugin.AccessRead, nil) },
		func(r plugin.Router) { r.Handle("GET", "/items", plugin.Access(42), nil) },
		func(r plugin.Router) { r.Handle("GET", "/items", plugin.AccessRead|plugin.DryRunnable, nil) },
	} {
		func() {
			defer func() {
				if recover() == nil {
					t.Error("an invalid plugin route was accepted")
				}
			}()
			routes(pluginRouter{s: e.srv, p: newFakePlugin("bad"), id: "bad"})
		}()
	}
}

// ctxPlugin is available exactly when Detect gets a live context, like a
// plugin whose Detect asks the cluster.
type ctxPlugin struct{ *fakePlugin }

func (p ctxPlugin) Detect(ctx context.Context, rt *kube.Runtime) plugin.Status {
	p.fakePlugin.Detect(ctx, rt)
	if err := ctx.Err(); err != nil {
		return plugin.Status{Reason: err.Error()}
	}
	return plugin.Status{Available: true}
}

// A capability probe slower than the Detect timeout must not leave Detect
// with an expired context, nor cache what a slow cluster made it report.
func TestDetectSurvivesASlowProbe(t *testing.T) {
	p := ctxPlugin{newFakePlugin("slow")}
	e := newTestEnv(t, nil, p)
	e.srv.detect.timeout = 50 * time.Millisecond
	finish := e.runtime().ProbeForTest()

	expect(t, e.do(http.MethodGet, "/api/plugins/slow/items", "bob", nil), http.StatusOK, "")
	expect(t, e.do(http.MethodGet, "/api/plugins/slow/items", "bob", nil), http.StatusOK, "")
	if n := p.detects.Load(); n != 2 {
		t.Fatalf("a status from an unfinished probe was cached (Detect ran %d times)", n)
	}
	finish(kube.Capabilities{})
	expect(t, e.do(http.MethodGet, "/api/plugins/slow/items", "bob", nil), http.StatusOK, "")
	expect(t, e.do(http.MethodGet, "/api/plugins/slow/items", "bob", nil), http.StatusOK, "")
	if n := p.detects.Load(); n != 3 {
		t.Fatalf("after the probe the status must be cached (Detect ran %d times)", n)
	}
}

// The router and the handler must read the same dryRun flag: a spelling
// the handler does not take for a dry run would make a real change that
// the caller meant as a preview.
func TestPluginDryRunFlagIsValidated(t *testing.T) {
	e := newTestEnv(t, nil, newFakePlugin("fake"))
	for _, spelling := range []string{"1", "True", "yes", "TRUE", "0"} {
		expect(t, e.do(http.MethodPost, "/api/plugins/fake/items/a?dryRun="+spelling, "alice", nil),
			http.StatusBadRequest, plugin.CodeBadRequest)
	}
	if items := decode[map[string]string](t, e.do(http.MethodGet, "/api/plugins/fake/items/stored", "bob", nil)); len(items) != 0 {
		t.Fatalf("a misspelt dry run made a change: %v", items)
	}
	expect(t, e.do(http.MethodPost, "/api/plugins/fake/items/a?dryRun=true", "bob", nil), http.StatusOK, "")
	expect(t, e.do(http.MethodPost, "/api/plugins/fake/items/a?dryRun=false", "alice", nil), http.StatusOK, "")
}

// A page of another site can navigate an operator's browser to a GET of a
// write or exec route, cookie included; such requests are refused.
func TestCrossSiteGetOfWriteRoutesIsRefused(t *testing.T) {
	e := newTestEnv(t, nil, newFakePlugin("fake"))
	e.srv.register("GET /api/plugins/fake/probe", accessWrite, nil, func(w http.ResponseWriter, _ *http.Request, _ *call) { writeOK(w) })
	get := func(path, site string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.AddCookie(&http.Cookie{Name: sessionCookie, Value: e.tokens["alice"]})
		if site != "" {
			req.Header.Set("Sec-Fetch-Site", site)
			req.Header.Set("Sec-Fetch-Mode", "navigate")
		}
		return e.serve(req)
	}
	expect(t, get("/api/plugins/fake/probe", "cross-site"), http.StatusForbidden, plugin.CodeForbidden)
	for _, site := range []string{"", "same-origin", "none"} {
		expect(t, get("/api/plugins/fake/probe", site), http.StatusOK, "")
	}
	// Read routes stay reachable by links from elsewhere.
	expect(t, get("/api/plugins/fake/items", "cross-site"), http.StatusOK, "")
}
