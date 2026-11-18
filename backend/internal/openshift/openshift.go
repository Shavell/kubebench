// Package openshift reads the OpenShift-specific APIs KubeBench uses
// (docs/design-v0.5.md §7.15): the cluster version and the
// SecurityContextConstraints. It works over the dynamic client and plain
// structs, without OpenShift's client libraries; on a cluster that does
// not serve an API, the readers say so instead of failing.
package openshift

import (
	"context"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

// API groups and resources read here.
var (
	ClusterVersionGVR = schema.GroupVersionResource{Group: "config.openshift.io", Version: "v1", Resource: "clusterversions"}
	SCCGVR            = schema.GroupVersionResource{Group: "security.openshift.io", Version: "v1", Resource: "securitycontextconstraints"}
)

// SCCAnnotation is the annotation OpenShift's admission sets on every pod:
// the name of the SCC that admitted it.
const SCCAnnotation = "openshift.io/scc"

// NotServed reports whether err means the cluster does not serve the API
// (a plain Kubernetes cluster), as opposed to a refusal or an outage.
func NotServed(err error) bool {
	return apierrors.IsNotFound(err) || meta.IsNoMatchError(err)
}

// clusterVersion is the part of config.openshift.io/v1 ClusterVersion read.
type clusterVersion struct {
	Status struct {
		Desired struct {
			Version string `json:"version"`
		} `json:"desired"`
		History []struct {
			State   string `json:"state"`
			Version string `json:"version"`
		} `json:"history"`
	} `json:"status"`
}

// Version reads the OpenShift version of the cluster ("5.0.0") from the
// ClusterVersion "version": the newest completed update in its history,
// else the desired version (an install still in progress). It answers ""
// on a cluster without the API, or when the credential may not read it.
func Version(ctx context.Context, dyn dynamic.Interface) string {
	obj, err := dyn.Resource(ClusterVersionGVR).Get(ctx, "version", metav1.GetOptions{})
	if err != nil {
		return ""
	}
	var cv clusterVersion
	if err := fromUnstructured(obj.Object, &cv); err != nil {
		return ""
	}
	// History is newest first.
	for _, h := range cv.Status.History {
		if h.State == "Completed" && h.Version != "" {
			return h.Version
		}
	}
	return cv.Status.Desired.Version
}
