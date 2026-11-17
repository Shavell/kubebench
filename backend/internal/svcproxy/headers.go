package svcproxy

import (
	"net/http"
	"strings"
)

// SandboxFlags are the sandbox flags of proxied content on KubeBench's own
// origin, in the response's Content-Security-Policy and on the iframe.
// allow-same-origin is missing on purpose: the content runs in an opaque
// origin and can neither read KubeBench's storage nor send credentialed
// requests that KubeBench would accept. Modal dialogs (alert, prompt) stay
// blocked, so a page cannot ask for a password in KubeBench's name.
const SandboxFlags = "allow-scripts allow-forms allow-popups allow-downloads"

// filterRequestHeaders removes what must never reach the API server or the
// backend from the outgoing request's headers h (the reverse proxy has
// already removed hop-by-hop and X-Forwarded-* headers):
//
//   - credentials: Authorization would replace KubeBench's own credential
//     towards the API server, Impersonate-* would make the API server act
//     as another user with it, X-Remote-* are request-header
//     authentication headers;
//   - the KubeBench session cookie, which would hand the user's session
//     to the backend (the backend's own cookies pass);
//   - Referer, which carries the grant token;
//   - Accept-Encoding: the transport then asks for gzip itself and
//     decompresses, so HTML can be rewritten.
func filterRequestHeaders(h http.Header, sessionCookie string) {
	for name := range h {
		canonical := http.CanonicalHeaderKey(name)
		switch {
		case canonical == "Authorization", canonical == "Proxy-Authorization",
			canonical == "Referer", canonical == "Accept-Encoding", canonical == "Forwarded",
			strings.HasPrefix(canonical, "Impersonate-"), strings.HasPrefix(canonical, "X-Remote-"):
			h.Del(name)
		}
	}
	filterCookies(h, sessionCookie)
}

// filterCookies drops the cookie named name from the Cookie headers of h.
func filterCookies(h http.Header, name string) {
	values := h.Values("Cookie")
	if len(values) == 0 {
		return
	}
	var kept []string
	for _, line := range values {
		for _, part := range strings.Split(line, ";") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			key, _, _ := strings.Cut(part, "=")
			if strings.TrimSpace(key) == name {
				continue
			}
			kept = append(kept, part)
		}
	}
	h.Del("Cookie")
	if len(kept) > 0 {
		h.Set("Cookie", strings.Join(kept, "; "))
	}
}

// responseHeadersRemoved are backend response headers that would act on
// KubeBench's origin rather than on the proxied page: HSTS for KubeBench's
// host, wiping KubeBench's cookies and storage, widening a service
// worker's scope, and framing rules (KubeBench sets its own).
var responseHeadersRemoved = []string{
	"Strict-Transport-Security",
	"Clear-Site-Data",
	"Service-Worker-Allowed",
	"X-Frame-Options",
	"Alt-Svc",
}

// stripFrameAncestors removes the frame-ancestors directive from each
// policy of a Content-Security-Policy header value: KubeBench decides who
// may frame proxied content. Other directives still apply.
func stripFrameAncestors(h http.Header, name string) {
	values := h.Values(name)
	if len(values) == 0 {
		return
	}
	h.Del(name)
	for _, policy := range values {
		var kept []string
		for _, directive := range strings.Split(policy, ";") {
			d := strings.TrimSpace(directive)
			if d == "" {
				continue
			}
			if field, _, _ := strings.Cut(d, " "); strings.EqualFold(field, "frame-ancestors") {
				continue
			}
			kept = append(kept, d)
		}
		if len(kept) > 0 {
			h.Add(name, strings.Join(kept, "; "))
		}
	}
}

// setIsolationHeaders adds KubeBench's isolation headers to a proxied
// response (§7.14). Sandboxed content (KubeBench's own origin) gets the
// CSP sandbox; content served on the separate proxy origin runs
// unsandboxed there. Either may be framed only by frameAncestor.
func setIsolationHeaders(h http.Header, sandboxed bool, frameAncestor string) {
	ancestors := "frame-ancestors " + frameAncestor
	if frameAncestor == "" {
		ancestors = "frame-ancestors 'none'"
	}
	if sandboxed {
		h.Add("Content-Security-Policy", "sandbox "+SandboxFlags+"; "+ancestors)
		h.Set("X-Frame-Options", "SAMEORIGIN")
	} else {
		h.Add("Content-Security-Policy", ancestors)
	}
	h.Set("Referrer-Policy", "no-referrer")
	// Requests of the sandboxed page come from an opaque origin, so its
	// own subresources are cross-origin loads: a stricter policy than
	// cross-origin would break every proxied page. Other sites cannot
	// name the URL anyway; it contains the grant's token.
	h.Set("Cross-Origin-Resource-Policy", "cross-origin")
}

// isUpgrade reports whether a request with header h asks to switch
// protocols (a WebSocket): the reverse proxy would then splice a two-way
// connection to the backend that no method rule sees.
func isUpgrade(h http.Header) bool {
	if h.Get("Upgrade") != "" {
		return true
	}
	for _, v := range h.Values("Connection") {
		for token := range strings.SplitSeq(v, ",") {
			if strings.EqualFold(strings.TrimSpace(token), "upgrade") {
				return true
			}
		}
	}
	return false
}
