package api

import (
	"net/http"
	"strconv"

	"kubebench/internal/kube"
	"kubebench/internal/plugin"
)

func (s *Server) handleCrds(w http.ResponseWriter, r *http.Request, c *call) {
	out, err := c.rt.ListCrds(r.Context())
	writeResult(w, out, err)
}

func (s *Server) handleCrdInstances(w http.ResponseWriter, r *http.Request, c *call) {
	out, err := c.rt.ListCustomResources(r.Context(), r.URL.Query().Get("crd"))
	writeResult(w, out, err)
}

// requireHelm refuses Helm calls on clusters where the probe found no
// access to the release secrets. Like /api/info it waits for the probe of
// a fresh runtime (after a restart or a cluster edit), so Helm is not
// refused merely because the probe is still running.
func requireHelm(r *http.Request, rt *kube.Runtime) error {
	if !probedCaps(r.Context(), rt).Helm {
		return unavailable("Helm is not available on this cluster (no access to release secrets)")
	}
	return nil
}

func (s *Server) handleHelmReleases(w http.ResponseWriter, r *http.Request, c *call) {
	if err := requireHelm(r, c.rt); err != nil {
		writeErr(w, err)
		return
	}
	out, err := c.rt.Helm.ListReleases()
	writeResult(w, out, err)
}

func (s *Server) handleHelmRepos(w http.ResponseWriter, r *http.Request, c *call) {
	if err := requireHelm(r, c.rt); err != nil {
		writeErr(w, err)
		return
	}
	out, err := c.rt.Helm.ListRepositories()
	writeResult(w, out, err)
}

func (s *Server) handleHelmValues(w http.ResponseWriter, r *http.Request, c *call) {
	if err := requireHelm(r, c.rt); err != nil {
		writeErr(w, err)
		return
	}
	q := r.URL.Query()
	text, err := c.rt.Helm.GetValuesYAML(q.Get("name"), q.Get("ns"))
	if err != nil {
		plugin.WriteK8sError(w, err)
		return
	}
	plugin.WriteJSON(w, map[string]string{"yaml": text})
}

func (s *Server) handleHelmRollback(w http.ResponseWriter, r *http.Request, c *call) {
	q := r.URL.Query()
	ns, name, err := objectParams(q)
	if err != nil {
		writeErr(w, err)
		return
	}
	if err := requireHelm(r, c.rt); err != nil {
		writeErr(w, err)
		return
	}
	revision, err := strconv.Atoi(q.Get("revision"))
	if err != nil || revision < 0 {
		writeErr(w, badRequest("revision must be a revision number (0 = the previous one)"))
		return
	}
	respondOK(w, c.rt.Helm.Rollback(name, ns, revision))
}

func (s *Server) handleHelmUninstall(w http.ResponseWriter, r *http.Request, c *call) {
	ns, name, err := objectParams(r.URL.Query())
	if err != nil {
		writeErr(w, err)
		return
	}
	if err := requireHelm(r, c.rt); err != nil {
		writeErr(w, err)
		return
	}
	respondOK(w, c.rt.Helm.Uninstall(name, ns))
}
