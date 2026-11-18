package api

import (
	"net/http"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/runtime/schema"

	"kubebench/internal/kube/kubetest"
	"kubebench/internal/plugin"
)

var routeKind = kubetest.Kind{GVK: schema.GroupVersionKind{Group: "route.openshift.io", Version: "v1", Kind: "Route"}, Resource: "routes", Namespaced: true}

const tlsRoute = "apiVersion: route.openshift.io/v1\nkind: Route\nmetadata: {name: web, namespace: shop}\nspec:\n  tls: {termination: edge, key: GUESSED-KEY}\n"

// A Route's TLS key is redacted for viewers, so a dry run that would tell
// a guess from the real key needs write access, like a Secret's, and no
// write may store the placeholder in its place.
func TestRouteCredentialsAreGuardedLikeSecretValues(t *testing.T) {
	route := kubetest.Object("route.openshift.io/v1", "Route", "shop", "web", map[string]any{
		"spec": map[string]any{"tls": map[string]any{"termination": "edge", "key": "REAL-KEY"}},
	})
	e, dyn := editingEnvWith(t, append(append([]kubetest.Kind{}, kubetest.Kinds...), routeKind), route)

	before := len(dynamicMutations(dyn))
	expect(t, sendText(e, http.MethodPost, "/api/resource/apply?dryRun=true", "bob", tlsRoute), http.StatusForbidden, plugin.CodeForbiddenRole)
	target := "/api/resource/edit?apiVersion=route.openshift.io/v1&kind=Route&ns=shop&name=web"
	original := strings.Replace(tlsRoute, "GUESSED-KEY", "'<8 bytes, redacted>'", 1)
	guess := editRequest{Original: original, Edited: tlsRoute}
	expect(t, e.do(http.MethodPost, target+"&dryRun=true", "bob", guess), http.StatusForbidden, plugin.CodeForbiddenRole)

	placeholder := strings.Replace(tlsRoute, "GUESSED-KEY", "'<8 bytes, redacted>'", 1)
	expect(t, sendText(e, http.MethodPost, "/api/resource/apply", "alice", placeholder), http.StatusBadRequest, plugin.CodeRedactedSecret)
	redacted := map[string]any{"annotations": map[string]any{"kubectl.kubernetes.io/last-applied-configuration": "<redacted>"}}
	expect(t, e.do(http.MethodPatch, "/api/resource/metadata?key=routes&ns=shop&name=web", "alice", redacted),
		http.StatusBadRequest, plugin.CodeRedactedSecret)
	if n := len(dynamicMutations(dyn)); n != before {
		t.Fatalf("%d refused requests reached the cluster", n-before)
	}

	rec := e.do(http.MethodGet, "/api/resource/get?key=routes&ns=shop&name=web", "bob", nil)
	expect(t, rec, http.StatusOK, "")
	if body := rec.Body.String(); strings.Contains(body, "REAL-KEY") || !strings.Contains(body, "bytes, redacted") {
		t.Fatalf("a viewer reads the Route as %s", body)
	}
}
