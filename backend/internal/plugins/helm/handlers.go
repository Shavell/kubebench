package helm

import (
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"strconv"
	"time"

	"helm.sh/helm/v3/pkg/chart"
	"k8s.io/apimachinery/pkg/util/validation"

	"kubebench/internal/plugin"
)

// maxBodyBytes bounds JSON request bodies (values of an install or upgrade).
const maxBodyBytes = 8 << 20

// releaseRef reads and checks the {ns} and {name} path values.
func releaseRef(r *http.Request) (namespace, name string, err error) {
	namespace, name = r.PathValue("ns"), r.PathValue("name")
	if len(validation.IsDNS1123Label(namespace)) > 0 {
		return "", "", badRequest("%q is not a namespace name", namespace)
	}
	if len(validation.IsDNS1123Subdomain(name)) > 0 || len(name) > 53 {
		return "", "", badRequest("%q is not a release name", name)
	}
	return namespace, name, nil
}

// revisionParam is the revision query parameter; absent means 0, the latest.
func revisionParam(r *http.Request) (int, error) {
	raw := r.URL.Query().Get("revision")
	if raw == "" {
		return 0, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 0 {
		return 0, badRequest("revision must be a revision number")
	}
	return n, nil
}

func decodeJSON(w http.ResponseWriter, r *http.Request, v any) error {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBodyBytes))
	if err := dec.Decode(v); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			return badRequest("the request body is larger than %d MiB", maxBodyBytes>>20)
		}
		return badRequest("invalid JSON body: %v", err)
	}
	return nil
}

// refineAudit names what the body of an audited request acted on.
func refineAudit(c *plugin.Context, target string, detail map[string]any) {
	if c.SetAudit == nil {
		return
	}
	text := ""
	if len(detail) > 0 {
		if b, err := json.Marshal(detail); err == nil {
			text = string(b)
		}
	}
	c.SetAudit(target, text)
}

func (p *Plugin) handleListReleases(w http.ResponseWriter, r *http.Request, c *plugin.Context) {
	ns := r.URL.Query().Get("ns")
	if ns != "" && len(validation.IsDNS1123Label(ns)) > 0 {
		writeError(w, badRequest("%q is not a namespace name", ns))
		return
	}
	cfg, err := p.actions(c.Runtime, ns)
	if err != nil {
		writeError(w, err)
		return
	}
	out, err := listReleases(cfg, ns == "")
	if err != nil {
		writeError(w, err)
		return
	}
	plugin.WriteJSON(w, out)
}

func (p *Plugin) handleGetRelease(w http.ResponseWriter, r *http.Request, c *plugin.Context) {
	ns, name, err := releaseRef(r)
	if err != nil {
		writeError(w, err)
		return
	}
	cfg, err := p.actions(c.Runtime, ns)
	if err != nil {
		writeError(w, err)
		return
	}
	out, err := getRelease(cfg, name)
	if err != nil {
		writeError(w, err)
		return
	}
	plugin.WriteJSON(w, out)
}

func (p *Plugin) handleReleaseValues(w http.ResponseWriter, r *http.Request, c *plugin.Context) {
	ns, name, err := releaseRef(r)
	if err != nil {
		writeError(w, err)
		return
	}
	revision, err := revisionParam(r)
	if err != nil {
		writeError(w, err)
		return
	}
	cfg, err := p.actions(c.Runtime, ns)
	if err != nil {
		writeError(w, err)
		return
	}
	text, err := releaseValues(cfg, name, revision, r.URL.Query().Get("all") == "true")
	if err != nil {
		writeError(w, err)
		return
	}
	plugin.WriteJSON(w, map[string]string{"values": text})
}

func (p *Plugin) handleReleaseManifest(w http.ResponseWriter, r *http.Request, c *plugin.Context) {
	ns, name, err := releaseRef(r)
	if err != nil {
		writeError(w, err)
		return
	}
	revision, err := revisionParam(r)
	if err != nil {
		writeError(w, err)
		return
	}
	cfg, err := p.actions(c.Runtime, ns)
	if err != nil {
		writeError(w, err)
		return
	}
	rel, err := releaseRevision(cfg, name, revision)
	if err != nil {
		writeError(w, err)
		return
	}
	plugin.WriteJSON(w, map[string]any{"revision": rel.Version, "manifest": redactManifest(manifestOf(rel))})
}

