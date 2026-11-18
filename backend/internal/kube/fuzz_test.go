package kube

import (
	"bytes"
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// manifestSeeds are apply documents around the edges of ParseManifests:
// multi-document YAML, concatenated JSON, lists, Secrets with redaction
// placeholders and the credential kinds.
var manifestSeeds = []string{
	"apiVersion: v1\nkind: ConfigMap\nmetadata: {name: a}\n---\n---\napiVersion: v1\nkind: ConfigMap\nmetadata: {name: b}\n",
	`{"apiVersion":"v1","kind":"ConfigMap","metadata":{"name":"a"}}{"apiVersion":"v1","kind":"ConfigMap","metadata":{"name":"b"}}`,
	"apiVersion: v1\nkind: List\nitems:\n- apiVersion: v1\n  kind: Secret\n  metadata: {name: s}\n  data: {k: <3 bytes, redacted>}\n",
	"apiVersion: v1\nkind: Secret\nmetadata:\n  name: s\n  annotations:\n    kubectl.kubernetes.io/last-applied-configuration: <redacted>\nstringData: {k: v}\n",
	"apiVersion: v1\nkind: Secret\nmetadata: {name: s}\nstringData: {k: \"<12 bytes, redacted>\"}\n",
	"apiVersion: route.openshift.io/v1\nkind: Route\nmetadata: {name: r}\nspec: {tls: {key: <redacted>}}\n",
	"apiVersion: build.openshift.io/v1\nkind: BuildConfig\nmetadata: {name: b}\nspec:\n  triggers:\n  - github: {secret: <redacted>}\n",
	"apiVersion: v1\nkind: List\nitems: [1, 2]\n", "- a\n- b\n", "null\n---\n{}\n", "kind: Secret\n", "{", ": :\n", "",
}

// FuzzParseManifests checks that ParseManifests never panics, explains
// every refusal with a document number, and returns only objects apply
// can address: named, no lists, and no redaction placeholder in a Secret
// or credential field. What it returns parses again to the same objects.
func FuzzParseManifests(f *testing.F) {
	for _, seed := range manifestSeeds {
		f.Add([]byte(seed))
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		objects, err := ParseManifests(data)
		if err != nil {
			var me *ManifestError
			if !errors.As(err, &me) || me.Document < 1 || me.Code.IsZero() {
				t.Fatalf("ParseManifests(%q): unexplained error %#v", data, err)
			}
			return
		}
		var again bytes.Buffer
		for _, obj := range objects {
			if obj.GetAPIVersion() == "" || obj.GetKind() == "" || obj.GetName() == "" || obj.IsList() {
				t.Fatalf("ParseManifests(%q) returned %v", data, obj.Object)
			}
			if err := checkManifest(obj); err != nil {
				t.Fatalf("ParseManifests(%q) returned a refused object: %v", data, err)
			}
			out, err := json.Marshal(obj.Object)
			if err != nil {
				t.Fatalf("returned object does not serialize: %v", err)
			}
			again.Write(out)
			again.WriteByte('\n')
		}
		reparsed, err := ParseManifests(again.Bytes())
		if err != nil || len(reparsed) != len(objects) {
			t.Fatalf("objects of %q do not parse again: %d objects, %v", data, len(reparsed), err)
		}
		for i := range objects {
			if !reflect.DeepEqual(normalizeJSON(t, objects[i].Object), normalizeJSON(t, reparsed[i].Object)) {
				t.Fatalf("object %d of %q changed on a round trip:\n%v\n%v", i, data, objects[i].Object, reparsed[i].Object)
			}
		}
	})
}

