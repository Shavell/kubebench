package storage

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8stesting "k8s.io/client-go/testing"

	"kubebench/internal/plugin"
)

func TestResizePVC(t *testing.T) {
	e := newEnv(t, envOptions{},
		class("fast", "hostpath.csi.k8s.io", true, false),
		class("fixed", "rancher.io/local-path", false, true),
		claim("apps", "data", "fast", "1Gi", corev1.ClaimBound),
		claim("apps", "local", "fixed", "1Gi", corev1.ClaimBound),
		claim("apps", "pending", "fast", "1Gi", corev1.ClaimPending),
		claim("apps", "static", "", "1Gi", corev1.ClaimBound),
		claim("apps", "orphan", "gone", "1Gi", corev1.ClaimBound),
	)
	resize := func(name, size string) *httptest.ResponseRecorder {
		return e.do(http.MethodPatch, "/pvcs/apps/"+name+"/resize", map[string]string{"size": size})
	}

	for _, tc := range []struct {
		name, size string
		status     int
		code, want string
	}{
		{"data", "1Gi", http.StatusBadRequest, plugin.CodeBadRequest, "cannot shrink"},
		{"data", "512Mi", http.StatusBadRequest, plugin.CodeBadRequest, "cannot shrink"},
		{"data", "lots", http.StatusBadRequest, plugin.CodeBadRequest, "size:"},
		{"local", "2Gi", http.StatusConflict, plugin.CodeConflict, "does not set allowVolumeExpansion"},
		{"pending", "2Gi", http.StatusConflict, plugin.CodeConflict, "only a Bound claim"},
		{"static", "2Gi", http.StatusConflict, plugin.CodeConflict, "has no StorageClass"},
		{"orphan", "2Gi", http.StatusConflict, plugin.CodeConflict, "does not exist"},
		{"missing", "2Gi", http.StatusNotFound, plugin.CodeKubernetes, "not found"},
	} {
		msg := decode[plugin.ErrorBody](t, expect(t, resize(tc.name, tc.size), tc.status, tc.code)).Error
		if !strings.Contains(msg, tc.want) {
			t.Errorf("%s to %s: %q does not contain %q", tc.name, tc.size, msg, tc.want)
		}
	}
	for _, a := range e.cs.Actions() {
		if a.GetVerb() == "patch" {
			t.Fatalf("a refused resize patched: %v", a)
		}
	}
	if len(e.audits) != 0 {
		t.Fatalf("refused resizes audited as done: %v", e.audits)
	}

	got := decode[PVCSummary](t, expect(t, resize("data", "3Gi"), http.StatusOK, ""))
	if got.Requested != "3Gi" || got.Name != "data" {
		t.Fatalf("summary = %+v", got)
	}
	var patch k8stesting.PatchActionImpl
	for _, a := range e.cs.Actions() {
		if p, ok := a.(k8stesting.PatchActionImpl); ok {
			patch = p
		}
	}
	if !strings.Contains(string(patch.GetPatch()), `"resourceVersion":"7"`) {
		t.Fatalf("patch %s does not guard the resourceVersion", patch.GetPatch())
	}
	pvc, _ := e.cs.CoreV1().PersistentVolumeClaims("apps").Get(context.Background(), "data", metav1.GetOptions{})
	if q := requested(pvc); q.String() != "3Gi" {
		t.Fatalf("request = %s", q.String())
	}
	if len(e.audits) != 1 || e.audits[0] != `storage.pvc.resize pvc/apps/data {"from":"1Gi","to":"3Gi"}` {
		t.Fatalf("audit = %v", e.audits)
	}
}

func TestExpandBlocker(t *testing.T) {
	classes := map[string]bool{"fast": true, "fixed": false}
	for _, tc := range []struct {
		pvc  *corev1.PersistentVolumeClaim
		want string
	}{
		{claim("a", "b", "fast", "1Gi", corev1.ClaimBound), ""},
		{claim("a", "b", "fast", "1Gi", corev1.ClaimPending), blockerNotBound},
		{claim("a", "b", "-", "1Gi", corev1.ClaimBound), blockerNoClass},
		{claim("a", "b", "fixed", "1Gi", corev1.ClaimBound), blockerNotAllowed},
		{claim("a", "b", "other", "1Gi", corev1.ClaimBound), blockerClassMissing},
	} {
		if got := expandBlocker(tc.pvc, classes); got != tc.want {
			t.Errorf("expandBlocker(%v) = %q, want %q", tc.pvc.Spec.StorageClassName, got, tc.want)
		}
	}
}
