package api

import (
	"net/http"
	"net/url"
	"strconv"

	"kubebench/internal/kube"
	"kubebench/internal/plugin"
	"kubebench/internal/reason"
)

// Pod log limits for GET /api/logs (§4.4).
const (
	defaultLogTail = 300
	maxLogTail     = 5000
)

func (s *Server) handleNodes(w http.ResponseWriter, r *http.Request, c *call) {
	out, err := c.rt.ListNodes(r.Context())
	writeResult(w, out, err)
}

func (s *Server) handleNamespaces(w http.ResponseWriter, r *http.Request, c *call) {
	out, err := c.rt.ListNamespaces(r.Context())
	writeResult(w, out, err)
}

func (s *Server) handlePods(w http.ResponseWriter, r *http.Request, c *call) {
	out, err := c.rt.ListPods(r.Context(), r.URL.Query().Get("ns"))
	writeResult(w, out, err)
}

func (s *Server) handleDeployments(w http.ResponseWriter, r *http.Request, c *call) {
	out, err := c.rt.ListDeployments(r.Context(), r.URL.Query().Get("ns"))
	writeResult(w, out, err)
}

func (s *Server) handleServices(w http.ResponseWriter, r *http.Request, c *call) {
	out, err := c.rt.ListServices(r.Context(), r.URL.Query().Get("ns"))
	writeResult(w, out, err)
}

// handleYAML renders one object (§7.2): by apiVersion and kind for any
// kind the cluster serves, or by kind alone for the kinds v0.4 knew;
// neat=true strips the fields the API server maintains.
func (s *Server) handleYAML(w http.ResponseWriter, r *http.Request, c *call) {
	q := r.URL.Query()
	neat, err := flagParam(q, "neat")
	if err != nil {
		writeErr(w, err)
		return
	}
	ref := kube.ObjectRef{APIVersion: q.Get("apiVersion"), Kind: q.Get("kind"), Namespace: q.Get("ns"), Name: q.Get("name")}
	if ref.Kind == "" || ref.Name == "" {
		writeErr(w, required("kind", "name"))
		return
	}
	text, version, err := c.rt.ObjectYAMLVersion(r.Context(), ref, neat)
	if err != nil {
		writeClusterErr(w, err)
		return
	}
	out := map[string]string{"yaml": text}
	if version != "" {
		out["resourceVersion"] = version
	}
	plugin.WriteJSON(w, out)
}

// handleEvents lists the events about one object; kind keeps objects of
// different kinds with the same name apart.
func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request, c *call) {
	q := r.URL.Query()
	out, err := c.rt.ListEvents(r.Context(), q.Get("ns"), q.Get("kind"), q.Get("name"))
	writeResult(w, out, err)
}

func (s *Server) handleLogs(w http.ResponseWriter, r *http.Request, c *call) {
	q := r.URL.Query()
	opts, err := logOptions(q)
	if err != nil {
		writeErr(w, err)
		return
	}
	out, err := c.rt.PodLogs(r.Context(), q.Get("ns"), q.Get("pod"), opts)
	writeResult(w, out, err)
}

// logOptions parses tail (default 300, at most 5000), previous and
// sinceSeconds.
func logOptions(q url.Values) (kube.LogOptions, error) {
	opts := kube.LogOptions{Container: q.Get("container"), TailLines: defaultLogTail}
	if raw := q.Get("tail"); raw != "" {
		n, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || n < 1 {
			return opts, badRequest(reason.FieldMin, reason.Params{"field": "tail", "min": 1})
		}
		opts.TailLines = min(n, maxLogTail)
	}
	if raw := q.Get("previous"); raw != "" {
		previous, err := strconv.ParseBool(raw)
		if err != nil {
			return opts, badRequest(reason.FieldBoolean, reason.Params{"field": "previous"})
		}
		opts.Previous = previous
	}
	if raw := q.Get("sinceSeconds"); raw != "" {
		n, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || n < 1 {
			return opts, badRequest(reason.FieldMin, reason.Params{"field": "sinceSeconds", "min": 1})
		}
		opts.SinceSeconds = n
	}
	return opts, nil
}

func (s *Server) handleDeletePod(w http.ResponseWriter, r *http.Request, c *call) {
	q := r.URL.Query()
	ns, name, err := objectParams(q)
	if err != nil {
		writeErr(w, err)
		return
	}
	// force=true deletes at once, with grace period 0 (§7.3).
	force, err := boolFlag(q, "force", false)
	if err != nil {
		writeErr(w, err)
		return
	}
	respondOK(w, c.rt.DeletePod(r.Context(), ns, name, force))
}

func (s *Server) handleScale(w http.ResponseWriter, r *http.Request, c *call) {
	q := r.URL.Query()
	ns, name, err := objectParams(q)
	if err != nil {
		writeErr(w, err)
		return
	}
	replicas, err := strconv.Atoi(q.Get("replicas"))
	if err != nil || replicas < 0 {
		writeErr(w, badRequest(reason.FieldMin, reason.Params{"field": "replicas", "min": 0}))
		return
	}
	respondOK(w, c.rt.ScaleDeployment(r.Context(), ns, name, replicas))
}

func (s *Server) handleRestart(w http.ResponseWriter, r *http.Request, c *call) {
	ns, name, err := objectParams(r.URL.Query())
	if err != nil {
		writeErr(w, err)
		return
	}
	respondOK(w, c.rt.RestartWorkload(r.Context(), "Deployment", ns, name))
}

func (s *Server) handleExec(w http.ResponseWriter, r *http.Request, c *call) {
	q := r.URL.Query()
	ns, pod := q.Get("ns"), q.Get("pod")
	if ns == "" || pod == "" || q.Get("cmd") == "" {
		writeErr(w, required("ns", "pod", "cmd"))
		return
	}
	out, err := c.rt.Exec(r.Context(), ns, pod, q.Get("container"), q.Get("cmd"))
	writeResult(w, out, err)
}

func (s *Server) handleResourceList(w http.ResponseWriter, r *http.Request, c *call) {
	q := r.URL.Query()
	if err := catalogKey(q.Get("key")); err != nil {
		writeErr(w, err)
		return
	}
	out, err := c.rt.ListGeneric(r.Context(), q.Get("key"), q.Get("ns"))
	writeResult(w, out, err)
}

func (s *Server) handleResourceGet(w http.ResponseWriter, r *http.Request, c *call) {
	q := r.URL.Query()
	if err := catalogKey(q.Get("key")); err != nil {
		writeErr(w, err)
		return
	}
	out, err := c.rt.GetGeneric(r.Context(), q.Get("key"), q.Get("ns"), q.Get("name"))
	writeResult(w, out, err)
}

func catalogKey(key string) error {
	if _, ok := kube.Catalog[key]; !ok {
		return badRequest(reasonUnknownResource, reason.Params{"key": key})
	}
	return nil
}

// objectParams returns the required ns and name of a namespaced object.
func objectParams(q url.Values) (ns, name string, err error) {
	ns, name = q.Get("ns"), q.Get("name")
	if ns == "" || name == "" {
		return "", "", required("ns", "name")
	}
	return ns, name, nil
}

// writeResult answers with a value read from the cluster, or its error.
func writeResult(w http.ResponseWriter, v any, err error) {
	if err != nil {
		plugin.WriteK8sError(w, err)
		return
	}
	plugin.WriteJSON(w, v)
}

// respondOK answers a cluster mutation that returns no data.
func respondOK(w http.ResponseWriter, err error) {
	if err != nil {
		plugin.WriteK8sError(w, err)
		return
	}
	writeOK(w)
}
