package api

import (
	"net/http"
	"strconv"

	"kubebench/internal/plugin"
	"kubebench/internal/reason"
)

// handleDeploymentHistory: GET /api/deployments/history?ns=&name= lists the
// rollout revisions, newest first.
func (s *Server) handleDeploymentHistory(w http.ResponseWriter, r *http.Request, c *call) {
	ns, name, err := objectParams(r.URL.Query())
	if err != nil {
		writeErr(w, err)
		return
	}
	history, err := c.rt.DeploymentHistory(r.Context(), ns, name)
	writeOpResult(w, history, err)
}

// handleRollback: POST /api/deployments/rollback?ns=&name=&revision=.
func (s *Server) handleRollback(w http.ResponseWriter, r *http.Request, c *call) {
	q := r.URL.Query()
	ns, name, err := objectParams(q)
	if err != nil {
		writeErr(w, err)
		return
	}
	revision, err := strconv.ParseInt(q.Get("revision"), 10, 64)
	if err != nil || revision < 1 {
		writeErr(w, badRequest(reason.FieldMin, reason.Params{"field": "revision", "min": 1}))
		return
	}
	respondOp(w, c.rt.RollbackDeployment(r.Context(), ns, name, revision))
}

// handlePause: POST /api/deployments/pause?ns=&name=&paused=true|false
// (default true; false resumes).
func (s *Server) handlePause(w http.ResponseWriter, r *http.Request, c *call) {
	q := r.URL.Query()
	ns, name, err := objectParams(q)
	if err != nil {
		writeErr(w, err)
		return
	}
	paused, err := boolFlag(q, "paused", true)
	if err != nil {
		writeErr(w, err)
		return
	}
	respondOp(w, c.rt.PauseDeployment(r.Context(), ns, name, paused))
}

// handleRestartWorkload: POST /api/workloads/restart?kind=Deployment|StatefulSet|DaemonSet&ns=&name=.
func (s *Server) handleRestartWorkload(w http.ResponseWriter, r *http.Request, c *call) {
	q := r.URL.Query()
	ns, name, err := objectParams(q)
	if err != nil {
		writeErr(w, err)
		return
	}
	respondOp(w, c.rt.RestartWorkload(r.Context(), q.Get("kind"), ns, name))
}

// handleScaleWorkload: POST /api/workloads/scale?kind=Deployment|StatefulSet|ReplicaSet&ns=&name=&replicas=,
// or with apiVersion= any kind with a scale subresource.
func (s *Server) handleScaleWorkload(w http.ResponseWriter, r *http.Request, c *call) {
	q := r.URL.Query()
	ns, name, err := objectParams(q)
	if err != nil {
		writeErr(w, err)
		return
	}
	replicas, err := strconv.ParseInt(q.Get("replicas"), 10, 32)
	if err != nil || replicas < 0 {
		writeErr(w, badRequest(reason.FieldMin, reason.Params{"field": "replicas", "min": 0}))
		return
	}
	respondOp(w, c.rt.ScaleWorkload(r.Context(), q.Get("apiVersion"), q.Get("kind"), ns, name, int32(replicas)))
}

// handleTriggerCronJob: POST /api/cronjobs/trigger?ns=&name= creates a Job
// from the CronJob now and answers with its name.
func (s *Server) handleTriggerCronJob(w http.ResponseWriter, r *http.Request, c *call) {
	ns, name, err := objectParams(r.URL.Query())
	if err != nil {
		writeErr(w, err)
		return
	}
	run, err := c.rt.TriggerCronJob(r.Context(), ns, name)
	if err != nil {
		writeOpError(w, err)
		return
	}
	c.setAudit("", auditJSON(map[string]string{"job": run.Job}))
	plugin.WriteJSON(w, run)
}

// handleSuspendCronJob: POST /api/cronjobs/suspend?ns=&name=&suspend=true|false
// (default true; false resumes the schedule).
func (s *Server) handleSuspendCronJob(w http.ResponseWriter, r *http.Request, c *call) {
	q := r.URL.Query()
	ns, name, err := objectParams(q)
	if err != nil {
		writeErr(w, err)
		return
	}
	suspend, err := boolFlag(q, "suspend", true)
	if err != nil {
		writeErr(w, err)
		return
	}
	respondOp(w, c.rt.SuspendCronJob(r.Context(), ns, name, suspend))
}
