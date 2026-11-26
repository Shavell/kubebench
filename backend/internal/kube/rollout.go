package kube

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"kubebench/internal/reason"
)

const (
	revisionAnnotation    = "deployment.kubernetes.io/revision"
	changeCauseAnnotation = "kubernetes.io/change-cause"
)

// rollbackSkippedAnnotations are the Deployment annotations a rollback
// keeps instead of taking them from the ReplicaSet (kubectl rollout undo).
var rollbackSkippedAnnotations = []string{
	"kubectl.kubernetes.io/last-applied-configuration",
	revisionAnnotation,
	"deployment.kubernetes.io/revision-history",
	"deployment.kubernetes.io/desired-replicas",
	"deployment.kubernetes.io/max-replicas",
	"apps.kubernetes.io/deprecated-deployment-rollback-to",
}

// RolloutRevision is one revision of a Deployment: one of its ReplicaSets
// (src/data/types.ts RolloutRevision).
type RolloutRevision struct {
	Revision      int64    `json:"revision"`
	ReplicaSet    string   `json:"replicaSet"`
	Images        []string `json:"images"`
	ChangeCause   string   `json:"changeCause"`
	Created       string   `json:"created"` // RFC 3339
	Replicas      int32    `json:"replicas"`
	ReadyReplicas int32    `json:"readyReplicas"`
	Current       bool     `json:"current"`
}

// RolloutHistory is a Deployment's revisions, newest first.
type RolloutHistory struct {
	CurrentRevision int64             `json:"currentRevision"`
	Paused          bool              `json:"paused"`
	Revisions       []RolloutRevision `json:"revisions"`
}

// DeploymentHistory lists the revisions of a Deployment from the
// ReplicaSets it owns, like kubectl rollout history.
func (r *Runtime) DeploymentHistory(ctx context.Context, namespace, name string) (*RolloutHistory, error) {
	d, err := r.CS.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	sets, err := r.ownedReplicaSets(ctx, d)
	if err != nil {
		return nil, err
	}
	// The current revision is the ReplicaSet running the Deployment's pod
	// template. The revision annotations lag behind it for a moment while
	// the controller moves the Deployment to another ReplicaSet; until that
	// ReplicaSet exists, the Deployment's annotation is all there is.
	current := revisionOf(d.Annotations)
	newRS := newReplicaSet(d, sets)
	if newRS != nil {
		current = revisionOf(newRS.Annotations)
	}
	history := &RolloutHistory{CurrentRevision: current, Paused: d.Spec.Paused, Revisions: []RolloutRevision{}}
	for i := range sets {
		rs := &sets[i]
		revision := revisionOf(rs.Annotations)
		if revision == 0 {
			continue // not (yet) part of the rollout history
		}
		isCurrent := revision == current
		if newRS != nil {
			isCurrent = rs.Name == newRS.Name
		}
		images := []string{}
		for _, c := range rs.Spec.Template.Spec.Containers {
			images = append(images, c.Image)
		}
		history.Revisions = append(history.Revisions, RolloutRevision{
			Revision:      revision,
			ReplicaSet:    rs.Name,
			Images:        images,
			ChangeCause:   rs.Annotations[changeCauseAnnotation],
			Created:       rs.CreationTimestamp.UTC().Format(time.RFC3339),
			Replicas:      rs.Status.Replicas,
			ReadyReplicas: rs.Status.ReadyReplicas,
			Current:       isCurrent,
		})
	}
	slices.SortFunc(history.Revisions, func(a, b RolloutRevision) int { return cmp.Compare(b.Revision, a.Revision) })
	return history, nil
}

// RollbackDeployment rolls a Deployment back to an earlier revision, like
// kubectl rollout undo --to-revision: the pod template of that revision's
// ReplicaSet (minus its pod-template-hash label) and its annotations go
// back into the Deployment (merged into the Deployment's own,
// rollbackAnnotationOps), which then rolls out as a new revision. A
// paused Deployment and the current revision (the ReplicaSet whose
// template the Deployment already has) are refused.
func (r *Runtime) RollbackDeployment(ctx context.Context, namespace, name string, revision int64) error {
	deployments := r.CS.AppsV1().Deployments(namespace)
	d, err := deployments.Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return err
	}
	if d.Spec.Paused {
		return opError(OpConflict, reasonRolloutPaused, reason.Params{"name": name})
	}
	sets, err := r.ownedReplicaSets(ctx, d)
	if err != nil {
		return err
	}
	var target *appsv1.ReplicaSet
	for i := range sets {
		if revisionOf(sets[i].Annotations) == revision {
			target = &sets[i]
		}
	}
	if target == nil {
		return opError(OpNotFound, reasonRolloutNoRevision, reason.Params{"name": name, "revision": revision})
	}
	if equalIgnoreHash(&target.Spec.Template, &d.Spec.Template) {
		return opError(OpConflict, reasonRolloutCurrent, reason.Params{"name": name, "revision": revision})
	}

	template := target.Spec.Template.DeepCopy()
	delete(template.Labels, appsv1.DefaultDeploymentUniqueLabelKey)
	ops := append([]map[string]any{{"op": "replace", "path": "/spec/template", "value": template}},
		rollbackAnnotationOps(d, target)...)
	patch, err := json.Marshal(ops)
	if err != nil {
		return err
	}
	_, err = deployments.Patch(ctx, name, types.JSONPatchType, patch, metav1.PatchOptions{FieldManager: FieldManager})
	return err
}