// normalizeJSON is v as encoding/json decodes it (numbers as float64).
func normalizeJSON(t *testing.T, v any) any {
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var out any
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

// FuzzRedactManifestObject checks Secret redaction: after
// RedactManifestObject no data or stringData value of a Secret, no
// credential of a credential kind and no last-applied annotation is left
// as it was — every one is a redaction marker — and applying the redacted
// object back is refused, so the markers never replace the real values.
func FuzzRedactManifestObject(f *testing.F) {
	for _, seed := range []string{
		`{"apiVersion":"v1","kind":"Secret","metadata":{"name":"s"},"data":{"a":"aHVudGVyMg==","b":""},"stringData":{"p":"hunter2"}}`,
		`{"apiVersion":"v1","kind":"Secret","metadata":{"name":"s","annotations":{"kubectl.kubernetes.io/last-applied-configuration":"{\"data\":{\"a\":\"x\"}}"}},"data":{"n":1,"m":{"x":"y"},"l":null}}`,
		`{"apiVersion":"v1","kind":"Secret","metadata":{"name":"s"},"data":"not a map","stringData":["x"]}`,
		`{"apiVersion":"route.openshift.io/v1","kind":"Route","metadata":{"name":"r"},"spec":{"tls":{"key":"PRIVATE","certificate":"PUBLIC"}}}`,
		`{"apiVersion":"build.openshift.io/v1","kind":"BuildConfig","metadata":{"name":"b"},"spec":{"triggers":[{"github":{"secret":"s1"}},{"generic":{"secret":2}},"x"]}}`,
		`{"apiVersion":"build.openshift.io/v1","kind":"Build","metadata":{"name":"b"},"spec":{"triggeredBy":[{"githubWebHook":{"secret":"s"}}]}}`,
		`{"apiVersion":"v1","kind":"ConfigMap","metadata":{"name":"c"},"data":{"a":"visible"}}`,
	} {
		f.Add([]byte(seed))
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		var obj map[string]any
		if json.Unmarshal(data, &obj) != nil || obj == nil {
			return
		}
		apiVersion, _ := obj["apiVersion"].(string)
		kind, _ := obj["kind"].(string)
		secret := apiVersion == "v1" && kind == "Secret"
		ck, credential := credentialKindByKind(apiVersion, kind)
		if got := RedactManifestObject(obj); got != (secret || credential) {
			t.Fatalf("RedactManifestObject(%s) = %v", data, got)
		}
		if !secret && !credential {
			return
		}
		hidden := 0
		if secret {
			for _, field := range []string{"data", "stringData"} {
				values, _ := obj[field].(map[string]any)
				for key, v := range values {
					if s, ok := v.(string); !ok || !IsRedacted(s) {
						t.Fatalf("Secret %s.%s survived redaction of %s: %v", field, key, data, v)
					}
					hidden++
				}
			}
		} else {
			for _, field := range ck.fields {
				eachField(obj, field, "", func(name string, parent map[string]any, key string) {
					if v := parent[key]; v != nil {
						if s, ok := v.(string); !ok || !IsRedacted(s) {
							t.Fatalf("%s %s survived redaction of %s: %v", kind, name, data, v)
						}
						hidden++
					}
				})
			}
		}
		meta, _ := obj["metadata"].(map[string]any)
		annotations, _ := meta["annotations"].(map[string]any)
		if v, found := annotations[lastAppliedAnnotation]; found {
			if v != redactedValue {
				t.Fatalf("the last-applied annotation survived redaction of %s: %v", data, v)
			}
			hidden++
		}
		if hidden == 0 || (&unstructured.Unstructured{Object: obj}).GetName() == "" {
			return
		}
		out, err := json.Marshal(obj)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := ParseManifests(out); err == nil {
			t.Fatalf("the redacted object of %s can be applied back: %s", data, out)
		}
	})
}

// applyMergePatch is RFC 7386's MergePatch procedure, as the API server
// applies a merge patch.
func applyMergePatch(target, patch any) any {
	p, ok := patch.(map[string]any)
	if !ok {
		return patch
	}
	t, ok := target.(map[string]any)
	if !ok {
		t = map[string]any{}
	}
	out := map[string]any{}
	for k, v := range t {
		out[k] = v
	}
	for k, v := range p {
		if v == nil {
			delete(out, k)
		} else {
			out[k] = applyMergePatch(out[k], v)
		}
	}
	return out
}

// dropNulls removes null members of objects: a merge patch cannot set a
// member to null, null means "remove".
func dropNulls(v any) any {
	switch v := v.(type) {
	case map[string]any:
		out := map[string]any{}
		for k, item := range v {
			if item != nil {
				out[k] = dropNulls(item)
			}
		}
		return out
	case []any:
		out := make([]any, len(v))
		for i, item := range v {
			out[i] = dropNulls(item)
		}
		return out
	}
	return v
}

// FuzzMergePatch checks that the merge patch of an edit turns the object
// as read into the object as edited when the API server applies it.
func FuzzMergePatch(f *testing.F) {
	for _, pair := range [][2]string{
		{`{"a":1,"b":{"c":2,"d":3}}`, `{"a":1,"b":{"c":4},"e":[1]}`},
		{`{"a":{"b":1}}`, `{"a":1}`},
		{`{"a":1}`, `{"a":{"b":null,"c":{}}}`},
		{`{"a":null}`, `{"a":null}`},
		{`{"a":[1,{"b":2}]}`, `{"a":[1,{"b":null}]}`},
		{`{}`, `{"metadata":{"labels":{"x":"y"}}}`},
	} {
		f.Add([]byte(pair[0]), []byte(pair[1]))
	}
	f.Fuzz(func(t *testing.T, fromJSON, toJSON []byte) {
		var from, to map[string]any
		if json.Unmarshal(fromJSON, &from) != nil || json.Unmarshal(toJSON, &to) != nil || from == nil || to == nil {
			return
		}
		patch := mergePatch(from, to)
		// The patch goes to the API server as JSON.
		wire := normalizeJSON(t, patch)
		got := applyMergePatch(normalizeJSON(t, from), wire)
		if want := dropNulls(normalizeJSON(t, to)); !reflect.DeepEqual(dropNulls(got), want) {
			t.Fatalf("patch %v turns %s into %v, want %v", wire, fromJSON, got, want)
		}
	})
}
