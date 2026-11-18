package api

import (
	"net/http"
	"strconv"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"

	"kubebench/internal/kube"
	"kubebench/internal/plugin"
	"kubebench/internal/reason"
	"kubebench/internal/svcproxy"
)

// openShiftRoute is the part of a route.openshift.io/v1 Route the service
// browser reads.
type openShiftRoute struct {
	Spec struct {
		Host string `json:"host"`
		Path string `json:"path"`
		To   struct {
			Kind string `json:"kind"`
			Name string `json:"name"`
		} `json:"to"`
		Port *struct {
			TargetPort intstr.IntOrString `json:"targetPort"`
		} `json:"port"`
	} `json:"spec"`
}

// routeServicePort is the port of svc a Route's targetPort selects, the
// way the OpenShift router picks endpoints: a name is a Service port name
// (or, failing that, a named targetPort), a number the Service port's
// targetPort (or, failing that, its port). Without a targetPort the router
// uses every port; the browser takes the first. "" when none matches.
func routeServicePort(svc *corev1.Service, target *intstr.IntOrString) string {
	ports := svc.Spec.Ports
	if len(ports) == 0 {
		return ""
	}
	portOf := func(p corev1.ServicePort) string {
		if p.Name != "" {
			return p.Name
		}
		return strconv.Itoa(int(p.Port))
	}
	if target == nil || (target.Type == intstr.String && target.StrVal == "") || (target.Type == intstr.Int && target.IntVal == 0) {
		return portOf(ports[0])
	}
	for _, match := range []func(corev1.ServicePort) bool{
		func(p corev1.ServicePort) bool {
			if target.Type == intstr.String {
				return p.Name == target.StrVal
			}
			return p.TargetPort.Type == intstr.Int && p.TargetPort.IntVal == target.IntVal
		},
		func(p corev1.ServicePort) bool {
			if target.Type == intstr.String {
				return p.TargetPort.Type == intstr.String && p.TargetPort.StrVal == target.StrVal
			}
			return p.Port == target.IntVal
		},
	} {
		for _, p := range ports {
			if match(p) {
				return portOf(p)
			}
		}
	}
	return ""
}

// handleOpenRouteProxy opens a grant for the Service port behind an
// OpenShift Route (§7.15; access browse, audited once as route.proxy),
// sending the Route's host as the Host header like an Ingress grant. The
// router itself (TLS termination, path-based routing to alternate
// backends, rewrites by annotation) is not involved: the Service is
// reached directly.
func (s *Server) handleOpenRouteProxy(w http.ResponseWriter, r *http.Request, c *call) {
	ns, name := r.PathValue("ns"), r.PathValue("name")
	scheme := r.URL.Query().Get("scheme")
	if scheme == "" {
		scheme = "http"
	}
	detail := map[string]any{"scheme": scheme}
	if c.cluster.ReadOnly {
		detail["readOnly"] = true
	}
	c.setAudit("route/"+ns+"/"+name, auditJSON(detail))

	obj, err := c.rt.Dyn.Resource(kube.Catalog["routes"].GVR).Namespace(ns).Get(r.Context(), name, metav1.GetOptions{})
	if err != nil {
		writeTargetErr(w, err)
		return
	}
	var route openShiftRoute
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &route); err != nil {
		writeErr(w, refuse(http.StatusBadGateway, plugin.CodeKubernetes, err))
		return
	}
	detail["host"], detail["path"] = route.Spec.Host, route.Spec.Path
	if route.Spec.To.Kind != "Service" || route.Spec.To.Name == "" {
		writeErr(w, badRequest(reasonRouteNotService, reason.Params{"namespace": ns, "name": name, "kind": route.Spec.To.Kind}))
		return
	}
	svc, err := c.rt.CS.CoreV1().Services(ns).Get(r.Context(), route.Spec.To.Name, metav1.GetOptions{})
	if err != nil {
		writeTargetErr(w, err)
		return
	}
	var target *intstr.IntOrString
	if route.Spec.Port != nil {
		target = &route.Spec.Port.TargetPort
	}
	port := routeServicePort(svc, target)
	if port == "" {
		wanted := ""
		if target != nil {
			wanted = target.String()
		}
		writeErr(w, badRequest(reasonRouteNoPort, reason.Params{"namespace": ns, "name": name, "service": svc.Name, "port": wanted}))
		return
	}
	t := svcproxy.Target{Kind: svcproxy.KindService, Namespace: ns, Name: svc.Name, Port: port, Scheme: scheme, Host: route.Spec.Host}
	if err := t.Validate(); err != nil {
		writeErr(w, refuse(http.StatusBadRequest, plugin.CodeBadRequest, err))
		return
	}
	detail["service"] = svc.Name + ":" + port
	c.setAudit("", auditJSON(detail))
	s.issueGrant(w, r, c, t)
}
