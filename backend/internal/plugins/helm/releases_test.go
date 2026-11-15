package helm

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"helm.sh/helm/v3/pkg/chart"
)

// upload stores ch as an uploaded archive and returns its id.
func (h *harness) upload(ch *chart.Chart) string {
	h.t.Helper()
	return ok[Upload](h.t, h.send(multipartRequest(h.t, "chart", "chart.tgz", chartArchive(h.t, ch)))).UploadID
}

func multipartRequest(t *testing.T, field, filename string, data []byte) *http.Request {
	t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	fw, err := mw.CreateFormFile(field, filename)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = fw.Write(data)
	_ = mw.Close()
	req := httptest.NewRequest(http.MethodPost, "/charts/upload", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	return req
}

func TestReleaseLifecycle(t *testing.T) {
	h := newHarness(t)
	v1 := h.upload(testChart("demo", "0.1.0"))

	install := InstallRequest{
		Name: "web", Namespace: "shop", CreateNamespace: true,
		Chart: ChartRef{UploadID: v1}, Values: "greeting: hi\n",
	}

	// A dry run (here asked for by the query, as the UI does) renders the
	// manifest and notes, stores nothing and is audited as a dry run.
	dry := ok[InstallResult](t, h.do(http.MethodPost, "/install?dryRun=true", install))
	contains(t, "dry-run manifest", dry.Manifest, `greeting: "hi"`)
	contains(t, "dry-run manifest", dry.Manifest, "helm.sh/hook: post-install")
	contains(t, "dry-run notes", dry.Notes, "Installed web saying hi.")
	if got := ok[[]Release](t, h.do(http.MethodGet, "/releases", nil)); len(got) != 0 {
		t.Fatalf("a dry run stored releases: %+v", got)
	}
	if h.audit.target != "release/shop/web" || !strings.Contains(h.audit.detail, `"chart":"upload `+v1+`"`) ||
		!strings.Contains(h.audit.detail, `"dryRun":true`) {
		t.Fatalf("install audit = %+v", h.audit)
	}

	res := ok[InstallResult](t, h.do(http.MethodPost, "/install", install))
	if res.Release.Revision != 1 || res.Release.Status != "deployed" || res.Release.Chart != "demo" {
		t.Fatalf("installed %+v", res.Release)
	}
	fails(t, h.do(http.MethodPost, "/install", install), http.StatusConflict, "conflict")

	list := ok[[]Release](t, h.do(http.MethodGet, "/releases", nil))
	if len(list) != 1 || list[0].Namespace != "shop" || list[0].Name != "web" || list[0].ChartVersion != "0.1.0" {
		t.Fatalf("list = %+v", list)
	}
	if got := ok[[]Release](t, h.do(http.MethodGet, "/releases?ns=other", nil)); len(got) != 0 {
		t.Fatalf("list in another namespace = %+v", got)
	}

	// Upgrade: a dry run shows both manifests for a diff and stores nothing.
	v2 := h.upload(testChart("demo", "0.2.0"))
	up := UpgradeRequest{Chart: ChartRef{UploadID: v2}, Values: "greeting: hey\n"}
	diff := ok[UpgradeResult](t, h.do(http.MethodPost, "/releases/shop/web/upgrade?dryRun=true", up))
	contains(t, "current manifest", diff.CurrentManifest, `greeting: "hi"`)
	contains(t, "upgraded manifest", diff.Manifest, `greeting: "hey"`)
	if diff.Release.ChartVersion != "0.2.0" || diff.Release.Revision != 2 {
		t.Fatalf("dry-run upgrade = %+v", diff.Release)
	}
	if detail := ok[ReleaseDetail](t, h.do(http.MethodGet, "/releases/shop/web", nil)); len(detail.History) != 1 {
		t.Fatalf("a dry-run upgrade stored a revision: %+v", detail.History)
	}
	ok[UpgradeResult](t, h.do(http.MethodPost, "/releases/shop/web/upgrade", up))

	// Values-only upgrade with reuseValues merges over revision 2's values.
	merged := ok[UpgradeResult](t, h.do(http.MethodPost, "/releases/shop/web/upgrade", UpgradeRequest{Values: "replicas: 3\n", ReuseValues: true}))
	contains(t, "merged manifest", merged.Manifest, `greeting: "hey"`)
	contains(t, "merged manifest", merged.Manifest, `replicas: "3"`)
	if merged.Release.ChartVersion != "0.2.0" {
		t.Fatalf("values-only upgrade changed the chart: %+v", merged.Release)
	}
	if h.audit.detail != `{"chart":"(current)","reuseValues":true}` {
		t.Fatalf("upgrade audit = %+v", h.audit)
	}

	detail := ok[ReleaseDetail](t, h.do(http.MethodGet, "/releases/shop/web", nil))
	if detail.Revision != 3 || len(detail.History) != 3 || detail.History[0].Revision != 3 || detail.History[2].Status != "superseded" {
		t.Fatalf("detail = %+v", detail)
	}
	if detail.ChartInfo.Name != "demo" || detail.ChartInfo.Version != "0.2.0" || detail.Notes != "Installed web saying hey." {
		t.Fatalf("chart info / notes = %+v / %q", detail.ChartInfo, detail.Notes)
	}
	if len(detail.Hooks) != 1 || detail.Hooks[0].Kind != "ConfigMap" || detail.Hooks[0].Events[0] != "post-install" {
		t.Fatalf("hooks = %+v", detail.Hooks)
	}
	if _, err := time.Parse(time.RFC3339, detail.Updated); err != nil {
		t.Fatalf("updated %q: %v", detail.Updated, err)
	}

	// Values per revision: user-supplied, or merged with the chart defaults.
	values := func(query string) string {
		return ok[map[string]string](t, h.do(http.MethodGet, "/releases/shop/web/values"+query, nil))["values"]
	}
	if got := values("?revision=1"); got != "greeting: hi\n" {
		t.Fatalf("user values of revision 1 = %q", got)
	}
	if got := values(""); got != "greeting: hey\nreplicas: 3\n" {
		t.Fatalf("user values of the latest revision = %q", got)
	}
	if got := values("?revision=1&all=true"); got != "greeting: hi\nreplicas: 1\n" {
		t.Fatalf("all values of revision 1 = %q", got)
	}
	manifest := ok[map[string]any](t, h.do(http.MethodGet, "/releases/shop/web/manifest?revision=1", nil))
	if manifest["revision"] != float64(1) || !strings.Contains(manifest["manifest"].(string), `greeting: "hi"`) {
		t.Fatalf("manifest of revision 1 = %v", manifest)
	}
	fails(t, h.do(http.MethodGet, "/releases/shop/web/manifest?revision=9", nil), http.StatusNotFound, "not_found")
	fails(t, h.do(http.MethodGet, "/releases/shop/web/values?revision=-1", nil), http.StatusBadRequest, "bad_request")

	// Rollback makes revision 4 from revision 1.
	rb := ok[map[string]Release](t, h.do(http.MethodPost, "/releases/shop/web/rollback", map[string]int{"revision": 1}))
	if rb["release"].Revision != 4 || rb["release"].ChartVersion != "0.1.0" {
		t.Fatalf("rollback = %+v", rb)
	}
	if got := values(""); got != "greeting: hi\n" {
		t.Fatalf("values after the rollback = %q", got)
	}
	if h.audit.detail != `{"revision":1}` {
		t.Fatalf("rollback audit = %+v", h.audit)
	}

	// Uninstall keeping the history: listed as uninstalled, can be rolled back.
	ok[map[string]any](t, h.do(http.MethodDelete, "/releases/shop/web?keepHistory=true", nil))
	list = ok[[]Release](t, h.do(http.MethodGet, "/releases", nil))
	if len(list) != 1 || list[0].Status != "uninstalled" {
		t.Fatalf("list after uninstall --keep-history = %+v", list)
	}
	ok[map[string]Release](t, h.do(http.MethodPost, "/releases/shop/web/rollback", map[string]int{"revision": 4}))

	// A plain uninstall removes the release and its history.
	ok[map[string]any](t, h.do(http.MethodDelete, "/releases/shop/web", nil))
	if got := ok[[]Release](t, h.do(http.MethodGet, "/releases", nil)); len(got) != 0 {
		t.Fatalf("list after uninstall = %+v", got)
	}
	fails(t, h.do(http.MethodGet, "/releases/shop/web", nil), http.StatusNotFound, "not_found")
	fails(t, h.do(http.MethodDelete, "/releases/shop/web", nil), http.StatusNotFound, "not_found")
}

func TestInstallValidation(t *testing.T) {
	h := newHarness(t)
	id := h.upload(testChart("demo", "0.1.0"))
	for _, tc := range []struct {
		name string
		in   InstallRequest
	}{
		{"bad name", InstallRequest{Name: "Web_1", Namespace: "shop", Chart: ChartRef{UploadID: id}}},
		{"bad namespace", InstallRequest{Name: "web", Namespace: "a/b", Chart: ChartRef{UploadID: id}}},
		{"no chart", InstallRequest{Name: "web", Namespace: "shop"}},
		{"two chart forms", InstallRequest{Name: "web", Namespace: "shop", Chart: ChartRef{UploadID: id, Ref: "oci://example.com/c"}}},
		{"local path", InstallRequest{Name: "web", Namespace: "shop", Chart: ChartRef{Repo: "/etc", Name: "passwd"}}},
		{"http ref", InstallRequest{Name: "web", Namespace: "shop", Chart: ChartRef{Ref: "https://example.com/c.tgz"}}},
		{"invalid values", InstallRequest{Name: "web", Namespace: "shop", Chart: ChartRef{UploadID: id}, Values: "a: [b"}},
		{"values that fail to render", InstallRequest{Name: "web", Namespace: "shop", Chart: ChartRef{UploadID: id}, Values: "greeting: null"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fails(t, h.do(http.MethodPost, "/install", tc.in), http.StatusBadRequest, "bad_request")
		})
	}
	fails(t, h.do(http.MethodPost, "/install", InstallRequest{Name: "web", Namespace: "shop", Chart: ChartRef{Repo: "nope", Name: "demo"}}),
		http.StatusNotFound, "not_found")
	fails(t, h.do(http.MethodPost, "/install", InstallRequest{Name: "web", Namespace: "shop", Chart: ChartRef{UploadID: strings.Repeat("a", 32)}}),
		http.StatusNotFound, "not_found")
	fails(t, h.do(http.MethodGet, "/releases/shop/Bad_Name", nil), http.StatusBadRequest, "bad_request")
	fails(t, h.do(http.MethodPost, "/releases/shop/missing/upgrade", UpgradeRequest{}), http.StatusNotFound, "not_found")
}

// A reset (values-only upgrade, no reuse, empty values) goes back to the
// chart's defaults even after an upgrade that reused values: reuse must
// not store the merged values as the chart's defaults.
func TestResetAfterReuseRestoresChartDefaults(t *testing.T) {
	h := newHarness(t)
	id := h.upload(testChart("demo", "0.1.0"))
	ok[InstallResult](t, h.do(http.MethodPost, "/install", InstallRequest{
		Name: "web", Namespace: "shop", Chart: ChartRef{UploadID: id}, Values: "greeting: custom\n",
	}))
	reused := ok[UpgradeResult](t, h.do(http.MethodPost, "/releases/shop/web/upgrade", UpgradeRequest{Values: "replicas: 3\n", ReuseValues: true}))
	contains(t, "reused manifest", reused.Manifest, `greeting: "custom"`)
	contains(t, "reused manifest", reused.Manifest, `replicas: "3"`)

	reset := ok[UpgradeResult](t, h.do(http.MethodPost, "/releases/shop/web/upgrade", UpgradeRequest{}))
	contains(t, "reset manifest", reset.Manifest, `greeting: "hello"`)
	contains(t, "reset manifest", reset.Manifest, `replicas: "1"`)
	all := ok[map[string]string](t, h.do(http.MethodGet, "/releases/shop/web/values?all=true", nil))["values"]
	if all != "greeting: hello\nreplicas: 1\n" {
		t.Fatalf("all values after the reset = %q", all)
	}
}
