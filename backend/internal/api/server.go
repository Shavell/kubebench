// Package api exposes the HTTP contract the frontend consumes, plus
// authentication and user management.
package api

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"

	apierrors "k8s.io/apimachinery/pkg/api/errors"

	"kubebench/internal/kube"
	"kubebench/internal/store"
)

type Server struct {
	Store   *store.Store
	Manager *kube.Manager
}

type ctxKey int

const userKey ctxKey = 0

func (s *Server) Routes(mux *http.ServeMux) {
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("POST /api/auth/login", s.handleLogin)
	mux.HandleFunc("POST /api/auth/logout", s.handleLogout)

	authed := func(pattern string, h http.HandlerFunc) {
		mux.Handle(pattern, s.requireSession(h))
	}
	admin := func(pattern string, h http.HandlerFunc) {
		mux.Handle(pattern, s.requireSession(s.requireAdmin(h)))
	}

	authed("GET /api/auth/me", s.handleMe)
	authed("GET /api/info", s.handleInfo)

	admin("GET /api/users", s.handleListUsers)
	admin("POST /api/users", s.handleCreateUser)
	admin("DELETE /api/users/{id}", s.handleDeleteUser)
	admin("PUT /api/users/{id}/clusters", s.handleAssignClusters)

	authed("GET /api/clusters", s.handleListClusters)
	admin("POST /api/clusters", s.handleAddCluster)
	admin("DELETE /api/clusters", s.handleDeleteCluster)
	authed("POST /api/clusters/activate", s.handleActivateCluster)

	authed("GET /api/nodes", s.withRuntime(s.handleNodes))
	authed("GET /api/namespaces", s.withRuntime(s.handleNamespaces))
	authed("GET /api/pods", s.withRuntime(s.handlePods))
	authed("DELETE /api/pods", s.withRuntime(s.handleDeletePod))
	authed("GET /api/deployments", s.withRuntime(s.handleDeployments))
	authed("POST /api/deployments/scale", s.withRuntime(s.handleScale))
	authed("POST /api/deployments/restart", s.withRuntime(s.handleRestart))
	authed("GET /api/services", s.withRuntime(s.handleServices))
	authed("GET /api/yaml", s.withRuntime(s.handleYAML))
	authed("GET /api/events", s.withRuntime(s.handleEvents))
	authed("GET /api/logs", s.withRuntime(s.handleLogs))
	authed("POST /api/exec", s.withRuntime(s.handleExec))

	authed("GET /api/metrics/sources", s.withRuntime(s.handleMetricSources))
	authed("GET /api/metrics/overview", s.withRuntime(s.handleOverview))
	authed("GET /api/metrics/allocation", s.withRuntime(s.handleAllocation))
	authed("GET /api/metrics/cluster", s.withRuntime(s.handleClusterSeries))
	authed("GET /api/metrics/node", s.withRuntime(s.handleNodeSeries))
	authed("GET /api/metrics/pod", s.withRuntime(s.handlePodSeries))

	authed("GET /api/crds", s.withRuntime(s.handleCrds))
	authed("GET /api/crds/instances", s.withRuntime(s.handleCrdInstances))

	authed("GET /api/helm/releases", s.withRuntime(s.handleHelmReleases))
	authed("GET /api/helm/repos", s.withRuntime(s.handleHelmRepos))
	authed("GET /api/helm/values", s.withRuntime(s.handleHelmValues))
	authed("POST /api/helm/rollback", s.withRuntime(s.handleHelmRollback))
	authed("POST /api/helm/uninstall", s.withRuntime(s.handleHelmUninstall))
}

// --- plumbing ---

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, err error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
}

// writeK8sErr maps Kubernetes API errors to their status codes so RBAC
// failures surface verbatim in the UI.
func writeK8sErr(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	var apiStatus apierrors.APIStatus
	if errors.As(err, &apiStatus) && apiStatus.Status().Code > 0 {
		status = int(apiStatus.Status().Code)
	}
	writeErr(w, status, err)
	log.Printf("[api] %d: %v", status, err)
}

func currentUser(r *http.Request) *store.User {
	u, _ := r.Context().Value(userKey).(*store.User)
	return u
}

func (s *Server) requireSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("kubebench_session")
		if err != nil {
			writeErr(w, http.StatusUnauthorized, errors.New("not authenticated"))
			return
		}
		user, err := s.Store.UserBySession(cookie.Value)
		if err != nil {
			writeErr(w, http.StatusUnauthorized, errors.New("session expired"))
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), userKey, user)))
	})
}

func (s *Server) requireAdmin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if currentUser(r).Role != "admin" {
			writeErr(w, http.StatusForbidden, errors.New("admin role required"))
			return
		}
		next(w, r)
	}
}

type runtimeHandler func(w http.ResponseWriter, r *http.Request, rt *kube.Runtime)

// withRuntime resolves the session user's active cluster to a Runtime.
func (s *Server) withRuntime(h runtimeHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rt, err := s.runtimeFor(currentUser(r))
		if err != nil {
			writeErr(w, http.StatusServiceUnavailable, err)
			return
		}
		h(w, r, rt)
	}
}

func (s *Server) runtimeFor(user *store.User) (*kube.Runtime, error) {
	clusterID := user.ActiveClusterID
	if clusterID == nil {
		clusters, err := s.Store.ListClusters(user)
		if err != nil {
			return nil, err
		}
		for i := range clusters {
			if clusters[i].Source != "address" {
				if err := s.Store.SetActiveCluster(user.ID, clusters[i].ID); err != nil {
					return nil, err
				}
				clusterID = &clusters[i].ID
				break
			}
		}
		if clusterID == nil {
			return nil, errors.New("no cluster assigned — ask an admin to assign one")
		}
	}
	if ok, err := s.Store.CanAccessCluster(user, *clusterID); err != nil || !ok {
		return nil, errors.New("no access to the active cluster")
	}
	cluster, err := s.Store.GetCluster(*clusterID)
	if err != nil {
		return nil, err
	}
	return s.Manager.Runtime(cluster)
}
