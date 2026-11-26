package helm

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
	k8stesting "k8s.io/client-go/testing"

	"kubebench/internal/kube"
	"kubebench/internal/plugin"
	"kubebench/internal/store"
)

// Every route has the access level §7.4 names: mutations are write, dry
// runs included (template lookups use KubeBench's credential); release
// values need exec; OCI fetches need write; repositories are added and
// removed by admins.
func TestRouteAccess(t *testing.T) {
	h := newHarness(t)
	want := map[string]plugin.Access{
		"GET /releases":                       plugin.AccessRead,
		"GET /releases/{ns}/{name}":           plugin.AccessRead,
		"GET /releases/{ns}/{name}/values":    plugin.AccessExec,
		"GET /releases/{ns}/{name}/manifest":  plugin.AccessRead,
		"POST /releases/{ns}/{name}/rollback": plugin.AccessWrite,
		"DELETE /releases/{ns}/{name}":        plugin.AccessWrite,
		"POST /releases/{ns}/{name}/upgrade":  plugin.AccessWrite,
		"POST /install":                       plugin.AccessWrite,
		"GET /repos":                          plugin.AccessRead,
		"POST /repos":                         plugin.AccessAdmin,
		"DELETE /repos/{name}":                plugin.AccessAdmin,
		"POST /repos/update":                  plugin.AccessWrite,
		"GET /charts/search":                  plugin.AccessRead,
		"GET /charts/values":                  plugin.AccessRead,
		"POST /charts/oci/values":             plugin.AccessWrite,
		"POST /charts/upload":                 plugin.AccessWrite,
	}
	if len(h.access) != len(want) {
		var got []string
		for k := range h.access {
			got = append(got, k)
		}
		sort.Strings(got)
		t.Fatalf("routes = %v", got)
	}
	for route, access := range want {
		if h.access[route] != access {
			t.Errorf("%s: access %v, want %v", route, h.access[route], access)
		}
	}
}

func TestNotStarted(t *testing.T) {
	h := newHarness(t)
	h.p.env = nil
	fails(t, h.do(http.MethodGet, "/repos", nil), http.StatusServiceUnavailable, plugin.CodeInternal)
}

func TestStartPreparesTheDataDirectory(t *testing.T) {
	dir := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	p := New()
	if err := plugin.StartAll(ctx, dir, []plugin.Plugin{p}); err != nil {
		t.Fatal(err)
	}
	for _, sub := range []string{"helm/cache", "helm/registry", "helm/uploads"} {
		if info, err := os.Stat(dir + "/" + sub); err != nil || info.Mode().Perm() != 0o700 {
			t.Fatalf("%s: %v %v", sub, err, info)
		}
	}
	if p.env.settings.RepositoryConfig != dir+"/helm/repositories.yaml" {
		t.Fatalf("repository config = %s", p.env.settings.RepositoryConfig)
	}
}

func TestDetect(t *testing.T) {
	cs := fake.NewClientset()
	rt := kube.NewRuntimeForTest(store.Cluster{ID: 1}, cs, nil)
	p := New()
	if st := p.Detect(context.Background(), rt); !st.Available {
		t.Fatalf("Detect = %+v", st)
	}
	cs.PrependReactor("list", "secrets", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, apierrors.NewForbidden(schema.GroupResource{Resource: "secrets"}, "", errors.New("RBAC"))
	})
	if st := p.Detect(context.Background(), rt); st.Available || !strings.Contains(st.Reason, "release Secrets") {
		t.Fatalf("Detect without access = %+v", st)
	}
}

func TestUploads(t *testing.T) {
	h := newHarness(t)
	id := h.upload(testChart("demo", "0.1.0"))
	if !uploadIDPattern.MatchString(id) || h.audit.target != "chart/demo" {
		t.Fatalf("upload id %q, audit %+v", id, h.audit)
	}
	values := ok[ChartValues](t, h.do(http.MethodGet, "/charts/values?uploadId="+id, nil))
	if values.Chart.Name != "demo" || !strings.Contains(values.Values, "greeting: hello") {
		t.Fatalf("values of the upload = %+v", values)
	}

	fails(t, h.send(multipartRequest(t, "chart", "x.tgz", []byte("not a tarball"))), http.StatusBadRequest, plugin.CodeBadRequest)
	fails(t, h.send(multipartRequest(t, "file", "x.tgz", chartArchive(t, testChart("demo", "0.1.0")))), http.StatusBadRequest, plugin.CodeBadRequest)
	big := bytes.Repeat([]byte{0}, maxUploadBytes+1)
	msg := fails(t, h.send(multipartRequest(t, "chart", "big.tgz", big)), http.StatusBadRequest, plugin.CodeBadRequest)
	if !strings.Contains(msg, "20 MiB") {
		t.Fatalf("too large: %s", msg)
	}
	plain := httptest.NewRequest(http.MethodPost, "/charts/upload", strings.NewReader("{}"))
	fails(t, h.send(plain), http.StatusBadRequest, plugin.CodeBadRequest)

	// Uploads expire after a day: they can no longer be used and are swept.
	old := time.Now().Add(-uploadTTL - time.Minute)
	if err := os.Chtimes(h.uploadPath(id), old, old); err != nil {
		t.Fatal(err)
	}
	fails(t, h.do(http.MethodGet, "/charts/values?uploadId="+id, nil), http.StatusNotFound, plugin.CodeNotFound)
	fresh := h.upload(testChart("demo", "0.2.0"))
	h.p.env.removeExpiredUploads(time.Now())
	if _, err := os.Stat(h.uploadPath(id)); !os.IsNotExist(err) {
		t.Fatalf("expired upload kept: %v", err)
	}
	if _, err := os.Stat(h.uploadPath(fresh)); err != nil {
		t.Fatalf("fresh upload swept: %v", err)
	}
}

