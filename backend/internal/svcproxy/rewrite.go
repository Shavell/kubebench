package svcproxy

import (
	"bytes"
	"io"
	"net/http"
	"net/url"
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
}

func newRewriter(base string, apiServer *url.URL, t Target) *rewriter {
	apiPath := t.APIPath()
	rw := &rewriter{base: base, apiHost: apiServer.Hostname()}
	if p := strings.TrimSuffix(apiServer.Path, "/"); p != "" {
		rw.prefixes = append(rw.prefixes, p+apiPath)
	}
	rw.prefixes = append(rw.prefixes, apiPath)
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
// absolute URLs naming the API server's host, which a backend builds from
// the Host header it got, are mapped below the base. Redirects to other
// hosts are passed on unchanged: the proxy never follows them, the
// browser navigates the (sandboxed) frame there.
func (rw *rewriter) location(loc string) string {
	u, err := url.Parse(loc)
	if err != nil {
		return loc
	}
	if u.Host != "" || u.Scheme != "" {
		if u.Hostname() != rw.apiHost && !rw.hasPrefix(u.Path) {
			return loc
		}
	} else if !strings.HasPrefix(u.Path, "/") {
		return loc
	}
	out := url.URL{Path: rw.mapPath(cleanAbs(u.Path)), RawQuery: u.RawQuery, Fragment: u.Fragment}
	return out.String()
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
// wrote into its links with the base, as a stream.
func (rw *rewriter) body(r io.ReadCloser) io.ReadCloser {
	pairs := make([][2][]byte, len(rw.prefixes))
	for i, p := range rw.prefixes {
		pairs[i] = [2][]byte{[]byte(p), []byte(rw.base)}
	}
	return &replacingReader{src: r, pairs: pairs, keep: len(rw.prefixes[0]) - 1}
}

// replacingReader replaces every occurrence of the first element of each
// pair with the second, the earliest listed pair winning at a position. It
// holds back keep bytes (one less than the longest pattern) between reads,
// so that a pattern split across reads is still found.
type replacingReader struct {
	src   io.ReadCloser
	pairs [][2][]byte
	keep  int

	in  []byte // read, not yet scanned
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
	buf := make([]byte, 32*1024)
	n, err := r.src.Read(buf)
	r.in = append(r.in, buf[:n]...)
	if err != nil {
		r.eof, r.err = true, err
	}
	limit := len(r.in) - r.keep // positions a whole pattern fits after
	if r.eof {
		limit = len(r.in)
	}
	i := 0
	for i < limit {
		j := bytes.IndexByte(r.in[i:limit], '/')
		if j < 0 {
			r.out.Write(r.in[i:limit])
			i = limit
			break
		}
		r.out.Write(r.in[i : i+j])
		i += j
		matched := false
		for _, pair := range r.pairs {
			if bytes.HasPrefix(r.in[i:], pair[0]) {
				r.out.Write(pair[1])
				i += len(pair[0])
				matched = true
				break
			}
		}
		if !matched {
			r.out.WriteByte('/')
			i++
		}
	}
	r.in = append(r.in[:0], r.in[i:]...)
}

func (r *replacingReader) Close() error { return r.src.Close() }
