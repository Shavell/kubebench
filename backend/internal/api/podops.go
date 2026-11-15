package api

import (
	"net/http"

	"kubebench/internal/kube"
	"kubebench/internal/plugin"
)

// handleBulkDeletePods: POST /api/pods/bulk-delete with
// {"items":[{"namespace","name"}],"force":bool} deletes up to 200 pods and
// answers {"results":[{"namespace","name","error"?}]}, one per item.
func (s *Server) handleBulkDeletePods(w http.ResponseWriter, r *http.Request, c *call) {
	var body struct {
		Items []kube.PodKey `json:"items"`
		Force bool          `json:"force"`
	}
	if err := decodeJSON(w, r, &body); err != nil {
		writeErr(w, err)
		return
	}
	results, err := c.rt.DeletePods(r.Context(), body.Items, body.Force)
	if err != nil {
		writeOpError(w, err)
		return
	}
	pods := make([]string, 0, len(results))
	failed := 0
	for _, res := range results {
		pods = append(pods, res.Namespace+"/"+res.Name)
		if res.Error != "" {
			failed++
		}
	}
	// Up to 200 pods do not fit one row; every pod is recorded all the same.
	c.setAuditParts(auditParts(map[string]any{"force": body.Force, "failed": failed}, "pods", pods))
	plugin.WriteJSON(w, map[string][]kube.PodDeleteResult{"results": results})
}

// handleDebugPod: POST /api/pods/debug?ns=&pod=&image=&target= adds an
// ephemeral debug container and answers {"namespace","pod","container"};
// the terminal attaches to it with container=<name> (§7.1).
func (s *Server) handleDebugPod(w http.ResponseWriter, r *http.Request, c *call) {
	q := r.URL.Query()
	ns, pod := q.Get("ns"), q.Get("pod")
	if ns == "" || pod == "" {
		writeErr(w, badRequest("ns and pod are required"))
		return
	}
	debug, err := c.rt.AddDebugContainer(r.Context(), ns, pod, q.Get("image"), q.Get("target"))
	if err != nil {
		writeOpError(w, err)
		return
	}
	image := q.Get("image")
	if image == "" {
		image = kube.DefaultDebugImage
	}
	c.setAudit("", auditJSON(map[string]string{"container": debug.Container, "image": image, "target": q.Get("target")}))
	plugin.WriteJSON(w, debug)
}
