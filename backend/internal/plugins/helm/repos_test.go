package helm

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"

	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/repo"
	"sigs.k8s.io/yaml"
)

// chartRepo is an HTTP chart repository behind basic auth.
type chartRepo struct {
	*httptest.Server
	mu       sync.Mutex
	archives map[string][]byte          // file name → .tgz
	charts   map[string]*chart.Metadata // file name → its chart
}

const repoUser, repoPassword = "reader", "s3cret-pass"

func newChartRepo(t *testing.T, charts ...*chart.Chart) *chartRepo {
	t.Helper()
	cr := &chartRepo{archives: map[string][]byte{}, charts: map[string]*chart.Metadata{}}
	cr.Server = httptest.NewServer(http.HandlerFunc(cr.serve))
	t.Cleanup(cr.Close)
	cr.publish(t, charts...)
	return cr
}

// publish adds charts to the repository and its index.
func (cr *chartRepo) publish(t *testing.T, charts ...*chart.Chart) {
	t.Helper()
	cr.mu.Lock()
	defer cr.mu.Unlock()
	for _, ch := range charts {
		file := ch.Metadata.Name + "-" + ch.Metadata.Version + ".tgz"
		cr.archives[file] = chartArchive(t, ch)
		cr.charts[file] = ch.Metadata
	}
}

