package api

import (
	"net/http"

	"kubebench/internal/plugin"
	"kubebench/internal/store"
)

// mountPlugins registers every built-in plugin's routes.
func (s *Server) mountPlugins() {
	for _, p := range s.plugins {
		id := p.Manifest().ID
		if !plugin.ValidID(id) {
			panic("api: invalid plugin id " + id)
		}
		p.Routes(pluginRouter{s: s, p: p, id: id})
	}
}

// pluginRouter mounts a plugin's routes under /api/plugins/<id>/ through
// the central register, so they get the core gates plus the plugin's
// enabled and Detect checks.
type pluginRouter struct {
	s  *Server
	p  plugin.Plugin
	id string
}

func (pr pluginRouter) Handle(method, path string, a plugin.Access, h plugin.HandlerFunc) {
	switch method {
	case http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
	default:
		panic("api: plugin " + pr.id + ": unsupported method " + method)
	}
	if len(path) < 2 || path[0] != '/' {
		panic("api: plugin " + pr.id + ": route path must start with / and name a resource: " + path)
	}
	level, ok := pluginLevels[a.Level()]
	if !ok {
		panic("api: plugin " + pr.id + ": invalid access for " + method + " " + path)
	}
	if a.IsDryRunnable() {
		level |= allowDryRun // register refuses it on anything but write and exec
	}
	kv := pr.s.store.PluginKV(pr.id)
	pr.s.register(method+" /api/plugins/"+pr.id+path, level, pr.p, func(w http.ResponseWriter, r *http.Request, c *call) {
		h(w, r, &plugin.Context{
			Runtime: c.rt, User: c.user, Role: c.role, Store: pr.s.store, KV: kv,
			Audit: pr.s.auditFunc(c), SetAudit: c.setAudit, Publish: pr.s.board.Publish,
		})
	})
}

var pluginLevels = map[plugin.Access]access{
	plugin.AccessRead:  accessRead,
	plugin.AccessWrite: accessWrite,
	plugin.AccessExec:  accessExec,
	plugin.AccessAdmin: accessAdmin,

	// Shared, KubeBench-side data about the cluster (§3.2).
	plugin.AccessContribute: accessContribute,
}

// auditFunc is plugin.Context.Audit: extra rows, not tied to a response.
func (s *Server) auditFunc(c *call) func(action, target, detail string) {
	return func(action, target, detail string) {
		s.writeAudit(store.AuditEntry{
			Username: c.user.Username, Cluster: c.clusterName(),
			Action: action, Target: target, Detail: detail,
		})
	}
}
