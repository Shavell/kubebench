package storage

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strings"

	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"kubebench/internal/kube"
	"kubebench/internal/plugin"
	"kubebench/internal/reason"
)

// Annotations marking the default StorageClass (the beta one is still
// honoured by the API server).
const (
	defaultClassAnnotation     = "storageclass.kubernetes.io/is-default-class"
	betaDefaultClassAnnotation = "storageclass.beta.kubernetes.io/is-default-class"
)

// StorageClassRequest is the body of POST /storageclasses.
type StorageClassRequest struct {
	Name        string            `json:"name"`
	Provisioner string            `json:"provisioner"`
	Parameters  map[string]string `json:"parameters,omitempty"`
	// ReclaimPolicy is Delete (the default) or Retain.
	ReclaimPolicy string `json:"reclaimPolicy,omitempty"`
	// VolumeBindingMode is Immediate (the default) or WaitForFirstConsumer.
	VolumeBindingMode    string            `json:"volumeBindingMode,omitempty"`
	AllowVolumeExpansion bool              `json:"allowVolumeExpansion,omitempty"`
	IsDefault            bool              `json:"isDefault,omitempty"`
	MountOptions         []string          `json:"mountOptions,omitempty"`
	Labels               map[string]string `json:"labels,omitempty"`
}

type storageClassSpec struct {
	req     StorageClassRequest
	reclaim corev1.PersistentVolumeReclaimPolicy
	binding storagev1.VolumeBindingMode
}

func (req StorageClassRequest) validate() (storageClassSpec, error) {
	s := storageClassSpec{req: req}
	if err := objectName("name", req.Name); err != nil {
		return s, err
	}
	if err := qualifiedDriver("provisioner", req.Provisioner); err != nil {
		return s, err
	}
	if len(req.Parameters) > maxParameters {
		return s, invalidField("parameters", reasonTooManyParams, reason.Params{"max": maxParameters})
	}
	for key := range req.Parameters {
		if key == "" {
			return s, invalidField("parameters", reasonEmptyKeys, nil)
		}
	}
	switch p := corev1.PersistentVolumeReclaimPolicy(req.ReclaimPolicy); p {
	case "":
		s.reclaim = corev1.PersistentVolumeReclaimDelete
	case corev1.PersistentVolumeReclaimDelete, corev1.PersistentVolumeReclaimRetain:
		s.reclaim = p
	default:
		return s, invalidField("reclaimPolicy", reasonOneOf, reason.Params{"value": string(req.ReclaimPolicy), "values": "Delete, Retain"})
	}
	switch m := storagev1.VolumeBindingMode(req.VolumeBindingMode); m {
	case "":
		s.binding = storagev1.VolumeBindingImmediate
	case storagev1.VolumeBindingImmediate, storagev1.VolumeBindingWaitForFirstConsumer:
		s.binding = m
	default:
		return s, invalidField("volumeBindingMode", reasonOneOf, reason.Params{"value": string(req.VolumeBindingMode), "values": "Immediate, WaitForFirstConsumer"})
	}
	if err := mountOptions("mountOptions", req.MountOptions); err != nil {
		return s, err
	}
	return s, labels("labels", req.Labels)
}

func (s storageClassSpec) object() *storagev1.StorageClass {
	req := s.req
	sc := &storagev1.StorageClass{
		TypeMeta:          metav1.TypeMeta{APIVersion: "storage.k8s.io/v1", Kind: "StorageClass"},
		ObjectMeta:        metav1.ObjectMeta{Name: req.Name, Labels: req.Labels},
		Provisioner:       req.Provisioner,
		Parameters:        req.Parameters,
		ReclaimPolicy:     &s.reclaim,
		VolumeBindingMode: &s.binding,
		MountOptions:      req.MountOptions,
	}
	if req.AllowVolumeExpansion {
		allow := true
		sc.AllowVolumeExpansion = &allow
	}
	if req.IsDefault {
		sc.Annotations = map[string]string{defaultClassAnnotation: "true"}
	}
	return sc
}

// defaultWarning advises when a new default class joins existing ones:
// the API server then gives claims without a class the newest default.
func defaultWarning(ctx context.Context, rt *kube.Runtime) (string, error) {
	list, err := rt.CS.StorageV1().StorageClasses().List(ctx, metav1.ListOptions{})
	if err != nil {
		return "", err
	}
	var defaults []string
	for i := range list.Items {
		if isDefaultClass(&list.Items[i]) {
			defaults = append(defaults, list.Items[i].Name)
		}
	}
	if len(defaults) == 0 {
		return "", nil
	}
	sort.Strings(defaults)
	return fmt.Sprintf("already marked as default: %s. Claims without a class get the newest default; "+
		"remove the %s annotation from the classes that should no longer be default",
		strings.Join(defaults, ", "), defaultClassAnnotation), nil
}

// createStorageClass serves POST /storageclasses?dryRun=.
func createStorageClass(w http.ResponseWriter, r *http.Request, c *plugin.Context) {
	var req StorageClassRequest
	if err := decodeBody(w, r, &req); err != nil {
		writeErr(w, err)
		return
	}
	spec, err := req.validate()
	if err != nil {
		writeErr(w, err)
		return
	}
	var warnings []string
	if req.IsDefault {
		warning, err := defaultWarning(r.Context(), c.Runtime)
		if err != nil {
			writeErr(w, err)
			return
		}
		if warning != "" {
			warnings = append(warnings, warning)
		}
	}
	auditCreate(c, "storageclass", req.Name, map[string]any{
		"provisioner": req.Provisioner, "reclaimPolicy": spec.reclaim, "volumeBindingMode": spec.binding,
		"allowVolumeExpansion": req.AllowVolumeExpansion, "isDefault": req.IsDefault,
	})

	created, err := c.Runtime.CS.StorageV1().StorageClasses().Create(r.Context(), spec.object(), createOptions(r))
	if err != nil {
		writeErr(w, err)
		return
	}
	plugin.WriteJSON(w, CreateResult{
		APIVersion: "storage.k8s.io/v1", Kind: "StorageClass", Name: created.Name,
		DryRun: plugin.IsDryRun(r), Warnings: warnings,
	})
}

func isDefaultClass(sc *storagev1.StorageClass) bool {
	return sc.Annotations[defaultClassAnnotation] == "true" || sc.Annotations[betaDefaultClassAnnotation] == "true"
}
