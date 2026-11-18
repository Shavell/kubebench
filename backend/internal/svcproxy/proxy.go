package svcproxy

import (
	"context"
	"errors"
	"fmt"
	"log"
	"mime"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"
)

// Limits of one proxied request (§7.14).
const (
	// MaxRequestBody bounds what a browser may upload through the proxy.
	MaxRequestBody = 32 << 20
	// HeaderTimeout bounds the wait for the backend's response headers;
	// the body may then stream for as long as it takes (downloads, server-
	// sent events, WebSocket connections).
	HeaderTimeout = time.Minute
)

// Request is one browser request for a grant's target.
type Request struct {
	Grant Grant
	// Path is the in-cluster path as CleanPath returned it.
	Path string
	// Base is the path the grant's content is served under: /proxy/<token>.
	Base string
	// Transport and APIServer reach the API server with KubeBench's
	// credential (kube.Runtime.ProxyTransport).
	Transport http.RoundTripper
	APIServer *url.URL
	// Sandboxed is set on KubeBench's own origin; content served on the
	// separate proxy origin runs unsandboxed.
	Sandboxed bool
	// ReadOnly allows GET and HEAD only, without protocol upgrades (a
	// read-only cluster).
	ReadOnly bool
	// SessionCookie is the name of KubeBench's session cookie.
	SessionCookie string
	// Navigated, when set, is told the in-cluster location (path and query)
	// of every document a frame loads with the grant: the frame KubeBench
	// shows and the frames nested in its page. Top-level documents (the
	// page opened in a browser tab of its own) are not frame navigations.
	Navigated func(location string)
}

var errHeaderTimeout = errors.New("the service did not answer within a minute")

// Serve forwards r to the target through the API server's proxy
// subresource and writes the shaped response (§7.14).
func Serve(w http.ResponseWriter, r *http.Request, req Request) {
	opaque := req.Sandboxed && r.Header.Get("Origin") == "null"
	if r.Method == http.MethodOptions && opaque && r.Header.Get("Access-Control-Request-Method") != "" {
		preflight(w, r, req)
		return
	}
	if req.Navigated != nil && r.Header.Get("Sec-Fetch-Mode") == "navigate" && r.Header.Get("Sec-Fetch-Dest") == "iframe" {
		loc := req.Path
		if r.URL.RawQuery != "" {
			loc += "?" + r.URL.RawQuery
		}
		req.Navigated(loc)
	}
	if req.ReadOnly && r.Method != http.MethodGet && r.Method != http.MethodHead {
		Fail(w, http.StatusForbidden, "the cluster is read-only: the service browser sends only GET and HEAD requests", req.Sandboxed, req.Grant.FrameAncestor)
		return
	}
	if req.ReadOnly && isUpgrade(r.Header) {
		Fail(w, http.StatusForbidden, "the cluster is read-only: the service browser opens no WebSocket connections", req.Sandboxed, req.Grant.FrameAncestor)
		return
	}
	if r.Body != nil {
		r.Body = http.MaxBytesReader(w, r.Body, MaxRequestBody)
	}

	rw := newRewriter(req.Base, req.APIServer, req.Grant.Target)
	proxy := &httputil.ReverseProxy{
		Transport:     headerTimeout{rt: req.Transport, wait: HeaderTimeout},
		FlushInterval: 50 * time.Millisecond,
		Rewrite: func(pr *httputil.ProxyRequest) {
			pr.Out.URL = upstreamURL(req.APIServer, req.Grant.Target, req.Path, r.URL.RawQuery)
			// The API server's proxy passes the Host header on to the
			// backend: an Ingress rule's host, or the API server's own.
			pr.Out.Host = req.Grant.Target.Host
			filterRequestHeaders(pr.Out.Header, req.SessionCookie)
		},
		ModifyResponse: func(resp *http.Response) error {
			shapeResponse(resp, rw, req, opaque)
			return nil
		},
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			var tooLarge *http.MaxBytesError
			switch {
			case errors.As(err, &tooLarge):
				Fail(w, http.StatusRequestEntityTooLarge, fmt.Sprintf("the request body is larger than %d MiB", MaxRequestBody>>20), req.Sandboxed, req.Grant.FrameAncestor)
			case errors.Is(err, errHeaderTimeout):
				Fail(w, http.StatusGatewayTimeout, errHeaderTimeout.Error(), req.Sandboxed, req.Grant.FrameAncestor)
			case r.Context().Err() != nil:
				// The browser went away; nobody reads an answer.
			default:
				log.Printf("[service-browser] %s: %v", req.Grant.Target.AuditTarget(), err)
				Fail(w, http.StatusBadGateway, "the API server could not be reached: "+err.Error(), req.Sandboxed, req.Grant.FrameAncestor)
			}
		},
	}
	proxy.ServeHTTP(w, r)
}

