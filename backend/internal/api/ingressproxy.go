package api

import (
	"net/http"
	"strconv"
	"strings"

	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"kubebench/internal/svcproxy"
)

// ingressRoute is one way into an Ingress: a rule's host and path, or the
// default backend (no host, no path), and the Service port it leads to.
type ingressRoute struct {
	Host    string
	Path    string
	Service string
	Port    string // number or name
}

// ingressRoutes lists the Service backends of ing: every path of every
// rule, then the default backend. Resource backends are left out: the
// service browser reaches Services only.
func ingressRoutes(ing *networkingv1.Ingress) []ingressRoute {
	var out []ingressRoute
	add := func(host, path string, b networkingv1.IngressBackend) {
		if b.Service == nil {
			return
		}
		port := b.Service.Port.Name
		if port == "" {
			port = strconv.Itoa(int(b.Service.Port.Number))
		}
		out = append(out, ingressRoute{Host: host, Path: path, Service: b.Service.Name, Port: port})
	}
	for _, rule := range ing.Spec.Rules {
		if rule.HTTP == nil {
			continue
		}
		for _, p := range rule.HTTP.Paths {
			add(rule.Host, p.Path, p.Backend)
		}
	}
	if ing.Spec.DefaultBackend != nil {
		add("", "", *ing.Spec.DefaultBackend)
	}
	return out
}

// handleOpenIngressProxy opens a grant for the Service behind one route of
// an Ingress (?host=&path=, as the rule names them; both empty for the
// default backend), sending the rule's host as the Host header (access
// browse; audited once as ingress.proxy). The browser starts at the
// rule's path; path rewriting done by the ingress controller (annotations,
// middlewares) does not apply, since the Service is reached directly.
func (s *Server) handleOpenIngressProxy(w http.ResponseWriter, r *http.Request, c *call) {
	ns, name := r.PathValue("ns"), r.PathValue("name")
	q := r.URL.Query()
	host, path := q.Get("host"), q.Get("path")
	scheme := q.Get("scheme")
	if scheme == "" {
		scheme = "http"
	}
	detail := map[string]any{"scheme": scheme, "host": host, "path": path}
	if c.cluster.ReadOnly {
		detail["readOnly"] = true
	}
	c.setAudit("ingress/"+ns+"/"+name, auditJSON(detail))

	ing, err := c.rt.CS.NetworkingV1().Ingresses(ns).Get(r.Context(), name, metav1.GetOptions{})
	if err != nil {
		writeTargetErr(w, err)
		return
	}
	var route *ingressRoute
	for _, candidate := range ingressRoutes(ing) {
		if candidate.Host == host && candidate.Path == path {
			route = &candidate
			break
		}
	}
	if route == nil {
		writeErr(w, badRequest("Ingress %s/%s has no rule for host %q and path %q that leads to a Service", ns, name, host, path))
		return
	}
	t := svcproxy.Target{Kind: svcproxy.KindService, Namespace: ns, Name: route.Service, Port: route.Port, Scheme: scheme}
	// A wildcard host names no single host to send.
	if !strings.HasPrefix(host, "*.") {
		t.Host = host
	}
	if err := t.Validate(); err != nil {
		writeErr(w, badRequest("%v", err))
		return
	}
	detail["service"] = route.Service + ":" + route.Port
	c.setAudit("", auditJSON(detail))
	s.issueGrant(w, r, c, t)
}
