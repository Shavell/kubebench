package svcproxy

import (
	"bytes"
	"io"
	"net/url"
	"path"
	"strings"
	"testing"
)

// FuzzCleanPath checks that a path CleanPath accepts stays below the proxy
// subresource once the API server decodes it: no dot segment, no encoded
// separator, nothing that climbs out of APIPath.
func FuzzCleanPath(f *testing.F) {
	for _, seed := range []string{
		"", "/", "x", "/a/b.js", "/a%20b", "/../secrets", "/a/./b", "/%2e%2e/x", "/%2E%2e/x",
		"/a%2fb", "/a%5Cb", `/a\b`, "/%zz", "/.%2e/", "/%2e./x", "//a//b", "/a/..;/b", "/%252e%252e/",
		"/a/%00/b", "/..", "/.", "%2e%2e", "/a/b/..%2f..%2f",
	} {
		f.Add(seed)
	}
	prefix := web.APIPath()
	f.Fuzz(func(t *testing.T, escaped string) {
		got, err := CleanPath(escaped)
		if err != nil {
			return
		}
		if !strings.HasPrefix(got, "/") {
			t.Fatalf("CleanPath(%q) = %q: no leading slash", escaped, got)
		}
		decoded, err := url.PathUnescape(got)
		if err != nil {
			t.Fatalf("CleanPath(%q) = %q, which does not decode: %v", escaped, got, err)
		}
		if strings.Count(decoded, "/") != strings.Count(got, "/") || strings.Contains(decoded, `\`) {
			t.Fatalf("CleanPath(%q) = %q decodes to %q: an encoded separator passed", escaped, got, decoded)
		}
		for seg := range strings.SplitSeq(decoded, "/") {
			if seg == "." || seg == ".." {
				t.Fatalf("CleanPath(%q) = %q decodes to a dot segment", escaped, got)
			}
		}
		if full := path.Clean(prefix + decoded); full != prefix && !strings.HasPrefix(full, prefix+"/") {
			t.Fatalf("CleanPath(%q) = %q leaves the subresource: %q", escaped, got, full)
		}
	})
}

// browserParse approximates how a browser reads a Location header
// (WHATWG URL parsing for the special schemes http and https): leading
// and trailing C0 controls and spaces are dropped, tabs and newlines
// removed anywhere, and a backslash is a slash.
func browserParse(loc string) (*url.URL, error) {
	loc = strings.TrimFunc(loc, func(r rune) bool { return r <= ' ' })
	loc = strings.NewReplacer("\t", "", "\n", "", "\r", "", `\`, "/").Replace(loc)
	return url.Parse(loc)
}

// FuzzLocation checks the Location rewrite: a rewritten redirect is a path
// that the browser resolves below the grant's base, and a redirect passed
// on unchanged is one the browser resolves relative to the proxied page
// (as it does the page's own relative links) or one to another host.
func FuzzLocation(f *testing.F) {
	for _, seed := range []string{
		"/login?next=%2F", "page.html", "../up", "http://10.0.0.1:8080/p#x", "https://example.com/x",
		"/api/v1/namespaces/shop/services/http:web:80/proxy/a", "/api/v1/namespaces/shop/services/http:web:80/proxy",
		"/api/v1/namespaces/shop/services/http:web:80/proxy/../../secrets", "/../../api/v1/secrets",
		"/gw/api/v1/namespaces/shop/services/http:web:80/proxy/x", "https://shop.example.com/cart?x=1",
		"//shop.example.com/x", "//evil.example/x", "/%2e%2e/%2e%2e/api", "/a%zz", `\api\v1`, ` /api`,
		"http:/x", "https:x", "javascript:alert(1)", "/\t/evil.example", "HTTPS://SHOP.EXAMPLE.COM/a/../..",
		"http://10.0.0.1:6443/./../x", "?q=1", "#frag", "",
	} {
		f.Add(seed)
	}
	api, _ := url.Parse("https://10.0.0.1:6443/gw")
	ingress := web
	ingress.Host = "shop.example.com"
	rewriters := []*rewriter{newRewriter("/proxy/tok", api, web), newRewriter("/proxy/tok", api, ingress)}
	page, _ := url.Parse("https://kb.example/proxy/tok/app/page")
	f.Fuzz(func(t *testing.T, loc string) {
		for _, rw := range rewriters {
			out := rw.location(loc)
			if out == "" {
				continue // dropped
			}
			if out == loc {
				u, err := browserParse(loc)
				if err != nil {
					t.Fatalf("location(%q) passed an unparsable redirect on", loc)
				}
				switch {
				case u.Scheme == "" && u.Host == "" && !strings.HasPrefix(u.Path, "/"):
					// Relative to the proxied page.
				case u.Host != "" && !strings.EqualFold(u.Hostname(), rw.apiHost) &&
					(rw.appHost == "" || !strings.EqualFold(u.Hostname(), rw.appHost)):
					// Another host (or none: "//:" does not resolve).
				case u.Scheme != "" && u.Scheme != "http" && u.Scheme != "https":
					// Not a web URL.
				default:
					t.Fatalf("location(%q) passed a redirect into KubeBench's origin on (%+v)", loc, *u)
				}
				continue
			}
			u, err := browserParse(out)
			if err != nil {
				t.Fatalf("location(%q) = %q, which does not parse: %v", loc, out, err)
			}
			if u.Scheme != "" || u.Host != "" || u.Opaque != "" || !strings.HasPrefix(u.Path, "/") {
				t.Fatalf("location(%q) = %q, not an absolute path", loc, out)
			}
			if r := page.ResolveReference(u); r.Host != page.Host || !strings.HasPrefix(r.Path, rw.base+"/") {
				t.Fatalf("location(%q) = %q resolves to %q, outside %s", loc, out, r, rw.base)
			}
		}
	})
}

// chunkReader returns its data in reads of the given sizes, in turn.
type chunkReader struct {
	data  []byte
	sizes []byte
	n     int
}

func (r *chunkReader) Read(p []byte) (int, error) {
	if len(r.data) == 0 {
		return 0, io.EOF
	}
	size := 1
	if len(r.sizes) > 0 {
		size = int(r.sizes[r.n%len(r.sizes)]) + 1
		r.n++
	}
	size = min(size, len(p), len(r.data))
	copy(p, r.data[:size])
	r.data = r.data[size:]
	return size, nil
}

// FuzzBodyRewrite checks that the streamed HTML rewrite does not depend on
// how the body arrives: whatever the read sizes, the output is the one of
// a single pass over the whole body.
func FuzzBodyRewrite(f *testing.F) {
	prefix := "/api/v1/namespaces/shop/services/http:web:80/proxy"
	for _, seed := range []string{
		`<a href="` + prefix + `/a">a</a>`, `<img src="/gw` + prefix + `/b.png">`,
		`https://10.0.0.1:6443/gw` + prefix + `/x`, `//shop.example.com/cart //shop.example.community`,
		prefix[:20], prefix + prefix, "/gw/gw" + prefix, "",
	} {
		f.Add([]byte(seed), []byte{0, 3, 200})
	}
	api, _ := url.Parse("https://10.0.0.1:6443/gw")
	ingress := web
	ingress.Host = "shop.example.com"
	rw := newRewriter("/proxy/tok", api, ingress)
	f.Fuzz(func(t *testing.T, body, sizes []byte) {
		if len(body) > 16<<10 {
			return
		}
		var want bytes.Buffer
		for i := 0; i < len(body); {
			matched := false
			for _, p := range rw.replacements {
				if bytes.HasPrefix(body[i:], p.from) {
					want.Write(p.to)
					i += len(p.from)
					matched = true
					break
				}
			}
			if !matched {
				want.WriteByte(body[i])
				i++
			}
		}
		got, err := io.ReadAll(rw.body(io.NopCloser(&chunkReader{data: body, sizes: sizes})))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, want.Bytes()) {
			t.Fatalf("streamed rewrite of %q differs:\n got %q\nwant %q", body, got, want.Bytes())
		}
	})
}