// upstreamURL is the API server URL of path below the target's proxy
// subresource.
func upstreamURL(apiServer *url.URL, t Target, path, rawQuery string) *url.URL {
	escaped := strings.TrimSuffix(apiServer.EscapedPath(), "/") + t.APIPath() + path
	u := *apiServer
	u.RawPath = escaped
	u.Path, _ = url.PathUnescape(escaped) // CleanPath checked the escapes
	u.RawQuery = rawQuery
	u.Fragment = ""
	return &u
}

// shapeResponse applies §7.14 to a backend response: headers that would act
// on KubeBench go, redirects, cookies and HTML links are moved below the
// base, and the isolation headers are added.
func shapeResponse(resp *http.Response, rw *rewriter, req Request, opaque bool) {
	h := resp.Header
	for _, name := range responseHeadersRemoved {
		h.Del(name)
	}
	stripFrameAncestors(h, "Content-Security-Policy")
	stripFrameAncestors(h, "Content-Security-Policy-Report-Only")
	if loc := h.Get("Location"); loc != "" {
		if rewritten := rw.location(loc); rewritten != "" {
			h.Set("Location", rewritten)
		} else {
			h.Del("Location")
		}
	}
	if cookies := h.Values("Set-Cookie"); len(cookies) > 0 {
		h.Del("Set-Cookie")
		for _, c := range cookies {
			if v, ok := rw.setCookie(c, req.SessionCookie); ok {
				h.Add("Set-Cookie", v)
			}
		}
	}
	if resp.StatusCode != http.StatusSwitchingProtocols && isHTML(h) && h.Get("Content-Encoding") == "" {
		resp.Body = rw.body(resp.Body)
		resp.ContentLength = -1
		h.Del("Content-Length")
	}
	if opaque {
		// The sandboxed page's own fetches come from an opaque origin; they
		// carry no credentials (the token in the URL is the credential).
		h.Set("Access-Control-Allow-Origin", "null")
		h.Del("Access-Control-Allow-Credentials")
		h.Add("Vary", "Origin")
	}
	setIsolationHeaders(h, req.Sandboxed, req.Grant.FrameAncestor)
}

func isHTML(h http.Header) bool {
	mediaType, _, err := mime.ParseMediaType(h.Get("Content-Type"))
	return err == nil && (mediaType == "text/html" || mediaType == "application/xhtml+xml")
}

// preflight answers a CORS preflight of the sandboxed page itself: the
// page's own non-simple requests (JSON posts, custom headers) are
// cross-origin from its opaque origin.
func preflight(w http.ResponseWriter, r *http.Request, req Request) {
	methods := "GET, HEAD, POST, PUT, PATCH, DELETE"
	if req.ReadOnly {
		methods = "GET, HEAD"
	}
	h := w.Header()
	h.Set("Access-Control-Allow-Origin", "null")
	h.Set("Access-Control-Allow-Methods", methods)
	if requested := r.Header.Get("Access-Control-Request-Headers"); requested != "" {
		h.Set("Access-Control-Allow-Headers", requested)
	}
	h.Set("Access-Control-Max-Age", "600")
	h.Add("Vary", "Origin")
	setIsolationHeaders(h, req.Sandboxed, req.Grant.FrameAncestor)
	w.WriteHeader(http.StatusNoContent)
}

// Fail writes a plain-text error page for the browser frame, with the
// isolation headers like every proxied answer.
func Fail(w http.ResponseWriter, status int, message string, sandboxed bool, frameAncestor string) {
	h := w.Header()
	setIsolationHeaders(h, sandboxed, frameAncestor)
	h.Set("Content-Type", "text/plain; charset=utf-8")
	h.Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	fmt.Fprintf(w, "KubeBench service browser: %s\n", message)
}

// headerTimeout ends a request whose response headers do not arrive within
// wait. The body is not limited.
type headerTimeout struct {
	rt   http.RoundTripper
	wait time.Duration
}

func (t headerTimeout) RoundTrip(r *http.Request) (*http.Response, error) {
	ctx, cancel := context.WithCancelCause(r.Context())
	timer := time.AfterFunc(t.wait, func() { cancel(errHeaderTimeout) })
	resp, err := t.rt.RoundTrip(r.WithContext(ctx))
	if !timer.Stop() {
		if resp != nil {
			resp.Body.Close()
		}
		return nil, errHeaderTimeout
	}
	if err != nil {
		cancel(err)
	}
	// Otherwise ctx ends with the request's own context: the body (or an
	// upgraded connection) stays usable until then.
	return resp, err
}
