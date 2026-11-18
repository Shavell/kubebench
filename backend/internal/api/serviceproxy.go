package api

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"kubebench/internal/kube"
	"kubebench/internal/plugin"
	"kubebench/internal/reason"
	"kubebench/internal/store"
	"kubebench/internal/svcproxy"
)

// The service browser (docs/design-v0.5.md §7.14): POST /api/proxy/…
// opens a grant for a Service or Pod port, and /proxy/<token>/<path>
// forwards the browser's requests to it through the API server's proxy
// subresource. Proxied requests are authorized by the grant's token
// alone; the session that opened the grant must still be alive and its
// user must still hold the browse access on the grant's cluster.

const proxyPrefix = "/proxy/"

func isProxyPath(p string) bool { return strings.HasPrefix(p, proxyPrefix) }

// proxySessionView is a grant as the frontend sees it
// (src/plugins/service-browser/data/types.ts ProxySession).
type proxySessionView struct {
	Token string `json:"token"`
	// Base is the URL the target's root is served under, without a
	// trailing slash: /proxy/<token> on KubeBench's origin, or absolute on
	// the proxy origin.
	Base      string          `json:"base"`
	Sandboxed bool            `json:"sandboxed"`
	ReadOnly  bool            `json:"readOnly"`
	Target    svcproxy.Target `json:"target"`
	// Navigations are the frame's latest document loads, oldest first
	// (the frontend picks its own frame's from them).
	Navigations []svcproxy.Navigation `json:"navigations"`
}

func (s *Server) proxySessionView(g svcproxy.Grant, readOnly bool) proxySessionView {
	return proxySessionView{
		Token:       g.Token,
		Base:        s.cfg.ProxyOrigin + proxyPrefix + g.Token,
		Sandboxed:   s.cfg.ProxyOrigin == "",
		ReadOnly:    readOnly,
		Target:      g.Target,
		Navigations: g.Navigations(),
	}
}

// handleOpenProxy opens a grant for the Service or Pod port in the path
// (access browse; audited once per grant as service.proxy or pod.proxy).
func (s *Server) handleOpenProxy(kind string) handlerFunc {
	return func(w http.ResponseWriter, r *http.Request, c *call) {
		t := svcproxy.Target{
			Kind:      kind,
			Namespace: r.PathValue("ns"),
			Name:      r.PathValue("name"),
			Port:      r.PathValue("port"),
			Scheme:    r.URL.Query().Get("scheme"),
		}
		if t.Scheme == "" {
			t.Scheme = "http"
		}
		if err := t.Validate(); err != nil {
			writeErr(w, refuse(http.StatusBadRequest, plugin.CodeBadRequest, err))
			return
		}
		detail := map[string]any{"scheme": t.Scheme}
		if c.cluster.ReadOnly {
			detail["readOnly"] = true
		}
		c.setAudit(t.AuditTarget(), auditJSON(detail))
		appOrigin := s.appOrigin(r)
		if err := s.checkProxyOrigin(appOrigin); err != nil {
			writeErr(w, err)
			return
		}
		if err := checkProxyTarget(r.Context(), c.rt, t); err != nil {
			var ae *apiError
			if errors.As(err, &ae) {
				writeErr(w, err)
			} else {
				plugin.WriteK8sError(w, err)
			}
			return
		}
		g, err := s.grants.Issue(svcproxy.Grant{
			Session: c.token, UserID: c.user.ID, ClusterID: c.cluster.ID,
			Target: t, FrameAncestor: appOrigin,
		})
		if err != nil {
			writeErr(w, err)
			return
		}
		plugin.WriteJSON(w, s.proxySessionView(g, c.cluster.ReadOnly))
	}
}

// checkProxyOrigin refuses to open grants on a proxy origin of the same
// site as appOrigin, KubeBench's origin as the browser sees it: cookies
// ignore ports, and a parent domain's cookies reach every subdomain, so a
// proxied page there could plant a kubebench_session cookie that KubeBench
// would read (§7.14).
func (s *Server) checkProxyOrigin(appOrigin string) error {
	if s.cfg.ProxyOrigin == "" {
		return nil
	}
	proxy, err := url.Parse(s.cfg.ProxyOrigin)
	if err != nil {
		return err
	}
	app, err := url.Parse(appOrigin)
	if err != nil || app.Host == "" {
		// 'self': the host is unknown, so it cannot be told apart.
		return fail(http.StatusServiceUnavailable, plugin.CodeCapabilityUnavailable, reasonProxyOriginHost,
			reason.Params{"origin": s.cfg.ProxyOrigin})
	}
	if svcproxy.SameSite(app.Hostname(), proxy.Hostname()) {
		return fail(http.StatusServiceUnavailable, plugin.CodeCapabilityUnavailable, reasonProxySameSite,
			reason.Params{"origin": s.cfg.ProxyOrigin, "app": appOrigin})
	}
	return nil
}

