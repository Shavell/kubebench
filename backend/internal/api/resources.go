package api

import (
	"net/http"
	"net/url"
	"strconv"

	"kubebench/internal/kube"
	"kubebench/internal/plugin"
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

func (s *Server) handleYAML(w http.ResponseWriter, r *http.Request, c *call) {
	q := r.URL.Query()
	text, err := c.rt.ObjectYAML(r.Context(), q.Get("kind"), q.Get("ns"), q.Get("name"))
	if err != nil {
		plugin.WriteK8sError(w, err)
		return
	}
	plugin.WriteJSON(w, map[string]string{"yaml": text})
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
			return opts, badRequest("tail must be a positive number of lines")
		}
		opts.TailLines = min(n, maxLogTail)
	}
	if raw := q.Get("previous"); raw != "" {
		previous, err := strconv.ParseBool(raw)
		if err != nil {
			return opts, badRequest("previous must be true or false")
		}
		opts.Previous = previous
	}
	if raw := q.Get("sinceSeconds"); raw != "" {
		n, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || n < 1 {
			return opts, badRequest("sinceSeconds must be a positive number")
		}
		opts.SinceSeconds = n
	}
	return opts, nil
}

func (s *Server) handleDeletePod(w http.ResponseWriter, r *http.Request, c *call) {
	ns, name, err := objectParams(r.URL.Query())
	if err != nil {
		writeErr(w, err)
		return
	}
	respondOK(w, c.rt.DeletePod(r.Context(), ns, name))
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
		writeErr(w, badRequest("replicas must be a number of at least 0"))
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
	respondOK(w, c.rt.RestartDeployment(r.Context(), ns, name))
}

func (s *Server) handleExec(w http.ResponseWriter, r *http.Request, c *call) {
	q := r.URL.Query()
	ns, pod := q.Get("ns"), q.Get("pod")
	if ns == "" || pod == "" || q.Get("cmd") == "" {
		writeErr(w, badRequest("ns, pod and cmd are required"))
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
		return badRequest("unknown resource %q", key)
	}
	return nil
}

// objectParams returns the required ns and name of a namespaced object.
func objectParams(q url.Values) (ns, name string, err error) {
	ns, name = q.Get("ns"), q.Get("name")
	if ns == "" || name == "" {
		return "", "", badRequest("ns and name are required")
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
