package storage

import (
	"context"
	"net/http"
	"slices"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"kubebench/internal/kube"
	"kubebench/internal/plugin"
	"kubebench/internal/reason"
)

// Kinds a claim may be cloned from (spec.dataSource).
const (
	sourcePVC      = "PersistentVolumeClaim"
	sourceSnapshot = "VolumeSnapshot"
)

// PVCRequest is the body of POST /pvcs.
type PVCRequest struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	// StorageClassName: nil (omitted) lets the cluster's default class
	// apply; "" asks for no class (static binding only).
	StorageClassName *string  `json:"storageClassName"`
	AccessModes      []string `json:"accessModes"`
	Size             string   `json:"size"`
	VolumeMode       string   `json:"volumeMode,omitempty"`
	// VolumeName binds the claim to this Available PersistentVolume.
	VolumeName string `json:"volumeName,omitempty"`
	// DataSource clones a PVC or restores a VolumeSnapshot of the same namespace.
	DataSource *DataSource       `json:"dataSource,omitempty"`
	Labels     map[string]string `json:"labels,omitempty"`
}

// DataSource names the object a new claim is populated from.
type DataSource struct {
	Kind string `json:"kind"`
	Name string `json:"name"`
}

// pvcSpec is a validated PVCRequest.
type pvcSpec struct {
	req   PVCRequest
	size  resource.Quantity
	modes []corev1.PersistentVolumeAccessMode
	mode  *corev1.PersistentVolumeMode
}

// validate checks the request on its own, without the cluster.
func (req PVCRequest) validate() (pvcSpec, error) {
	s := pvcSpec{req: req}
	var err error
	if err = namespaceName("namespace", req.Namespace); err != nil {
		return s, err
	}
	if err = objectName("name", req.Name); err != nil {
		return s, err
	}
	if req.StorageClassName != nil {
		if err = storageClassRef("storageClassName", *req.StorageClassName); err != nil {
			return s, err
		}
	}
	if s.modes, err = accessModes("accessModes", req.AccessModes); err != nil {
		return s, err
	}
	if s.size, err = storageSize("size", req.Size); err != nil {
		return s, err
	}
	if s.mode, err = volumeMode("volumeMode", req.VolumeMode); err != nil {
		return s, err
	}
	if req.VolumeName != "" {
		if err = objectName("volumeName", req.VolumeName); err != nil {
			return s, err
		}
		if req.DataSource != nil {
			return s, invalidField("dataSource", reasonBoundClone, nil)
		}
	}
	if ds := req.DataSource; ds != nil {
		if ds.Kind != sourcePVC && ds.Kind != sourceSnapshot {
			return s, invalidField("dataSource.kind", reasonSourceKind, reason.Params{"kind": ds.Kind, "pvc": sourcePVC, "snapshot": sourceSnapshot})
		}
		if err = objectName("dataSource.name", ds.Name); err != nil {
			return s, err
		}
	}
	return s, labels("labels", req.Labels)
}

// object is the claim to create.
func (s pvcSpec) object() *corev1.PersistentVolumeClaim {
	pvc := &corev1.PersistentVolumeClaim{
		TypeMeta:   metav1.TypeMeta{APIVersion: "v1", Kind: "PersistentVolumeClaim"},
		ObjectMeta: metav1.ObjectMeta{Name: s.req.Name, Namespace: s.req.Namespace, Labels: s.req.Labels},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes:      s.modes,
			StorageClassName: s.req.StorageClassName,
			VolumeMode:       s.mode,
			VolumeName:       s.req.VolumeName,
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceStorage: s.size},
			},
		},
	}
	if ds := s.req.DataSource; ds != nil {
		ref := &corev1.TypedLocalObjectReference{Kind: ds.Kind, Name: ds.Name}
		if ds.Kind == sourceSnapshot {
			group := snapshotGroup
			ref.APIGroup = &group
		}
		pvc.Spec.DataSource = ref
	}
	return pvc
}

