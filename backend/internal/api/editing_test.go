package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	k8stesting "k8s.io/client-go/testing"

	"kubebench/internal/kube"
	"kubebench/internal/kube/kubetest"
	"kubebench/internal/plugin"
	"kubebench/internal/store"
)

// editingEnv is a testEnv whose cluster also serves kubetest.Kinds
// through a dynamic client with server-side apply semantics.
func editingEnv(t *testing.T, objects ...*unstructured.Unstructured) (*testEnv, *dynamicfake.FakeDynamicClient) {
	t.Helper()
	e := newTestEnv(t, nil)
	e.cs.Fake.Resources = kubetest.Resources(kubetest.Kinds)
	dyn := kubetest.NewDynamic(kubetest.Kinds, objects...)
	e.srv.manager = kube.NewManagerWith(func(c store.Cluster) (*kube.Runtime, error) {
		return kube.NewRuntimeForTest(c, e.cs, dyn), nil
	})
	return e, dyn
}

// sendText sends a request with a plain-text body (manifests) as user.
func sendText(e *testEnv, method, target, user, body string) *httptest.ResponseRecorder {
	e.t.Helper()
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/yaml")
	req.AddCookie(&http.Cookie{Name: sessionCookie, Value: e.tokens[user]})
	return e.serve(req)
}

type applyResponse struct {
	Results []applyResult `json:"results"`
}

const shopManifests = `apiVersion: v1
kind: ConfigMap
metadata: {name: app}
data: {mode: blue}
---
apiVersion: v1
kind: ConfigMap
metadata: {name: flags, namespace: billing}
data: {beta: "true"}
`

var configMaps = schema.GroupVersionResource{Version: "v1", Resource: "configmaps"}

func dynamicMutations(dyn *dynamicfake.FakeDynamicClient) []k8stesting.Action {
	var out []k8stesting.Action
	for _, a := range dyn.Actions() {
		if v := a.GetVerb(); v != "get" && v != "list" && v != "watch" {
			out = append(out, a)
		}
	}
	return out
}

func TestApplyAccessAndAudit(t *testing.T) {
	e, dyn := editingEnv(t)

	// A viewer may dry run: it is read access, and not audited.
	rec := sendText(e, http.MethodPost, "/api/resource/apply?dryRun=true&ns=shop", "bob", shopManifests)
	expect(t, rec, http.StatusOK, "")
	res := decode[applyResponse](t, rec).Results
	if len(res) != 2 || res[0].Action != "created" || res[0].Namespace != "shop" || res[1].Namespace != "billing" {
		t.Fatalf("dry run results = %+v", res)
	}
	for _, a := range dynamicMutations(dyn) {
		if p, ok := a.(k8stesting.PatchActionImpl); !ok || !slices.Equal(p.PatchOptions.DryRun, []string{metav1.DryRunAll}) {
			t.Fatalf("a dry run sent %#v", a)
		}
	}
	if rows := e.audit(); len(rows) != 0 {
		t.Fatalf("a dry run was audited: %+v", rows)
	}

	// Applying is a write: refused for the viewer, audited either way.
	before := len(dyn.Actions())
	expect(t, sendText(e, http.MethodPost, "/api/resource/apply?ns=shop", "bob", shopManifests), http.StatusForbidden, plugin.CodeForbiddenRole)
	if len(dyn.Actions()) != before {
		t.Fatal("a refused apply reached the cluster")
	}
	rec = sendText(e, http.MethodPost, "/api/resource/apply?ns=shop", "alice", shopManifests)
	expect(t, rec, http.StatusOK, "")
	if res := decode[applyResponse](t, rec).Results; res[0].Error != "" || res[1].Error != "" {
		t.Fatalf("apply results = %+v", res)
	}
	rows := e.audit()
	if len(rows) != 2 {
		t.Fatalf("audit rows = %+v", rows)
	}
	if r := rows[0]; r.Username != "alice" || r.Action != "POST /api/resource/apply" || r.Status != 200 ||
		r.Target != "configmap/shop/app (+1 more)" || r.Detail == "" {
		t.Fatalf("apply row = %+v", r)
	}
	if r := rows[1]; r.Username != "bob" || r.Status != http.StatusForbidden || r.Target != "resource/shop" {
		t.Fatalf("refused row = %+v", r)
	}

	// Only exactly dryRun=true is a dry run; other spellings are refused
	// instead of being applied for real.
	expect(t, sendText(e, http.MethodPost, "/api/resource/apply?dryRun=1", "alice", shopManifests), http.StatusBadRequest, plugin.CodeBadRequest)
	expect(t, sendText(e, http.MethodPost, "/api/resource/apply?dryRun=1", "bob", shopManifests), http.StatusForbidden, plugin.CodeForbiddenRole)
}

