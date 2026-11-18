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

// scalableKinds are the kinds ScaleWorkload scales through their scale
// subresource.
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

// ScaleWorkload sets the replicas of a Deployment, StatefulSet or
// ReplicaSet through its scale subresource, like kubectl scale. Only the
// scale subresource is patched, so update permission on <kind>/scale is
// enough.
func (r *Runtime) ScaleWorkload(ctx context.Context, kind, namespace, name string, replicas int32) error {
	resource, ok := scalableKinds[kind]
	if !ok {
		return opError(OpInvalid, reasonScaleKind, reason.Params{"kind": kind})
	}
	if replicas < 0 {
		return opError(OpInvalid, reasonReplicasNegative, nil)
	}
	patch := []byte(fmt.Sprintf(`{"spec":{"replicas":%d}}`, replicas))
	_, err := r.Dyn.Resource(resource).Namespace(namespace).Patch(ctx, name, types.MergePatchType, patch, metav1.PatchOptions{FieldManager: FieldManager}, "scale")
	return err
}
