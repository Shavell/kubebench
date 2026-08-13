package api

import (
	"net/http"
	"strconv"

	"kubebench/internal/kube"
)

func (s *Server) handleNodes(w http.ResponseWriter, r *http.Request, rt *kube.Runtime) {
	out, err := rt.ListNodes(r.Context())
	if err != nil {
		writeK8sErr(w, err)
		return
	}
	writeJSON(w, out)
}

func (s *Server) handleNamespaces(w http.ResponseWriter, r *http.Request, rt *kube.Runtime) {
	out, err := rt.ListNamespaces(r.Context())
	if err != nil {
		writeK8sErr(w, err)
		return
	}
	writeJSON(w, out)
}

func (s *Server) handlePods(w http.ResponseWriter, r *http.Request, rt *kube.Runtime) {
	out, err := rt.ListPods(r.Context(), r.URL.Query().Get("ns"))
	if err != nil {
		writeK8sErr(w, err)
		return
	}
	writeJSON(w, out)
}

func (s *Server) handleDeployments(w http.ResponseWriter, r *http.Request, rt *kube.Runtime) {
	out, err := rt.ListDeployments(r.Context(), r.URL.Query().Get("ns"))
	if err != nil {
		writeK8sErr(w, err)
		return
	}
	writeJSON(w, out)
}

func (s *Server) handleServices(w http.ResponseWriter, r *http.Request, rt *kube.Runtime) {
	out, err := rt.ListServices(r.Context(), r.URL.Query().Get("ns"))
	if err != nil {
		writeK8sErr(w, err)
		return
	}
	writeJSON(w, out)
}

func (s *Server) handleYAML(w http.ResponseWriter, r *http.Request, rt *kube.Runtime) {
	q := r.URL.Query()
	text, err := rt.ObjectYAML(r.Context(), q.Get("kind"), q.Get("ns"), q.Get("name"))
	if err != nil {
		writeK8sErr(w, err)
		return
	}
	writeJSON(w, map[string]string{"yaml": text})
}

func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request, rt *kube.Runtime) {
	q := r.URL.Query()
	out, err := rt.ListEvents(r.Context(), q.Get("ns"), q.Get("name"))
	if err != nil {
		writeK8sErr(w, err)
		return
	}
	writeJSON(w, out)
}

func (s *Server) handleLogs(w http.ResponseWriter, r *http.Request, rt *kube.Runtime) {
	q := r.URL.Query()
	out, err := rt.PodLogs(r.Context(), q.Get("ns"), q.Get("pod"), q.Get("container"))
	if err != nil {
		writeK8sErr(w, err)
		return
	}
	writeJSON(w, out)
}

func (s *Server) handleDeletePod(w http.ResponseWriter, r *http.Request, rt *kube.Runtime) {
	q := r.URL.Query()
	if err := rt.DeletePod(r.Context(), q.Get("ns"), q.Get("name")); err != nil {
		writeK8sErr(w, err)
		return
	}
	writeJSON(w, map[string]bool{"ok": true})
}

func (s *Server) handleScale(w http.ResponseWriter, r *http.Request, rt *kube.Runtime) {
	q := r.URL.Query()
	replicas, _ := strconv.Atoi(q.Get("replicas"))
	if err := rt.ScaleDeployment(r.Context(), q.Get("ns"), q.Get("name"), replicas); err != nil {
		writeK8sErr(w, err)
		return
	}
	writeJSON(w, map[string]bool{"ok": true})
}

func (s *Server) handleRestart(w http.ResponseWriter, r *http.Request, rt *kube.Runtime) {
	q := r.URL.Query()
	if err := rt.RestartDeployment(r.Context(), q.Get("ns"), q.Get("name")); err != nil {
		writeK8sErr(w, err)
		return
	}
	writeJSON(w, map[string]bool{"ok": true})
}

func (s *Server) handleExec(w http.ResponseWriter, r *http.Request, rt *kube.Runtime) {
	q := r.URL.Query()
	out, err := rt.Exec(r.Context(), q.Get("ns"), q.Get("pod"), q.Get("container"), q.Get("cmd"))
	if err != nil {
		writeK8sErr(w, err)
		return
	}
	writeJSON(w, out)
}
