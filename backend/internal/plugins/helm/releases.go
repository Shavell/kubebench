package helm

import (
	"sort"
	"time"

	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/release"
	"helm.sh/helm/v3/pkg/storage/driver"
	"sigs.k8s.io/yaml"
)

// maxHistory is how many revisions upgrades and rollbacks keep, the helm
// CLI's default (--history-max 10).
const maxHistory = 10

// Release is one release as the list shows it: its latest revision.
type Release struct {
	Name         string `json:"name"`
	Namespace    string `json:"namespace"`
	Chart        string `json:"chart"`
	ChartVersion string `json:"chartVersion"`
	AppVersion   string `json:"appVersion"`
	Revision     int    `json:"revision"`
	// Status is Helm's: deployed, failed, pending-install, pending-upgrade,
	// pending-rollback, superseded, uninstalling, uninstalled or unknown.
	Status      string `json:"status"`
	Updated     string `json:"updated"` // RFC 3339; "" when never deployed
	Description string `json:"description"`
}

// Revision is one entry of a release's history.
type Revision struct {
	Revision     int    `json:"revision"`
	Status       string `json:"status"`
	Updated      string `json:"updated"`
	Description  string `json:"description"`
	Chart        string `json:"chart"`
	ChartVersion string `json:"chartVersion"`
	AppVersion   string `json:"appVersion"`
}

// ChartInfo is the metadata of a chart (Chart.yaml).
type ChartInfo struct {
	Name        string   `json:"name"`
	Version     string   `json:"version"`
	AppVersion  string   `json:"appVersion"`
	Description string   `json:"description"`
	Home        string   `json:"home"`
	Icon        string   `json:"icon"`
	Sources     []string `json:"sources"`
	Keywords    []string `json:"keywords"`
	KubeVersion string   `json:"kubeVersion"`
	Deprecated  bool     `json:"deprecated"`
}

// Hook is a chart hook of a release and its last run.
type Hook struct {
	Name   string   `json:"name"`
	Kind   string   `json:"kind"`
	Events []string `json:"events"`
	Weight int      `json:"weight"`
	// Phase of the last run: Unknown, Running, Succeeded or Failed; "" when
	// the hook never ran.
	Phase       string `json:"phase"`
	StartedAt   string `json:"startedAt"`
	CompletedAt string `json:"completedAt"`
}

// ReleaseDetail is GET /releases/{ns}/{name}: the latest revision with its
// chart, notes, hooks and history (newest first).
type ReleaseDetail struct {
	Release
	FirstDeployed string     `json:"firstDeployed"`
	Notes         string     `json:"notes"`
	ChartInfo     ChartInfo  `json:"chartInfo"`
	Hooks         []Hook     `json:"hooks"`
	History       []Revision `json:"history"`
}

