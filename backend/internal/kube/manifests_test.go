package kube

import (
	"errors"
	"strings"
	"testing"
)

func TestParseManifests(t *testing.T) {
	for _, tc := range []struct {
		name string
		text string
		want []string // kind/name of each object
	}{
		{
			name: "multi-document YAML with empty documents and comments",
			text: `---
# the app
apiVersion: v1
kind: ConfigMap
metadata: {name: app}
---
---
# nothing but a comment
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: web
spec:
  replicas: 3
---
`,
			want: []string{"ConfigMap/app", "Deployment/web"},
		},
		{
			name: "concatenated JSON",
			text: `{"apiVersion":"v1","kind":"ConfigMap","metadata":{"name":"a"}}
{"apiVersion":"v1","kind":"ConfigMap","metadata":{"name":"b"}}`,
			want: []string{"ConfigMap/a", "ConfigMap/b"},
		},
		{
			name: "a list, as kubectl get -o yaml prints it",
			text: `apiVersion: v1
kind: List
items:
- {apiVersion: v1, kind: ConfigMap, metadata: {name: a}}
- {apiVersion: v1, kind: Service, metadata: {name: b}}
`,
			want: []string{"ConfigMap/a", "Service/b"},
		},
		{name: "nothing", text: "  \n---\n", want: nil},
		{
			name: "real Secret values",
			text: `apiVersion: v1
kind: Secret
metadata: {name: tls}
data: {tls.key: c2VjcmV0}
stringData: {password: "<not redacted"}
`,
			want: []string{"Secret/tls"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			objects, err := ParseManifests([]byte(tc.text))
			if err != nil {
				t.Fatal(err)
			}
			var got []string
			for _, obj := range objects {
				got = append(got, obj.GetKind()+"/"+obj.GetName())
			}
			if strings.Join(got, ",") != strings.Join(tc.want, ",") {
				t.Fatalf("objects = %v, want %v", got, tc.want)
			}
		})
	}
}

// Integers must stay integers (int64), as the API server decodes them:
// float64 would turn large values into exponent notation.
func TestParseManifestsKeepsIntegers(t *testing.T) {
	objects, err := ParseManifests([]byte("apiVersion: apps/v1\nkind: Deployment\nmetadata: {name: web}\nspec: {replicas: 3, revisionHistoryLimit: 9007199254740993}\n"))
	if err != nil {
		t.Fatal(err)
	}
	spec := objects[0].Object["spec"].(map[string]any)
	if v, ok := spec["replicas"].(int64); !ok || v != 3 {
		t.Fatalf("replicas = %#v", spec["replicas"])
	}
	if v, ok := spec["revisionHistoryLimit"].(int64); !ok || v != 9007199254740993 {
		t.Fatalf("a large integer = %#v", spec["revisionHistoryLimit"])
	}
}

func TestParseManifestsRefusals(t *testing.T) {
	for _, tc := range []struct {
		name     string
		text     string
		document int
		problem  string
		redacted bool
	}{
		{"invalid YAML", "apiVersion: v1\nkind: ConfigMap\nmetadata: {name: a}\n---\nkind: [unclosed\n", 2, "not valid YAML or JSON", false},
		{"a scalar", "just text\n", 1, "not an object", false},
		{"no apiVersion", "kind: ConfigMap\nmetadata: {name: a}\n", 1, "apiVersion is required", false},
		{"no kind", "apiVersion: v1\nmetadata: {name: a}\n", 1, "kind is required", false},
		{"generateName", "apiVersion: v1\nkind: ConfigMap\nmetadata: {generateName: a-}\n", 1, "ConfigMap: metadata.name is required", false},
		{"list item without a name", "apiVersion: v1\nkind: List\nitems:\n- {apiVersion: v1, kind: Pod, metadata: {}}\n", 1, "items[0]: Pod: metadata.name is required", false},
		{
			"redacted data",
			"apiVersion: v1\nkind: ConfigMap\nmetadata: {name: ok}\n---\napiVersion: v1\nkind: Secret\nmetadata: {name: tls}\ndata:\n  tls.crt: \"<1180 bytes, redacted>\"\n",
			2, "Secret tls: data.tls.crt holds a redaction placeholder", true,
		},
		{
			"redacted stringData",
			"apiVersion: v1\nkind: Secret\nmetadata: {name: db}\nstringData: {password: <redacted>}\n",
			1, "stringData.password", true,
		},
		{
			"redacted last-applied annotation",
			"apiVersion: v1\nkind: Secret\nmetadata:\n  name: db\n  annotations: {kubectl.kubernetes.io/last-applied-configuration: <redacted>}\n",
			1, "kubectl.kubernetes.io/last-applied-configuration annotation", true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseManifests([]byte(tc.text))
			var me *ManifestError
			if !errors.As(err, &me) {
				t.Fatalf("err = %v, want a ManifestError", err)
			}
			if me.Document != tc.document || !strings.Contains(me.Problem, tc.problem) || me.Redacted != tc.redacted {
				t.Fatalf("error = %+v, want document %d, problem containing %q, redacted %v", me, tc.document, tc.problem, tc.redacted)
			}
		})
	}
}
