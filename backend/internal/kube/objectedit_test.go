package kube

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"kubebench/internal/kube/kubetest"
)

const cmOriginal = `apiVersion: v1
kind: ConfigMap
metadata:
  name: cm1
  namespace: shop
  labels: {team: a}
data: {a: "1", b: "2"}
`

func editTarget(t *testing.T, rt *Runtime, apiVersion, kind, name string) Object {
	t.Helper()
	o, err := rt.ResolveObject(apiVersion, kind, "shop", name)
	if err != nil {
		t.Fatal(err)
	}
	return o
}

// Removing a field in the editor removes it from the object, also when
// another field manager owns it (server-side apply would keep it).
func TestEditObjectRemovesFields(t *testing.T) {
	cm := kubetest.Object("v1", "ConfigMap", "shop", "cm1", map[string]any{"data": map[string]any{"a": "1", "b": "2"}})
	cm.SetLabels(map[string]string{"team": "a"})
	cm.SetManagedFields([]metav1.ManagedFieldsEntry{{Manager: "kubectl", Operation: metav1.ManagedFieldsOperationUpdate}})
	rt, dyn := editingRuntime(t, cm)
	ctx := context.Background()
	o := editTarget(t, rt, "v1", "ConfigMap", "cm1")

	edited := strings.Replace(cmOriginal, `data: {a: "1", b: "2"}`, `data: {a: "10"}`, 1)
	edited = strings.Replace(edited, "labels: {team: a}", "labels: {team: a, tier: web}", 1)
	res, err := rt.EditObject(ctx, o, []byte(cmOriginal), []byte(edited), false)
	if err != nil || res.Err != nil || res.Action != ActionConfigured {
		t.Fatalf("edit = %+v, %v", res, err)
	}
	sent := patchRequests(dyn)
	if len(sent) != 1 || sent[0].GetPatchType() != types.MergePatchType || sent[0].PatchOptions.FieldManager != FieldManager {
		t.Fatalf("patch requests = %+v", sent)
	}
	var patch map[string]any
	if err := json.Unmarshal(sent[0].GetPatch(), &patch); err != nil {
		t.Fatal(err)
	}
	want := `{"data":{"a":"10","b":null},"metadata":{"labels":{"tier":"web"}}}`
	if got, _ := json.Marshal(patch); string(got) != want {
		t.Fatalf("patch = %s, want %s", got, want)
	}
	got := getFake(t, dyn, fakeKind(t, "ConfigMap"), "shop", "cm1")
	if data := got.Object["data"].(map[string]any); len(data) != 1 || data["a"] != "10" {
		t.Fatalf("data = %v, want b removed", data)
	}

	// Nothing edited: unchanged, and nothing is sent.
	res, err = rt.EditObject(ctx, o, []byte(edited), []byte(edited), false)
	if err != nil || res.Action != ActionUnchanged || len(patchRequests(dyn)) != 1 {
		t.Fatalf("an edit without changes = %+v, %v", res, err)
	}

	// A dry run sends the patch as one and changes nothing.
	res, err = rt.EditObject(ctx, o, []byte(edited), []byte(cmOriginal), true)
	if err != nil || res.Err != nil || res.Action != ActionConfigured {
		t.Fatalf("dry run = %+v, %v", res, err)
	}
	if last := patchRequests(dyn)[1]; !slices.Equal(last.PatchOptions.DryRun, []string{metav1.DryRunAll}) {
		t.Fatalf("dry run options = %+v", last.PatchOptions)
	}
}

