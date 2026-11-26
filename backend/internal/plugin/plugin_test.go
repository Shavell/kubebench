package plugin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"kubebench/internal/kube"
)

type stub struct{ id string }

func (s stub) Manifest() Manifest                         { return Manifest{ID: s.id} }
func (stub) Routes(Router)                                {}
func (stub) Detect(context.Context, *kube.Runtime) Status { return Status{Available: true} }

func TestRegistry(t *testing.T) {
	Register(stub{"zeta"})
	Register(stub{"alpha"})
	if p, ok := Get("alpha"); !ok || p.Manifest().ID != "alpha" {
		t.Fatal("Get(alpha) failed")
	}
	if _, ok := Get("missing"); ok {
		t.Fatal("Get(missing) succeeded")
	}
	all := All()
	if len(all) != 2 || all[0].Manifest().ID != "alpha" || all[1].Manifest().ID != "zeta" {
		t.Fatalf("All() = %v, want alpha, zeta", all)
	}
	for _, id := range []string{"alpha", "Bad_ID", ""} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("Register(%q) must panic", id)
				}
			}()
			Register(stub{id})
		}()
	}
}

func TestValidID(t *testing.T) {
	for id, want := range map[string]bool{
		"argocd": true, "hello-cluster": true, "a": true, "0x": true,
		"": false, "-lead": false, "Upper": false, "under_score": false, "dots.no": false,
		strings.Repeat("a", 40): true,
		strings.Repeat("a", 41): false,
	} {
		if got := ValidID(id); got != want {
			t.Errorf("ValidID(%q) = %v", id, got)
		}
	}
}

func TestAccessFlags(t *testing.T) {
	a := AccessWrite | DryRunnable
	if a.Level() != AccessWrite || !a.IsDryRunnable() || AccessWrite.IsDryRunnable() {
		t.Fatal("DryRunnable must combine with an access level")
	}
	r := httptest.NewRequest(http.MethodPost, "/x?dryRun=true", nil)
	if !IsDryRun(r) || IsDryRun(httptest.NewRequest(http.MethodPost, "/x?dryRun=1", nil)) {
		t.Fatal("only dryRun=true is a dry run")
	}
}

func errorOf(t *testing.T, write func(http.ResponseWriter)) (int, ErrorBody) {
	t.Helper()
	rec := httptest.NewRecorder()
	write(rec)
	var body ErrorBody
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body %q: %v", rec.Body.String(), err)
	}
	return rec.Code, body
}

func TestWriteK8sError(t *testing.T) {
	gr := schema.GroupResource{Resource: "pods"}
	for _, tc := range []struct {
		name   string
		err    error
		status int
		code   string
	}{
		{"not found", apierrors.NewNotFound(gr, "web"), http.StatusNotFound, CodeKubernetes},
		{"rbac", apierrors.NewForbidden(gr, "web", errors.New("no")), http.StatusForbidden, CodeKubernetes},
		{"wrapped", fmt.Errorf("listing: %w", apierrors.NewConflict(gr, "web", errors.New("x"))), http.StatusConflict, CodeKubernetes},
		{"rejected credential", apierrors.NewUnauthorized("token expired"), http.StatusBadGateway, CodeKubernetes},
		{"unknown kind", &meta.NoKindMatchError{GroupKind: schema.GroupKind{Kind: "Nope"}}, http.StatusNotFound, CodeNotFound},
		{"timeout", fmt.Errorf("get: %w", context.DeadlineExceeded), http.StatusGatewayTimeout, CodeKubernetes},
		{"unreachable", errors.New("dial tcp: connection refused"), http.StatusBadGateway, CodeKubernetes},
	} {
		status, body := errorOf(t, func(w http.ResponseWriter) { WriteK8sError(w, tc.err) })
		if status != tc.status || body.Code != tc.code || body.Error != tc.err.Error() {
			t.Errorf("%s: %d %+v, want %d %s", tc.name, status, body, tc.status, tc.code)
		}
	}
}

// A 403 says what the credential may not do (§7.16).
func TestWriteK8sErrorForbidden(t *testing.T) {
	denial := func(scope string) error {
		return &apierrors.StatusError{ErrStatus: metav1.Status{
			Status: metav1.StatusFailure, Code: http.StatusForbidden, Reason: metav1.StatusReasonForbidden,
			Message: `deployments.apps is forbidden: User "system:serviceaccount:team-a:alice" cannot list resource "deployments" in API group "apps" ` + scope,
		}}
	}
	for _, tc := range []struct {
		name, reason, text string
		err                error
	}{
		{"namespace", "kubernetes.forbidden_namespace",
			`no permission to list deployments.apps in namespace "default" (Kubernetes RBAC of system:serviceaccount:team-a:alice)`,
			denial(`in the namespace "default"`)},
		{"cluster", "kubernetes.forbidden_cluster",
			`no permission to list deployments.apps at the cluster scope (Kubernetes RBAC of system:serviceaccount:team-a:alice)`,
			denial("at the cluster scope")},
		{"other wording", "kubernetes.forbidden", `pods "web" is forbidden: no`,
			apierrors.NewForbidden(schema.GroupResource{Resource: "pods"}, "web", errors.New("no"))},
	} {
		status, body := errorOf(t, func(w http.ResponseWriter) { WriteK8sError(w, tc.err) })
		if status != http.StatusForbidden || body.Code != CodeKubernetes || body.Reason != tc.reason || body.Error != tc.text {
			t.Errorf("%s: %d %+v", tc.name, status, body)
		}
	}
	if body := ForbiddenReason(denial("at the cluster scope")); !apierrors.IsForbidden(body) {
		t.Fatalf("the cause is kept: %v", body)
	}
}

func TestWriteJSONAndError(t *testing.T) {
	rec := httptest.NewRecorder()
	WriteJSON(rec, map[string]int{"n": 1})
	if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != "application/json" || rec.Body.String() != "{\"n\":1}\n" {
		t.Fatalf("WriteJSON: %d %q", rec.Code, rec.Body.String())
	}
	// An unencodable value is a clean 500, not a truncated 200.
	status, body := errorOf(t, func(w http.ResponseWriter) { WriteJSON(w, math.NaN()) })
	if status != http.StatusInternalServerError || body.Code != CodeInternal {
		t.Fatalf("NaN: %d %+v", status, body)
	}
	status, body = errorOf(t, func(w http.ResponseWriter) { WriteError(w, http.StatusTeapot, "x", nil) })
	if status != http.StatusTeapot || body.Error != "I'm a teapot" || body.Code != "x" {
		t.Fatalf("nil error: %d %+v", status, body)
	}
}
