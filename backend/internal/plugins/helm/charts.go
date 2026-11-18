package helm

import (
	"io"
	"regexp"
	"strings"

	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/downloader"
	"helm.sh/helm/v3/pkg/getter"
	"helm.sh/helm/v3/pkg/registry"

	"kubebench/internal/reason"
)

// ChartRef names the chart of an install or upgrade, in one of three
// forms (§7.4): {repo, name, version} from an added repository,
// {ref, version} from an OCI registry, or {uploadId} for an uploaded
// archive.
type ChartRef struct {
	Repo     string `json:"repo,omitempty"`
	Name     string `json:"name,omitempty"`
	Ref      string `json:"ref,omitempty"`
	Version  string `json:"version,omitempty"`
	UploadID string `json:"uploadId,omitempty"`
}

var (
	// chartNamePattern matches chart names as repository indexes list them.
	chartNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)
	// versionPattern is lenient (semver, ranges such as "^1.2"), but keeps
	// a version from smuggling anything else into a reference.
	versionPattern = regexp.MustCompile(`^[A-Za-z0-9.+^~<>=*, |-]{0,64}$`)
)

// IsZero reports that no chart is named.
func (c ChartRef) IsZero() bool { return c == ChartRef{} }

// validate checks that exactly one form is used and its fields are sound,
// before anything is downloaded or read. A local path is never accepted:
// the chart comes from a repository, a registry or an upload.
func (c ChartRef) validate() error {
	forms := 0
	if c.Repo != "" || c.Name != "" {
		forms++
		if !repoNamePattern.MatchString(c.Repo) {
			return badRequest(reasonRepoRequired, nil)
		}
		if !chartNamePattern.MatchString(c.Name) {
			return badRequest(reasonChartName, reason.Params{"name": c.Name})
		}
	}
	if c.Ref != "" {
		forms++
		if !registry.IsOCI(c.Ref) || len(c.Ref) > 512 || strings.ContainsAny(c.Ref, " \t\r\n@") {
			return badRequest(reasonOCIRef, nil)
		}
	}
	if c.UploadID != "" {
		forms++
		if !uploadIDPattern.MatchString(c.UploadID) {
			return badRequest(reasonUploadID, nil)
		}
		if c.Version != "" {
			return badRequest(reasonUploadVersion, nil)
		}
	}
	if forms != 1 {
		return badRequest(reasonChartSource, nil)
	}
	if !versionPattern.MatchString(c.Version) {
		return badRequest(reasonChartVersion, reason.Params{"version": c.Version})
	}
	return nil
}

// loadChart downloads (or reads the upload of) and loads the chart c names.
func (e *environment) loadChart(c ChartRef) (*chart.Chart, error) {
	if err := c.validate(); err != nil {
		return nil, err
	}
	if c.UploadID != "" {
		return e.loadUpload(c.UploadID)
	}
	ref := c.Ref
	if ref == "" {
		if !e.hasRepo(c.Repo) {
			return nil, notFound(reasonRepoNotAdded, reason.Params{"name": c.Repo})
		}
		ref = c.Repo + "/" + c.Name
	}
	dl := downloader.ChartDownloader{
		Out:              io.Discard,
		Verify:           downloader.VerifyNever,
		Getters:          e.getters,
		RegistryClient:   e.registry,
		RepositoryConfig: e.settings.RepositoryConfig,
		RepositoryCache:  e.settings.RepositoryCache,
	}
	if registry.IsOCI(ref) {
		dl.Options = append(dl.Options, getter.WithRegistryClient(e.registry))
	}
	path, _, err := dl.DownloadTo(ref, c.Version, e.settings.RepositoryCache)
	if err != nil {
		return nil, badRequest(reasonDownload, reason.Params{"chart": describeRef(c), "detail": err.Error()})
	}
	ch, err := loader.Load(path)
	if err != nil {
		return nil, badRequest(reasonInvalidChart, reason.Params{"chart": describeRef(c), "detail": err.Error()})
	}
	return ch, nil
}

// describeRef names a chart reference for messages and audit details.
func describeRef(c ChartRef) string {
	name := c.Ref
	switch {
	case c.UploadID != "":
		return "upload " + c.UploadID
	case name == "":
		name = c.Repo + "/" + c.Name
	}
	if c.Version != "" {
		name += "@" + c.Version
	}
	return name
}
