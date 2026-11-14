package api

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"kubebench/internal/plugin"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// pluginsDir lays out one valid external plugin ("hello") next to broken
// ones, plus a file outside the plugins directory to escape to.
func pluginsDir(t *testing.T) (dir, outside string) {
	t.Helper()
	base := t.TempDir()
	dir = filepath.Join(base, "plugins")
	outside = filepath.Join(base, "secret.js")
	writeFile(t, outside, "stolen()")

	writeFile(t, filepath.Join(dir, "hello", "plugin.json"),
		`{"id":"hello","name":"Hello cluster","version":"1.0.0","description":"says hello","entry":"index.js"}`)
	writeFile(t, filepath.Join(dir, "hello", "index.js"), "KubeBench.registerPlugin({})")
	writeFile(t, filepath.Join(dir, "hello", "assets", "style.css"), "body{}")
	writeFile(t, filepath.Join(dir, "hello", "assets", "logo.svg"), "<svg/>")
	writeFile(t, filepath.Join(dir, "hello", "page.html"), "<script>alert(1)</script>")
	if err := os.Symlink(outside, filepath.Join(dir, "hello", "escape.js")); err != nil {
		t.Fatal(err)
	}

	writeFile(t, filepath.Join(dir, "Bad_Id", "plugin.json"), `{"id":"Bad_Id","entry":"index.js"}`)
	writeFile(t, filepath.Join(dir, "mismatch", "plugin.json"), `{"id":"other","entry":"index.js"}`)
	writeFile(t, filepath.Join(dir, "traversal", "plugin.json"), `{"id":"traversal","entry":"../secret.js"}`)
	writeFile(t, filepath.Join(dir, "noentry", "plugin.json"), `{"id":"noentry","entry":"missing.js"}`)
	writeFile(t, filepath.Join(dir, "broken", "plugin.json"), `{"id":`)
	writeFile(t, filepath.Join(dir, "fake", "plugin.json"), `{"id":"fake","entry":"index.js"}`) // a built-in id
	writeFile(t, filepath.Join(dir, "fake", "index.js"), "")
	return dir, outside
}

func TestExternalPluginDiscovery(t *testing.T) {
	dir, _ := pluginsDir(t)
	e := newTestEnv(t, map[string]string{"KUBEBENCH_PLUGINS_DIR": dir}, newFakePlugin("fake"))

	catalog := decode[pluginCatalog](t, e.do(http.MethodGet, "/api/plugins", "bob", nil))
	var external []pluginView
	for _, p := range catalog.Plugins {
		if p.External {
			external = append(external, p)
		}
	}
	want := pluginView{
		ID: "hello", Name: "Hello cluster", Version: "1.0.0", Description: "says hello", Category: "tools",
		External: true, Status: plugin.Status{Available: true}, Entry: "/plugins/hello/index.js",
	}
	if len(external) != 1 || external[0] != want {
		t.Fatalf("external plugins = %+v, want only %+v", external, want)
	}
	if info := decode[infoView](t, e.do(http.MethodGet, "/api/info", "bob", nil)); !info.Features.ExternalPlugins {
		t.Fatal("features.externalPlugins must be true when the directory exists")
	}

	expect(t, e.do(http.MethodPut, "/api/plugins/hello", "admin", map[string]bool{"enabled": true}), http.StatusOK, "")
	catalog = decode[pluginCatalog](t, e.do(http.MethodGet, "/api/plugins", "bob", nil))
	for _, p := range catalog.Plugins {
		if p.ID == "hello" && !p.Enabled {
			t.Fatal("hello should be enabled now")
		}
	}
}

func TestExternalPluginFiles(t *testing.T) {
	dir, _ := pluginsDir(t)
	e := newTestEnv(t, map[string]string{"KUBEBENCH_PLUGINS_DIR": dir})

	// External plugins stay off until an admin enables them.
	expect(t, e.do(http.MethodGet, "/plugins/hello/index.js", "bob", nil), http.StatusNotFound, plugin.CodePluginDisabled)
	expect(t, e.do(http.MethodPut, "/api/plugins/hello", "admin", map[string]bool{"enabled": true}), http.StatusOK, "")

	for _, tc := range []struct{ path, contentType, body string }{
		{"/plugins/hello/index.js", "text/javascript; charset=utf-8", "KubeBench.registerPlugin({})"},
		{"/plugins/hello/assets/style.css", "text/css; charset=utf-8", "body{}"},
		{"/plugins/hello/assets/logo.svg", "image/svg+xml", "<svg/>"},
		{"/plugins/hello/plugin.json", "application/json", ""},
	} {
		rec := e.do(http.MethodGet, tc.path, "bob", nil)
		expect(t, rec, http.StatusOK, "")
		if rec.Header().Get("Content-Type") != tc.contentType || rec.Header().Get("X-Content-Type-Options") != "nosniff" {
			t.Errorf("%s headers = %v", tc.path, rec.Header())
		}
		if tc.body != "" && rec.Body.String() != tc.body {
			t.Errorf("%s body = %q", tc.path, rec.Body.String())
		}
	}

	for _, path := range []string{
		"/plugins/hello/escape.js",               // symlink out of the plugin directory
		"/plugins/hello/%2e%2e/%2e%2e/secret.js", // encoded traversal
		"/plugins/hello/..%2f..%2fsecret.js",
		"/plugins/hello/assets",    // directories are never listed
		"/plugins/hello/page.html", // not a served file type
		"/plugins/hello/missing.js",
		"/plugins/traversal/plugin.json", // invalid manifest
		"/plugins/nope/index.js",
		"/plugins/Bad_Id/plugin.json",
	} {
		rec := e.do(http.MethodGet, path, "bob", nil)
		if rec.Code != http.StatusNotFound && rec.Code != http.StatusMovedPermanently {
			t.Errorf("%s: status %d, body %s", path, rec.Code, rec.Body.String())
		}
		if rec.Body.String() == "stolen()" {
			t.Errorf("%s escaped the plugin directory", path)
		}
	}
	expect(t, e.do(http.MethodGet, "/plugins/hello/index.js", "", nil), http.StatusUnauthorized, plugin.CodeUnauthenticated)
}

func TestExternalPluginsWithoutDirectory(t *testing.T) {
	e := newTestEnv(t, map[string]string{"KUBEBENCH_PLUGINS_DIR": filepath.Join(t.TempDir(), "absent")})
	catalog := decode[pluginCatalog](t, e.do(http.MethodGet, "/api/plugins", "bob", nil))
	if len(catalog.Plugins) != 0 {
		t.Fatalf("plugins = %+v", catalog.Plugins)
	}
	if info := decode[infoView](t, e.do(http.MethodGet, "/api/info", "bob", nil)); info.Features.ExternalPlugins {
		t.Fatal("features.externalPlugins must be false without the directory")
	}
}
