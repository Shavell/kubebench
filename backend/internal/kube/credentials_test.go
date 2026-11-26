package kube

import (
	"context"
	"errors"
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/fake"

	"kubebench/internal/kube/kubetest"
	"kubebench/internal/store"
)

const (
	routeKey      = "-----BEGIN PRIVATE KEY-----\nMIIEvQIBADANBgkqhkiG9w0BAQEFAASC\n-----END PRIVATE KEY-----\n"
	webhookSecret = "TOPSECRET-WEBHOOK-VALUE"
)

// openShiftKinds are the fake cluster's kinds plus the credential kinds.
var openShiftKinds = append(append([]kubetest.Kind{}, kubetest.Kinds...),
	kubetest.Kind{GVK: schema.GroupVersionKind{Group: "route.openshift.io", Version: "v1", Kind: "Route"}, Resource: "routes", Namespaced: true},
	kubetest.Kind{GVK: schema.GroupVersionKind{Group: "build.openshift.io", Version: "v1", Kind: "BuildConfig"}, Resource: "buildconfigs", Namespaced: true},
	kubetest.Kind{GVK: schema.GroupVersionKind{Group: "build.openshift.io", Version: "v1", Kind: "Build"}, Resource: "builds", Namespaced: true},
)

func openShiftRuntime(t *testing.T, objects ...*unstructured.Unstructured) *Runtime {
	t.Helper()
	cs := fake.NewClientset()
	cs.Fake.Resources = kubetest.Resources(openShiftKinds)
	return NewRuntimeForTest(store.Cluster{ID: 1, Name: "test"}, cs, kubetest.NewDynamic(openShiftKinds, objects...))
}

func credentialObjects() []*unstructured.Unstructured {
	route := kubetest.Object("route.openshift.io/v1", "Route", "shop", "web", map[string]any{
		"spec": map[string]any{
			"host": "web.apps.example.com",
			"tls":  map[string]any{"termination": "edge", "certificate": "PUBLIC-CERT", "key": routeKey},
		},
	})
	route.SetAnnotations(map[string]string{lastAppliedAnnotation: `{"spec":{"tls":{"key":"` + strings.ReplaceAll(routeKey, "\n", `\n`) + `"}}}`, "team": "web"})
	bc := kubetest.Object("build.openshift.io/v1", "BuildConfig", "shop", "api", map[string]any{
		"spec": map[string]any{"triggers": []any{
			map[string]any{"type": "ConfigChange"},
			map[string]any{"type": "GitHub", "github": map[string]any{"secret": webhookSecret}},
			map[string]any{"type": "Generic", "generic": map[string]any{"secret": webhookSecret, "allowEnv": true}},
			map[string]any{"type": "GitLab", "gitlab": map[string]any{"secretReference": map[string]any{"name": "hook"}}},
		}},
	})
	build := kubetest.Object("build.openshift.io/v1", "Build", "shop", "api-1", map[string]any{
		"spec": map[string]any{"triggeredBy": []any{
			map[string]any{"message": "GitHub WebHook", "githubWebHook": map[string]any{"revision": map[string]any{}, "secret": webhookSecret}},
		}},
	})
	return []*unstructured.Unstructured{route, bc, build}
}

// A viewer reads Routes, BuildConfigs and Builds at read level: their TLS
// keys and webhook secrets are redacted on every read path, like Secret
// values, and the rest stays visible.
func TestCredentialsAreRedactedOnEveryReadPath(t *testing.T) {
	rt := openShiftRuntime(t, credentialObjects()...)
	ctx := context.Background()
	for _, tc := range []struct {
		key, apiVersion, kind, name string
		keep                        []string
	}{
		{"routes", "route.openshift.io/v1", "Route", "web", []string{"PUBLIC-CERT", "web.apps.example.com", "87 bytes, redacted", "team"}},
		{"buildconfigs", "build.openshift.io/v1", "BuildConfig", "api", []string{"23 bytes, redacted", "allowEnv", "hook"}},
		{"builds", "build.openshift.io/v1", "Build", "api-1", []string{"23 bytes, redacted", "GitHub WebHook"}},
	} {
		list, _, err := rt.ListGeneric(ctx, tc.key, "shop", 0)
		if err != nil || len(list) != 1 {
			t.Fatalf("%s: ListGeneric = %v, %v", tc.key, list, err)
		}
		one, err := rt.GetGeneric(ctx, tc.key, "shop", tc.name)
		if err != nil {
			t.Fatal(err)
		}
		byKind, err := rt.ObjectYAML(ctx, ObjectRef{Kind: tc.kind, Namespace: "shop", Name: tc.name}, false)
		if err != nil {
			t.Fatal(err)
		}
		byAPIVersion, err := rt.ObjectYAML(ctx, ObjectRef{APIVersion: tc.apiVersion, Kind: tc.kind, Namespace: "shop", Name: tc.name}, true)
		if err != nil {
			t.Fatal(err)
		}
		for path, got := range map[string]string{"ListGeneric": asJSON(t, list), "GetGeneric": asJSON(t, one), "ObjectYAML": byKind, "ObjectYAML apiVersion": byAPIVersion} {
			if strings.Contains(got, "PRIVATE KEY") || strings.Contains(got, "MIIEvQ") || strings.Contains(got, webhookSecret) {
				t.Errorf("%s %s leaks a credential: %s", tc.kind, path, got)
			}
			for _, want := range tc.keep {
				if !strings.Contains(got, want) {
					t.Errorf("%s %s lacks %q: %s", tc.kind, path, want, got)
				}
			}
		}
	}
}

