package api

import (
	"errors"
	"net/http"
	"strings"

	"kubebench/internal/kube"
	"kubebench/internal/plugin"
	"kubebench/internal/store"
)

// access is the permission a route requires (docs/design-v0.5.md §3.2).
type access int

const (
	accessPublic access = iota // no session
	accessAuthed               // any session
	accessRead                 // session + active cluster + any role on it
	accessWrite                // read + operator/admin + cluster not read-only; audited
	accessExec                 // same gate as write; audited
	accessAdmin                // global admin; mutations audited

	// allowDryRun marks a write or exec route whose handler performs a dry
	// run whenever plugin.IsDryRun(r): such requests need only read access
	// and are not audited.
	allowDryRun access = 1 << 8
)

func (a access) level() access { return a &^ allowDryRun }

// handlerFunc serves a request after the router authorized it.
type handlerFunc func(w http.ResponseWriter, r *http.Request, c *call)

// call is what the router resolved for one request.
type call struct {
	user    *store.User    // nil on public routes
	token   string         // the caller's session token
	cluster *store.Cluster // the active cluster, on read/write/exec and plugin routes
	role    string         // the caller's effective role on cluster
	rt      *kube.Runtime  // the runtime of cluster
	audit   *auditWriter   // set when the request is audited
}

// setAudit refines the audit row of an audited request, e.g. with a target
// that only the request body names. Empty values keep the defaults the
// router derived from the URL.
func (c *call) setAudit(target, detail string) {
	if c.audit == nil {
		return
	}
	if target != "" {
		c.audit.target = target
	}
	if detail != "" {
		c.audit.detail = detail
	}
}

// handle registers a core route.
func (s *Server) handle(pattern string, a access, h handlerFunc) { s.register(pattern, a, nil, h) }

// register is the only way routes reach the mux, so every endpoint gets
// the same session, permission, audit and error handling. pattern is a
// ServeMux pattern with a method ("POST /api/deployments/scale"). For
// plugin routes p adds the enabled and Detect gates; they always run on
// the caller's active cluster.
func (s *Server) register(pattern string, a access, p plugin.Plugin, h handlerFunc) {
	method, path, ok := strings.Cut(pattern, " ")
	if !ok {
		panic("api: route pattern without a method: " + pattern)
	}
	level := a.level()
	dryRunnable := a&allowDryRun != 0
	if dryRunnable && level != accessWrite && level != accessExec {
		panic("api: only write and exec routes can allow dry runs: " + pattern)
	}
	target := auditTarget{kind: targetKind(path, p), wildcards: wildcardNames(path)}
	target.kindParam = p == nil && kindParamKinds[target.kind]
	if kind, ok := auditKinds[pattern]; ok {
		target.kind = kind
	}
	action := pattern
	if name, ok := auditActions[pattern]; ok {
		action = name
	}

	s.mux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
		c := &call{}
		if level == accessPublic {
			h(w, r, c)
			return
		}
		if err := s.authenticate(r, c); err != nil {
			writeErr(w, err)
			return
		}
		effective := level
		if dryRunnable && plugin.IsDryRun(r) {
			effective = accessRead
		}
		if audited(effective, method) {
			aw := s.newAuditWriter(w, r, c, action, target)
			w, c.audit = aw, aw
			defer func() {
				if v := recover(); v != nil {
					aw.status = http.StatusInternalServerError
					aw.finish()
					panic(v)
				}
				aw.finish()
			}()
		}
		if err := s.authorize(c, effective, p); err != nil {
			writeErr(w, err)
			return
		}
		if err := c.audit.recordable(); err != nil {
			writeErr(w, err)
			return
		}
		h(w, r, c)
	})
}

// authenticate resolves the session cookie to its user.
func (s *Server) authenticate(r *http.Request, c *call) error {
	cookie, err := r.Cookie(sessionCookie)
	if err != nil || cookie.Value == "" {
		return errorf(http.StatusUnauthorized, plugin.CodeUnauthenticated, "sign in to continue")
	}
	user, err := s.store.UserBySession(cookie.Value)
	if errors.Is(err, store.ErrNotFound) {
		return errorf(http.StatusUnauthorized, plugin.CodeSessionExpired, "your session has expired; sign in again")
	}
	if err != nil {
		return err
	}
	c.user, c.token = user, cookie.Value
	return nil
}

// authorize applies the §3 gates in order and, when the route works on a
// cluster, fills in c's cluster, role and runtime. Every denial happens
// here, before the handler runs, so a denied request never reaches the
// cluster.
func (s *Server) authorize(c *call, level access, p plugin.Plugin) error {
	if level >= accessRead && c.user.MustChangePassword {
		return errorf(http.StatusForbidden, plugin.CodePasswordChangeRequired, "change your password to continue")
	}
	if level == accessAdmin && !c.user.IsAdmin() {
		return errorf(http.StatusForbidden, plugin.CodeAdminRequired, "this action needs an administrator")
	}
	if p != nil {
		if err := s.checkPluginEnabled(p); err != nil {
			return err
		}
	} else if level == accessAuthed || level == accessAdmin {
		return nil // KubeBench's own state; no cluster involved
	}
	if err := s.resolveCluster(c); err != nil {
		return err
	}
	if level == accessWrite || level == accessExec {
		if err := mutationDenied(c.role, c.cluster); err != nil {
			return err
		}
	}
	if p != nil {
		if st := s.detect.status(p, c.rt); !st.Available {
			reason := st.Reason
			if reason == "" {
				reason = "the plugin is not available on this cluster"
			}
			return errorf(http.StatusServiceUnavailable, plugin.CodePluginUnavailable, "%s", reason)
		}
	}
	return nil
}

// audited reports whether requests at level with method are written to the
// audit log: every write and exec request, and admin mutations.
func audited(level access, method string) bool {
	switch level {
	case accessWrite, accessExec:
		return true
	case accessAdmin:
		return method != http.MethodGet && method != http.MethodHead
	}
	return false
}
