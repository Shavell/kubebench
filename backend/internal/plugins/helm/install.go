package helm

import (
	"context"
	"strings"
	"time"

	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/release"
	"k8s.io/apimachinery/pkg/util/validation"

	"kubebench/internal/reason"
)

// actionTimeout bounds hooks and deletions of one install, upgrade,
// rollback or uninstall. Helm does not wait for the resources themselves
// to become ready (no --wait): the release page shows their state.
const actionTimeout = 5 * time.Minute

// InstallRequest is POST /install.
type InstallRequest struct {
	Name            string   `json:"name"`
	Namespace       string   `json:"namespace"`
	CreateNamespace bool     `json:"createNamespace"`
	Chart           ChartRef `json:"chart"`
	// Values is YAML that overrides the chart's defaults ("" = none).
	Values string `json:"values"`
	DryRun bool   `json:"dryRun"`
}

// InstallResult is POST /install's answer; for a dry run it shows what an
// install would create, and nothing was created.
type InstallResult struct {
	Release  Release `json:"release"`
	Manifest string  `json:"manifest"`
	Notes    string  `json:"notes"`
}

// UpgradeRequest is POST /releases/{ns}/{name}/upgrade.
type UpgradeRequest struct {
	// Chart is the chart to upgrade to; omitted, the release's current
	// chart is used again (a values-only upgrade).
	Chart ChartRef `json:"chart"`
	// Values is YAML. With ReuseValues it is merged over the release's
	// current values; without, it replaces them ("" resets the release to
	// the chart's defaults).
	Values      string `json:"values"`
	ReuseValues bool   `json:"reuseValues"`
	DryRun      bool   `json:"dryRun"`
}

// UpgradeResult is the upgraded (or, for a dry run, the would-be) revision
// and the manifest of the revision it replaces, for a diff.
type UpgradeResult struct {
	Release         Release `json:"release"`
	Manifest        string  `json:"manifest"`
	Notes           string  `json:"notes"`
	CurrentManifest string  `json:"currentManifest"`
}

func (in InstallRequest) validate() error {
	if errs := validation.IsDNS1123Subdomain(in.Name); len(errs) > 0 || len(in.Name) > 53 {
		return badRequest(reasonInstallName, nil)
	}
	if errs := validation.IsDNS1123Label(in.Namespace); len(errs) > 0 {
		return badRequest(reasonInstallNS, nil)
	}
	return in.Chart.validate()
}

func parseValues(text string) (map[string]any, error) {
	if strings.TrimSpace(text) == "" {
		return map[string]any{}, nil
	}
	values, err := chartutil.ReadValues([]byte(text))
	if err != nil {
		return nil, badRequest(reasonValuesYAML, reason.Params{"detail": err.Error()})
	}
	return values, nil
}

// detached keeps an install or upgrade running when the browser goes away:
// cancelling half-way would leave a failed release behind.
func detached(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.WithoutCancel(ctx), actionTimeout+time.Minute)
}

// install installs ch as in describes. A dry run renders the manifest with
// the cluster's API versions and lookups, without creating anything: as
// template lookups read the cluster with KubeBench's own credential, the
// route needs write access for dry runs too (§7.4).
func install(ctx context.Context, cfg *action.Configuration, in InstallRequest, ch *chart.Chart) (*InstallResult, error) {
	values, err := parseValues(in.Values)
	if err != nil {
		return nil, err
	}
	inst := action.NewInstall(cfg)
	inst.ReleaseName = in.Name
	inst.Namespace = in.Namespace
	inst.CreateNamespace = in.CreateNamespace
	inst.Timeout = actionTimeout
	if in.DryRun {
		inst.DryRun = true
		inst.DryRunOption = "server"
	}
	ctx, cancel := detached(ctx)
	defer cancel()
	rel, err := inst.RunWithContext(ctx, ch, values)
	if err != nil {
		return nil, err
	}
	return &InstallResult{Release: summarize(rel), Manifest: redactManifest(manifestOf(rel)), Notes: notesOf(rel)}, nil
}

// upgrade upgrades the release name to ch (nil: its current chart).
func upgrade(ctx context.Context, cfg *action.Configuration, namespace, name string, in UpgradeRequest, ch *chart.Chart) (*UpgradeResult, error) {
	values, err := parseValues(in.Values)
	if err != nil {
		return nil, err
	}
	current, err := releaseRevision(cfg, name, 0)
	if err != nil {
		return nil, err
	}
	if ch == nil {
		ch = current.Chart
	}
	up := action.NewUpgrade(cfg)
	up.Namespace = namespace
	up.Timeout = actionTimeout
	up.MaxHistory = maxHistory
	// Helm reuses the current values when none are given and neither flag
	// is set; reset instead, so the request's values are the whole truth.
	// Reuse merges the current user values under the request's while
	// keeping the chart's own defaults: Helm's ReuseValues would instead
	// store the merged values as the chart's defaults, so a later reset
	// could never go back to them.
	up.ResetThenReuseValues = in.ReuseValues
	up.ResetValues = !in.ReuseValues
	if in.DryRun {
		up.DryRun = true
		up.DryRunOption = "server"
	}
	// Never cancel the upgrade's context: Helm (3.22) watches it from a
	// goroutine that can still see the cancellation after a successful
	// upgrade has returned, and then marks that release failed (and rolls
	// it back when atomic) in the background. up.Timeout bounds the upgrade.
	rel, err := up.RunWithContext(context.WithoutCancel(ctx), name, ch, values)
	if err != nil {
		return nil, err
	}
	return &UpgradeResult{
		Release: summarize(rel), Manifest: redactManifest(manifestOf(rel)), Notes: notesOf(rel),
		CurrentManifest: redactManifest(manifestOf(current)),
	}, nil
}

// manifestOf is a revision's rendered manifest; hooks are listed after it
// as Helm stores them separately.
func manifestOf(rel *release.Release) string {
	var b strings.Builder
	b.WriteString(rel.Manifest)
	for _, h := range rel.Hooks {
		if b.Len() > 0 && !strings.HasSuffix(b.String(), "\n") {
			b.WriteByte('\n')
		}
		b.WriteString("---\n# Source: " + h.Path + "\n" + h.Manifest)
	}
	return b.String()
}

func notesOf(rel *release.Release) string {
	if rel.Info == nil {
		return ""
	}
	return rel.Info.Notes
}
