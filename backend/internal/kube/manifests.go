package kube

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"strconv"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	utiljson "k8s.io/apimachinery/pkg/util/json"
	utilyaml "k8s.io/apimachinery/pkg/util/yaml"

	"kubebench/internal/reason"
)

// ManifestError explains why a document of an apply request is refused
// before anything is sent to the cluster.
type ManifestError struct {
	// Document is the 1-based position of the YAML document or JSON value.
	Document int
	// Code and Params explain the problem; Params["document"] names the
	// document (and the item of a list).
	Code   reason.Code
	Params reason.Params
	// Redacted marks a Secret that still holds KubeBench's redaction
	// placeholders instead of its values.
	Redacted bool
}

func (e *ManifestError) Error() string { return e.Code.Text(e.Params) }

// Reason implements reason.Reasoned.
func (e *ManifestError) Reason() (reason.Code, reason.Params) { return e.Code, e.Params }

// manifestError places problem in document doc: "2", or "2, items[3]"
// for an item of a list.
func manifestError(doc int, problem *reason.Error, redacted bool) *ManifestError {
	params := reason.Params{}
	for k, v := range problem.Params {
		params[k] = v
	}
	params["document"] = strconv.Itoa(doc)
	if item, ok := params["item"]; ok {
		params["document"] = fmt.Sprintf("%d, items[%v]", doc, item)
		delete(params, "item")
	}
	return &ManifestError{Document: doc, Code: problem.Code, Params: params, Redacted: redacted}
}

// ParseManifests splits YAML ("---"-separated documents) or JSON
// (concatenated values) into objects, skipping empty documents and
// flattening lists (kind: List, as kubectl get -o yaml prints them). Every
// object must name its apiVersion, kind and metadata.name, and a Secret
// must carry real values, not the placeholders KubeBench shows instead.
func ParseManifests(data []byte) ([]*unstructured.Unstructured, error) {
	decoder := utilyaml.NewYAMLOrJSONDecoder(bytes.NewReader(data), 4096)
	var objects []*unstructured.Unstructured
	for doc := 1; ; doc++ {
		var raw json.RawMessage
		if err := decoder.Decode(&raw); err != nil {
			if errors.Is(err, io.EOF) {
				return objects, nil
			}
			return nil, manifestError(doc, reason.New(reasonManifestYAML, reason.Params{"detail": err.Error()}), false)
		}
		if len(raw) == 0 || string(raw) == "null" {
			continue // an empty document, e.g. after a trailing "---"
		}
		var content map[string]any
		if err := utiljson.Unmarshal(raw, &content); err != nil {
			return nil, manifestError(doc, reason.New(reasonManifestNotObject, nil), false)
		}
		found, err := documentObjects(content)
		if err != nil {
			return nil, manifestError(doc, err, redactionRefusal(err.Code))
		}
		objects = append(objects, found...)
	}
}

// documentObjects validates one document; a list yields its items.
func documentObjects(content map[string]any) ([]*unstructured.Unstructured, *reason.Error) {
	if len(content) == 0 {
		return nil, nil
	}
	obj := &unstructured.Unstructured{Object: content}
	if !obj.IsList() {
		if err := checkManifest(obj); err != nil {
			return nil, err
		}
		return []*unstructured.Unstructured{obj}, nil
	}
	items := obj.Object["items"].([]any) // IsList checked the type
	objects := make([]*unstructured.Unstructured, 0, len(items))
	for i, item := range items {
		itemContent, ok := item.(map[string]any)
		if !ok {
			return nil, reason.New(reasonManifestNotObject, reason.Params{"item": i})
		}
		itemObj := &unstructured.Unstructured{Object: itemContent}
		if err := checkManifest(itemObj); err != nil {
			err.Params["item"] = i
			return nil, err
		}
		objects = append(objects, itemObj)
	}
	return objects, nil
}

// redactionRefusal reports whether code refuses a redaction placeholder.
func redactionRefusal(code reason.Code) bool {
	switch code {
	case reasonManifestRedacted, reasonManifestAnnotation, reasonManifestCredential, reasonManifestCredentialAnnotation:
		return true
	}
	return false
}

// checkManifest refuses objects server-side apply cannot address, and
// Secrets and credential kinds whose values are redaction placeholders:
// applying those would replace the real values with the placeholders.
func checkManifest(obj *unstructured.Unstructured) *reason.Error {
	switch {
	case obj.GetAPIVersion() == "":
		return reason.New(reasonManifestAPIVersion, reason.Params{})
	case obj.GetKind() == "":
		return reason.New(reasonManifestKind, reason.Params{})
	case obj.GetName() == "":
		return reason.New(reasonManifestName, reason.Params{"kind": obj.GetKind()})
	}
	if ck, ok := credentialKindByKind(obj.GetAPIVersion(), obj.GetKind()); ok {
		return checkCredentialManifest(ck, obj)
	}
	if obj.GetAPIVersion() != "v1" || obj.GetKind() != "Secret" {
		return nil
	}
	for _, field := range []string{"data", "stringData"} {
		values, _ := obj.Object[field].(map[string]any)
		for _, key := range sortedKeys(values) {
			if s, ok := values[key].(string); ok && IsRedacted(s) {
				return reason.New(reasonManifestRedacted, reason.Params{"name": obj.GetName(), "field": field + "." + key})
			}
		}
	}
	if IsRedacted(obj.GetAnnotations()[lastAppliedAnnotation]) {
		return reason.New(reasonManifestAnnotation, reason.Params{"name": obj.GetName(), "annotation": lastAppliedAnnotation})
	}
	return nil
}

// checkCredentialManifest is checkManifest for a credential kind.
func checkCredentialManifest(ck credentialKind, obj *unstructured.Unstructured) *reason.Error {
	if field, found := ck.placeholder(obj.Object); found {
		return reason.New(reasonManifestCredential, reason.Params{"kind": ck.kind, "name": obj.GetName(), "field": field})
	}
	if IsRedacted(obj.GetAnnotations()[lastAppliedAnnotation]) {
		return reason.New(reasonManifestCredentialAnnotation, reason.Params{"kind": ck.kind, "name": obj.GetName(), "annotation": lastAppliedAnnotation})
	}
	return nil
}

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return keys
}
