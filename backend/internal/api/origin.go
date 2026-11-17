package api

import (
	"net"
	"net/http"
	"net/url"
	"strings"

	"kubebench/internal/plugin"
)

// checkOrigin rejects state-changing requests (and WebSocket upgrades)
// whose Origin header names another host: defence in depth against CSRF on
// top of the SameSite=Lax session cookie (§3.5). Requests without an
// Origin (curl, scripts) pass, except browser WebSocket upgrades.
func (s *Server) checkOrigin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := s.originError(r); err != nil {
			writeErr(w, err)
			return
		}
		next.ServeHTTP(w, r)
	})
}

var (
	errOriginMismatch = errorf(http.StatusForbidden, plugin.CodeForbidden, "cross-origin request rejected")
	errOriginMissing  = errorf(http.StatusForbidden, plugin.CodeForbidden, "WebSocket request without an Origin rejected")
)

// originError is why r fails the origin check, or nil. A WebSocket upgrade
// without an Origin passes only when it does not come from a browser:
// browsers always send Origin on WebSocket handshakes, and they mark their
// requests with Fetch Metadata headers (Sec-Fetch-*) that page scripts
// cannot set or remove.
func (s *Server) originError(r *http.Request) error {
	if isProxyPath(r.URL.Path) {
		// The service browser's proxy (§7.14) authorizes with the grant
		// token in its URL, never with the session cookie, so a forged
		// cross-site request carries no authority; and the sandboxed
		// pages it serves send "Origin: null" with their own forms.
		return nil
	}
	origin := r.Header.Get("Origin")
	switch {
	case origin == "":
		if isWebSocketUpgrade(r) && fromBrowser(r) {
			return errOriginMissing
		}
	case needsOriginCheck(r) && !s.sameOrigin(origin, r):
		return errOriginMismatch
	}
	return nil
}

var errCrossSite = errorf(http.StatusForbidden, plugin.CodeForbidden, "cross-site request rejected")

// crossSiteError refuses a GET or HEAD of a write or exec route that a page
// of another site made the browser send: the SameSite=Lax cookie goes
// along with top-level navigations, and such requests (an OCI fetch from a
// host of the page's choosing, revealing Helm release values) would run,
// and be audited, as the victim's deliberate action. Fetch Metadata
// headers cannot be set by page scripts; clients without them pass.
func crossSiteError(r *http.Request, level access) error {
	if level != accessWrite && level != accessExec && level != accessBrowse {
		return nil
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		return nil // covered by the Origin check
	}
	if r.Header.Get("Sec-Fetch-Site") == "cross-site" {
		return errCrossSite
	}
	return nil
}

func fromBrowser(r *http.Request) bool {
	return r.Header.Get("Sec-Fetch-Mode") != "" || r.Header.Get("Sec-Fetch-Site") != ""
}

func needsOriginCheck(r *http.Request) bool {
	switch r.Method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return isWebSocketUpgrade(r)
	}
	return true
}

func isWebSocketUpgrade(r *http.Request) bool {
	return headerHasToken(r.Header, "Connection", "upgrade") && strings.EqualFold(r.Header.Get("Upgrade"), "websocket")
}

func headerHasToken(h http.Header, name, token string) bool {
	for _, value := range h.Values(name) {
		for _, t := range strings.Split(value, ",") {
			if strings.EqualFold(strings.TrimSpace(t), token) {
				return true
			}
		}
	}
	return false
}

// sameOrigin compares the Origin's host with the Host the request was sent
// to (or, behind a trusted proxy, the forwarded host).
func (s *Server) sameOrigin(origin string, r *http.Request) bool {
	u, err := url.Parse(origin)
	if err != nil || u.Host == "" {
		return false // includes the opaque "null" origin
	}
	want := withDefaultPort(u.Host, u.Scheme)
	if strings.EqualFold(want, withDefaultPort(r.Host, u.Scheme)) {
		return true
	}
	if forwarded := firstHeaderValue(r.Header.Get("X-Forwarded-Host")); s.cfg.TrustProxy && forwarded != "" {
		return strings.EqualFold(want, withDefaultPort(forwarded, u.Scheme))
	}
	return false
}

// withDefaultPort makes "example.com" and "example.com:443" compare equal
// for https (and :80 for http).
func withDefaultPort(host, scheme string) string {
	if _, _, err := net.SplitHostPort(host); err == nil {
		return host
	}
	port := "80"
	if scheme == "https" || scheme == "wss" {
		port = "443"
	}
	return net.JoinHostPort(strings.Trim(host, "[]"), port)
}

// firstHeaderValue is the first entry of a comma-separated header value.
func firstHeaderValue(v string) string {
	first, _, _ := strings.Cut(v, ",")
	return strings.TrimSpace(first)
}
