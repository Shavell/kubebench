package kube

import (
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/client-go/util/retry"
)

// Taint is a node taint as the API reports and accepts it (src/data/types.ts NodeTaint).
type Taint struct {
	Key    string `json:"key"`
	Value  string `json:"value"`
	Effect string `json:"effect"`
}

func (t Taint) String() string {
	if t.Value == "" {
		return t.Key + ":" + t.Effect
	}
	return t.Key + "=" + t.Value + ":" + t.Effect
}

var taintEffects = []corev1.TaintEffect{
	corev1.TaintEffectNoSchedule, corev1.TaintEffectPreferNoSchedule, corev1.TaintEffectNoExecute,
}

// CordonNode marks the node unschedulable, or schedulable again, with a
// merge patch on spec.unschedulable (kubectl cordon / uncordon). Pods that
// already run on the node stay.
func (r *Runtime) CordonNode(ctx context.Context, name string, unschedulable bool) error {
	patch := fmt.Sprintf(`{"spec":{"unschedulable":%t}}`, unschedulable)
	_, err := r.CS.CoreV1().Nodes().Patch(ctx, name, types.MergePatchType, []byte(patch), metav1.PatchOptions{})
	return err
}

// SetNodeTaints replaces the node's taints with taints and returns the
// stored list. A taint that stays (same key, value and effect) keeps its
// timeAdded, so the tolerationSeconds of pods on a NoExecute taint keep
// counting from when it was first added.
func (r *Runtime) SetNodeTaints(ctx context.Context, name string, taints []Taint) ([]Taint, error) {
	if err := validateTaints(taints); err != nil {
		return nil, err
	}
	nodes := r.CS.CoreV1().Nodes()
	var updated *corev1.Node
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		node, err := nodes.Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		node.Spec.Taints = withTimeAdded(taints, node.Spec.Taints)
		updated, err = nodes.Update(ctx, node, metav1.UpdateOptions{})
		return err
	})
	if err != nil {
		return nil, err
	}
	return mapTaints(updated.Spec.Taints), nil
}

// validateTaints applies the API server's rules up front so the reason is
// clear: a qualified-name key, a label-syntax value, a known effect, and
// each key at most once per effect.
func validateTaints(taints []Taint) error {
	seen := map[string]bool{}
	for i, t := range taints {
		if problems := validation.IsQualifiedName(t.Key); len(problems) > 0 {
			return opError(OpInvalid, "taint %d: key %q: %s", i+1, t.Key, strings.Join(problems, "; "))
		}
		if problems := validation.IsValidLabelValue(t.Value); len(problems) > 0 {
			return opError(OpInvalid, "taint %q: value %q: %s", t.Key, t.Value, strings.Join(problems, "; "))
		}
		if !validEffect(t.Effect) {
			return opError(OpInvalid, "taint %q: effect must be NoSchedule, PreferNoSchedule or NoExecute, not %q", t.Key, t.Effect)
		}
		id := t.Key + ":" + t.Effect
		if seen[id] {
			return opError(OpInvalid, "taint %q with effect %s is listed twice", t.Key, t.Effect)
		}
		seen[id] = true
	}
	return nil
}

func validEffect(effect string) bool {
	for _, e := range taintEffects {
		if string(e) == effect {
			return true
		}
	}
	return false
}

// withTimeAdded builds the node's new taint list, taking timeAdded over
// from the current taints that are kept unchanged.
func withTimeAdded(desired []Taint, current []corev1.Taint) []corev1.Taint {
	out := make([]corev1.Taint, 0, len(desired))
	for _, t := range desired {
		taint := corev1.Taint{Key: t.Key, Value: t.Value, Effect: corev1.TaintEffect(t.Effect)}
		for _, c := range current {
			if c.Key == taint.Key && c.Value == taint.Value && c.Effect == taint.Effect {
				taint.TimeAdded = c.TimeAdded
			}
		}
		out = append(out, taint)
	}
	return out
}

func mapTaints(taints []corev1.Taint) []Taint {
	out := make([]Taint, 0, len(taints))
	for _, t := range taints {
		out = append(out, Taint{Key: t.Key, Value: t.Value, Effect: string(t.Effect)})
	}
	return out
}