func (p *Plugin) handleRollback(w http.ResponseWriter, r *http.Request, c *plugin.Context) {
	ns, name, err := releaseRef(r)
	if err != nil {
		writeError(w, err)
		return
	}
	var in struct {
		Revision int `json:"revision"`
	}
	if err := decodeJSON(w, r, &in); err != nil {
		writeError(w, err)
		return
	}
	if in.Revision < 0 {
		writeError(w, badRequest("revision must be a revision number (0 = the previous one)"))
		return
	}
	refineAudit(c, "", map[string]any{"revision": in.Revision})
	cfg, err := p.actions(c.Runtime, ns)
	if err != nil {
		writeError(w, err)
		return
	}
	if err := rollback(cfg, name, in.Revision); err != nil {
		writeError(w, err)
		return
	}
	rel, err := releaseRevision(cfg, name, 0)
	if err != nil {
		writeError(w, err)
		return
	}
	plugin.WriteJSON(w, map[string]Release{"release": summarize(rel)})
}

func (p *Plugin) handleUninstall(w http.ResponseWriter, r *http.Request, c *plugin.Context) {
	ns, name, err := releaseRef(r)
	if err != nil {
		writeError(w, err)
		return
	}
	cfg, err := p.actions(c.Runtime, ns)
	if err != nil {
		writeError(w, err)
		return
	}
	res, err := uninstall(cfg, name, r.URL.Query().Get("keepHistory") == "true")
	if err != nil {
		writeError(w, err)
		return
	}
	out := map[string]any{"ok": true}
	if res != nil && res.Release != nil {
		out["release"] = summarize(res.Release)
	}
	plugin.WriteJSON(w, out)
}

func (p *Plugin) handleInstall(w http.ResponseWriter, r *http.Request, c *plugin.Context) {
	var in InstallRequest
	if err := decodeJSON(w, r, &in); err != nil {
		writeError(w, err)
		return
	}
	// Either flag asks for a dry run. Dry runs need write and are audited
	// like the install itself: template lookups read the cluster with
	// KubeBench's credential (§7.4).
	in.DryRun = in.DryRun || plugin.IsDryRun(r)
	if err := in.validate(); err != nil {
		writeError(w, err)
		return
	}
	detail := map[string]any{"chart": describeRef(in.Chart), "createNamespace": in.CreateNamespace}
	if in.DryRun {
		detail["dryRun"] = true
	}
	refineAudit(c, "release/"+in.Namespace+"/"+in.Name, detail)
	ch, err := p.env.loadChart(in.Chart)
	if err != nil {
		writeError(w, err)
		return
	}
	cfg, err := p.actions(c.Runtime, in.Namespace)
	if err != nil {
		writeError(w, err)
		return
	}
	out, err := install(r.Context(), cfg, in, ch)
	if err != nil {
		writeError(w, err)
		return
	}
	plugin.WriteJSON(w, out)
}

func (p *Plugin) handleUpgrade(w http.ResponseWriter, r *http.Request, c *plugin.Context) {
	ns, name, err := releaseRef(r)
	if err != nil {
		writeError(w, err)
		return
	}
	var in UpgradeRequest
	if err := decodeJSON(w, r, &in); err != nil {
		writeError(w, err)
		return
	}
	in.DryRun = in.DryRun || plugin.IsDryRun(r)
	detail := map[string]any{"chart": "(current)", "reuseValues": in.ReuseValues}
	if in.DryRun {
		detail["dryRun"] = true
	}
	var ch *chart.Chart // nil: the release's current chart
	if !in.Chart.IsZero() {
		detail["chart"] = describeRef(in.Chart)
	}
	refineAudit(c, "", detail)
	if !in.Chart.IsZero() {
		if ch, err = p.env.loadChart(in.Chart); err != nil {
			writeError(w, err)
			return
		}
	}
	cfg, err := p.actions(c.Runtime, ns)
	if err != nil {
		writeError(w, err)
		return
	}
	out, err := upgrade(r.Context(), cfg, ns, name, in, ch)
	if err != nil {
		writeError(w, err)
		return
	}
	plugin.WriteJSON(w, out)
}

func (p *Plugin) handleListRepos(w http.ResponseWriter, _ *http.Request, _ *plugin.Context) {
	out, err := p.env.listRepos()
	if err != nil {
		writeError(w, err)
		return
	}
	plugin.WriteJSON(w, out)
}

