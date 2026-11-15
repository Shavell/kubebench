package kube

import (
	"reflect"
	"testing"

	"sigs.k8s.io/yaml"
)

func yamlObject(t *testing.T, text string) map[string]any {
	t.Helper()
	var obj map[string]any
	if err := yaml.Unmarshal([]byte(text), &obj); err != nil {
		t.Fatal(err)
	}
	return obj
}

func TestNeat(t *testing.T) {
	for _, tc := range []struct{ name, in, want string }{
		{
			name: "server-maintained fields",
			in: `
apiVersion: v1
kind: ConfigMap
metadata:
  name: app
  namespace: shop
  uid: 0b1c
  resourceVersion: "42"
  generation: 3
  creationTimestamp: "2026-11-01T10:00:00Z"
  selfLink: /api/v1/namespaces/shop/configmaps/app
  managedFields:
  - manager: kubectl
  labels: {app: shop}
  ownerReferences:
  - {apiVersion: v1, kind: Pod, name: owner, uid: 9f}
data: {key: value}
status: {phase: Active}
`,
			want: `
apiVersion: v1
kind: ConfigMap
metadata:
  name: app
  namespace: shop
  labels: {app: shop}
  ownerReferences:
  - {apiVersion: v1, kind: Pod, name: owner, uid: 9f}
data: {key: value}
`,
		},
		{
			name: "tool annotations go, others stay",
			in: `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: web
  annotations:
    kubectl.kubernetes.io/last-applied-configuration: '{"kind":"Deployment"}'
    deployment.kubernetes.io/revision: "7"
    team: payments
`,
			want: `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: web
  annotations: {team: payments}
`,
		},
		{
			name: "annotations left empty are dropped",
			in: `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: web
  annotations: {deployment.kubernetes.io/revision: "2"}
`,
			want: `
apiVersion: apps/v1
kind: Deployment
metadata: {name: web}
`,
		},
		{
			name: "null creationTimestamps of templates",
			in: `
apiVersion: batch/v1
kind: CronJob
metadata: {name: report}
spec:
  jobTemplate:
    metadata: {creationTimestamp: null}
    spec:
      template:
        metadata:
          creationTimestamp: null
          labels: {app: report}
  volumeClaimTemplates:
  - metadata: {name: data, creationTimestamp: null}
`,
			want: `
apiVersion: batch/v1
kind: CronJob
metadata: {name: report}
spec:
  jobTemplate:
    metadata: {}
    spec:
      template:
        metadata:
          labels: {app: report}
  volumeClaimTemplates:
  - metadata: {name: data}
`,
		},
		{
			name: "only null template timestamps go",
			in: `
apiVersion: example.com/v1
kind: Snapshot
metadata: {name: s}
spec:
  source:
    metadata: {creationTimestamp: "2026-01-01T00:00:00Z"}
`,
			want: `
apiVersion: example.com/v1
kind: Snapshot
metadata: {name: s}
spec:
  source:
    metadata: {creationTimestamp: "2026-01-01T00:00:00Z"}
`,
		},
		{
			name: "no metadata at all",
			in:   `{kind: Thing, status: {ok: true}}`,
			want: `{kind: Thing}`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in := yamlObject(t, tc.in)
			untouched := yamlObject(t, tc.in)
			got := Neat(in)
			if want := yamlObject(t, tc.want); !reflect.DeepEqual(got, want) {
				gotYAML, _ := yaml.Marshal(got)
				t.Fatalf("Neat =\n%s\nwant\n%s", gotYAML, tc.want)
			}
			if !reflect.DeepEqual(in, untouched) {
				t.Fatal("Neat modified its input")
			}
		})
	}
}
