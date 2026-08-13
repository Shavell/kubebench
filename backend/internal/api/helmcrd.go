package api

import (
	"errors"
	"net/http"
	"strconv"

	"kubebench/internal/kube"
)

func (s *Server) handleCrds(w http.ResponseWriter, r *http.Request, rt *kube.Runtime) {
	out, err := rt.ListCrds(r.Context())
	if err != nil {
		writeK8sErr(w, err)
		return
	}
	writeJSON(w, out)
}

func (s *Server) handleCrdInstances(w http.ResponseWriter, r *http.Request, rt *kube.Runtime) {
	out, err := rt.ListCustomResources(r.Context(), r.URL.Query().Get("crd"))
	if err != nil {
		writeK8sErr(w, err)
		return
	}
	writeJSON(w, out)
}

func requireHelm(rt *kube.Runtime) error {
	if !rt.Caps().Helm {
		return errors.New("helm is not available for this cluster")
	}
	return nil
}

func (s *Server) handleHelmReleases(w http.ResponseWriter, _ *http.Request, rt *kube.Runtime) {
	if err := requireHelm(rt); err != nil {
		writeErr(w, http.StatusNotImplemented, err)
		return
	}
	out, err := rt.Helm.ListReleases()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, out)
}

func (s *Server) handleHelmRepos(w http.ResponseWriter, _ *http.Request, rt *kube.Runtime) {
	if err := requireHelm(rt); err != nil {
		writeErr(w, http.StatusNotImplemented, err)
		return
	}
	out, err := rt.Helm.ListRepositories()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, out)
}

func (s *Server) handleHelmValues(w http.ResponseWriter, r *http.Request, rt *kube.Runtime) {
	q := r.URL.Query()
	text, err := rt.Helm.GetValuesYAML(q.Get("name"), q.Get("ns"))
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, map[string]string{"yaml": text})
}

func (s *Server) handleHelmRollback(w http.ResponseWriter, r *http.Request, rt *kube.Runtime) {
	q := r.URL.Query()
	revision, _ := strconv.Atoi(q.Get("revision"))
	if err := rt.Helm.Rollback(q.Get("name"), q.Get("ns"), revision); err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, map[string]bool{"ok": true})
}

func (s *Server) handleHelmUninstall(w http.ResponseWriter, r *http.Request, rt *kube.Runtime) {
	q := r.URL.Query()
	if err := rt.Helm.Uninstall(q.Get("name"), q.Get("ns")); err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, map[string]bool{"ok": true})
}
