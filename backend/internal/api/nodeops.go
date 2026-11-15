package api

import (
	"net/http"
	"net/url"
	"time"

	"kubebench/internal/kube"
	"kubebench/internal/plugin"
)

// Limits of POST /api/nodes/drain, in seconds.
const (
	defaultDrainTimeout = 120
	maxDrainTimeout     = 600
	maxGracePeriod      = 3600
)

// handleCordon: POST /api/nodes/cordon?name=&unschedulable=true|false
// (default true; false uncordons).
func (s *Server) handleCordon(w http.ResponseWriter, r *http.Request, c *call) {
	q := r.URL.Query()
	name, err := requiredParam(q, "name")
	if err != nil {
		writeErr(w, err)
		return
	}
	unschedulable, err := boolFlag(q, "unschedulable", true)
	if err != nil {
		writeErr(w, err)
		return
	}
	respondOp(w, c.rt.CordonNode(r.Context(), name, unschedulable))
}

// handleDrain: POST /api/nodes/drain?name=&ignoreDaemonSets=true&
// deleteEmptyDirData=false&force=false&gracePeriod=-1&timeoutSeconds=120.
// It answers when the drain ended, with every pod's outcome.
func (s *Server) handleDrain(w http.ResponseWriter, r *http.Request, c *call) {
	q := r.URL.Query()
	name, err := requiredParam(q, "name")
	if err != nil {
		writeErr(w, err)
		return
	}
	opts, err := drainOptions(q)
	if err != nil {
		writeErr(w, err)
		return
	}
	res, err := c.rt.DrainNode(r.Context(), name, opts)
	if err != nil {
		writeOpError(w, err)
		return
	}
	c.setAudit("", auditJSON(map[string]any{
		"ignoreDaemonSets":   opts.IgnoreDaemonSets,
		"deleteEmptyDirData": opts.DeleteEmptyDirData,
		"force":              opts.Force,
		"gracePeriod":        opts.GracePeriodSeconds,
		"timeoutSeconds":     int(opts.Timeout / time.Second),
		"completed":          res.Completed,
		"evicted":            len(res.Evicted),
		"failed":             len(res.Failed),
		"blockedByPDB":       len(res.BlockedByPDB),
	}))
	plugin.WriteJSON(w, res)
}

func drainOptions(q url.Values) (kube.DrainOptions, error) {
	var o kube.DrainOptions
	var err error
	if o.IgnoreDaemonSets, err = boolFlag(q, "ignoreDaemonSets", true); err != nil {
		return o, err
	}
	if o.DeleteEmptyDirData, err = boolFlag(q, "deleteEmptyDirData", false); err != nil {
		return o, err
	}
	if o.Force, err = boolFlag(q, "force", false); err != nil {
		return o, err
	}
	if o.GracePeriodSeconds, err = boundedInt(q, "gracePeriod", -1, -1, maxGracePeriod); err != nil {
		return o, err
	}
	timeout, err := boundedInt(q, "timeoutSeconds", defaultDrainTimeout, 1, maxDrainTimeout)
	if err != nil {
		return o, err
	}
	o.Timeout = time.Duration(timeout) * time.Second
	return o, nil
}

// handleSetTaints: PUT /api/nodes/taints?name= with {"taints":[{key,value,effect}]}
// replaces the node's taints ([] removes them all) and answers with the
// stored list.
func (s *Server) handleSetTaints(w http.ResponseWriter, r *http.Request, c *call) {
	name, err := requiredParam(r.URL.Query(), "name")
	if err != nil {
		writeErr(w, err)
		return
	}
	var body struct {
		Taints *[]kube.Taint `json:"taints"`
	}
	if err := decodeJSON(w, r, &body); err != nil {
		writeErr(w, err)
		return
	}
	if body.Taints == nil {
		writeErr(w, badRequest("taints is required; send [] to remove every taint"))
		return
	}
	taints, err := c.rt.SetNodeTaints(r.Context(), name, *body.Taints)
	if err != nil {
		writeOpError(w, err)
		return
	}
	listed := make([]string, 0, len(taints))
	for _, t := range taints {
		listed = append(listed, t.String())
	}
	c.setAudit("", auditJSON(map[string][]string{"taints": listed}))
	plugin.WriteJSON(w, map[string][]kube.Taint{"taints": taints})
}
