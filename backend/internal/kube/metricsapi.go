package kube

import (
	"context"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"kubebench/internal/reason"
)

// The resource metrics API (metrics.k8s.io) is an aggregated API: an
// APIService that the API server proxies to metrics-server (or OpenShift's
// metrics-server in openshift-monitoring). A cluster may not register it
// at all, register it while its backend is down, or refuse KubeBench.
const (
	metricsGroupVersion = "metrics.k8s.io/v1beta1"
	metricsAPIService   = "v1beta1.metrics.k8s.io"
)

var apiServiceGVR = schema.GroupVersionResource{
	Group: "apiregistration.k8s.io", Version: "v1", Resource: "apiservices",
}

// Why the metrics API has no data for KubeBench (Sampler.Status).
var (
	reasonMetricsMissing     = reason.Define("metrics.api_missing", "this cluster does not serve the metrics API (metrics.k8s.io); install metrics-server or enable cluster monitoring")
	reasonMetricsUnavailable = reason.Define("metrics.api_unavailable", "the metrics API (APIService {{apiservice}}) is not available: {{detail}}")
	reasonMetricsForbidden   = reason.Define("metrics.api_forbidden", "KubeBench may not read the metrics API: {{detail}}")
	reasonMetricsFailed      = reason.Define("metrics.api_failed", "reading the metrics API failed: {{detail}}")
	reasonMetricsEmpty       = reason.Define("metrics.api_empty", "the metrics API reports no node metrics yet")
	reasonMetricsPending     = reason.Define("metrics.not_sampled", "the metrics API has not been read yet; try again in a moment")
	reasonNodeCapacity       = reason.Define("metrics.node_capacity", "listing the nodes for their capacity failed: {{detail}}")
)

// explainMetricsAPI turns a failed metrics.k8s.io request into the reason
// the metrics API is unusable: not served at all (discovery does not know
// the group version), served by an APIService that is not Available (its
// condition says why), forbidden, or failing otherwise.
func (r *Runtime) explainMetricsAPI(ctx context.Context, err error) error {
	if apierrors.IsForbidden(err) {
		return reason.Wrap(err, reasonMetricsForbidden, reason.Params{"detail": err.Error()})
	}
	if apierrors.IsNotFound(err) {
		return reason.Wrap(err, reasonMetricsMissing, nil)
	}
	if _, derr := r.CS.Discovery().ServerResourcesForGroupVersion(metricsGroupVersion); apierrors.IsNotFound(derr) {
		return reason.Wrap(err, reasonMetricsMissing, nil)
	}
	if detail, unavailable := r.apiServiceUnavailable(ctx); unavailable {
		return reason.Wrap(err, reasonMetricsUnavailable, reason.Params{"apiservice": metricsAPIService, "detail": detail})
	}
	return reason.Wrap(err, reasonMetricsFailed, reason.Params{"detail": err.Error()})
}

// apiServiceUnavailable reports whether the metrics APIService says it is
// not Available, with its condition's reason and message. An APIService
// that cannot be read (RBAC, gone) reports nothing: the caller falls back
// to the request's own error.
func (r *Runtime) apiServiceUnavailable(ctx context.Context) (string, bool) {
	obj, err := r.Dyn.Resource(apiServiceGVR).Get(ctx, metricsAPIService, metav1.GetOptions{})
	if err != nil {
		return "", false
	}
	conditions, _, _ := unstructured.NestedSlice(obj.Object, "status", "conditions")
	for _, c := range conditions {
		cond, _ := c.(map[string]any)
		if cond["type"] != "Available" {
			continue
		}
		if cond["status"] == "True" {
			return "", false
		}
		why, _ := cond["reason"].(string)
		msg, _ := cond["message"].(string)
		switch {
		case why != "" && msg != "":
			return why + ": " + msg, true
		case msg != "":
			return msg, true
		case why != "":
			return why, true
		}
		return "Available=" + stringOr(cond["status"], "Unknown"), true
	}
	return "", false
}

func stringOr(v any, fallback string) string {
	if s, ok := v.(string); ok && s != "" {
		return s
	}
	return fallback
}
