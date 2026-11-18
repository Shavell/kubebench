package svcproxy

import (
	"bytes"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strings"
)

// rewriter maps paths of the backend and of the API server's proxy
// subresource to paths under the grant's base (/proxy/<token>).
type rewriter struct {
	base string
	// prefixes are the subresource path as the API server writes it into
	// rewritten HTML and Location headers, longest first: with the server
	// URL's own path (a cluster behind a path-prefixed gateway) and without.
	prefixes []string
	// apiHost is the API server's host name: backends build absolute
	// redirects from the Host header, which names the API server.
	apiHost string
	// appHost is the Host header the backend gets instead (Target.Host),
	// if any: its absolute redirects and links name that host.
	appHost string
	// replacements are what body replaces, longest first: the prefixes,
	// each also as an absolute URL naming the API server or appHost (a
	// backend builds absolute links from the Host header too), with the
	// base; and absolute URLs of appHost's root with the base's root.
	replacements []replacement
}

// replacement is one pattern of a streamed HTML body and what replaces it.
type replacement struct{ from, to []byte }

func newRewriter(base string, apiServer *url.URL, t Target) *rewriter {
	apiPath := t.APIPath()
	rw := &rewriter{base: base, apiHost: apiServer.Hostname(), appHost: t.Host}
	if p := strings.TrimSuffix(apiServer.Path, "/"); p != "" {
		rw.prefixes = append(rw.prefixes, p+apiPath)
	}
	rw.prefixes = append(rw.prefixes, apiPath)
	hosts := []string{apiServer.Host}
	if bare := apiServer.Hostname(); apiServer.Port() != "" {
		if strings.Contains(bare, ":") {
			bare = "[" + bare + "]"
		}
		hosts = append(hosts, bare)
	}
	if t.Host != "" {
		// The API server rewrites the backend's absolute links naming the
		// Host it sent into links below the subresource on that host.
		hosts = append(hosts, t.Host)
	}
	to := []byte(base)
	for _, prefix := range rw.prefixes {
		for _, host := range hosts {
			for _, scheme := range []string{"https:", "http:", ""} {
				rw.replacements = append(rw.replacements, replacement{[]byte(scheme + "//" + host + prefix), to})
			}
		}
		rw.replacements = append(rw.replacements, replacement{[]byte(prefix), to})
	}
	if t.Host != "" {
		// Only URLs of the host's root path ("//host/…"): "//host" alone
		// could be the start of another host's name.
		root := []byte(base + "/")
		for _, scheme := range []string{"https:", "http:", ""} {
			rw.replacements = append(rw.replacements, replacement{[]byte(scheme + "//" + t.Host + "/"), root})
		}
	}
	slices.SortStableFunc(rw.replacements, func(a, b replacement) int { return len(b.from) - len(a.from) })
	return rw
}

// mapPath maps an absolute path of the backend, or of the subresource, to
// the base.
func (rw *rewriter) mapPath(p string) string {
	for _, prefix := range rw.prefixes {
		if p == prefix {
			return rw.base + "/"
		}
		if strings.HasPrefix(p, prefix+"/") {
			return rw.base + strings.TrimPrefix(p, prefix)
		}
	}
	return rw.base + p
}

// location rewrites a redirect target. Relative references stay as they
// are (the browser resolves them below the base); absolute paths and
// absolute URLs naming the API server's host, or the Ingress host the
// backend got (both: the Host header it got), are mapped below the base,
// their dot segments resolved first so that the result cannot climb out
// of it. Redirects to other hosts are passed on unchanged: the proxy never
// follows them, the browser navigates the (sandboxed) frame there.
//
// The target is read as a browser reads it (see browserReference). One
// that does not parse, or an http(s) URL without a host (which a browser
// may resolve on KubeBench's own origin), is dropped: location returns ""
// and the Location header is removed.
func (rw *rewriter) location(loc string) string {
	u, err := url.Parse(browserReference(loc))
	if err != nil {
		return ""
	}
	if u.Scheme != "" && u.Host == "" && (strings.EqualFold(u.Scheme, "http") || strings.EqualFold(u.Scheme, "https")) {
		return ""
	}
	p := removeDotSegments(u.Path)
	if u.Host != "" || u.Scheme != "" {
		own := strings.EqualFold(u.Hostname(), rw.apiHost) || (rw.appHost != "" && strings.EqualFold(u.Hostname(), rw.appHost))
		if !own && !rw.hasPrefix(p) {
			return loc
		}
	} else if !strings.HasPrefix(u.Path, "/") {
		return loc
	}
	out := url.URL{Path: rw.mapPath(cleanAbs(p)), RawQuery: u.RawQuery, Fragment: u.Fragment}
	return out.String()
}