// Helm's Kubernetes clients only reach the API server: a redirect to
// another host fails before the credential is sent there.
func TestRESTGetterPinsTheAPIServer(t *testing.T) {
	var leaked bool
	elsewhere := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		leaked = leaked || r.Header.Get("Authorization") != ""
		w.WriteHeader(http.StatusOK)
	}))
	defer elsewhere.Close()
	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/same" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"kind":"Status"}`))
			return
		}
		http.Redirect(w, r, elsewhere.URL+"/steal", http.StatusFound)
	}))
	defer apiServer.Close()

	getter := &restGetter{config: &rest.Config{Host: apiServer.URL, BearerToken: "secret-token"}}
	cfg, err := getter.ToRESTConfig()
	if err != nil {
		t.Fatal(err)
	}
	client, err := rest.HTTPClientFor(cfg)
	if err != nil {
		t.Fatal(err)
	}
	res, err := client.Get(apiServer.URL + "/same")
	if err != nil {
		t.Fatalf("request to the API server: %v", err)
	}
	_ = res.Body.Close()
	if _, err := client.Get(apiServer.URL + "/redirect"); err == nil || !strings.Contains(err.Error(), "refusing a request") {
		t.Fatalf("redirect elsewhere: %v", err)
	}
	if leaked {
		t.Fatal("the credential reached another host")
	}
}

func TestChartRefValidation(t *testing.T) {
	for _, tc := range []struct {
		ref ChartRef
		ok  bool
	}{
		{ChartRef{Repo: "bitnami", Name: "nginx", Version: "^15.0"}, true},
		{ChartRef{Ref: "oci://ghcr.io/stefanprodan/charts/podinfo", Version: "6.5.0"}, true},
		{ChartRef{UploadID: strings.Repeat("0", 32)}, true},
		{ChartRef{Repo: "..", Name: "x"}, false},
		{ChartRef{Repo: "r", Name: "../etc"}, false},
		{ChartRef{Name: "nginx"}, false},
		{ChartRef{Ref: "/tmp/chart.tgz"}, false},
		{ChartRef{Ref: "oci://ghcr.io/x/y@sha256:abc"}, false},
		{ChartRef{UploadID: "../../etc/passwd"}, false},
		{ChartRef{UploadID: strings.Repeat("0", 32), Version: "1.0.0"}, false},
		{ChartRef{Repo: "r", Name: "x", Version: "1.0; rm -rf"}, false},
		{ChartRef{}, false},
	} {
		if err := tc.ref.validate(); (err == nil) != tc.ok {
			t.Errorf("%+v: %v", tc.ref, err)
		}
	}
}

// The registry client never downgrades to plain HTTP: Helm's default
// transport would retry over HTTP when a host answers HTTPS with HTTP,
// and then stay on HTTP for every registry (the client is shared).
func TestRegistryClientStaysOnHTTPS(t *testing.T) {
	h := newHarness(t)
	var reached bool
	plain := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reached = true
		w.WriteHeader(http.StatusNotFound)
	}))
	defer plain.Close()
	ref := "oci://" + strings.TrimPrefix(plain.URL, "http://") + "/charts/demo"
	for range 2 {
		msg := fails(t, h.do(http.MethodPost, "/charts/oci/values", map[string]string{"ref": ref, "version": "1.0.0"}), http.StatusBadRequest, plugin.CodeBadRequest)
		if !strings.Contains(msg, "https://") {
			t.Fatalf("download error %q: want an HTTPS attempt", msg)
		}
	}
	if reached {
		t.Fatal("the registry client fell back to plain HTTP")
	}
	// The read-level route takes no OCI reference: it would let a viewer
	// make the server connect to any host.
	fails(t, h.do(http.MethodGet, "/charts/values?ref="+ref+"&version=1.0.0", nil), http.StatusBadRequest, plugin.CodeBadRequest)
}