// checkCluster checks what the request refers to: the volume to bind and
// the claim or snapshot to clone. It returns warnings that do not stop the
// create.
func (s pvcSpec) checkCluster(ctx context.Context, rt *kube.Runtime) ([]string, error) {
	if s.req.VolumeName != "" {
		return nil, s.checkVolume(ctx, rt)
	}
	ds := s.req.DataSource
	if ds == nil {
		return nil, nil
	}
	if ds.Kind == sourcePVC {
		src, err := rt.CS.CoreV1().PersistentVolumeClaims(s.req.Namespace).Get(ctx, ds.Name, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			return nil, invalidField("dataSource.name", reasonNoSourceClaim, reason.Params{"name": ds.Name, "namespace": s.req.Namespace})
		}
		if err != nil {
			return nil, err
		}
		if want := requested(src); s.size.Cmp(want) < 0 {
			return nil, invalidField("size", reasonCloneSize, reason.Params{"size": want.String()})
		}
		return nil, nil
	}
	snap, err := getSnapshot(ctx, rt, s.req.Namespace, ds.Name)
	if err != nil {
		return nil, err
	}
	if snap.RestoreSize != "" {
		if want, err := resource.ParseQuantity(snap.RestoreSize); err == nil && s.size.Cmp(want) < 0 {
			return nil, invalidField("size", reasonRestoreSize, reason.Params{"size": snap.RestoreSize})
		}
	}
	if !snap.ReadyToUse {
		return []string{"the snapshot is not ready to use yet; the claim stays Pending until it is"}, nil
	}
	return nil, nil
}

// checkVolume checks that the PV to bind exists, is Available and can
// satisfy the claim, so the claim does not wait Pending forever.
func (s pvcSpec) checkVolume(ctx context.Context, rt *kube.Runtime) error {
	pv, err := rt.CS.CoreV1().PersistentVolumes().Get(ctx, s.req.VolumeName, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return invalidField("volumeName", reasonNoVolume, reason.Params{"name": s.req.VolumeName})
	}
	if err != nil {
		return err
	}
	if pv.Status.Phase != corev1.VolumeAvailable {
		return invalidField("volumeName", reasonVolumePhase, reason.Params{"name": pv.Name, "phase": phaseOr(string(pv.Status.Phase), "Pending")})
	}
	if ref := pv.Spec.ClaimRef; ref != nil && (ref.Namespace != s.req.Namespace || ref.Name != s.req.Name) {
		return invalidField("volumeName", reasonVolumeReserved, reason.Params{"name": pv.Name, "claim": ref.Namespace + "/" + ref.Name})
	}
	if class := pv.Spec.StorageClassName; s.req.StorageClassName == nil || *s.req.StorageClassName != class {
		return invalidField("storageClassName", reasonVolumeClass, reason.Params{"class": phaseOr(class, `""`)})
	}
	if capacity := pv.Spec.Capacity[corev1.ResourceStorage]; capacity.Cmp(s.size) < 0 {
		return invalidField("size", reasonVolumeCapacity, reason.Params{"name": pv.Name, "size": capacity.String()})
	}
	for _, m := range s.modes {
		if !slices.Contains(pv.Spec.AccessModes, m) {
			return invalidField("accessModes", reasonVolumeAccessMode, reason.Params{"name": pv.Name, "mode": string(m)})
		}
	}
	if modeOf(pv.Spec.VolumeMode) != modeOf(s.mode) {
		return invalidField("volumeMode", reasonVolumeMode, reason.Params{"name": pv.Name, "mode": string(modeOf(pv.Spec.VolumeMode))})
	}
	return nil
}

// createPVC serves POST /pvcs?dryRun=.
func createPVC(w http.ResponseWriter, r *http.Request, c *plugin.Context) {
	var req PVCRequest
	if err := decodeBody(w, r, &req); err != nil {
		writeErr(w, err)
		return
	}
	spec, err := req.validate()
	if err != nil {
		writeErr(w, err)
		return
	}
	warnings, err := spec.checkCluster(r.Context(), c.Runtime)
	if err != nil {
		writeErr(w, err)
		return
	}
	detail := map[string]any{"size": req.Size, "accessModes": spec.modes}
	if req.StorageClassName != nil {
		detail["storageClass"] = *req.StorageClassName
	}
	if req.VolumeName != "" {
		detail["volumeName"] = req.VolumeName
	}
	if ds := req.DataSource; ds != nil {
		detail["dataSource"] = ds.Kind + "/" + ds.Name
	}
	auditCreate(c, "pvc", req.Namespace+"/"+req.Name, detail)

	created, err := c.Runtime.CS.CoreV1().PersistentVolumeClaims(req.Namespace).Create(r.Context(), spec.object(), createOptions(r))
	if err != nil {
		writeErr(w, err)
		return
	}
	plugin.WriteJSON(w, CreateResult{
		APIVersion: "v1", Kind: "PersistentVolumeClaim", Namespace: created.Namespace, Name: created.Name,
		DryRun: plugin.IsDryRun(r), Warnings: warnings,
	})
}

// requested is a claim's requested storage.
func requested(pvc *corev1.PersistentVolumeClaim) resource.Quantity {
	return pvc.Spec.Resources.Requests[corev1.ResourceStorage]
}

func modeOf(m *corev1.PersistentVolumeMode) corev1.PersistentVolumeMode {
	if m == nil || *m == "" {
		return corev1.PersistentVolumeFilesystem
	}
	return *m
}

func phaseOr(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}
