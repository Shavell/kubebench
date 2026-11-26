package api

import (
	"context"
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
	// accessContribute is read + operator/admin on a cluster that may be
	// read-only; mutations audited. Plugin routes only: KubeBench-side
	// data that other users of the cluster see, such as shared notes.
	accessContribute
	// accessBrowse opens the service browser (§7.14): read + operator/
	// admin; allowed on read-only clusters, where the proxy then sends
	// only GET and HEAD. Audited.
	accessBrowse

	// allowDryRun marks a write or exec route whose handler performs a dry
	// run whenever plugin.IsDryRun(r): such requests need only read access
	// and are not audited.
	allowDryRun access = 1 << 8
	// longRunning exempts a route from the request timeout
	// (config.RequestTimeout): streams, terminals and operations that
	// bound their own duration.
	longRunning access = 1 << 9
)

func (a access) level() access { return a &^ (allowDryRun | longRunning) }

// bounded reports whether requests of a route with access a from plugin p
// run under the request timeout: core routes that work on a cluster,
// unless marked longRunning. Plugin routes bound their own work (Helm
// operations carry their own timeouts).
func (a access) bounded(p plugin.Plugin) bool {
	level := a.level()
	return p == nil && a&longRunning == 0 && level >= accessRead && level != accessAdmin
}

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

// setAuditParts records the request as one row per detail (see
// auditParts), all with the same action, target and status: for handlers
// whose record does not fit the size of one row.
func (c *call) setAuditParts(details []string) {
	if c.audit == nil || len(details) == 0 {
		return
	}
	c.audit.detail, c.audit.more = details[0], details[1:]
}

// setAuditItems records fields plus the list items under key, across as
// many rows as the items need (see auditParts), so that a long list (one
// entry per cluster, label or taint) is never cut off.
func (c *call) setAuditItems(fields map[string]any, key string, items []string) {
	c.setAuditParts(auditParts(fields, key, items))
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
	bounded := a.bounded(p)
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
		if err := crossSiteError(r, level); err != nil {
			writeErr(w, err)
			return
		}
		if err := s.authenticate(r, c); err != nil {
			writeErr(w, err)
			return
		}
		effective := level
		// The router and the handler must agree on what a dry run is: a
		// misspelt flag would otherwise make a real change. It is refused
		// once the caller passed the gate of a real change.
		var dryRunErr error
		if dryRunnable {
			var dryRun bool
			dryRun, dryRunErr = flagParam(r.URL.Query(), "dryRun")
			if dryRun {
				effective = accessRead
			}
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
		if dryRunErr != nil {
			writeErr(w, dryRunErr)
			return
		}
		if err := c.audit.recordable(); err != nil {
			writeErr(w, err)
			return
		}
		if bounded && s.cfg.RequestTimeout > 0 {
			// A hung API server must not pin the request: its calls end
			// with context.DeadlineExceeded, reported as 504.
			ctx, cancel := context.WithTimeout(r.Context(), s.cfg.RequestTimeout)
			defer cancel()
			r = r.WithContext(ctx)
		}
		h(w, r, c)
	})
}

// authenticate resolves the session cookie to its user.
func (s *Server) authenticate(r *http.Request, c *call) error {
	cookie, err := r.Cookie(sessionCookie)
	if err != nil || cookie.Value == "" {
		return fail(http.StatusUnauthorized, plugin.CodeUnauthenticated, reasonSignIn, nil)
	}
	user, err := s.store.UserBySession(cookie.Value)
	if errors.Is(err, store.ErrNotFound) {
		return fail(http.StatusUnauthorized, plugin.CodeSessionExpired, reasonSessionExpired, nil)
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
		return fail(http.StatusForbidden, plugin.CodePasswordChangeRequired, reasonPasswordChange, nil)
	}
	if level == accessAdmin && !c.user.IsAdmin() {
		return fail(http.StatusForbidden, plugin.CodeAdminRequired, reasonAdminRequired, nil)
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
	if level == accessContribute {
		if err := contributionDenied(c.role, c.cluster); err != nil {
			return err
		}
	}
	if level == accessBrowse {
		if err := browseDenied(c.role, c.cluster); err != nil {
			return err
		}
	}
	if p != nil {
		if st := s.detect.status(p, c.rt); !st.Available {
			return pluginUnavailable(st)
		}
	}
	return nil
}

// audited reports whether requests at level with method are written to the
// audit log: every write and exec request, and admin mutations.
func audited(level access, method string) bool {
	switch level {
	case accessWrite, accessExec, accessBrowse:
		return true
	case accessAdmin, accessContribute:
		return method != http.MethodGet && method != http.MethodHead
	}
	return false
}
