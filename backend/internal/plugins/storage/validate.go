package storage

import (
	"path"
	"slices"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/validation"

	"kubebench/internal/reason"
)

// Limits on lists a request may carry; real objects stay far below.
const (
	maxLabels       = 64
	maxParameters   = 512 // the API server's limit for StorageClass parameters
	maxMountOptions = 32
	maxNodes        = 50
)

// objectName checks a DNS-1123 subdomain, the name rule of PVs, PVCs and
// StorageClasses.
func objectName(field, name string) error {
	if name == "" {
		return invalidField(field, reason.FieldRequired, nil)
	}
	if errs := validation.IsDNS1123Subdomain(name); len(errs) > 0 {
		return invalidField(field, reasonInvalidName, reason.Params{"name": name, "detail": strings.Join(errs, "; ")})
	}
	return nil
}

// namespaceName checks a DNS-1123 label, the name rule of namespaces.
func namespaceName(field, name string) error {
	if name == "" {
		return invalidField(field, reason.FieldRequired, nil)
	}
	if errs := validation.IsDNS1123Label(name); len(errs) > 0 {
		return invalidField(field, reasonInvalidNamespace, reason.Params{"name": name, "detail": strings.Join(errs, "; ")})
	}
	return nil
}

// storageSize parses a positive storage quantity such as "10Gi" or "500M".
func storageSize(field, text string) (resource.Quantity, error) {
	if text == "" {
		return resource.Quantity{}, invalidField(field, reason.FieldRequired, nil)
	}
	q, err := resource.ParseQuantity(text)
	if err != nil {
		return resource.Quantity{}, invalidField(field, reasonQuantity, reason.Params{"value": text})
	}
	if q.Sign() <= 0 {
		return resource.Quantity{}, invalidField(field, reasonPositive, nil)
	}
	return q, nil
}

// accessModeAliases maps kubectl's short names to the API's.
var accessModeAliases = map[string]corev1.PersistentVolumeAccessMode{
	"RWO":  corev1.ReadWriteOnce,
	"ROX":  corev1.ReadOnlyMany,
	"RWX":  corev1.ReadWriteMany,
	"RWOP": corev1.ReadWriteOncePod,
}

var accessModeNames = []corev1.PersistentVolumeAccessMode{
	corev1.ReadWriteOnce, corev1.ReadOnlyMany, corev1.ReadWriteMany, corev1.ReadWriteOncePod,
}

// accessModes checks a non-empty list of distinct modes (API names or the
// short RWO/ROX/RWX/RWOP) and returns their API names. ReadWriteOncePod
// excludes every other mode, as the API server requires.
func accessModes(field string, modes []string) ([]corev1.PersistentVolumeAccessMode, error) {
	if len(modes) == 0 {
		return nil, invalidField(field, reasonNoAccessMode, nil)
	}
	out := make([]corev1.PersistentVolumeAccessMode, 0, len(modes))
	for _, m := range modes {
		mode := corev1.PersistentVolumeAccessMode(m)
		if alias, ok := accessModeAliases[m]; ok {
			mode = alias
		}
		if !slices.Contains(accessModeNames, mode) {
			return nil, invalidField(field, reasonAccessMode, reason.Params{"mode": m})
		}
		if slices.Contains(out, mode) {
			return nil, invalidField(field, reasonListedTwice, reason.Params{"value": string(mode)})
		}
		out = append(out, mode)
	}
	if len(out) > 1 && slices.Contains(out, corev1.ReadWriteOncePod) {
		return nil, invalidField(field, reasonRWOPCombined, nil)
	}
	return out, nil
}

// volumeMode checks Filesystem or Block; "" leaves the API default
// (Filesystem) and returns nil.
func volumeMode(field, mode string) (*corev1.PersistentVolumeMode, error) {
	switch m := corev1.PersistentVolumeMode(mode); m {
	case "":
		return nil, nil
	case corev1.PersistentVolumeFilesystem, corev1.PersistentVolumeBlock:
		return &m, nil
	}
	return nil, invalidField(field, reasonVolumeModeValue, reason.Params{"mode": mode})
}

// labels checks label keys (qualified names) and values.
func labels(field string, l map[string]string) error {
	if len(l) > maxLabels {
		return invalidField(field, reasonTooManyLabels, reason.Params{"max": maxLabels})
	}
	for key, value := range l {
		if errs := validation.IsQualifiedName(key); len(errs) > 0 {
			return invalidField(field, reasonLabelKey, reason.Params{"key": key, "detail": strings.Join(errs, "; ")})
		}
		if errs := validation.IsValidLabelValue(value); len(errs) > 0 {
			return invalidField(field, reasonLabelValue, reason.Params{"key": key, "detail": strings.Join(errs, "; ")})
		}
	}
	return nil
}

// mountOptions checks a short list of non-empty options without spaces.
func mountOptions(field string, opts []string) error {
	if len(opts) > maxMountOptions {
		return invalidField(field, reasonTooManyOptions, reason.Params{"max": maxMountOptions})
	}
	for _, o := range opts {
		if o == "" || strings.ContainsAny(o, " \t\n") {
			return invalidField(field, reasonMountOption, reason.Params{"value": o})
		}
	}
	return nil
}

// absolutePath checks a clean absolute path such as /mnt/data.
func absolutePath(field, p string) error {
	if p == "" {
		return invalidField(field, reason.FieldRequired, nil)
	}
	if !strings.HasPrefix(p, "/") || path.Clean(p) != p {
		return invalidField(field, reasonAbsolutePath, reason.Params{"value": p})
	}
	return nil
}

// qualifiedDriver checks a provisioner or CSI driver name the way the API
// server does: a qualified name, compared in lower case.
func qualifiedDriver(field, name string) error {
	if name == "" {
		return invalidField(field, reason.FieldRequired, nil)
	}
	if errs := validation.IsQualifiedName(strings.ToLower(name)); len(errs) > 0 {
		return invalidField(field, reasonInvalidName, reason.Params{"name": name, "detail": strings.Join(errs, "; ")})
	}
	return nil
}

// storageClassRef checks the class a claim or volume names, if any.
func storageClassRef(field, name string) error {
	if name == "" {
		return nil
	}
	return objectName(field, name)
}
