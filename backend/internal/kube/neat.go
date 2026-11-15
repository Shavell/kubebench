package kube

import "k8s.io/apimachinery/pkg/runtime"

// neatMetadata are the metadata fields the API server maintains.
var neatMetadata = []string{"managedFields", "uid", "resourceVersion", "generation", "creationTimestamp", "selfLink"}

// neatAnnotations are annotations tools maintain: kubectl's client-side
// apply state and the Deployment controller's revision counter.
var neatAnnotations = []string{lastAppliedAnnotation, "deployment.kubernetes.io/revision"}

// Neat returns a copy of obj without the fields the API server maintains
// (status and metadata.managedFields, uid, resourceVersion, generation,
// creationTimestamp, selfLink) and without the annotations tools maintain
// (kubectl's last-applied-configuration, deployment.kubernetes.io/revision),
// so that it reads like a manifest to apply again. Annotations left empty
// are dropped, as are the null creationTimestamps of object templates such
// as a Deployment's pod template. obj must hold JSON values only, as an
// unstructured object does; it is not modified.
func Neat(obj map[string]any) map[string]any {
	out := runtime.DeepCopyJSON(obj)
	delete(out, "status")
	if meta, ok := out["metadata"].(map[string]any); ok {
		for _, field := range neatMetadata {
			delete(meta, field)
		}
		if annotations, ok := meta["annotations"].(map[string]any); ok {
			for _, key := range neatAnnotations {
				delete(annotations, key)
			}
			if len(annotations) == 0 {
				delete(meta, "annotations")
			}
		}
	}
	dropNullTimestamps(out)
	return out
}

// dropNullTimestamps removes "creationTimestamp: null" from every nested
// metadata map: the zero time of templates, which carry metadata but are
// never stored as objects of their own.
func dropNullTimestamps(value any) {
	switch v := value.(type) {
	case map[string]any:
		for key, child := range v {
			if meta, ok := child.(map[string]any); ok && key == "metadata" {
				if ts, found := meta["creationTimestamp"]; found && ts == nil {
					delete(meta, "creationTimestamp")
				}
			}
			dropNullTimestamps(child)
		}
	case []any:
		for _, child := range v {
			dropNullTimestamps(child)
		}
	}
}