// A dry run of a Secret tells whether a guessed value is the real one
// ("unchanged"), so it is treated like the apply: write access, audited.
func TestSecretDryRunNeedsWriteAccess(t *testing.T) {
	db := kubetest.Object("v1", "Secret", "shop", "db", map[string]any{"type": "Opaque", "data": map[string]any{"pin": "NDgyMQ=="}})
	e, dyn := editingEnv(t, db)
	guess := "apiVersion: v1\nkind: Secret\nmetadata: {name: db, namespace: shop}\ntype: Opaque\ndata: {pin: NDgyMQ==}\n"

	before := len(dyn.Actions())
	expect(t, sendText(e, http.MethodPost, "/api/resource/apply?dryRun=true", "bob", guess), http.StatusForbidden, plugin.CodeForbiddenRole)
	if len(dyn.Actions()) != before {
		t.Fatal("a viewer's Secret dry run reached the cluster")
	}
	// Mixed in with other objects too.
	expect(t, sendText(e, http.MethodPost, "/api/resource/apply?dryRun=true&ns=shop", "bob", shopManifests+"---\n"+guess), http.StatusForbidden, plugin.CodeForbiddenRole)

	rec := sendText(e, http.MethodPost, "/api/resource/apply?dryRun=true", "alice", guess)
	expect(t, rec, http.StatusOK, "")
	if res := decode[applyResponse](t, rec).Results; len(res) != 1 || res[0].Error != "" {
		t.Fatalf("operator's dry run = %+v", res)
	}
	rows := e.audit()
	if len(rows) != 3 {
		t.Fatalf("audit rows = %+v, want every Secret dry run", rows)
	}
	if r := rows[0]; r.Username != "alice" || r.Action != "POST /api/resource/apply" || r.Status != 200 ||
		r.Target != "secret/shop/db" || !strings.Contains(r.Detail, `"dryRun":true`) {
		t.Fatalf("dry run row = %+v", r)
	}
	if r := rows[2]; r.Username != "bob" || r.Status != http.StatusForbidden || !strings.HasPrefix(r.Detail, "forbidden_role: ") {
		t.Fatalf("refused row = %+v", r)
	}

	// Other dry runs stay read access and unaudited.
	expect(t, sendText(e, http.MethodPost, "/api/resource/apply?dryRun=true&ns=shop", "bob", shopManifests), http.StatusOK, "")
	if n := len(e.audit()); n != 3 {
		t.Fatalf("a ConfigMap dry run was audited (%d rows)", n)
	}
}

