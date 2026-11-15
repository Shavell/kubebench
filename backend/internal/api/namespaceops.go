package api

import (
	"net/http"
)

// handleCreateNamespace: POST /api/namespaces with {"name","labels"?}.
func (s *Server) handleCreateNamespace(w http.ResponseWriter, r *http.Request, c *call) {
	var body struct {
		Name   string            `json:"name"`
		Labels map[string]string `json:"labels"`
	}
	if err := decodeJSON(w, r, &body); err != nil {
		writeErr(w, err)
		return
	}
	detail := ""
	if len(body.Labels) > 0 {
		detail = auditJSON(map[string]map[string]string{"labels": body.Labels})
	}
	c.setAudit(joinTarget("namespace", body.Name), detail)
	respondOp(w, c.rt.CreateNamespace(r.Context(), body.Name, body.Labels))
}

// handleDeleteNamespace: DELETE /api/namespaces?name=&force=. System
// namespaces and the one KubeBench runs in (config Namespace) are refused
// with 403 forbidden unless force=true.
func (s *Server) handleDeleteNamespace(w http.ResponseWriter, r *http.Request, c *call) {
	q := r.URL.Query()
	name, err := requiredParam(q, "name")
	if err != nil {
		writeErr(w, err)
		return
	}
	force, err := boolFlag(q, "force", false)
	if err != nil {
		writeErr(w, err)
		return
	}
	respondOp(w, c.rt.DeleteNamespace(r.Context(), name, s.cfg.Namespace, force))
}