// rollbackAnnotationOps are the JSON patch operations that bring the
// annotations of the ReplicaSet rs (those the Deployment had when it made
// that revision, e.g. its change-cause) back into the Deployment d, like
// kubectl rollout undo, except the bookkeeping ones
// (rollbackSkippedAnnotations), which the Deployment keeps. Unlike kubectl,
// which replaces the annotations wholesale, the Deployment's other
// annotations stay: each key is set on its own, so annotations added since
// (or by others meanwhile) survive. Only the change-cause, which describes
// the revision, goes when rs has none, so that the revision rolled back to
// does not inherit the cause of the one rolled back from.
func rollbackAnnotationOps(d *appsv1.Deployment, rs *appsv1.ReplicaSet) []map[string]any {
	taken := map[string]string{}
	for k, v := range rs.Annotations {
		if !slices.Contains(rollbackSkippedAnnotations, k) {
			taken[k] = v
		}
	}
	if len(d.Annotations) == 0 {
		if len(taken) == 0 {
			return nil
		}
		// "add" creates the map the Deployment does not have.
		return []map[string]any{{"op": "add", "path": "/metadata/annotations", "value": taken}}
	}
	var ops []map[string]any
	for _, k := range slices.Sorted(maps.Keys(taken)) {
		if old, ok := d.Annotations[k]; !ok || old != taken[k] {
			ops = append(ops, map[string]any{"op": "add", "path": "/metadata/annotations/" + jsonPointerEscape(k), "value": taken[k]})
		}
	}
	if _, had := d.Annotations[changeCauseAnnotation]; had {
		if _, has := taken[changeCauseAnnotation]; !has {
			ops = append(ops, map[string]any{"op": "remove", "path": "/metadata/annotations/" + jsonPointerEscape(changeCauseAnnotation)})
		}
	}
	return ops
}

// jsonPointerEscape escapes a key for a JSON pointer (RFC 6901).
func jsonPointerEscape(key string) string {
	return strings.ReplaceAll(strings.ReplaceAll(key, "~", "~0"), "/", "~1")
}

// PauseDeployment pauses or resumes a Deployment's rollouts (kubectl
// rollout pause / resume). Template changes made while paused roll out
// once it is resumed.
func (r *Runtime) PauseDeployment(ctx context.Context, namespace, name string, paused bool) error {
	patch := fmt.Sprintf(`{"spec":{"paused":%t}}`, paused)
	_, err := r.CS.AppsV1().Deployments(namespace).Patch(ctx, name, types.MergePatchType, []byte(patch), metav1.PatchOptions{FieldManager: FieldManager})
	return err
}

// ownedReplicaSets lists the ReplicaSets the Deployment controls: those
// its selector matches whose controller reference is the Deployment.
func (r *Runtime) ownedReplicaSets(ctx context.Context, d *appsv1.Deployment) ([]appsv1.ReplicaSet, error) {
	selector, err := metav1.LabelSelectorAsSelector(d.Spec.Selector)
	if err != nil {
		return nil, err
	}
	list, err := r.CS.AppsV1().ReplicaSets(d.Namespace).List(ctx, metav1.ListOptions{LabelSelector: selector.String()})
	if err != nil {
		return nil, err
	}
	var owned []appsv1.ReplicaSet
	for _, rs := range list.Items {
		if ref := metav1.GetControllerOf(&rs); ref != nil && ref.UID == d.UID {
			owned = append(owned, rs)
		}
	}
	return owned, nil
}

// newReplicaSet is the ReplicaSet running the Deployment's pod template,
// found the way the Deployment controller and kubectl find it: the oldest
// one whose template equals the Deployment's. Nil until the controller
// created it.
func newReplicaSet(d *appsv1.Deployment, sets []appsv1.ReplicaSet) *appsv1.ReplicaSet {
	var found *appsv1.ReplicaSet
	for i := range sets {
		rs := &sets[i]
		if equalIgnoreHash(&rs.Spec.Template, &d.Spec.Template) &&
			(found == nil || rs.CreationTimestamp.Before(&found.CreationTimestamp)) {
			found = rs
		}
	}
	return found
}

// equalIgnoreHash compares pod templates without the pod-template-hash
// label, which only ReplicaSets carry.
func equalIgnoreHash(a, b *corev1.PodTemplateSpec) bool {
	a, b = a.DeepCopy(), b.DeepCopy()
	delete(a.Labels, appsv1.DefaultDeploymentUniqueLabelKey)
	delete(b.Labels, appsv1.DefaultDeploymentUniqueLabelKey)
	return apiequality.Semantic.DeepEqual(a, b)
}

// revisionOf reads the deployment.kubernetes.io/revision annotation; 0 when
// it is missing or invalid.
func revisionOf(annotations map[string]string) int64 {
	n, err := strconv.ParseInt(annotations[revisionAnnotation], 10, 64)
	if err != nil || n < 0 {
		return 0
	}
	return n
}
