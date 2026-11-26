package helm

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chartutil"
	kubefake "helm.sh/helm/v3/pkg/kube/fake"
	"helm.sh/helm/v3/pkg/storage"
	"helm.sh/helm/v3/pkg/storage/driver"
	"k8s.io/client-go/kubernetes/fake"

	"kubebench/internal/kube"
	"kubebench/internal/plugin"
	"kubebench/internal/store"
)

// harness serves the plugin's routes like the API server does, minus its
// gates (tested in internal/api), with releases kept in memory and a fake
// Kubernetes client that creates nothing.
type harness struct {
	t      *testing.T
	p      *Plugin
	mux    *http.ServeMux
	access map[string]plugin.Access // "METHOD /path" → access
	rt     *kube.Runtime
	// audit is the last refinement a handler made of its audit row.
	audit struct{ target, detail string }
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	env, err := newEnvironment(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	mem := driver.NewMemory()
	h := &harness{
		t:      t,
		p:      &Plugin{env: env},
		mux:    http.NewServeMux(),
		access: map[string]plugin.Access{},
		rt:     kube.NewRuntimeForTest(store.Cluster{ID: 1, Name: "test"}, fake.NewClientset(), nil),
	}
	h.p.actions = func(_ *kube.Runtime, namespace string) (*action.Configuration, error) {
		mem.SetNamespace(namespace)
		return &action.Configuration{
			Releases:       storage.Init(mem),
			KubeClient:     &kubefake.PrintingKubeClient{Out: io.Discard},
			Capabilities:   chartutil.DefaultCapabilities,
			RegistryClient: env.registry,
			Log:            func(string, ...any) {},
		}, nil
	}
	h.p.Routes(h)
	return h
}

// Handle implements plugin.Router.
// uploadPath is the archive file of upload id.
func (h *harness) uploadPath(id string) string {
	h.t.Helper()
	path, err := h.p.env.uploadPath(id)
	if err != nil {
		h.t.Fatal(err)
	}
	return path
}

func (h *harness) Handle(method, path string, access plugin.Access, fn plugin.HandlerFunc) {
	pattern := method + " " + path
	h.access[pattern] = access
	h.mux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
		fn(w, r, &plugin.Context{
			Runtime:  h.rt,
			SetAudit: func(target, detail string) { h.audit.target, h.audit.detail = target, detail },
		})
	})
}

// do sends a request with a JSON body (nil for none).
func (h *harness) do(method, target string, body any) *httptest.ResponseRecorder {
	h.t.Helper()
	var r io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			h.t.Fatal(err)
		}
		r = bytes.NewReader(b)
	}
	return h.send(httptest.NewRequest(method, target, r))
}

func (h *harness) send(req *http.Request) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.mux.ServeHTTP(rec, req)
	return rec
}

// ok decodes a 200 answer into T.
func ok[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	var v T
	if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil {
		t.Fatalf("decode %s: %v", rec.Body, err)
	}
	return v
}

// fails checks an error answer's status and code.
func fails(t *testing.T, rec *httptest.ResponseRecorder, status int, code string) string {
	t.Helper()
	var body plugin.ErrorBody
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if rec.Code != status || body.Code != code {
		t.Fatalf("got %d %q (%s), want %d %q", rec.Code, body.Code, body.Error, status, code)
	}
	return body.Error
}

// testChart is a chart rendering a ConfigMap from .Values.greeting, a hook
// and notes.
func testChart(name, version string) *chart.Chart {
	return &chart.Chart{
		Metadata: &chart.Metadata{
			APIVersion: chart.APIVersionV2, Name: name, Version: version, AppVersion: "1.0." + version[len(version)-1:],
			Description: "A " + name + " chart for tests", Keywords: []string{"testing"},
		},
		Values: map[string]any{"greeting": "hello", "replicas": 1},
		Raw:    []*chart.File{{Name: "values.yaml", Data: []byte("# The greeting.\ngreeting: hello\nreplicas: 1\n")}},
		Templates: []*chart.File{
			{Name: "templates/configmap.yaml", Data: []byte(
				"apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: {{ .Release.Name }}\ndata:\n  greeting: {{ required \"greeting is required\" .Values.greeting | quote }}\n  replicas: {{ .Values.replicas | quote }}\n")},
			{Name: "templates/hook.yaml", Data: []byte(
				"apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: {{ .Release.Name }}-hook\n  annotations:\n    helm.sh/hook: post-install\ndata: {}\n")},
			{Name: "templates/NOTES.txt", Data: []byte("Installed {{ .Release.Name }} saying {{ .Values.greeting }}.")},
		},
	}
}

// chartArchive is ch packaged as a .tgz.
func chartArchive(t *testing.T, ch *chart.Chart) []byte {
	t.Helper()
	path, err := chartutil.Save(ch, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func contains(t *testing.T, what, text, want string) {
	t.Helper()
	if !strings.Contains(text, want) {
		t.Fatalf("%s does not contain %q:\n%s", what, want, text)
	}
}