func (cr *chartRepo) serve(w http.ResponseWriter, r *http.Request) {
	if user, pass, _ := r.BasicAuth(); user != repoUser || pass != repoPassword {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	cr.mu.Lock()
	defer cr.mu.Unlock()
	file := strings.TrimPrefix(r.URL.Path, "/")
	if file != "index.yaml" {
		if data, ok := cr.archives[file]; ok {
			_, _ = w.Write(data)
			return
		}
		http.NotFound(w, r)
		return
	}
	idx := repo.NewIndexFile()
	for file, md := range cr.charts {
		if err := idx.MustAdd(md, file, cr.URL, "sha256:0"); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
	idx.SortEntries()
	out, err := yaml.Marshal(idx)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	_, _ = w.Write(out)
}

func TestRepositories(t *testing.T) {
	h := newHarness(t)
	cr := newChartRepo(t, testChart("demo", "0.1.0"), testChart("other", "1.0.0"))
	add := RepoInput{Name: "local", URL: cr.URL, Username: repoUser, Password: repoPassword}

	for _, tc := range []struct {
		name string
		in   RepoInput
	}{
		{"bad name", RepoInput{Name: "a/b", URL: cr.URL}},
		{"oci url", RepoInput{Name: "x", URL: "oci://ghcr.io/org"}},
		{"credentials in the url", RepoInput{Name: "x", URL: strings.Replace(cr.URL, "http://", "http://reader:s3cret-pass@", 1)}},
		{"half the credentials", RepoInput{Name: "x", URL: cr.URL, Username: repoUser}},
		{"wrong password", RepoInput{Name: "x", URL: cr.URL, Username: repoUser, Password: "wrong"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fails(t, h.do(http.MethodPost, "/repos", tc.in), http.StatusBadRequest, "bad_request")
		})
	}

	added := ok[Repo](t, h.do(http.MethodPost, "/repos", add))
	if added.Name != "local" || !added.Authenticated || added.ChartCount != 2 || added.Updated == "" {
		t.Fatalf("added %+v", added)
	}
	if h.audit.target != "repo/local" || strings.Contains(h.audit.detail, repoPassword) {
		t.Fatalf("repo audit = %+v", h.audit)
	}
	fails(t, h.do(http.MethodPost, "/repos", add), http.StatusConflict, "conflict")

	// Credentials stay on the server, readable by its account only.
	rec := h.do(http.MethodGet, "/repos", nil)
	if strings.Contains(rec.Body.String(), repoPassword) || strings.Contains(rec.Body.String(), repoUser) {
		t.Fatalf("GET /repos shows credentials: %s", rec.Body)
	}
	info, err := os.Stat(h.p.env.settings.RepositoryConfig)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("repositories.yaml: %v, mode %v", err, info.Mode())
	}

	// Search and the default values of a version.
	found := ok[[]SearchResult](t, h.do(http.MethodGet, "/charts/search?q=DEMO", nil))
	if len(found) != 1 || found[0].Repo != "local" || found[0].Version != "0.1.0" || len(found[0].Versions) != 1 {
		t.Fatalf("search demo = %+v", found)
	}
	if all := ok[[]SearchResult](t, h.do(http.MethodGet, "/charts/search?q=testing", nil)); len(all) != 2 {
		t.Fatalf("search by keyword = %+v", all)
	}
	values := ok[ChartValues](t, h.do(http.MethodGet, "/charts/values?repo=local&chart=demo&version=0.1.0", nil))
	if values.Chart.Version != "0.1.0" || !strings.HasPrefix(values.Values, "# The greeting.") {
		t.Fatalf("chart values = %+v", values)
	}
	fails(t, h.do(http.MethodGet, "/charts/values?repo=local&chart=demo&version=9.9.9", nil), http.StatusBadRequest, "bad_request")

	// Install from the repository (credentials come from repositories.yaml).
	res := ok[InstallResult](t, h.do(http.MethodPost, "/install", InstallRequest{
		Name: "web", Namespace: "shop", Chart: ChartRef{Repo: "local", Name: "demo", Version: "0.1.0"},
	}))
	if res.Release.ChartVersion != "0.1.0" {
		t.Fatalf("installed %+v", res.Release)
	}

	// A new version shows up after an update, and the release upgrades to it.
	cr.publish(t, testChart("demo", "0.2.0"))
	updated := ok[map[string][]RepoUpdate](t, h.do(http.MethodPost, "/repos/update", nil))
	if len(updated["repos"]) != 1 || !updated["repos"][0].OK {
		t.Fatalf("update = %+v", updated)
	}
	found = ok[[]SearchResult](t, h.do(http.MethodGet, "/charts/search?q=demo", nil))
	if found[0].Version != "0.2.0" || len(found[0].Versions) != 2 || found[0].Versions[1].Version != "0.1.0" {
		t.Fatalf("search after update = %+v", found)
	}
	up := ok[UpgradeResult](t, h.do(http.MethodPost, "/releases/shop/web/upgrade", UpgradeRequest{
		Chart: ChartRef{Repo: "local", Name: "demo", Version: "0.2.0"}, ReuseValues: true,
	}))
	if up.Release.ChartVersion != "0.2.0" || up.Release.Revision != 2 {
		t.Fatalf("upgraded %+v", up.Release)
	}

	// A failing repository is reported, not fatal.
	cr.Close()
	updated = ok[map[string][]RepoUpdate](t, h.do(http.MethodPost, "/repos/update", nil))
	if updated["repos"][0].OK || updated["repos"][0].Error == "" {
		t.Fatalf("update of a stopped repository = %+v", updated)
	}

	ok[map[string]bool](t, h.do(http.MethodDelete, "/repos/local", nil))
	if repos := ok[[]Repo](t, h.do(http.MethodGet, "/repos", nil)); len(repos) != 0 {
		t.Fatalf("repos after remove = %+v", repos)
	}
	if _, err := os.Stat(h.p.env.indexPath("local")); !os.IsNotExist(err) {
		t.Fatalf("the index of a removed repository is kept: %v", err)
	}
	fails(t, h.do(http.MethodDelete, "/repos/local", nil), http.StatusNotFound, "not_found")
	if found := ok[[]SearchResult](t, h.do(http.MethodGet, "/charts/search?q=demo", nil)); len(found) != 0 {
		t.Fatalf("search after remove = %+v", found)
	}
}

func TestSearchRanking(t *testing.T) {
	h := newHarness(t)
	cr := newChartRepo(t, testChart("podinfo-extra", "1.0.0"), testChart("app-podinfo", "1.0.0"), testChart("podinfo", "1.0.0"))
	ok[Repo](t, h.do(http.MethodPost, "/repos", RepoInput{Name: "r", URL: cr.URL, Username: repoUser, Password: repoPassword}))
	found := ok[[]SearchResult](t, h.do(http.MethodGet, "/charts/search?q=podinfo", nil))
	var got []string
	for _, f := range found {
		got = append(got, f.Name)
	}
	if strings.Join(got, ",") != "podinfo,podinfo-extra,app-podinfo" {
		t.Fatalf("ranking = %v", got)
	}
}
