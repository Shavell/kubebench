package kube

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	utiljson "k8s.io/apimachinery/pkg/util/json"
	utilyaml "k8s.io/apimachinery/pkg/util/yaml"
)

// EditError explains why an edit is refused before anything is sent.
type EditError struct {
	Problem string
	// Redacted marks a Secret value changed to a redaction placeholder.
	Redacted bool
}

func (e *EditError) Error() string { return e.Problem }

// EditObject changes the existing object o from the manifest the user
// started editing (original) to the edited one, the way kubectl edit does:
// with a JSON merge patch of their differences, as FieldManager. A field
// the edit removes is removed, which server-side apply would not do for a
// field another manager owns; what the edit leaves alone stays as it is on
// the cluster, also when it changed in the meantime. Both manifests must
// be o itself (apiVersion, kind, name, namespace). Redaction placeholders
// the edit leaves unchanged are not part of the patch, so a Secret's
// labels can be edited without revealing it; a value changed to a
// placeholder is refused. Refusals are *EditError; the cluster's answer is
// in the result.
func (r *Runtime) EditObject(ctx context.Context, o Object, original, edited []byte, dryRun bool) (ApplyResult, error) {
	res := ApplyResult{APIVersion: o.Resource.GroupVersion().String(), Kind: o.Kind, Namespace: o.Namespace, Name: o.Name}
	from, err := parseEdited(original, "the original manifest", o)
	if err != nil {
		return res, err
	}
	to, err := parseEdited(edited, "the edited manifest", o)
	if err != nil {
		return res, err
	}
	patch := mergePatch(from, to)
	if meta, ok := patch["metadata"].(map[string]any); ok {
		// Checked to be o's by parseEdited: an omitted namespace is no change.
		delete(meta, "namespace")
		if len(meta) == 0 {
			delete(patch, "metadata")
		}
	}
	if err := checkPatchValues(o, patch); err != nil {
		return res, err
	}

	client := r.client(o)
	before, err := client.Get(ctx, o.Name, metav1.GetOptions{})
	if err != nil {
		res.Err = err
		return res, nil
	}
	if len(patch) == 0 {
		res.Action = ActionUnchanged
		return res, nil
	}
	body, err := json.Marshal(patch)
	if err != nil {
		return res, err
	}
	options := metav1.PatchOptions{FieldManager: FieldManager}
	if dryRun {
		options.DryRun = []string{metav1.DryRunAll}
	}
	after, err := client.Patch(ctx, o.Name, types.MergePatchType, body, options)
	if err != nil {
		res.Err = err
		return res, nil
	}
	res.Action = applyAction(before, after)
	return res, nil
}

// parseEdited parses the one object (YAML or JSON) of an edit and checks
// that it is o. Redaction placeholders are accepted here; see
// checkPatchValues.
func parseEdited(data []byte, what string, o Object) (map[string]any, error) {
	decoder := utilyaml.NewYAMLOrJSONDecoder(bytes.NewReader(data), 4096)
	var content map[string]any
	for {
		var raw json.RawMessage
		if err := decoder.Decode(&raw); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, &EditError{Problem: fmt.Sprintf("%s is not valid YAML or JSON: %v", what, err)}
		}
		if len(raw) == 0 || string(raw) == "null" {
			continue
		}
		if content != nil {
			return nil, &EditError{Problem: what + " holds more than one document; edit one object at a time"}
		}
		if err := utiljson.Unmarshal(raw, &content); err != nil || content == nil {
			return nil, &EditError{Problem: what + " is not an object"}
		}
	}
	if content == nil {
		return nil, &EditError{Problem: what + " is empty"}
	}
	obj := &unstructured.Unstructured{Object: content}
	ns := obj.GetNamespace()
	if obj.GetAPIVersion() != o.Resource.GroupVersion().String() || obj.GetKind() != o.Kind || obj.GetName() != o.Name ||
		(o.Namespaced && ns != "" && ns != o.Namespace) {
		return nil, &EditError{Problem: fmt.Sprintf(
			"%s describes %s %s, not the edited %s; create other objects with New resource", what,
			obj.GetKind(), joinName(ns, obj.GetName()), joinName(o.Namespace, o.Name))}
	}
	return content, nil
}

func joinName(namespace, name string) string {
	if namespace == "" {
		return name
	}
	return namespace + "/" + name
}

// mergePatch is the JSON merge patch (RFC 7386) that turns from into to:
// changed and added fields with their new values, removed ones as null.
// Maps present on both sides are patched key by key; lists and other
// values are replaced whole, as merge patches do.
func mergePatch(from, to map[string]any) map[string]any {
	patch := map[string]any{}
	for key := range from {
		if _, kept := to[key]; !kept {
			patch[key] = nil
		}
	}
	for key, value := range to {
		old, existed := from[key]
		if existed && equality.Semantic.DeepEqual(old, value) {
			continue
		}
		oldMap, oldIsMap := old.(map[string]any)
		newMap, newIsMap := value.(map[string]any)
		if existed && oldIsMap && newIsMap {
			patch[key] = mergePatch(oldMap, newMap)
			continue
		}
		patch[key] = value
	}
	return patch
}

// checkPatchValues refuses a patch that would write a redaction
// placeholder into a Secret: as a value, or as kubectl's last-applied
// annotation, which repeats the values.
func checkPatchValues(o Object, patch map[string]any) error {
	if !isSecrets(o.Resource) {
		return nil
	}
	for _, field := range []string{"data", "stringData"} {
		values, _ := patch[field].(map[string]any)
		for _, key := range sortedKeys(values) {
			if s, ok := values[key].(string); ok && IsRedacted(s) {
				return &EditError{Redacted: true, Problem: fmt.Sprintf(
					"Secret %s: %s.%s is set to a redaction placeholder; leave the placeholder unchanged to keep the value, or enter the real value",
					o.Name, field, key)}
			}
		}
	}
	meta, _ := patch["metadata"].(map[string]any)
	annotations, _ := meta["annotations"].(map[string]any)
	if s, ok := annotations[lastAppliedAnnotation].(string); ok && IsRedacted(s) {
		return &EditError{Redacted: true, Problem: fmt.Sprintf(
			"Secret %s: the %s annotation holds a redaction placeholder; remove the annotation", o.Name, lastAppliedAnnotation)}
	}
	return nil
}
