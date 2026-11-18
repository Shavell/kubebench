package svcproxy

import (
	"io"
	"net/url"
	"strings"
	"testing"
	"testing/iotest"
)

// An Ingress grant sends the rule's host; the backend builds its absolute
// links and redirects from it, and they stay below the grant.

func ingressRewriter() *rewriter {
	u, _ := url.Parse("https://10.0.0.1:6443")
	t := web
	t.Host = "shop.example.com"
	return newRewriter("/proxy/tok", u, t)
}

func TestTargetHost(t *testing.T) {
	ok := web
	ok.Host = "shop.example.com"
	if err := ok.Validate(); err != nil {
		t.Fatal(err)
	}
	for name, host := range map[string]string{
		"wildcard":    "*.example.com",
		"upper case":  "Shop.example.com",
		"port":        "shop.example.com:8080",
		"header line": "shop.example.com\r\nX-Remote-User: admin",
		"too long":    strings.Repeat("a.", 127) + "aa",
	} {
		bad := web
		bad.Host = host
		if err := bad.Validate(); err == nil {
			t.Errorf("%s: host %q passed validation", name, host)
		}
	}
	pod := Target{Kind: KindPod, Namespace: "shop", Name: "web-1", Port: "80", Scheme: "http", Host: "shop.example.com"}
	if err := pod.Validate(); err == nil {
		t.Error("a Pod target took a host")
	}
}

func TestLocationRewriteOfTheIngressHost(t *testing.T) {
	rw := ingressRewriter()
	for in, want := range map[string]string{
		"https://shop.example.com/cart?x=1": "/proxy/tok/cart?x=1",
		"http://SHOP.example.com/":          "/proxy/tok/",
		"https://shop.example.com":          "/proxy/tok/",
		"https://shop.example.com.evil/x":   "https://shop.example.com.evil/x",
		"https://example.com/x":             "https://example.com/x",
		"/login":                            "/proxy/tok/login",
	} {
		if got := rw.location(in); got != want {
			t.Errorf("location(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestBodyRewriteOfTheIngressHost(t *testing.T) {
	rw := ingressRewriter()
	prefix := "/api/v1/namespaces/shop/services/http:web:80/proxy"
	in := `<link href="https://shop.example.com/app.css"><a href="//shop.example.com/cart">` +
		// The API server's own rewrite of a link naming the Host it sent.
		`<a href="http://shop.example.com` + prefix + `/page.html">` +
		`<a href="http://shop.example.com/">` +
		`<a href="https://shop.example.community/x"><a href="https://cdn.example.com/y.js">`
	want := `<link href="/proxy/tok/app.css"><a href="/proxy/tok/cart">` +
		`<a href="/proxy/tok/page.html">` +
		`<a href="/proxy/tok/">` +
		`<a href="https://shop.example.community/x"><a href="https://cdn.example.com/y.js">`
	for name, src := range map[string]io.Reader{
		"one byte per read": iotest.OneByteReader(strings.NewReader(in)),
		"large reads":       strings.NewReader(in),
	} {
		got, err := io.ReadAll(rw.body(io.NopCloser(src)))
		if err != nil || string(got) != want {
			t.Errorf("%s: %q (err %v)", name, got, err)
		}
	}
}