func (p *Plugin) handleAddRepo(w http.ResponseWriter, r *http.Request, c *plugin.Context) {
	var in RepoInput
	if err := decodeJSON(w, r, &in); err != nil {
		writeError(w, err)
		return
	}
	// Never the credentials: the URL carries none (validate refuses them).
	refineAudit(c, "repo/"+in.Name, map[string]any{"url": in.URL, "authenticated": in.Username != ""})
	out, err := p.env.addRepo(in)
	if err != nil {
		writeError(w, err)
		return
	}
	plugin.WriteJSON(w, out)
}

func (p *Plugin) handleRemoveRepo(w http.ResponseWriter, r *http.Request, _ *plugin.Context) {
	name := r.PathValue("name")
	if !repoNamePattern.MatchString(name) {
		writeError(w, badRequest("%q is not a repository name", name))
		return
	}
	if err := p.env.removeRepo(name); err != nil {
		writeError(w, err)
		return
	}
	plugin.WriteJSON(w, map[string]bool{"ok": true})
}

func (p *Plugin) handleUpdateRepos(w http.ResponseWriter, _ *http.Request, _ *plugin.Context) {
	out, err := p.env.updateRepos()
	if err != nil {
		writeError(w, err)
		return
	}
	plugin.WriteJSON(w, map[string][]RepoUpdate{"repos": out})
}

func (p *Plugin) handleSearchCharts(w http.ResponseWriter, r *http.Request, _ *plugin.Context) {
	out, err := p.env.searchCharts(r.URL.Query().Get("q"))
	if err != nil {
		writeError(w, err)
		return
	}
	plugin.WriteJSON(w, out)
}

// handleChartValues serves the default values of a chart from an added
// repository or an upload. OCI references go to handleOCIChartValues,
// which needs write: the caller names the host the server connects to.
func (p *Plugin) handleChartValues(w http.ResponseWriter, r *http.Request, _ *plugin.Context) {
	q := r.URL.Query()
	if q.Get("ref") != "" {
		writeError(w, badRequest("the values of an OCI chart are read through GET /charts/oci/values"))
		return
	}
	ref := ChartRef{Repo: q.Get("repo"), Name: q.Get("chart"), Version: q.Get("version"), UploadID: q.Get("uploadId")}
	p.writeChartValues(w, ref)
}

// handleOCIChartValues serves the default values of an OCI chart.
func (p *Plugin) handleOCIChartValues(w http.ResponseWriter, r *http.Request, c *plugin.Context) {
	q := r.URL.Query()
	ref := ChartRef{Ref: q.Get("ref"), Version: q.Get("version")}
	if ref.Ref == "" {
		writeError(w, badRequest("ref must name an OCI chart"))
		return
	}
	refineAudit(c, "chart/"+ref.Ref, map[string]any{"version": ref.Version})
	p.writeChartValues(w, ref)
}

func (p *Plugin) writeChartValues(w http.ResponseWriter, ref ChartRef) {
	out, err := p.env.chartValues(ref)
	if err != nil {
		writeError(w, err)
		return
	}
	plugin.WriteJSON(w, out)
}

// handleUploadChart stores the multipart file field "chart" (a .tgz of at
// most 20 MiB) for an install.
func (p *Plugin) handleUploadChart(w http.ResponseWriter, r *http.Request, c *plugin.Context) {
	// Room for the multipart framing around the archive.
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadBytes+64<<10)
	part, err := chartPart(r)
	if err != nil {
		writeError(w, err)
		return
	}
	archive, err := readLimited(part, maxUploadBytes)
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			err = badRequest("the chart archive is larger than %d MiB", maxUploadBytes>>20)
		}
		writeError(w, err)
		return
	}
	out, err := p.env.saveUpload(archive, time.Now())
	if err != nil {
		writeError(w, err)
		return
	}
	refineAudit(c, "chart/"+out.Chart.Name, map[string]any{"version": out.Chart.Version, "uploadId": out.UploadID})
	plugin.WriteJSON(w, out)
}

// chartPart finds the "chart" file field of a multipart request.
func chartPart(r *http.Request) (*multipart.Part, error) {
	mr, err := r.MultipartReader()
	if err != nil {
		return nil, badRequest("send the chart as multipart/form-data with a file field named chart")
	}
	for {
		part, err := mr.NextPart()
		if errors.Is(err, io.EOF) {
			return nil, badRequest("the request has no file field named chart")
		}
		if err != nil {
			return nil, badRequest("invalid multipart body: %v", err)
		}
		if part.FormName() == "chart" && part.FileName() != "" {
			return part, nil
		}
	}
}
