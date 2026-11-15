// Package api serves the HTTP API the frontend consumes
// (docs/design-v0.5.md §3-§4): authentication, users, the cluster
// inventory, the audit log, plugins and the Kubernetes endpoints of each
// user's active cluster. Every route is registered through Server.handle
// with the access level it requires; see router.go.
package api

import (
	"net/http"
	"slices"
	"strings"
	"time"

	"kubebench/internal/config"
	"kubebench/internal/kube"
	"kubebench/internal/plugin"
	"kubebench/internal/store"
)

// BuildInfo identifies the running binary (§1).
type BuildInfo struct {
	Version string `json:"version"`
	Commit  string `json:"commit"`
	Date    string `json:"date"`
}

// Server holds the API's dependencies. Create it with New.
type Server struct {
	cfg     *config.Config
	store   *store.Store
	manager *kube.Manager
	build   BuildInfo
	plugins []plugin.Plugin // built-in backend plugins, ordered by id

	mux     *http.ServeMux
	limiter *loginLimiter
	detect  *detectCache
	streams streamSettings // log streams and terminals
	// streamLimits counts each user's open log streams and terminals.
	streamLimits *streamLimiter
}

// New wires every route. plugins are the built-in backend plugins to mount
// (plugin.All() in production); their ids must be valid and unique.
func New(cfg *config.Config, st *store.Store, manager *kube.Manager, build BuildInfo, plugins []plugin.Plugin) *Server {
	s := &Server{
		cfg:     cfg,
		store:   st,
		manager: manager,
		build:   build,
		plugins: slices.Clone(plugins),
		mux:     http.NewServeMux(),
		limiter: newLoginLimiter(time.Now),
		detect:  newDetectCache(time.Now),
		streams: defaultStreamSettings(),

		streamLimits: newStreamLimiter(),
	}
	slices.SortFunc(s.plugins, func(a, b plugin.Plugin) int {
		return strings.Compare(a.Manifest().ID, b.Manifest().ID)
	})
	s.routes()
	return s
}

// Handler is the complete HTTP handler: the API, external plugin files
// and, when spa is non-nil, the single-page app for every other GET.
// Cross-origin mutations are rejected for all of them.
func (s *Server) Handler(spa http.Handler) http.Handler {
	return s.checkOrigin(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if spa != nil && (r.Method == http.MethodGet || r.Method == http.MethodHead) {
			if _, pattern := s.mux.Handler(r); pattern == "" {
				spa.ServeHTTP(w, r)
				return
			}
		}
		s.mux.ServeHTTP(w, r)
	}))
}

// handleUnknownAPI answers /api/ paths no route matches, in JSON: 405 with
// an Allow header when the path exists for other methods, else 404.
func (s *Server) handleUnknownAPI(w http.ResponseWriter, r *http.Request) {
	var allowed []string
	for _, method := range []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete} {
		probe := r.Clone(r.Context())
		probe.Method = method
		if _, pattern := s.mux.Handler(probe); pattern != "" && pattern != "/api/" {
			allowed = append(allowed, method)
		}
	}
	if len(allowed) == 0 {
		writeErr(w, errorf(http.StatusNotFound, plugin.CodeNotFound, "no API endpoint %s", r.URL.Path))
		return
	}
	w.Header().Set("Allow", strings.Join(allowed, ", "))
	writeErr(w, errorf(http.StatusMethodNotAllowed, plugin.CodeBadRequest, "%s is not allowed on %s", r.Method, r.URL.Path))
}
