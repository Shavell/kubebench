package kube

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	utiljson "k8s.io/apimachinery/pkg/util/json"
	utilyaml "k8s.io/apimachinery/pkg/util/yaml"
)

// ManifestError explains why a document of an apply request is refused
// before anything is sent to the cluster.
type ManifestError struct {
	// Document is the 1-based position of the YAML document or JSON value.
	Document int
	Problem  string
	// Redacted marks a Secret that still holds KubeBench's redaction
	// placeholders instead of its values.
	Redacted bool
}

func (e *ManifestError) Error() string { return fmt.Sprintf("document %d: %s", e.Document, e.Problem) }

// redactedError is a Secret value that is a redaction placeholder.
type redactedError struct{ problem string }

func (e *redactedError) Error() string { return e.problem }

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
			return nil, &ManifestError{Document: doc, Problem: "not valid YAML or JSON: " + err.Error()}
		}
		if len(raw) == 0 || string(raw) == "null" {
			continue // an empty document, e.g. after a trailing "---"
		}
		var content map[string]any
		if err := utiljson.Unmarshal(raw, &content); err != nil {
			return nil, &ManifestError{Document: doc, Problem: "not an object"}
		}
		found, err := documentObjects(content)
		if err != nil {
			var redacted *redactedError
			return nil, &ManifestError{Document: doc, Problem: err.Error(), Redacted: errors.As(err, &redacted)}
		}
		objects = append(objects, found...)
	}
}

// documentObjects validates one document; a list yields its items.
func documentObjects(content map[string]any) ([]*unstructured.Unstructured, error) {
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
			return nil, fmt.Errorf("items[%d] is not an object", i)
		}
		itemObj := &unstructured.Unstructured{Object: itemContent}
		if err := checkManifest(itemObj); err != nil {
			return nil, fmt.Errorf("items[%d]: %w", i, err)
		}
		objects = append(objects, itemObj)
	}
	return objects, nil
}

// checkManifest refuses objects server-side apply cannot address, and
// Secrets whose values are redaction placeholders: applying those would
// replace the real values with the placeholders.
func checkManifest(obj *unstructured.Unstructured) error {
	switch {
	case obj.GetAPIVersion() == "":
		return errors.New("apiVersion is required")
	case obj.GetKind() == "":
		return errors.New("kind is required")
	case obj.GetName() == "":
		return fmt.Errorf("%s: metadata.name is required (server-side apply cannot generate names)", obj.GetKind())
	}
	if obj.GetAPIVersion() != "v1" || obj.GetKind() != "Secret" {
		return nil
	}
	for _, field := range []string{"data", "stringData"} {
		values, _ := obj.Object[field].(map[string]any)
		for _, key := range sortedKeys(values) {
			if s, ok := values[key].(string); ok && IsRedacted(s) {
				return &redactedError{fmt.Sprintf(
					"Secret %s: %s.%s holds a redaction placeholder, not its value; reveal the Secret and use the real values",
					obj.GetName(), field, key)}
			}
		}
	}
	if IsRedacted(obj.GetAnnotations()[lastAppliedAnnotation]) {
		return &redactedError{fmt.Sprintf(
			"Secret %s: the %s annotation holds a redaction placeholder; remove the annotation",
			obj.GetName(), lastAppliedAnnotation)}
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
