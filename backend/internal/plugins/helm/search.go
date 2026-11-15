package helm

import (
	"sort"
	"strings"

	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/repo"
)

// maxSearchResults bounds GET /charts/search; a narrower query finds the rest.
const maxSearchResults = 100

// ChartVersion is one version of a chart in a repository index.
type ChartVersion struct {
	Version    string `json:"version"`
	AppVersion string `json:"appVersion"`
	Created    string `json:"created"`
	Deprecated bool   `json:"deprecated"`
}

// SearchResult is a chart of an added repository, described by its latest
// version, with every version for the version picker (newest first).
type SearchResult struct {
	Repo        string         `json:"repo"`
	Name        string         `json:"name"`
	Version     string         `json:"version"`
	AppVersion  string         `json:"appVersion"`
	Description string         `json:"description"`
	Icon        string         `json:"icon"`
	Deprecated  bool           `json:"deprecated"`
	Versions    []ChartVersion `json:"versions"`
}

// ChartValues is GET /charts/values: a chart's metadata and its default
// values.yaml, comments included.
type ChartValues struct {
	Chart  ChartInfo `json:"chart"`
	Values string    `json:"values"`
}

// searchCharts finds charts of the cached repository indexes whose name,
// description or keywords contain q (case-insensitive; "" lists all).
// Exact name matches come first, then name prefixes, then the rest by name.
func (e *environment) searchCharts(q string) ([]SearchResult, error) {
	f, err := e.loadRepoFile()
	if err != nil {
		return nil, err
	}
	q = strings.ToLower(strings.TrimSpace(q))
	out := []SearchResult{}
	for _, entry := range f.Repositories {
		idx, _, err := e.indexes.load(e.indexPath(entry.Name))
		if err != nil {
			continue // listed with its error by GET /repos
		}
		for name, versions := range idx.Entries {
			if len(versions) == 0 || !matches(name, versions[0], q) {
				continue
			}
			out = append(out, searchResult(entry.Name, name, versions))
		}
	}
	sort.Slice(out, func(i, j int) bool {
		ri, rj := rank(out[i].Name, q), rank(out[j].Name, q)
		if ri != rj {
			return ri < rj
		}
		if out[i].Name != out[j].Name {
			return out[i].Name < out[j].Name
		}
		return out[i].Repo < out[j].Repo
	})
	if len(out) > maxSearchResults {
		out = out[:maxSearchResults]
	}
	return out, nil
}

func matches(name string, latest *repo.ChartVersion, q string) bool {
	if q == "" || strings.Contains(strings.ToLower(name), q) {
		return true
	}
	if strings.Contains(strings.ToLower(latest.Description), q) {
		return true
	}
	for _, k := range latest.Keywords {
		if strings.Contains(strings.ToLower(k), q) {
			return true
		}
	}
	return false
}

func rank(name, q string) int {
	name = strings.ToLower(name)
	switch {
	case q == "" || name == q:
		return 0
	case strings.HasPrefix(name, q):
		return 1
	}
	return 2
}

// searchResult describes a chart by its versions, which LoadIndexFile
// sorted newest first.
func searchResult(repoName, name string, versions repo.ChartVersions) SearchResult {
	latest := versions[0]
	r := SearchResult{
		Repo: repoName, Name: name, Version: latest.Version, AppVersion: latest.AppVersion,
		Description: latest.Description, Icon: latest.Icon, Deprecated: latest.Deprecated,
		Versions: make([]ChartVersion, 0, len(versions)),
	}
	for _, v := range versions {
		r.Versions = append(r.Versions, ChartVersion{
			Version: v.Version, AppVersion: v.AppVersion, Created: timestamp(v.Created), Deprecated: v.Deprecated,
		})
	}
	return r
}

// chartValues is the metadata and default values of the chart c names.
func (e *environment) chartValues(c ChartRef) (*ChartValues, error) {
	ch, err := e.loadChart(c)
	if err != nil {
		return nil, err
	}
	return &ChartValues{Chart: chartInfo(ch), Values: defaultValues(ch)}, nil
}

// defaultValues is the chart's values.yaml as written, comments included.
func defaultValues(ch *chart.Chart) string {
	for _, f := range ch.Raw {
		if f.Name == "values.yaml" {
			return string(f.Data)
		}
	}
	return ""
}