// checkProxyTarget makes sure the object exists and has the port, so that
// a typo is reported here rather than as an API server error page in the
// browser frame.
func checkProxyTarget(ctx context.Context, rt *kube.Runtime, t svcproxy.Target) error {
	switch t.Kind {
	case svcproxy.KindService:
		svc, err := rt.CS.CoreV1().Services(t.Namespace).Get(ctx, t.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		if svc.Spec.Type == corev1.ServiceTypeExternalName {
			return badRequest(reasonExternalName, reason.Params{"namespace": t.Namespace, "name": t.Name, "externalName": svc.Spec.ExternalName})
		}
		if !slices.ContainsFunc(svc.Spec.Ports, func(p corev1.ServicePort) bool {
			return p.Name == t.Port || strconv.Itoa(int(p.Port)) == t.Port
		}) {
			return badRequest(reasonNoServicePort, reason.Params{"namespace": t.Namespace, "name": t.Name, "port": t.Port})
		}
	case svcproxy.KindPod:
		pod, err := rt.CS.CoreV1().Pods(t.Namespace).Get(ctx, t.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		if _, err := strconv.Atoi(t.Port); err == nil {
			return nil // a container may listen on a port it does not declare
		}
		for _, ctr := range pod.Spec.Containers {
			if slices.ContainsFunc(ctr.Ports, func(p corev1.ContainerPort) bool { return p.Name == t.Port }) {
				return nil
			}
		}
		return badRequest(reasonNoPodPort, reason.Params{"namespace": t.Namespace, "name": t.Name, "port": t.Port})
	}
	return nil
}

// handleProxySession describes a grant of the caller's session (authed).
func (s *Server) handleProxySession(w http.ResponseWriter, r *http.Request, c *call) {
	g, ok := s.grants.Get(r.PathValue("token"), c.token)
	if !ok {
		writeErr(w, fail(http.StatusNotFound, plugin.CodeNotFound, reasonBrowserEnded, nil))
		return
	}
	readOnly := false
	if cluster, err := s.store.GetCluster(g.ClusterID); err == nil {
		readOnly = cluster.ReadOnly
	}
	plugin.WriteJSON(w, s.proxySessionView(g, readOnly))
}

// handleCloseProxySession ends a grant of the caller's session (authed),
// when its browser tab closes.
func (s *Server) handleCloseProxySession(w http.ResponseWriter, r *http.Request, c *call) {
	if !s.grants.Revoke(r.PathValue("token"), c.token) {
		writeErr(w, fail(http.StatusNotFound, plugin.CodeNotFound, reasonBrowserEnded, nil))
		return
	}
	writeOK(w)
}

// handleProxyRoot sends /proxy/<token> to /proxy/<token>/, so that the
// target's relative links resolve below the token.
func (s *Server) handleProxyRoot(w http.ResponseWriter, r *http.Request, _ *call) {
	target := proxyPrefix + r.PathValue("token") + "/"
	if r.URL.RawQuery != "" {
		target += "?" + r.URL.RawQuery
	}
	http.Redirect(w, r, target, http.StatusFound)
}

// handleProxy serves /proxy/<token>/<path> (access public: the token is
// the credential).
func (s *Server) handleProxy(w http.ResponseWriter, r *http.Request, _ *call) {
	token := r.PathValue("token")
	sandboxed := !s.onProxyOrigin(r)
	g, ok := s.grants.Use(token)
	if !ok {
		// The frame may show the answer only on KubeBench's own origin.
		ancestor := "'self'"
		if !sandboxed {
			ancestor = ""
		}
		svcproxy.Fail(w, http.StatusNotFound, "this browser session has ended; open the service again from KubeBench", sandboxed, ancestor)
		return
	}
	failPage := func(status int, message string) {
		svcproxy.Fail(w, status, message, sandboxed, g.FrameAncestor)
	}
	base := proxyPrefix + token
	path, err := svcproxy.CleanPath(strings.TrimPrefix(r.URL.EscapedPath(), base))
	if err != nil {
		failPage(http.StatusBadRequest, err.Error())
		return
	}
	cluster, readOnly, err := s.proxyAccess(g)
	if err != nil {
		var ae *apiError
		if errors.As(err, &ae) {
			failPage(ae.status, ae.Error())
		} else {
			failPage(http.StatusInternalServerError, err.Error())
		}
		return
	}
	rt, err := s.manager.Runtime(cluster)
	if err != nil {
		failPage(http.StatusServiceUnavailable, "cluster "+cluster.Name+" is unavailable: "+err.Error())
		return
	}
	transport, apiServer, err := rt.ProxyTransport()
	if err != nil {
		failPage(http.StatusServiceUnavailable, "cluster "+cluster.Name+" is unavailable: "+err.Error())
		return
	}
	ctx, cancel := s.streamContext(r.Context())
	defer cancel()
	svcproxy.Serve(w, r.WithContext(ctx), svcproxy.Request{
		Grant: g, Path: path, Base: base,
		Transport: transport, APIServer: apiServer,
		Sandboxed: sandboxed, ReadOnly: readOnly,
		SessionCookie: sessionCookie,
		Navigated:     func(location string) { s.grants.RecordNavigation(token, location) },
	})
}

// proxyAccess checks, for every proxied request, that the grant's session
// is alive and its user still holds the browse access on the grant's
// cluster; readOnly reports a read-only cluster.
func (s *Server) proxyAccess(g svcproxy.Grant) (cluster *store.Cluster, readOnly bool, err error) {
	user, err := s.store.UserBySession(g.Session)
	if errors.Is(err, store.ErrNotFound) {
		return nil, false, fail(http.StatusUnauthorized, plugin.CodeSessionExpired, reasonKubeBenchEnded, nil)
	}
	if err != nil {
		return nil, false, err
	}
	if user.MustChangePassword {
		return nil, false, fail(http.StatusForbidden, plugin.CodePasswordChangeRequired, reasonPasswordChange, nil)
	}
	cluster, err = s.store.GetCluster(g.ClusterID)
	if errors.Is(err, store.ErrNotFound) {
		return nil, false, fail(http.StatusNotFound, plugin.CodeNoCluster, reasonClusterGone, nil)
	}
	if err != nil {
		return nil, false, err
	}
	role, err := s.store.ClusterRole(user, cluster.ID)
	if err != nil {
		return nil, false, err
	}
	if role == "" {
		return nil, false, fail(http.StatusForbidden, plugin.CodeForbidden, reasonAccessLost, reason.Params{"cluster": cluster.Name})
	}
	if err := browseDenied(role, cluster); err != nil {
		return nil, false, err
	}
	return cluster, cluster.ReadOnly, nil
}

// onProxyOrigin reports whether r was sent to the proxy origin
// (KUBEBENCH_PROXY_ORIGIN) rather than to KubeBench itself.
func (s *Server) onProxyOrigin(r *http.Request) bool {
	if s.cfg.ProxyOrigin == "" {
		return false
	}
	u, err := url.Parse(s.cfg.ProxyOrigin)
	if err != nil {
		return false
	}
	host := r.Host
	if forwarded := firstHeaderValue(r.Header.Get("X-Forwarded-Host")); s.cfg.TrustProxy && forwarded != "" {
		host = forwarded
	}
	return strings.EqualFold(withDefaultPort(host, u.Scheme), withDefaultPort(u.Host, u.Scheme))
}

// appOrigin is KubeBench's origin as the browser that opened a grant sees
// it, for the frame-ancestors of its pages: the request's Origin
// (checkOrigin made sure it names this host), or else the scheme and host
// it was sent to; 'self' when neither is a plain origin, since the value
// goes into a CSP header.
func (s *Server) appOrigin(r *http.Request) string {
	if u, err := url.Parse(r.Header.Get("Origin")); err == nil && (u.Scheme == "http" || u.Scheme == "https") && plainHost.MatchString(u.Host) {
		return u.Scheme + "://" + u.Host
	}
	scheme := "http"
	if r.TLS != nil || (s.cfg.TrustProxy && strings.EqualFold(firstHeaderValue(r.Header.Get("X-Forwarded-Proto")), "https")) {
		scheme = "https"
	}
	host := r.Host
	if forwarded := firstHeaderValue(r.Header.Get("X-Forwarded-Host")); s.cfg.TrustProxy && forwarded != "" {
		host = forwarded
	}
	if !plainHost.MatchString(host) {
		return "'self'"
	}
	return scheme + "://" + host
}

// plainHost is a host[:port] without characters that mean something in a
// CSP header.
var plainHost = regexp.MustCompile(`^[A-Za-z0-9.\-]+(:[0-9]+)?$|^\[[0-9A-Fa-f:.]+\](:[0-9]+)?$`)
