package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"kubebench/internal/plugin"
	_ "kubebench/internal/plugins/all" // the plugins define their reasons on import
	"kubebench/internal/reason"
	"kubebench/internal/store"
)

// The frontend's translations of the reason codes (src/core/reasons) and
// the UI languages (every src/i18n/locales/<lang>.json).
var (
	reasonLocalesDir = filepath.Join("..", "..", "..", "src", "core", "reasons", "locales")
	uiLocalesDir     = filepath.Join("..", "..", "..", "src", "i18n", "locales")
)

// TestEveryReasonIsTranslated checks that every reason code the backend
// can emit (every reason.Define of the binary, plugins included) has a
// string in every UI language, with the same placeholders, and that the
// English string is the API's English fallback, so the two cannot drift.
func TestEveryReasonIsTranslated(t *testing.T) {
	catalog := reason.Catalog()
	if len(catalog) == 0 {
		t.Fatal("no reason codes are defined")
	}
	languages := localeLanguages(t, uiLocalesDir)
	if !slices.Contains(languages, "en") {
		t.Fatalf("UI languages %v lack en", languages)
	}
	for _, lang := range languages {
		texts := loadReasonLocale(t, filepath.Join(reasonLocalesDir, lang+".json"))
		for code, english := range catalog {
			text, ok := texts[code]
			switch {
			case !ok || strings.TrimSpace(text) == "":
				t.Errorf("%s.json: no string for reason %q (%q)", lang, code, english)
			case lang == "en" && text != english:
				t.Errorf("en.json: %q is %q, but the backend sends %q", code, text, english)
			case !samePlaceholders(text, english):
				t.Errorf("%s.json: %q uses %v, the backend sends %v", lang, code, reason.Placeholders(text), reason.Placeholders(english))
			}
		}
		for code := range texts {
			if _, ok := catalog[code]; !ok {
				t.Errorf("%s.json: %q is not a reason the backend defines", lang, code)
			}
		}
	}
}

// Refusals carry their reason code and parameters next to the English
// message: the router's role and read-only checks, and a plugin's own
// unavailable reason.
func TestErrorsCarryReasonCodes(t *testing.T) {
	fake := newFakePlugin("fake")
	e := newTestEnv(t, nil, fake)

	body := decode[plugin.ErrorBody](t, expectRec(t, e.do(http.MethodPost, "/api/deployments/scale?ns=default&name=web&replicas=1", "bob", nil),
		http.StatusForbidden, plugin.CodeForbiddenRole))
	if body.Reason != "role.cannot_change" || body.Params["cluster"] != "test" || body.Params["role"] != "viewer" ||
		body.Error != `your role on cluster "test" is viewer, which cannot change resources` {
		t.Fatalf("viewer refusal = %+v", body)
	}

	readOnly := true
	if err := e.store.UpdateCluster(e.cluster, store.ClusterUpdate{ReadOnly: &readOnly}); err != nil {
		t.Fatal(err)
	}
	body = decode[plugin.ErrorBody](t, expectRec(t, e.do(http.MethodPost, "/api/deployments/scale?ns=default&name=web&replicas=1", "alice", nil),
		http.StatusForbidden, plugin.CodeClusterReadOnly))
	if body.Reason != "cluster.read_only" || body.Params["cluster"] != "test" {
		t.Fatalf("read-only refusal = %+v", body)
	}

	unavailable := plugin.Unavailable(reason.FieldRequired, reason.Params{"field": "x"})
	fake.status.Store(&unavailable)
	e.srv.detect.now = func() time.Time { return e.clock.Now().Add(time.Hour) } // past the Detect cache
	body = decode[plugin.ErrorBody](t, expectRec(t, e.do(http.MethodGet, "/api/plugins/fake/items", "alice", nil),
		http.StatusServiceUnavailable, plugin.CodePluginUnavailable))
	if body.Reason != "field.required" || body.Params["field"] != "x" || body.Error != "x is required" {
		t.Fatalf("plugin refusal = %+v", body)
	}

	// Errors of the cluster carry no reason: the UI shows their message.
	body = decode[plugin.ErrorBody](t, expectRec(t, e.do(http.MethodGet, "/api/resource/get?key=configmaps&ns=default&name=missing", "alice", nil),
		http.StatusNotFound, plugin.CodeKubernetes))
	if body.Reason != "" || body.Params != nil {
		t.Fatalf("Kubernetes error = %+v", body)
	}
}

// expectRec is expect returning the recorder, for decoding the body.
func expectRec(t *testing.T, rec *httptest.ResponseRecorder, status int, code string) *httptest.ResponseRecorder {
	t.Helper()
	expect(t, rec, status, code)
	return rec
}

func localeLanguages(t *testing.T, dir string) []string {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no languages in %s (err %v)", dir, err)
	}
	var langs []string
	for _, f := range files {
		langs = append(langs, strings.TrimSuffix(filepath.Base(f), ".json"))
	}
	return langs
}

// loadReasonLocale reads a nested i18next bundle as code → text.
func loadReasonLocale(t *testing.T, path string) map[string]string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	var nested map[string]any
	if err := json.Unmarshal(data, &nested); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	flat := map[string]string{}
	var walk func(prefix string, node map[string]any)
	walk = func(prefix string, node map[string]any) {
		for key, value := range node {
			switch v := value.(type) {
			case map[string]any:
				walk(prefix+key+".", v)
			case string:
				flat[prefix+key] = v
			default:
				t.Errorf("%s: %s%s is neither a string nor an object", path, prefix, key)
			}
		}
	}
	walk("", nested)
	return flat
}

func samePlaceholders(a, b string) bool {
	pa, pb := reason.Placeholders(a), reason.Placeholders(b)
	slices.Sort(pa)
	slices.Sort(pb)
	return slices.Equal(slices.Compact(pa), slices.Compact(pb))
}
