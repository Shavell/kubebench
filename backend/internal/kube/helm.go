package kube

import (
	"sort"

	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/release"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/yaml"
)

// HelmService drives Helm through its Go SDK against one cluster.
type HelmService struct {
	config *rest.Config
}

func newHelmService(config *rest.Config) *HelmService {
	return &HelmService{config: config}
}

type HelmRevision struct {
	Revision    int    `json:"revision"`
	Status      string `json:"status"`
	Updated     string `json:"updated"`
	Description string `json:"description"`
}

type HelmRelease struct {
	Name         string         `json:"name"`
	Namespace    string         `json:"namespace"`
	Chart        string         `json:"chart"`
	ChartVersion string         `json:"chartVersion"`
	AppVersion   string         `json:"appVersion"`
	Revision     int            `json:"revision"`
	Status       string         `json:"status"`
	Updated      string         `json:"updated"`
	History      []HelmRevision `json:"history"`
}

type HelmRepo struct {
	Name       string `json:"name"`
	URL        string `json:"url"`
	ChartCount int    `json:"chartCount"`
}

func (h *HelmService) actionConfig(namespace string) (*action.Configuration, error) {
	cfg := new(action.Configuration)
	getter := &restGetter{config: h.config, namespace: namespace}
	if err := cfg.Init(getter, namespace, "secret", func(string, ...any) {}); err != nil {
		return nil, err
	}
	return cfg, nil
}

func mapHelmStatus(s release.Status) string {
	if s.IsPending() {
		return "pending-upgrade"
	}
	return string(s)
}

func mapRevision(r *release.Release) HelmRevision {
	updated := ""
	if !r.Info.LastDeployed.IsZero() {
		updated = r.Info.LastDeployed.Format("2006-01-02 15:04")
	}
	return HelmRevision{
		Revision:    r.Version,
		Status:      mapHelmStatus(r.Info.Status),
		Updated:     updated,
		Description: r.Info.Description,
	}
}

func (h *HelmService) ListReleases() ([]HelmRelease, error) {
	cfg, err := h.actionConfig("")
	if err != nil {
		return nil, err
	}
	list := action.NewList(cfg)
	list.All = true
	list.AllNamespaces = true
	list.SetStateMask()
	releases, err := list.Run()
	if err != nil {
		return nil, err
	}

	out := []HelmRelease{}
	for _, r := range releases {
		history := h.history(r.Name, r.Namespace)
		item := HelmRelease{
			Name:         r.Name,
			Namespace:    r.Namespace,
			Chart:        r.Chart.Metadata.Name,
			ChartVersion: r.Chart.Metadata.Version,
			AppVersion:   r.Chart.Metadata.AppVersion,
			Revision:     r.Version,
			Status:       mapHelmStatus(r.Info.Status),
			Updated:      mapRevision(r).Updated,
			History:      history,
		}
		out = append(out, item)
	}
	return out, nil
}

func (h *HelmService) history(name, namespace string) []HelmRevision {
	cfg, err := h.actionConfig(namespace)
	if err != nil {
		return nil
	}
	hist := action.NewHistory(cfg)
	hist.Max = 10
	releases, err := hist.Run(name)
	if err != nil {
		return nil
	}
	out := []HelmRevision{}
	for _, r := range releases {
		out = append(out, mapRevision(r))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Revision > out[j].Revision })
	return out
}

// ListRepositories: repositories are a client-side concept; when running
// in-cluster there is no repo config, so derive "repos" from the charts
// seen in releases instead of pretending.
func (h *HelmService) ListRepositories() ([]HelmRepo, error) {
	releases, err := h.ListReleases()
	if err != nil {
		return nil, err
	}
	seen := map[string]int{}
	for _, r := range releases {
		seen[r.Chart]++
	}
	repos := []HelmRepo{}
	for chart, count := range seen {
		repos = append(repos, HelmRepo{Name: chart, URL: "(installed chart)", ChartCount: count})
	}
	sort.Slice(repos, func(i, j int) bool { return repos[i].Name < repos[j].Name })
	return repos, nil
}

func (h *HelmService) GetValuesYAML(name, namespace string) (string, error) {
	cfg, err := h.actionConfig(namespace)
	if err != nil {
		return "", err
	}
	get := action.NewGetValues(cfg)
	values, err := get.Run(name)
	if err != nil {
		return "", err
	}
	if len(values) == 0 {
		return "# no user-supplied values\n", nil
	}
	out, err := yaml.Marshal(values)
	return string(out), err
}

func (h *HelmService) Rollback(name, namespace string, revision int) error {
	cfg, err := h.actionConfig(namespace)
	if err != nil {
		return err
	}
	rollback := action.NewRollback(cfg)
	rollback.Version = revision
	rollback.Wait = false
	return rollback.Run(name)
}

func (h *HelmService) Uninstall(name, namespace string) error {
	cfg, err := h.actionConfig(namespace)
	if err != nil {
		return err
	}
	uninstall := action.NewUninstall(cfg)
	uninstall.Wait = false
	_, err = uninstall.Run(name)
	return err
}

var _ genericclioptions.RESTClientGetter = (*restGetter)(nil)