// browserReference is loc as a browser reads a URL of the special schemes
// (http, https; WHATWG URL): leading and trailing C0 controls and spaces
// dropped, tabs and newlines removed anywhere, backslashes read as slashes.
// " /x" and "\x" are absolute paths to a browser, not relative ones.
func browserReference(loc string) string {
	loc = strings.TrimFunc(loc, func(r rune) bool { return r <= ' ' })
	return strings.NewReplacer("\t", "", "\n", "", "\r", "", `\`, "/").Replace(loc)
}

// removeDotSegments resolves the "." and ".." segments of an absolute path
// (RFC 3986 §5.2.4), as the browser would after the rewrite: "/a/../../x"
// is "/x", which is mapped below the base, not above it.
func removeDotSegments(p string) string {
	if !strings.HasPrefix(p, "/") {
		return p
	}
	return (&url.URL{Path: "/"}).ResolveReference(&url.URL{Path: p}).Path
}

func (rw *rewriter) hasPrefix(p string) bool {
	for _, prefix := range rw.prefixes {
		if p == prefix || strings.HasPrefix(p, prefix+"/") {
			return true
		}
	}
	return false
}

func cleanAbs(p string) string {
	if p == "" {
		return "/"
	}
	return p
}

// setCookie rewrites one Set-Cookie value of the backend; ok is false
// when it must be dropped: the KubeBench session cookie's name may not be
// set by proxied content. A cookie gets no Domain (it belongs to the host
// serving the proxy) and its Path is moved below the base, so the
// backend's cookies reach neither KubeBench nor other proxied services.
func (rw *rewriter) setCookie(value, sessionCookie string) (string, bool) {
	c, err := http.ParseSetCookie(value)
	if err != nil || c.Name == sessionCookie {
		return "", false
	}
	c.Domain = ""
	if strings.HasPrefix(c.Path, "/") {
		c.Path = rw.mapPath(c.Path)
	}
	return c.String(), true
}

// body wraps an HTML body, replacing the subresource paths the API server
// wrote into its links, and absolute URLs of them naming the API server
// or the Ingress host, with the base, as a stream.
func (rw *rewriter) body(r io.ReadCloser) io.ReadCloser {
	rr := &replacingReader{src: r, replacements: rw.replacements, keep: len(rw.replacements[0].from) - 1}
	starts := map[byte]bool{}
	for _, p := range rw.replacements {
		if !starts[p.from[0]] {
			starts[p.from[0]] = true
			rr.starts += string(p.from[0])
		}
	}
	return rr
}

// replacingReader replaces every occurrence of each pattern, the earliest
// listed pattern winning at a position. It holds back keep bytes (one
// less than the longest pattern) between reads, so that a pattern split
// across reads is still found.
type replacingReader struct {
	src          io.ReadCloser
	replacements []replacement
	// starts holds the first byte of every pattern.
	starts string
	keep   int

	in  []byte // read, not yet scanned
	buf []byte // the buffer reads go to, reused
	out bytes.Buffer
	eof bool
	err error
}

func (r *replacingReader) Read(p []byte) (int, error) {
	for r.out.Len() == 0 {
		if r.eof {
			return 0, r.err
		}
		r.fill()
	}
	return r.out.Read(p)
}

func (r *replacingReader) fill() {
	if r.buf == nil {
		r.buf = make([]byte, 32*1024)
	}
	n, err := r.src.Read(r.buf)
	r.in = append(r.in, r.buf[:n]...)
	if err != nil {
		r.eof, r.err = true, err
	}
	limit := len(r.in) - r.keep // positions a whole pattern fits after
	if r.eof {
		limit = len(r.in)
	}
	i := 0
	for i < limit {
		j := bytes.IndexAny(r.in[i:limit], r.starts)
		if j < 0 {
			r.out.Write(r.in[i:limit])
			i = limit
			break
		}
		r.out.Write(r.in[i : i+j])
		i += j
		matched := false
		for _, p := range r.replacements {
			if bytes.HasPrefix(r.in[i:], p.from) {
				r.out.Write(p.to)
				i += len(p.from)
				matched = true
				break
			}
		}
		if !matched {
			r.out.WriteByte(r.in[i])
			i++
		}
	}
	r.in = append(r.in[:0], r.in[i:]...)
}

func (r *replacingReader) Close() error { return r.src.Close() }
