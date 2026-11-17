package svcproxy

import (
	"net"
	"strings"

	"golang.org/x/net/publicsuffix"
)

// SameSite reports whether the host names a and b (without ports) belong
// to the same site: the same host, one a subdomain of the other, or the
// same registrable domain. Cookies
// are scoped by host and parent domain, never by port, so a page on one
// can set cookies that the browser sends to the other. The proxy origin
// must therefore be another site than KubeBench's (§7.14).
func SameSite(a, b string) bool {
	a, b = normalizeHost(a), normalizeHost(b)
	if a == b || strings.HasSuffix(a, "."+b) || strings.HasSuffix(b, "."+a) {
		return true // one is the other's parent domain
	}
	if net.ParseIP(a) != nil || net.ParseIP(b) != nil {
		return false // IP addresses have no parent domain
	}
	// A host that is itself a public suffix or a single label (localhost)
	// has no registrable domain: only the same host shares its cookies.
	siteA, errA := publicsuffix.EffectiveTLDPlusOne(a)
	siteB, errB := publicsuffix.EffectiveTLDPlusOne(b)
	return errA == nil && errB == nil && siteA == siteB
}

func normalizeHost(h string) string {
	return strings.TrimSuffix(strings.ToLower(strings.Trim(h, "[]")), ".")
}