func timestamp(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

func chartMeta(c *chart.Chart) *chart.Metadata {
	if c == nil || c.Metadata == nil {
		return &chart.Metadata{}
	}
	return c.Metadata
}

func summarize(r *release.Release) Release {
	meta := chartMeta(r.Chart)
	out := Release{
		Name: r.Name, Namespace: r.Namespace, Revision: r.Version,
		Chart: meta.Name, ChartVersion: meta.Version, AppVersion: meta.AppVersion,
	}
	if r.Info != nil {
		out.Status = r.Info.Status.String()
		out.Updated = timestamp(r.Info.LastDeployed.Time)
		out.Description = r.Info.Description
	}
	return out
}

func revisionOf(r *release.Release) Revision {
	s := summarize(r)
	return Revision{
		Revision: s.Revision, Status: s.Status, Updated: s.Updated, Description: s.Description,
		Chart: s.Chart, ChartVersion: s.ChartVersion, AppVersion: s.AppVersion,
	}
}

func chartInfo(c *chart.Chart) ChartInfo {
	m := chartMeta(c)
	return ChartInfo{
		Name: m.Name, Version: m.Version, AppVersion: m.AppVersion, Description: m.Description,
		Home: m.Home, Icon: m.Icon, Sources: nonNil(m.Sources), Keywords: nonNil(m.Keywords),
		KubeVersion: m.KubeVersion, Deprecated: m.Deprecated,
	}
}

func hooksOf(r *release.Release) []Hook {
	out := []Hook{}
	for _, h := range r.Hooks {
		hook := Hook{Name: h.Name, Kind: h.Kind, Weight: h.Weight, Events: []string{}}
		for _, e := range h.Events {
			hook.Events = append(hook.Events, e.String())
		}
		if h.LastRun.Phase != "" {
			hook.Phase = h.LastRun.Phase.String()
			hook.StartedAt = timestamp(h.LastRun.StartedAt.Time)
			hook.CompletedAt = timestamp(h.LastRun.CompletedAt.Time)
		}
		out = append(out, hook)
	}
	return out
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

// listReleases lists the latest revision of every release in cfg's
// namespace (all namespaces for a configuration made for ""), uninstalled
// ones kept with --keep-history included.
func listReleases(cfg *action.Configuration, allNamespaces bool) ([]Release, error) {
	list := action.NewList(cfg)
	list.All = true
	list.AllNamespaces = allNamespaces
	list.SetStateMask()
	releases, err := list.Run()
	if err != nil {
		return nil, err
	}
	out := make([]Release, 0, len(releases))
	for _, r := range releases {
		out = append(out, summarize(r))
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Namespace != out[j].Namespace {
			return out[i].Namespace < out[j].Namespace
		}
		return out[i].Name < out[j].Name
	})
	return out, nil
}

// getRelease is one release's latest revision with its history.
func getRelease(cfg *action.Configuration, name string) (*ReleaseDetail, error) {
	revisions, err := action.NewHistory(cfg).Run(name)
	if err != nil {
		return nil, err
	}
	if len(revisions) == 0 {
		return nil, driver.ErrReleaseNotFound
	}
	sort.Slice(revisions, func(i, j int) bool { return revisions[i].Version > revisions[j].Version })
	latest := revisions[0]
	detail := &ReleaseDetail{
		Release:   summarize(latest),
		ChartInfo: chartInfo(latest.Chart),
		Hooks:     hooksOf(latest),
		History:   make([]Revision, 0, len(revisions)),
	}
	if latest.Info != nil {
		detail.FirstDeployed = timestamp(latest.Info.FirstDeployed.Time)
		detail.Notes = latest.Info.Notes
	}
	for _, r := range revisions {
		detail.History = append(detail.History, revisionOf(r))
	}
	return detail, nil
}

// releaseValues is the values of a revision (0 = the latest) as YAML: the
// user-supplied ones, or with all those merged with the chart defaults.
func releaseValues(cfg *action.Configuration, name string, revision int, all bool) (string, error) {
	get := action.NewGetValues(cfg)
	get.Version = revision
	get.AllValues = all
	values, err := get.Run(name)
	if err != nil {
		return "", err
	}
	if len(values) == 0 {
		return "", nil
	}
	out, err := yaml.Marshal(values)
	return string(out), err
}

// releaseRevision is one revision (0 = the latest) of a release.
func releaseRevision(cfg *action.Configuration, name string, revision int) (*release.Release, error) {
	get := action.NewGet(cfg)
	get.Version = revision
	return get.Run(name)
}

// rollback makes a new revision from the chart and values of revision (0 =
// the previous one), without waiting for the resources to become ready.
func rollback(cfg *action.Configuration, name string, revision int) error {
	rb := action.NewRollback(cfg)
	rb.Version = revision
	rb.Timeout = actionTimeout
	rb.MaxHistory = maxHistory
	return rb.Run(name)
}

// uninstall deletes a release; with keepHistory its revisions stay, so it
// remains listed as uninstalled and can be rolled back.
func uninstall(cfg *action.Configuration, name string, keepHistory bool) (*release.UninstallReleaseResponse, error) {
	un := action.NewUninstall(cfg)
	un.KeepHistory = keepHistory
	un.Timeout = actionTimeout
	return un.Run(name)
}
