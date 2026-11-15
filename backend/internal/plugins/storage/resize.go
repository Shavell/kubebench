package storage

import (
	"context"
	"encoding/json"
	"net/http"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"kubebench/internal/kube"
	"kubebench/internal/plugin"
)

// ResizeRequest is the body of PATCH /pvcs/{ns}/{name}/resize.
type ResizeRequest struct {
	Size string `json:"size"`
}

// Why a claim cannot be expanded (PVCSummary.ExpandBlocker); "" means it can.
const (
	blockerNotBound     = "not_bound"
	blockerNoClass      = "no_class"
	blockerClassMissing = "class_missing"
	blockerNotAllowed   = "not_allowed"
)

// expandBlocker says why pvc cannot be expanded: expansion needs a bound
// claim whose StorageClass sets allowVolumeExpansion. classes maps class
// names to whether they allow it.
func expandBlocker(pvc *corev1.PersistentVolumeClaim, classes map[string]bool) string {
	if pvc.Status.Phase != corev1.ClaimBound {
		return blockerNotBound
	}
	class := ""
	if pvc.Spec.StorageClassName != nil {
		class = *pvc.Spec.StorageClassName
	}
	if class == "" {
		return blockerNoClass
	}
	allowed, ok := classes[class]
	switch {
	case !ok:
		return blockerClassMissing
	case !allowed:
		return blockerNotAllowed
	}
	return ""
}

// blockerMessage explains a blocker to API clients.
func blockerMessage(pvc *corev1.PersistentVolumeClaim, blocker string) string {
	switch blocker {
	case blockerNotBound:
		return "only a Bound claim can be expanded (it is " + phaseOr(string(pvc.Status.Phase), "Pending") + ")"
	case blockerNoClass:
		return "the claim has no StorageClass, so it cannot be expanded"
	case blockerClassMissing:
		return "the claim's StorageClass " + *pvc.Spec.StorageClassName + " does not exist"
	default:
		return "the StorageClass " + *pvc.Spec.StorageClassName + " does not set allowVolumeExpansion"
	}
}

// resizePVC serves PATCH /pvcs/{ns}/{name}/resize: it raises the claim's
// storage request, which the volume's driver then expands. Volumes only
// grow, and only when their class allows expansion.
func resizePVC(w http.ResponseWriter, r *http.Request, c *plugin.Context) {
	ns, name := r.PathValue("ns"), r.PathValue("name")
	var req ResizeRequest
	if err := decodeBody(w, r, &req); err != nil {
		writeErr(w, err)
		return
	}
	size, err := storageSize("size", req.Size)
	if err != nil {
		writeErr(w, err)
		return
	}
	updated, from, err := resize(r.Context(), c.Runtime, ns, name, size)
	if err != nil {
		writeErr(w, err)
		return
	}
	if c.SetAudit != nil {
		c.SetAudit("", auditJSON(map[string]any{"from": from.String(), "to": size.String()}))
	}
	classes, err := classExpansion(r.Context(), c.Runtime)
	if err != nil {
		writeErr(w, err)
		return
	}
	plugin.WriteJSON(w, summarizePVC(updated, classes))
}

// resize checks and patches the claim, returning it and its former request.
func resize(ctx context.Context, rt *kube.Runtime, ns, name string, size resource.Quantity) (*corev1.PersistentVolumeClaim, resource.Quantity, error) {
	claims := rt.CS.CoreV1().PersistentVolumeClaims(ns)
	pvc, err := claims.Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, resource.Quantity{}, err
	}
	current := requested(pvc)
	if size.Cmp(current) <= 0 {
		return nil, current, invalidf("size", "must be larger than the current request (%s): volumes cannot shrink", current.String())
	}
	if capacity, ok := pvc.Status.Capacity[corev1.ResourceStorage]; ok && size.Cmp(capacity) <= 0 {
		return nil, current, invalidf("size", "must be larger than the current capacity (%s)", capacity.String())
	}
	classes := map[string]bool{}
	if class := pvc.Spec.StorageClassName; class != nil && *class != "" {
		sc, err := rt.CS.StorageV1().StorageClasses().Get(ctx, *class, metav1.GetOptions{})
		switch {
		case err == nil:
			classes[sc.Name] = sc.AllowVolumeExpansion != nil && *sc.AllowVolumeExpansion
		case !apierrors.IsNotFound(err):
			return nil, current, err
		}
	}
	if blocker := expandBlocker(pvc, classes); blocker != "" {
		return nil, current, conflictf("%s", blockerMessage(pvc, blocker))
	}
	// The resourceVersion makes the patch fail with a conflict if the claim
	// changed since it was checked.
	patch, err := json.Marshal(map[string]any{
		"metadata": map[string]any{"resourceVersion": pvc.ResourceVersion},
		"spec":     map[string]any{"resources": map[string]any{"requests": map[string]string{"storage": size.String()}}},
	})
	if err != nil {
		return nil, current, err
	}
	updated, err := claims.Patch(ctx, name, types.MergePatchType, patch, metav1.PatchOptions{FieldManager: fieldManager})
	return updated, current, err
}
