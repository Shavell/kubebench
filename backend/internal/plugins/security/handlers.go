package security

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"

	"kubebench/internal/plugin"
)

// errNoReport answers an export before the cluster's first scan.
var errNoReport = errors.New("no scan has run on this cluster yet")

func listChecks(w http.ResponseWriter, _ *http.Request, _ *plugin.Context) {
	plugin.WriteJSON(w, catalog)
}

// scan runs the checks (body {"includeSystem": bool}, optional) and
// answers the report.
func (p *Plugin) scan(w http.ResponseWriter, r *http.Request, c *plugin.Context) {
	var opts ScanOptions
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBody))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&opts); err != nil && !errors.Is(err, io.EOF) {
		plugin.WriteError(w, http.StatusBadRequest, plugin.CodeBadRequest, fmt.Errorf("invalid JSON body: %w", err))
		return
	}
	res, err := p.scans.run(r.Context(), c.Runtime, opts)
	switch {
	case errors.Is(err, errBusy):
		plugin.WriteError(w, http.StatusConflict, plugin.CodeConflict, err)
		return
	case errors.Is(err, context.Canceled) && r.Context().Err() != nil:
		return // the client left
	case err != nil:
		plugin.WriteK8sError(w, err)
		return
	}
	p.writeReport(w, c, res)
}

// report answers the last report, or null before the cluster's first scan
// (a state the UI shows, not an error).
func (p *Plugin) report(w http.ResponseWriter, _ *http.Request, c *plugin.Context) {
	res := p.scans.last(c.Runtime.Cluster.ID)
	if res == nil {
		plugin.WriteJSON(w, nil)
		return
	}
	p.writeReport(w, c, res)
}

func (p *Plugin) writeReport(w http.ResponseWriter, c *plugin.Context, res *result) {
	rep, err := p.build(c, res)
	if err != nil {
		plugin.WriteError(w, http.StatusInternalServerError, plugin.CodeInternal, err)
		return
	}
	plugin.WriteJSON(w, rep)
}

// build is the report of res with the cluster's current accepted risks.
func (p *Plugin) build(c *plugin.Context, res *result) (*Report, error) {
	records, err := loadAccepted(c.KV, c.Runtime.Cluster.ID)
	if err != nil {
		return nil, err
	}
	return buildReport(res, acceptances(records), p.clock()), nil
}

// unsafeFileChars are replaced in the export's file name.
var unsafeFileChars = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

func (p *Plugin) export(w http.ResponseWriter, r *http.Request, c *plugin.Context) {
	format := r.URL.Query().Get("format")
	if format != "md" && format != "csv" {
		plugin.WriteError(w, http.StatusBadRequest, plugin.CodeBadRequest, errors.New("format: md or csv"))
		return
	}
	res := p.scans.last(c.Runtime.Cluster.ID)
	if res == nil {
		plugin.WriteError(w, http.StatusNotFound, plugin.CodeNotFound, errNoReport)
		return
	}
	rep, err := p.build(c, res)
	if err != nil {
		plugin.WriteError(w, http.StatusInternalServerError, plugin.CodeInternal, err)
		return
	}
	body, contentType := exportMarkdown(rep), "text/markdown; charset=utf-8"
	if format == "csv" {
		if body, err = exportCSV(rep); err != nil {
			plugin.WriteError(w, http.StatusInternalServerError, plugin.CodeInternal, err)
			return
		}
		contentType = "text/csv; charset=utf-8"
	}
	name := fmt.Sprintf("security-%s-%s.%s", unsafeFileChars.ReplaceAllString(rep.Cluster, "_"),
		rep.GeneratedAt.UTC().Format("20060102-150405"), format)
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", `attachment; filename="`+name+`"`)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Last-Modified", rep.GeneratedAt.UTC().Format(http.TimeFormat))
	_, _ = w.Write(body)
}
