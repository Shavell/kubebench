package kube

import (
	"context"
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"

	"kubebench/internal/reason"
)

// restartedAtAnnotation is the pod template annotation kubectl rollout
// restart sets; changing it rolls the pods.
const restartedAtAnnotation = "kubectl.kubernetes.io/restartedAt"

// scalableKinds are the kinds ScaleWorkload scales by kind alone, without
// an apiVersion.
var scalableKinds = map[string]schema.GroupVersionResource{
	"Deployment":  gvr("apps", "v1", "deployments"),
	"StatefulSet": gvr("apps", "v1", "statefulsets"),
	"ReplicaSet":  gvr("apps", "v1", "replicasets"),
}

// RestartWorkload restarts the pods of a Deployment, StatefulSet or
// DaemonSet the way kubectl rollout restart does: stamping the pod template
// with the time makes the controller replace the pods following the
// workload's update strategy.
func (r *Runtime) RestartWorkload(ctx context.Context, kind, namespace, name string) error {
	patch := []byte(fmt.Sprintf(`{"spec":{"template":{"metadata":{"annotations":{%q:%q}}}}}`,
		restartedAtAnnotation, time.Now().Format(time.RFC3339)))
	apps := r.CS.AppsV1()
	var err error
	switch kind {
	case "Deployment":
		_, err = apps.Deployments(namespace).Patch(ctx, name, types.StrategicMergePatchType, patch, metav1.PatchOptions{FieldManager: FieldManager})
	case "StatefulSet":
		_, err = apps.StatefulSets(namespace).Patch(ctx, name, types.StrategicMergePatchType, patch, metav1.PatchOptions{FieldManager: FieldManager})
	case "DaemonSet":
		_, err = apps.DaemonSets(namespace).Patch(ctx, name, types.StrategicMergePatchType, patch, metav1.PatchOptions{FieldManager: FieldManager})
	default:
		return opError(OpInvalid, reasonRestartKind, reason.Params{"kind": kind})
	}
	return err
}

// ScaleWorkload sets the replicas of a workload through its scale
// subresource, like kubectl scale. Without apiVersion kind is a
// Deployment, StatefulSet or ReplicaSet (apps/v1); with it, any kind the
// cluster serves with a scale subresource (discovered: e.g. an Argo
// Rollout or another custom resource whose CRD enables it). Only the scale
// subresource is patched, so update permission on <resource>/scale is
// enough.
func (r *Runtime) ScaleWorkload(ctx context.Context, apiVersion, kind, namespace, name string, replicas int32) error {
	if replicas < 0 {
		return opError(OpInvalid, reasonReplicasNegative, nil)
	}
	resource, namespaced, err := r.scaleResource(apiVersion, kind)
	if err != nil {
		return err
	}
	client := r.Dyn.Resource(resource)
	if !namespaced {
		namespace = ""
	}
	patch := []byte(fmt.Sprintf(`{"spec":{"replicas":%d}}`, replicas))
	_, err = client.Namespace(namespace).Patch(ctx, name, types.MergePatchType, patch, metav1.PatchOptions{FieldManager: FieldManager}, "scale")
	return err
}

// scaleResource is the resource ScaleWorkload scales kind through, and
// whether it is namespaced.
func (r *Runtime) scaleResource(apiVersion, kind string) (schema.GroupVersionResource, bool, error) {
	if apiVersion == "" {
		resource, ok := scalableKinds[kind]
		if !ok {
			return resource, false, opError(OpInvalid, reasonScaleKind, reason.Params{"kind": kind})
		}
		return resource, true, nil
	}
	resource, namespaced, err := r.ResolveKind(apiVersion, kind)
	if err != nil {
		return resource, false, err
	}
	found, err := r.hasSubresource(resource, "scale")
	if err == nil && !found && r.resetMapper() {
		// A CRD may have enabled the subresource since discovery was cached.
		found, err = r.hasSubresource(resource, "scale")
	}
	if err != nil {
		return resource, false, err
	}
	if !found {
		return resource, false, opError(OpInvalid, reasonScaleKind, reason.Params{"kind": kind})
	}
	return resource, namespaced, nil
}

// hasSubresource reports whether discovery lists resource/sub.
func (r *Runtime) hasSubresource(resource schema.GroupVersionResource, sub string) (bool, error) {
	list, err := r.discovery.ServerResourcesForGroupVersion(resource.GroupVersion().String())
	if err != nil {
		return false, err
	}
	for _, res := range list.APIResources {
		if res.Name == resource.Resource+"/"+sub {
			return true, nil
		}
	}
	return false, nil
}
