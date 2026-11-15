package kube

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"

	"kubebench/internal/kube/kubetest"
)

const twoConfigMaps = `apiVersion: v1
kind: ConfigMap
metadata: {name: app, namespace: shop}
data: {mode: blue}
---
apiVersion: v1
kind: ConfigMap
metadata: {name: flags, namespace: shop}
data: {beta: "true"}
`

func applyActions(results []ApplyResult) []ApplyAction {
	out := make([]ApplyAction, len(results))
	for i, res := range results {
		if res.Err != nil {
			out[i] = ApplyAction("error: " + res.Err.Error())
		} else {
			out[i] = res.Action
		}
	}
	return out
}

func TestApplyCreatedConfiguredUnchanged(t *testing.T) {
	rt, dyn := editingRuntime(t)
	ctx := context.Background()

	if got := applyActions(rt.Apply(ctx, mustParseManifests(t, twoConfigMaps), ApplyOptions{})); !slices.Equal(got, []ApplyAction{ActionCreated, ActionCreated}) {
		t.Fatalf("first apply: %v", got)
	}
	changed := strings.Replace(twoConfigMaps, "mode: blue", "mode: green", 1)
	if got := applyActions(rt.Apply(ctx, mustParseManifests(t, changed), ApplyOptions{})); !slices.Equal(got, []ApplyAction{ActionConfigured, ActionUnchanged}) {
		t.Fatalf("second apply: %v", got)
	}
	if got := applyActions(rt.Apply(ctx, mustParseManifests(t, changed), ApplyOptions{})); !slices.Equal(got, []ApplyAction{ActionUnchanged, ActionUnchanged}) {
		t.Fatalf("third apply: %v", got)
	}
	if mode := getFake(t, dyn, fakeKind(t, "ConfigMap"), "shop", "app").Object["data"].(map[string]any)["mode"]; mode != "green" {
		t.Fatalf("data.mode = %v", mode)
	}
	for _, p := range patchRequests(dyn) {
		if p.GetPatchType() != types.ApplyPatchType || p.PatchOptions.FieldManager != FieldManager ||
			p.PatchOptions.Force != nil || p.PatchOptions.DryRun != nil {
			t.Fatalf("patch request = %s %+v", p.GetPatchType(), p.PatchOptions)
		}
	}
}

// Applying what is already in effect is unchanged even when KubeBench
// becomes a co-owner of the fields (managedFields changes, content not).
func TestApplyUnchangedForAnotherManagersObject(t *testing.T) {
	rt, dyn := editingRuntime(t)
	ctx := context.Background()
	body, _ := json.Marshal(mustParseManifests(t, twoConfigMaps)[0].Object)
	if _, err := dyn.Resource(fakeKind(t, "ConfigMap").GVR()).Namespace("shop").Patch(ctx, "app", types.ApplyPatchType, body,
		metav1.PatchOptions{FieldManager: "kubectl"}); err != nil {
		t.Fatal(err)
	}
	res := rt.Apply(ctx, mustParseManifests(t, twoConfigMaps)[:1], ApplyOptions{})
	if got := applyActions(res); !slices.Equal(got, []ApplyAction{ActionUnchanged}) {
		t.Fatalf("apply = %v", got)
	}
}

func TestApplyDryRunAndForceReachTheAPI(t *testing.T) {
	rt, dyn := editingRuntime(t)
	rt.Apply(context.Background(), mustParseManifests(t, twoConfigMaps), ApplyOptions{DryRun: true, Force: true})
	sent := patchRequests(dyn)
	if len(sent) != 2 {
		t.Fatalf("%d patch requests", len(sent))
	}
	for _, p := range sent {
		if !slices.Equal(p.PatchOptions.DryRun, []string{metav1.DryRunAll}) || p.PatchOptions.Force == nil || !*p.PatchOptions.Force {
			t.Fatalf("options = %+v", p.PatchOptions)
		}
	}
}

func TestApplyNamespaces(t *testing.T) {
	rt, dyn := editingRuntime(t)
	manifests := `apiVersion: v1
kind: ConfigMap
metadata: {name: defaulted}
---
apiVersion: v1
kind: ConfigMap
metadata: {name: explicit, namespace: billing}
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata: {name: reader, namespace: shop}
rules: []
`
	res := rt.Apply(context.Background(), mustParseManifests(t, manifests), ApplyOptions{Namespace: "shop"})
	if got := applyActions(res); !slices.Equal(got, []ApplyAction{ActionCreated, ActionCreated, ActionCreated}) {
		t.Fatalf("apply = %v", got)
	}
	if res[0].Namespace != "shop" || res[1].Namespace != "billing" || res[2].Namespace != "" {
		t.Fatalf("namespaces = %q %q %q", res[0].Namespace, res[1].Namespace, res[2].Namespace)
	}
	if len(res[2].Warnings) != 1 || !strings.Contains(res[2].Warnings[0], "cluster-scoped") || res[0].Warnings != nil {
		t.Fatalf("warnings = %v / %v", res[2].Warnings, res[0].Warnings)
	}
	getFake(t, dyn, fakeKind(t, "ConfigMap"), "shop", "defaulted")
	getFake(t, dyn, fakeKind(t, "ConfigMap"), "billing", "explicit")
	if role := getFake(t, dyn, fakeKind(t, "ClusterRole"), "", "reader"); role.GetNamespace() != "" {
		t.Fatalf("the cluster-scoped object kept namespace %q", role.GetNamespace())
	}

	// Without a namespace for the request either, "default" it is.
	res = rt.Apply(context.Background(), mustParseManifests(t, manifests)[:1], ApplyOptions{})
	if res[0].Namespace != "default" || res[0].Err != nil {
		t.Fatalf("result = %+v", res[0])
	}
}

