package kube

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"

	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	utiljson "k8s.io/apimachinery/pkg/util/json"
	utilyaml "k8s.io/apimachinery/pkg/util/yaml"

	"kubebench/internal/reason"
)

// EditError explains why an edit is refused before anything is sent.
type EditError struct {
	Code   reason.Code
	Params reason.Params
	// Redacted marks a Secret value changed to a redaction placeholder.
	Redacted bool
}

func (e *EditError) Error() string { return e.Code.Text(e.Params) }

// Reason implements reason.Reasoned.
func (e *EditError) Reason() (reason.Code, reason.Params) { return e.Code, e.Params }

// StaleEditError is an edit the API server refused (409) because the
// object changed since the editor read it: its resourceVersion is no
// longer the one the edit started from.
type StaleEditError struct {
	Err        error
	Kind, Name string
}

func (e *StaleEditError) Error() string { return e.Err.Error() }
func (e *StaleEditError) Unwrap() error { return e.Err }

// Reason implements reason.Reasoned.
func (e *StaleEditError) Reason() (reason.Code, reason.Params) {
	return reasonEditStale, reason.Params{"kind": e.Kind, "name": e.Name}
}

// IsStaleEdit reports whether err is a *StaleEditError.
func IsStaleEdit(err error) bool {
	var stale *StaleEditError
	return errors.As(err, &stale)
}

// editReasons are the refusals of parseEdited for one of the two manifests.
type editReasons struct {
	invalidYAML, severalDocuments, notObject, empty, otherObject reason.Code
}

var (
	originalManifest = editReasons{reasonOriginalYAML, reasonOriginalDocuments, reasonOriginalNotObject, reasonOriginalEmpty, reasonOriginalOther}
	editedManifest   = editReasons{reasonEditYAML, reasonEditDocuments, reasonEditNotObject, reasonEditEmpty, reasonEditOtherObject}
)

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
//
// resourceVersion is the version of o the original manifest was read at
// (a neat manifest has none; "" takes the original's
// metadata.resourceVersion, if it has one). The patch carries it as a
// precondition, so an edit of an object that changed on the server since
// fails with a *StaleEditError instead of overwriting the change: a merge
// patch replaces lists (containers, env, tolerations) whole.
func (r *Runtime) EditObject(ctx context.Context, o Object, original, edited []byte, resourceVersion string, dryRun bool) (ApplyResult, error) {
	res := ApplyResult{APIVersion: o.Resource.GroupVersion().String(), Kind: o.Kind, Namespace: o.Namespace, Name: o.Name}
	from, err := parseEdited(original, originalManifest, o)
	if err != nil {
		return res, err
	}
	to, err := parseEdited(edited, editedManifest, o)
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
	if resourceVersion == "" {
		resourceVersion = (&unstructured.Unstructured{Object: from}).GetResourceVersion()
	}
	if resourceVersion != "" {
		meta, _ := patch["metadata"].(map[string]any)
		if meta == nil {
			meta = map[string]any{}
			patch["metadata"] = meta
		}
		meta["resourceVersion"] = resourceVersion
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
	if apierrors.IsConflict(err) && resourceVersion != "" {
		err = &StaleEditError{Err: err, Kind: o.Kind, Name: joinName(o.Namespace, o.Name)}
	}
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
func parseEdited(data []byte, what editReasons, o Object) (map[string]any, error) {
	decoder := utilyaml.NewYAMLOrJSONDecoder(bytes.NewReader(data), 4096)
	var content map[string]any
	for {
		var raw json.RawMessage
		if err := decoder.Decode(&raw); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, &EditError{Code: what.invalidYAML, Params: reason.Params{"detail": err.Error()}}
		}
		if len(raw) == 0 || string(raw) == "null" {
			continue
		}
		if content != nil {
			return nil, &EditError{Code: what.severalDocuments}
		}
		if err := utiljson.Unmarshal(raw, &content); err != nil || content == nil {
			return nil, &EditError{Code: what.notObject}
		}
	}
	if content == nil {
		return nil, &EditError{Code: what.empty}
	}
	obj := &unstructured.Unstructured{Object: content}
	ns := obj.GetNamespace()
	if obj.GetAPIVersion() != o.Resource.GroupVersion().String() || obj.GetKind() != o.Kind || obj.GetName() != o.Name ||
		(o.Namespaced && ns != "" && ns != o.Namespace) {
		return nil, &EditError{Code: what.otherObject, Params: reason.Params{
			"kind": obj.GetKind(), "name": joinName(ns, obj.GetName()), "edited": joinName(o.Namespace, o.Name)}}
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
// placeholder into a Secret or a credential kind's credential: as a value,
// or as kubectl's last-applied annotation, which repeats the values. A
// merge patch replaces lists whole, so a changed list holding a redacted
// credential (a BuildConfig's triggers) is refused too.
func checkPatchValues(o Object, patch map[string]any) error {
	if ck, ok := credentialKindOf(o.Resource.GroupResource()); ok {
		return checkCredentialPatch(ck, o, patch)
	}
	if !isSecrets(o.Resource) {
		return nil
	}
	for _, field := range []string{"data", "stringData"} {
		values, _ := patch[field].(map[string]any)
		for _, key := range sortedKeys(values) {
			if s, ok := values[key].(string); ok && IsRedacted(s) {
				return &EditError{Redacted: true, Code: reasonEditRedacted,
					Params: reason.Params{"name": o.Name, "field": field + "." + key}}
			}
		}
	}
	meta, _ := patch["metadata"].(map[string]any)
	annotations, _ := meta["annotations"].(map[string]any)
	if s, ok := annotations[lastAppliedAnnotation].(string); ok && IsRedacted(s) {
		return &EditError{Redacted: true, Code: reasonEditAnnotation,
			Params: reason.Params{"name": o.Name, "annotation": lastAppliedAnnotation}}
	}
	return nil
}

// checkCredentialPatch is checkPatchValues for a credential kind.
func checkCredentialPatch(ck credentialKind, o Object, patch map[string]any) error {
	if field, found := ck.placeholder(patch); found {
		return &EditError{Redacted: true, Code: reasonEditCredential,
			Params: reason.Params{"kind": ck.kind, "name": o.Name, "field": field}}
	}
	meta, _ := patch["metadata"].(map[string]any)
	annotations, _ := meta["annotations"].(map[string]any)
	if s, ok := annotations[lastAppliedAnnotation].(string); ok && IsRedacted(s) {
		return &EditError{Redacted: true, Code: reasonEditCredentialAnnotation,
			Params: reason.Params{"kind": ck.kind, "name": o.Name, "annotation": lastAppliedAnnotation}}
	}
	return nil
}