// An edit that leaves a redacted credential alone keeps its value; one
// that would store a placeholder, also inside a changed list, is refused.
func TestEditObjectCredentialPlaceholders(t *testing.T) {
	rt := openShiftRuntime(t, credentialObjects()...)
	ctx := context.Background()
	route, err := rt.ResolveObject("route.openshift.io/v1", "Route", "shop", "web")
	if err != nil {
		t.Fatal(err)
	}
	original, err := rt.ObjectYAML(ctx, ObjectRef{APIVersion: "route.openshift.io/v1", Kind: "Route", Namespace: "shop", Name: "web"}, true)
	if err != nil {
		t.Fatal(err)
	}
	relabelled := strings.Replace(original, "team: web", "team: platform", 1)
	if res, err := rt.EditObject(ctx, route, []byte(original), []byte(relabelled), "", false); err != nil || res.Err != nil {
		t.Fatalf("annotation edit = %+v, %v", res, err)
	}
	stored, err := rt.Dyn.Resource(route.Resource).Namespace("shop").Get(ctx, "web", metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if key, _, _ := unstructured.NestedString(stored.Object, "spec", "tls", "key"); key != routeKey {
		t.Fatalf("the edit changed the TLS key to %q", key)
	}

	bc, err := rt.ResolveObject("build.openshift.io/v1", "BuildConfig", "shop", "api")
	if err != nil {
		t.Fatal(err)
	}
	bcOriginal, err := rt.ObjectYAML(ctx, ObjectRef{APIVersion: "build.openshift.io/v1", Kind: "BuildConfig", Namespace: "shop", Name: "api"}, true)
	if err != nil {
		t.Fatal(err)
	}
	moreTriggers := strings.Replace(bcOriginal, "  - type: ConfigChange\n", "  - type: ConfigChange\n  - type: ImageChange\n", 1)
	if moreTriggers == bcOriginal {
		t.Fatalf("unexpected YAML:\n%s", bcOriginal)
	}
	_, err = rt.EditObject(ctx, bc, []byte(bcOriginal), []byte(moreTriggers), "", false)
	var ee *EditError
	if !errors.As(err, &ee) || !ee.Redacted || ee.Code != reasonEditCredential || ee.Params["field"] != "spec.triggers[2].github.secret" {
		t.Fatalf("a changed trigger list with a placeholder: %v", err)
	}
	withAnnotation := strings.Replace(bcOriginal, "  name: api\n", "  name: api\n  annotations:\n    "+lastAppliedAnnotation+": <redacted>\n", 1)
	if _, err := rt.EditObject(ctx, bc, []byte(bcOriginal), []byte(withAnnotation), "", false); !errors.As(err, &ee) || ee.Code != reasonEditCredentialAnnotation {
		t.Fatalf("a placeholder last-applied annotation: %v", err)
	}
}

func TestHoldsCredentials(t *testing.T) {
	for gr, want := range map[schema.GroupResource]bool{
		{Resource: "secrets"}:                                   true,
		{Group: "route.openshift.io", Resource: "routes"}:       true,
		{Group: "build.openshift.io", Resource: "buildconfigs"}: true,
		{Group: "build.openshift.io", Resource: "builds"}:       true,
		{Group: "config.openshift.io", Resource: "builds"}:      false,
		{Resource: "configmaps"}:                                false,
	} {
		if got := HoldsCredentials(gr); got != want {
			t.Errorf("HoldsCredentials(%v) = %v", gr, got)
		}
	}
}

func TestRedactManifestObject(t *testing.T) {
	route := credentialObjects()[0].Object
	if !RedactManifestObject(route) {
		t.Fatal("a Route holds credentials")
	}
	if key, _, _ := unstructured.NestedString(route, "spec", "tls", "key"); key != "<87 bytes, redacted>" {
		t.Fatalf("key = %q", key)
	}
	other := map[string]any{"apiVersion": "example.com/v1", "kind": "Route", "spec": map[string]any{"tls": map[string]any{"key": "kept"}}}
	if RedactManifestObject(other) {
		t.Fatal("a Route of another group is not OpenShift's")
	}
}