func TestEditEndpoint(t *testing.T) {
	cm := kubetest.Object("v1", "ConfigMap", "shop", "cm1", map[string]any{"data": map[string]any{"a": "1", "b": "2"}})
	db := kubetest.Object("v1", "Secret", "shop", "db", map[string]any{"type": "Opaque", "data": map[string]any{"pin": "NDgyMQ=="}})
	e, dyn := editingEnv(t, cm, db)
	const target = "/api/resource/edit?apiVersion=v1&kind=ConfigMap&ns=shop&name=cm1"
	original := "apiVersion: v1\nkind: ConfigMap\nmetadata: {name: cm1, namespace: shop}\ndata: {a: \"1\", b: \"2\"}\n"
	edited := strings.Replace(original, `, b: "2"`, "", 1)
	body := editRequest{Original: original, Edited: edited}

	// A viewer may dry run an edit (read, not audited), not apply it.
	retuned := editRequest{Original: original, Edited: strings.Replace(original, `a: "1"`, `a: "9"`, 1)}
	rec := e.do(http.MethodPost, target+"&dryRun=true", "bob", retuned)
	expect(t, rec, http.StatusOK, "")
	if res := decode[applyResponse](t, rec).Results; len(res) != 1 || res[0].Action != "configured" || res[0].Name != "cm1" {
		t.Fatalf("dry run = %+v", res)
	}
	expect(t, e.do(http.MethodPost, target, "bob", body), http.StatusForbidden, plugin.CodeForbiddenRole)

	rec = e.do(http.MethodPost, target, "alice", body)
	expect(t, rec, http.StatusOK, "")
	if res := decode[applyResponse](t, rec).Results; len(res) != 1 || res[0].Action != "configured" {
		t.Fatalf("edit = %+v", res)
	}
	live, err := dyn.Resource(configMaps).Namespace("shop").Get(context.Background(), "cm1", metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if data := live.Object["data"].(map[string]any); len(data) != 1 {
		t.Fatalf("data = %v, want b removed", data)
	}
	rows := e.audit()
	if len(rows) != 2 {
		t.Fatalf("audit rows = %+v", rows)
	}
	if r := rows[0]; r.Username != "alice" || r.Action != "POST /api/resource/edit" || r.Target != "configmap/shop/cm1" ||
		r.Status != 200 || r.Detail != `{"configured":1}` {
		t.Fatalf("edit row = %+v", r)
	}

	// Refusals before anything is sent.
	before := len(dynamicMutations(dyn))
	renamed := editRequest{Original: original, Edited: strings.Replace(edited, "name: cm1", "name: cm2", 1)}
	expect(t, e.do(http.MethodPost, target, "alice", renamed), http.StatusBadRequest, plugin.CodeBadRequest)
	expect(t, e.do(http.MethodPost, target+"&dryRun=1", "alice", body), http.StatusBadRequest, plugin.CodeBadRequest)
	secretTarget := "/api/resource/edit?apiVersion=v1&kind=Secret&ns=shop&name=db"
	secretOriginal := "apiVersion: v1\nkind: Secret\nmetadata: {name: db, namespace: shop}\ndata: {pin: '<8 bytes, redacted>'}\n"
	placeholder := editRequest{Original: secretOriginal, Edited: strings.Replace(secretOriginal, "\ndata: {", "\ndata: {other: '<4 bytes, redacted>', ", 1)}
	expect(t, e.do(http.MethodPost, secretTarget, "alice", placeholder), http.StatusBadRequest, plugin.CodeRedactedSecret)
	// A Secret dry run is the edit itself (§3.2).
	guess := editRequest{Original: secretOriginal, Edited: strings.Replace(secretOriginal, "'<8 bytes, redacted>'", "NDgyMQ==", 1)}
	expect(t, e.do(http.MethodPost, secretTarget+"&dryRun=true", "bob", guess), http.StatusForbidden, plugin.CodeForbiddenRole)
	if n := len(dynamicMutations(dyn)); n != before {
		t.Fatalf("%d refused edits reached the cluster", n-before)
	}
	if r := e.lastAudit(); r.Username != "bob" || r.Target != "secret/shop/db" || r.Status != http.StatusForbidden {
		t.Fatalf("refused Secret dry run row = %+v", r)
	}
}

func TestApplyOnReadOnlyCluster(t *testing.T) {
	e, _ := editingEnv(t)
	expect(t, e.do(http.MethodPatch, "/api/clusters/1", "admin", map[string]any{"readOnly": true}), http.StatusOK, "")
	expect(t, sendText(e, http.MethodPost, "/api/resource/apply", "admin", shopManifests), http.StatusForbidden, plugin.CodeClusterReadOnly)
	// Dry runs change nothing and stay available, as reads do.
	expect(t, sendText(e, http.MethodPost, "/api/resource/apply?dryRun=true", "admin", shopManifests), http.StatusOK, "")
}

func TestApplyRefusesTheWholeRequest(t *testing.T) {
	e, dyn := editingEnv(t)
	for _, tc := range []struct {
		name, query, body string
		status            int
		code              string
	}{
		{"invalid YAML", "", "kind: [oops\n", http.StatusBadRequest, plugin.CodeBadRequest},
		{"no objects", "", "---\n# nothing\n", http.StatusBadRequest, plugin.CodeBadRequest},
		{"no name", "", "apiVersion: v1\nkind: ConfigMap\nmetadata: {}\n", http.StatusBadRequest, plugin.CodeBadRequest},
		{"force spelled wrong", "?force=yes", shopManifests, http.StatusBadRequest, plugin.CodeBadRequest},
		{
			"redacted Secret", "",
			shopManifests + "---\napiVersion: v1\nkind: Secret\nmetadata: {name: db}\ndata: {password: \"<12 bytes, redacted>\"}\n",
			http.StatusBadRequest, plugin.CodeRedactedSecret,
		},
		{"too large", "", "apiVersion: v1\nkind: ConfigMap\nmetadata: {name: big}\ndata: {blob: " + strings.Repeat("x", maxManifestBytes) + "}\n",
			http.StatusRequestEntityTooLarge, plugin.CodeBadRequest},
	} {
		t.Run(tc.name, func(t *testing.T) {
			expect(t, sendText(e, http.MethodPost, "/api/resource/apply"+tc.query, "alice", tc.body), tc.status, tc.code)
		})
	}
	if m := dynamicMutations(dyn); len(m) != 0 {
		t.Fatalf("refused requests reached the cluster: %v", m)
	}
}

func TestApplyConflictCode(t *testing.T) {
	e, dyn := editingEnv(t)
	theirs := `{"apiVersion":"v1","kind":"ConfigMap","metadata":{"name":"app","namespace":"shop"},"data":{"mode":"blue"}}`
	if _, err := dyn.Resource(configMaps).Namespace("shop").Patch(context.Background(), "app", types.ApplyPatchType, []byte(theirs),
		metav1.PatchOptions{FieldManager: "other"}); err != nil {
		t.Fatal(err)
	}
	ours := strings.Replace(shopManifests, "mode: blue", "mode: green", 1)

	res := decode[applyResponse](t, sendText(e, http.MethodPost, "/api/resource/apply?ns=shop", "alice", ours)).Results
	if res[0].Code != plugin.CodeConflict || res[0].Action != "" || !strings.Contains(res[0].Error, `conflict with "other"`) {
		t.Fatalf("conflicting object = %+v", res[0])
	}
	if res[1].Action != "created" || res[1].Code != "" {
		t.Fatalf("the other object = %+v", res[1])
	}
	if row := e.audit()[0]; row.Status != 200 || row.Detail != `{"created":1,"failed":1}` {
		t.Fatalf("audit row = %+v", row)
	}

	res = decode[applyResponse](t, sendText(e, http.MethodPost, "/api/resource/apply?ns=shop&force=true", "alice", ours)).Results
	if res[0].Action != "configured" || res[1].Action != "unchanged" {
		t.Fatalf("forced results = %+v", res)
	}
	if row := e.audit()[0]; row.Detail != `{"configured":1,"force":true,"unchanged":1}` {
		t.Fatalf("forced audit row = %+v", row)
	}
}

func TestApplyUnknownKind(t *testing.T) {
	e, _ := editingEnv(t)
	res := decode[applyResponse](t, sendText(e, http.MethodPost, "/api/resource/apply", "alice",
		"apiVersion: example.com/v1\nkind: Widget\nmetadata: {name: w}\n")).Results
	if len(res) != 1 || res[0].Code != plugin.CodeNotFound || res[0].Error == "" {
		t.Fatalf("results = %+v", res)
	}
}

func TestDeleteResource(t *testing.T) {
	e, dyn := editingEnv(t,
		kubetest.Object("apps/v1", "Deployment", "shop", "web", nil),
		kubetest.Object("v1", "ConfigMap", "shop", "app", nil),
		kubetest.Object("v1", "Namespace", "", "shop", nil),
	)
	expect(t, e.do(http.MethodDelete, "/api/resource?apiVersion=apps/v1&kind=Deployment&ns=shop&name=web&propagation=Foreground&gracePeriod=5", "bob", nil),
		http.StatusForbidden, plugin.CodeForbiddenRole)
	expect(t, e.do(http.MethodDelete, "/api/resource?apiVersion=apps/v1&kind=Deployment&ns=shop&name=web&propagation=Foreground&gracePeriod=5", "alice", nil),
		http.StatusOK, "")
	var deleted *k8stesting.DeleteActionImpl
	for _, a := range dyn.Actions() {
		if d, ok := a.(k8stesting.DeleteActionImpl); ok {
			deleted = &d
		}
	}
	if deleted == nil || deleted.GetName() != "web" || *deleted.DeleteOptions.PropagationPolicy != metav1.DeletePropagationForeground ||
		*deleted.DeleteOptions.GracePeriodSeconds != 5 {
		t.Fatalf("delete request = %+v", deleted)
	}
	if row := e.audit()[0]; row.Action != "DELETE /api/resource" || row.Target != "deployment/shop/web" ||
		row.Detail != `{"apiVersion":"apps/v1","gracePeriod":"5","propagation":"Foreground"}` {
		t.Fatalf("audit row = %+v", row)
	}

	// The generic catalog key names the kind too.
	expect(t, e.do(http.MethodDelete, "/api/resource?key=configmaps&ns=shop&name=app", "alice", nil), http.StatusOK, "")
	if row := e.audit()[0]; row.Target != "configmap/shop/app" {
		t.Fatalf("audit row = %+v", row)
	}

	before := len(dynamicMutations(dyn))
	for _, tc := range []struct {
		target string
		status int
		code   string
	}{
		{"/api/resource?apiVersion=apps/v1&kind=Deployment&ns=shop&name=web&propagation=Cascade", http.StatusBadRequest, plugin.CodeBadRequest},
		{"/api/resource?apiVersion=apps/v1&kind=Deployment&ns=shop&name=web&gracePeriod=-1", http.StatusBadRequest, plugin.CodeBadRequest},
		{"/api/resource?apiVersion=apps/v1&kind=Deployment&name=web", http.StatusBadRequest, plugin.CodeBadRequest},
		{"/api/resource?kind=Deployment&ns=shop&name=web", http.StatusBadRequest, plugin.CodeBadRequest},
		{"/api/resource?key=configmaps&kind=ConfigMap&ns=shop&name=app", http.StatusBadRequest, plugin.CodeBadRequest},
		{"/api/resource?key=nope&ns=shop&name=app", http.StatusBadRequest, plugin.CodeBadRequest},
		{"/api/resource?apiVersion=example.com/v1&kind=Widget&ns=shop&name=w", http.StatusNotFound, plugin.CodeNotFound},
		// Namespaces go through DELETE /api/namespaces and its protections.
		{"/api/resource?apiVersion=v1&kind=Namespace&name=shop", http.StatusBadRequest, plugin.CodeBadRequest},
	} {
		expect(t, e.do(http.MethodDelete, tc.target, "alice", nil), tc.status, tc.code)
	}
	if m := dynamicMutations(dyn); len(m) != before {
		t.Fatalf("refused deletes reached the cluster: %v", m[before:])
	}
}

func TestPatchMetadataEndpoint(t *testing.T) {
	app := kubetest.Object("v1", "ConfigMap", "shop", "app", nil)
	app.SetLabels(map[string]string{"app": "shop", "tier": "web"})
	db := kubetest.Object("v1", "Secret", "shop", "db", map[string]any{"data": map[string]any{"password": "aHVudGVyMg=="}})
	e, dyn := editingEnv(t, app, db)
	patch := map[string]any{"labels": map[string]any{"tier": nil, "release": "canary"}, "annotations": map[string]any{"note": "hello"}}

	expect(t, e.do(http.MethodPatch, "/api/resource/metadata?apiVersion=v1&kind=ConfigMap&ns=shop&name=app", "bob", patch), http.StatusForbidden, plugin.CodeForbiddenRole)
	expect(t, e.do(http.MethodPatch, "/api/resource/metadata?apiVersion=v1&kind=ConfigMap&ns=shop&name=app", "alice", patch), http.StatusOK, "")
	got, err := dyn.Resource(configMaps).Namespace("shop").Get(context.Background(), "app", metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if labels := got.GetLabels(); len(labels) != 2 || labels["release"] != "canary" || labels["app"] != "shop" || got.GetAnnotations()["note"] != "hello" {
		t.Fatalf("metadata = %v %v", labels, got.GetAnnotations())
	}
	// The audit row names the keys, never the values.
	if row := e.audit()[0]; row.Target != "configmap/shop/app" || row.Detail != `{"keys":["annotation:note","label:release","label:tier"]}` {
		t.Fatalf("audit row = %+v", row)
	}

	expect(t, e.do(http.MethodPatch, "/api/resource/metadata?apiVersion=v1&kind=ConfigMap&ns=shop&name=app", "alice", map[string]any{}),
		http.StatusBadRequest, plugin.CodeBadRequest)
	// Writing the placeholder a redacted read shows would destroy the real value.
	redacted := map[string]any{"annotations": map[string]any{"kubectl.kubernetes.io/last-applied-configuration": "<redacted>"}}
	expect(t, e.do(http.MethodPatch, "/api/resource/metadata?key=secrets&ns=shop&name=db", "alice", redacted),
		http.StatusBadRequest, plugin.CodeRedactedSecret)
}

func TestRevealSecret(t *testing.T) {
	db := kubetest.Object("v1", "Secret", "shop", "db", map[string]any{"type": "Opaque", "data": map[string]any{"password": "aHVudGVyMg=="}})
	e, _ := editingEnv(t, db)

	// Reading the Secret anywhere else shows placeholders only.
	if text := decode[map[string]string](t, e.do(http.MethodGet, "/api/yaml?apiVersion=v1&kind=Secret&ns=shop&name=db", "bob", nil))["yaml"]; strings.Contains(text, "aHVudGVyMg") {
		t.Fatalf("the YAML view shows the value:\n%s", text)
	}

	// Viewers cannot reveal; the attempt is audited.
	expect(t, e.do(http.MethodPost, "/api/secrets/reveal?ns=shop&name=db", "bob", nil), http.StatusForbidden, plugin.CodeForbiddenRole)
	rec := e.do(http.MethodPost, "/api/secrets/reveal?ns=shop&name=db", "alice", nil)
	expect(t, rec, http.StatusOK, "")
	if cc := rec.Header().Get("Cache-Control"); cc != "no-store" {
		t.Fatalf("Cache-Control = %q", cc)
	}
	var secret struct {
		Kind string            `json:"kind"`
		Data map[string]string `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &secret); err != nil || secret.Kind != "Secret" || secret.Data["password"] != "aHVudGVyMg==" {
		t.Fatalf("revealed = %s (%v)", rec.Body.String(), err)
	}

	// A reveal changes nothing but is audited like every exec request.
	rows := e.audit()
	if len(rows) != 2 {
		t.Fatalf("audit rows = %+v", rows)
	}
	if r := rows[0]; r.Username != "alice" || r.Action != "POST /api/secrets/reveal" || r.Target != "secret/shop/db" || r.Status != 200 {
		t.Fatalf("reveal row = %+v", r)
	}
	if r := rows[1]; r.Username != "bob" || r.Target != "secret/shop/db" || r.Status != http.StatusForbidden {
		t.Fatalf("refused row = %+v", r)
	}

	expect(t, e.do(http.MethodPost, "/api/secrets/reveal?ns=shop", "alice", nil), http.StatusBadRequest, plugin.CodeBadRequest)
	expect(t, e.do(http.MethodGet, "/api/secrets/reveal?ns=shop&name=db", "alice", nil), http.StatusMethodNotAllowed, plugin.CodeBadRequest)
	expect(t, e.do(http.MethodPatch, "/api/clusters/1", "admin", map[string]any{"readOnly": true}), http.StatusOK, "")
	expect(t, e.do(http.MethodPost, "/api/secrets/reveal?ns=shop&name=db", "admin", nil), http.StatusForbidden, plugin.CodeClusterReadOnly)
}

func TestYAMLEndpoint(t *testing.T) {
	web := kubetest.Object("apps/v1", "Deployment", "shop", "web", map[string]any{
		"spec": map[string]any{"replicas": int64(2)}, "status": map[string]any{"readyReplicas": int64(1)},
	})
	web.SetResourceVersion("9")
	e, _ := editingEnv(t, web)
	yamlOf := func(target string) string {
		t.Helper()
		rec := e.do(http.MethodGet, target, "bob", nil)
		expect(t, rec, http.StatusOK, "")
		return decode[map[string]string](t, rec)["yaml"]
	}

	full := yamlOf("/api/yaml?apiVersion=apps/v1&kind=Deployment&ns=shop&name=web")
	if !strings.Contains(full, "readyReplicas: 1") || !strings.Contains(full, `resourceVersion: "9"`) {
		t.Fatalf("full YAML:\n%s", full)
	}
	neat := yamlOf("/api/yaml?apiVersion=apps/v1&kind=Deployment&ns=shop&name=web&neat=true")
	if strings.Contains(neat, "status") || strings.Contains(neat, "resourceVersion") || !strings.Contains(neat, "replicas: 2") {
		t.Fatalf("neat YAML:\n%s", neat)
	}
	// Without apiVersion, the kinds v0.4 knew by name still work (a typed Pod here).
	if pod := yamlOf("/api/yaml?kind=Pod&ns=default&name=web-1&neat=true"); !strings.HasPrefix(pod, "apiVersion: v1\nkind: Pod\n") || strings.Contains(pod, "creationTimestamp") {
		t.Fatalf("legacy Pod YAML:\n%s", pod)
	}

	for _, tc := range []struct {
		target string
		status int
		code   string
	}{
		{"/api/yaml?apiVersion=apps/v1&kind=Deployment&ns=shop&name=web&neat=yes", http.StatusBadRequest, plugin.CodeBadRequest},
		{"/api/yaml?apiVersion=apps/v1&kind=Deployment&ns=shop", http.StatusBadRequest, plugin.CodeBadRequest},
		{"/api/yaml?apiVersion=apps/v1&kind=Deployment&name=web", http.StatusBadRequest, plugin.CodeBadRequest},
		{"/api/yaml?apiVersion=apps/v1&kind=Deployment&ns=shop&name=gone", http.StatusNotFound, plugin.CodeKubernetes},
		{"/api/yaml?apiVersion=example.com/v1&kind=Widget&ns=shop&name=w", http.StatusNotFound, plugin.CodeNotFound},
	} {
		expect(t, e.do(http.MethodGet, tc.target, "bob", nil), tc.status, tc.code)
	}
}