func TestEditObjectRefusals(t *testing.T) {
	cm := kubetest.Object("v1", "ConfigMap", "shop", "cm1", map[string]any{"data": map[string]any{"a": "1"}})
	rt, dyn := editingRuntime(t, cm)
	ctx := context.Background()
	o := editTarget(t, rt, "v1", "ConfigMap", "cm1")

	for name, edited := range map[string]string{
		"renamed":       strings.Replace(cmOriginal, "name: cm1", "name: cm2", 1),
		"other kind":    strings.Replace(cmOriginal, "kind: ConfigMap", "kind: Secret", 1),
		"other ns":      strings.Replace(cmOriginal, "namespace: shop", "namespace: billing", 1),
		"two documents": cmOriginal + "---\n" + cmOriginal,
		"empty":         "",
		"invalid":       "data: [",
		"not an object": "- a\n- b\n",
	} {
		_, err := rt.EditObject(ctx, o, []byte(cmOriginal), []byte(edited), false)
		var ee *EditError
		if !errors.As(err, &ee) || ee.Redacted {
			t.Errorf("%s: %v, want an EditError", name, err)
		}
	}
	// The namespace may be left out: the object's own is meant, so that
	// is no change.
	withoutNS := strings.Replace(cmOriginal, "  namespace: shop\n", "", 1)
	if res, err := rt.EditObject(ctx, o, []byte(cmOriginal), []byte(withoutNS), false); err != nil || res.Action != ActionUnchanged {
		t.Fatalf("an edit without the namespace: %+v, %v", res, err)
	}
	if n := len(patchRequests(dyn)); n != 0 {
		t.Fatalf("%d patches sent for refused or empty edits", n)
	}
	if _, err := rt.EditObject(ctx, editTarget(t, rt, "v1", "ConfigMap", "ghost"), []byte(strings.ReplaceAll(cmOriginal, "cm1", "ghost")),
		[]byte(strings.ReplaceAll(withoutNS, "cm1", "ghost")), false); err != nil {
		t.Fatalf("a missing object is the cluster's answer, not a refusal: %v", err)
	}
}

// Placeholders the edit leaves alone are not sent, so a Secret's labels
// can be edited without its values; a value set to a placeholder is
// refused.
func TestEditObjectSecretPlaceholders(t *testing.T) {
	secret := kubetest.Object("v1", "Secret", "shop", "db", map[string]any{"type": "Opaque", "data": map[string]any{"pin": "NDgyMQ=="}})
	rt, dyn := editingRuntime(t, secret)
	ctx := context.Background()
	o := editTarget(t, rt, "v1", "Secret", "db")
	original := "apiVersion: v1\nkind: Secret\nmetadata: {name: db, namespace: shop}\ntype: Opaque\ndata: {pin: '<8 bytes, redacted>'}\n"

	labelled := strings.Replace(original, "namespace: shop}", "namespace: shop, labels: {app: db}}", 1)
	res, err := rt.EditObject(ctx, o, []byte(original), []byte(labelled), false)
	if err != nil || res.Err != nil {
		t.Fatalf("label edit = %+v, %v", res, err)
	}
	got := getFake(t, dyn, fakeKind(t, "Secret"), "shop", "db")
	if got.GetLabels()["app"] != "db" || got.Object["data"].(map[string]any)["pin"] != "NDgyMQ==" {
		t.Fatalf("secret = %v", got.Object)
	}

	added := strings.Replace(original, "data: {pin: '<8 bytes, redacted>'}", "data: {pin: '<8 bytes, redacted>', pin2: '<8 bytes, redacted>'}", 1)
	_, err = rt.EditObject(ctx, o, []byte(original), []byte(added), false)
	var ee *EditError
	if !errors.As(err, &ee) || !ee.Redacted || !strings.Contains(ee.Problem, "data.pin2") {
		t.Fatalf("a placeholder as a new value: %v", err)
	}
	removed := strings.Replace(original, "data: {pin: '<8 bytes, redacted>'}", "data: {}", 1)
	if res, err := rt.EditObject(ctx, o, []byte(original), []byte(removed), false); err != nil || res.Err != nil {
		t.Fatalf("removing a value: %+v, %v", res, err)
	}
}

func TestMergePatch(t *testing.T) {
	from := map[string]any{"a": "1", "keep": "x", "m": map[string]any{"x": int64(1), "y": int64(2)}, "list": []any{"a", "b"}, "gone": true}
	to := map[string]any{"a": "2", "keep": "x", "m": map[string]any{"x": int64(1)}, "list": []any{"a"}, "new": map[string]any{"k": "v"}}
	got, _ := json.Marshal(mergePatch(from, to))
	if want := `{"a":"2","gone":null,"list":["a"],"m":{"y":null},"new":{"k":"v"}}`; string(got) != want {
		t.Fatalf("patch = %s, want %s", got, want)
	}
}