func TestApplyConflictThenForce(t *testing.T) {
	rt, dyn := editingRuntime(t)
	ctx := context.Background()
	// Another field manager owns data.mode ("kubectl apply --server-side --field-manager=other").
	theirs, _ := json.Marshal(mustParseManifests(t, twoConfigMaps)[0].Object)
	if _, err := dyn.Resource(fakeKind(t, "ConfigMap").GVR()).Namespace("shop").Patch(ctx, "app", types.ApplyPatchType, theirs,
		metav1.PatchOptions{FieldManager: "other"}); err != nil {
		t.Fatal(err)
	}
	ours := mustParseManifests(t, strings.Replace(twoConfigMaps, "mode: blue", "mode: green", 1))

	res := rt.Apply(ctx, ours, ApplyOptions{})
	if !IsApplyConflict(res[0].Err) || res[0].Action != "" {
		t.Fatalf("result = %+v, want a conflict", res[0])
	}
	if !strings.Contains(res[0].Err.Error(), `conflict with "other"`) || !strings.Contains(res[0].Err.Error(), ".data.mode") {
		t.Fatalf("conflict message = %q", res[0].Err)
	}
	if res[1].Err != nil || res[1].Action != ActionCreated {
		t.Fatalf("an object without a conflict must still be applied: %+v", res[1])
	}

	res = rt.Apply(ctx, ours, ApplyOptions{Force: true})
	if res[0].Err != nil || res[0].Action != ActionConfigured {
		t.Fatalf("forced result = %+v", res[0])
	}
	if mode := getFake(t, dyn, fakeKind(t, "ConfigMap"), "shop", "app").Object["data"].(map[string]any)["mode"]; mode != "green" {
		t.Fatalf("data.mode = %v", mode)
	}
}

func TestApplyPerObjectErrors(t *testing.T) {
	rt, dyn := editingRuntime(t)
	manifests := `apiVersion: example.com/v1
kind: Widget
metadata: {name: w}
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: app
  managedFields: [{manager: kubectl, operation: Update}]
`
	res := rt.Apply(context.Background(), mustParseManifests(t, manifests), ApplyOptions{})
	if !meta.IsNoMatchError(res[0].Err) {
		t.Fatalf("an unknown kind: %+v", res[0])
	}
	if res[1].Err != nil || res[1].Action != ActionCreated {
		t.Fatalf("the next object: %+v", res[1])
	}
	// The API server refuses apply requests that set managedFields.
	sent := patchRequests(dyn)
	if len(sent) != 1 || strings.Contains(string(sent[0].GetPatch()), "managedFields") {
		t.Fatalf("patch body = %s", sent[0].GetPatch())
	}
}

func TestIsApplyConflict(t *testing.T) {
	fieldConflict := apierrors.NewApplyConflict([]metav1.StatusCause{{
		Type: metav1.CauseTypeFieldManagerConflict, Message: `conflict with "other"`, Field: ".data.mode",
	}}, `Apply failed with 1 conflict: conflict with "other": .data.mode`)
	configMaps := schema.GroupResource{Resource: "configmaps"}
	for _, tc := range []struct {
		name string
		err  error
		want bool
	}{
		{"field manager conflict", fieldConflict, true},
		{"wrapped", fmt.Errorf("document 2: %w", fieldConflict), true},
		// A stale resourceVersion is a 409 too, but forcing would not help.
		{"stale resourceVersion", apierrors.NewConflict(configMaps, "app", errors.New("the object has been modified")), false},
		{"not found", apierrors.NewNotFound(configMaps, "app"), false},
		{"nil", nil, false},
	} {
		if got := IsApplyConflict(tc.err); got != tc.want {
			t.Errorf("%s: IsApplyConflict = %v", tc.name, got)
		}
	}
}

func TestApplyAppliesSecretsWithRealValues(t *testing.T) {
	rt, dyn := editingRuntime(t, kubetest.Object("v1", "Namespace", "", "shop", nil))
	res := rt.Apply(context.Background(), mustParseManifests(t, "apiVersion: v1\nkind: Secret\nmetadata: {name: db, namespace: shop}\ndata: {password: aHVudGVyMg==}\n"), ApplyOptions{})
	if res[0].Err != nil || res[0].Action != ActionCreated {
		t.Fatalf("result = %+v", res[0])
	}
	if got := getFake(t, dyn, fakeKind(t, "Secret"), "shop", "db").Object["data"].(map[string]any)["password"]; got != "aHVudGVyMg==" {
		t.Fatalf("data.password = %v", got)
	}
}
