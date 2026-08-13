package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"

	"k8s.io/client-go/tools/clientcmd"

	"kubebench/internal/store"
)

type clusterView struct {
	ID      int64  `json:"id"`
	Name    string `json:"name"`
	Source  string `json:"source"`
	Active  bool   `json:"active"`
	Context string `json:"context,omitempty"`
	Server  string `json:"server,omitempty"`
	Address string `json:"address,omitempty"`
	Error   string `json:"error,omitempty"`
	// kubeconfigPath kept for frontend compatibility; shows the source label.
	KubeconfigPath string `json:"kubeconfigPath,omitempty"`
}

func describeCluster(c *store.Cluster, activeID *int64) clusterView {
	view := clusterView{
		ID: c.ID, Name: c.Name, Source: c.Source, Address: c.Address, Context: c.Context,
		Active: activeID != nil && *activeID == c.ID,
	}
	switch c.Source {
	case "incluster":
		view.KubeconfigPath = "(in-cluster service account)"
		view.Server = "kubernetes.default.svc"
	case "address":
		view.Server = c.Address
	default:
		view.KubeconfigPath = "(stored kubeconfig)"
		if cfg, err := clientcmd.Load([]byte(c.Kubeconfig)); err == nil {
			contextName := c.Context
			if contextName == "" {
				contextName = cfg.CurrentContext
			}
			view.Context = contextName
			if ctx, ok := cfg.Contexts[contextName]; ok {
				if cluster, ok := cfg.Clusters[ctx.Cluster]; ok {
					view.Server = cluster.Server
				}
			}
		} else {
			view.Error = "unparseable kubeconfig"
		}
	}
	return view
}

func (s *Server) handleListClusters(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	clusters, err := s.Store.ListClusters(user)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	out := []clusterView{}
	for i := range clusters {
		out = append(out, describeCluster(&clusters[i], user.ActiveClusterID))
	}
	writeJSON(w, out)
}

func (s *Server) handleAddCluster(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name    string `json:"name"`
		Content string `json:"content"`
		Path    string `json:"path"`
		Address string `json:"address"`
		Context string `json:"context"`
	}
	// Accept both JSON body and legacy query params.
	if r.Header.Get("Content-Type") == "application/json" {
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeErr(w, http.StatusBadRequest, err)
			return
		}
	} else {
		q := r.URL.Query()
		body.Name, body.Path, body.Address = q.Get("name"), q.Get("path"), q.Get("address")
	}
	if body.Name == "" {
		writeErr(w, http.StatusBadRequest, errors.New("name is required"))
		return
	}

	cluster := store.Cluster{Name: body.Name, Context: body.Context}
	switch {
	case body.Content != "":
		cluster.Source = "kubeconfig"
		cluster.Kubeconfig = body.Content
	case body.Path != "":
		content, err := os.ReadFile(body.Path)
		if err != nil {
			writeErr(w, http.StatusBadRequest, err)
			return
		}
		cluster.Source = "kubeconfig"
		cluster.Kubeconfig = string(content)
	case body.Address != "":
		cluster.Source = "address"
		cluster.Address = body.Address
	default:
		writeErr(w, http.StatusBadRequest, errors.New("kubeconfig content, path or API address is required"))
		return
	}

	if cluster.Source == "kubeconfig" {
		if _, err := clientcmd.Load([]byte(cluster.Kubeconfig)); err != nil {
			writeErr(w, http.StatusBadRequest, errors.New("invalid kubeconfig: "+err.Error()))
			return
		}
	}
	if _, err := s.Store.AddCluster(cluster); err != nil {
		writeErr(w, http.StatusConflict, err)
		return
	}
	writeJSON(w, map[string]bool{"ok": true})
}

func (s *Server) handleDeleteCluster(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("name")
	cluster, err := s.Store.ClusterByName(name)
	if err != nil {
		writeErr(w, http.StatusNotFound, err)
		return
	}
	s.Manager.Drop(cluster.ID)
	if err := s.Store.DeleteCluster(cluster.ID); err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, map[string]bool{"ok": true})
}

func (s *Server) handleActivateCluster(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	cluster, err := s.Store.ClusterByName(r.URL.Query().Get("name"))
	if err != nil {
		writeErr(w, http.StatusNotFound, err)
		return
	}
	if cluster.Source == "address" {
		writeErr(w, http.StatusBadRequest, errors.New("address-only entries cannot be activated"))
		return
	}
	if ok, err := s.Store.CanAccessCluster(user, cluster.ID); err != nil || !ok {
		writeErr(w, http.StatusForbidden, errors.New("no access to this cluster"))
		return
	}
	if _, err := s.Manager.Runtime(cluster); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	if err := s.Store.SetActiveCluster(user.ID, cluster.ID); err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, map[string]bool{"ok": true})
}
