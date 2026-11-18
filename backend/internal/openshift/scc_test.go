package openshift

import (
	"maps"
	"testing"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	k8stesting "k8s.io/client-go/testing"
)

func TestSCCLevel(t *testing.T) {
	anyUser := func(s *SCC) { s.RunAsUser.Type = "RunAsAny" }
	tests := []struct {
		name   string
		mutate func(*SCC)
		want   string
	}{
		{"restricted-v2", func(s *SCC) { s.RunAsUser.Type = "MustRunAsRange" }, LevelRestricted},
		{"nonroot", func(s *SCC) { s.RunAsUser.Type = "MustRunAsNonRoot" }, LevelRestricted},
		{"anyuid", anyUser, LevelAnyUID},
		{"privileged", func(s *SCC) { s.AllowPrivilegedContainer = true }, LevelPrivileged},
		{"hostnetwork", func(s *SCC) { s.AllowHostNetwork = true }, LevelPrivileged},
		{"hostaccess by volume", func(s *SCC) { s.Volumes = []string{"configMap", "hostPath"} }, LevelPrivileged},
		{"any volume", func(s *SCC) { s.Volumes = []string{"*"} }, LevelPrivileged},
		{"any capability", func(s *SCC) { s.AllowedCapabilities = []string{"*"} }, LevelPrivileged},
	}
	for _, tt := range tests {
		s := &SCC{}
		tt.mutate(s)
		if got := s.Level(); got != tt.want {
			t.Errorf("%s: level %s, want %s", tt.name, got, tt.want)
		}
	}
}

func TestSCCDefault(t *testing.T) {
	shipped := SCC{ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{"include.release.openshift.io/single-node-developer": "true"}}}
	owned := SCC{ObjectMeta: metav1.ObjectMeta{OwnerReferences: []metav1.OwnerReference{{APIVersion: "config.openshift.io/v1", Kind: "ClusterVersion", Name: "version"}}}}
	operator := SCC{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app.kubernetes.io/part-of": "openshift-monitoring"}}}
	if !operator.Default() {
		t.Fatal("an SCC of a platform operator is not a default one")
	}
	custom := SCC{ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{"kubernetes.io/description": "mine"}}}
	if !shipped.Default() || !owned.Default() || custom.Default() {
		t.Fatalf("default: shipped %v, owned %v, custom %v", shipped.Default(), owned.Default(), custom.Default())
	}
}

func TestRiskyLevels(t *testing.T) {
	got := RiskyLevels(nil)
	if got["privileged"] != LevelPrivileged || got["anyuid"] != LevelAnyUID || got["restricted-v2"] != "" {
		t.Fatalf("fallback = %v", got)
	}
	sccs := []SCC{{ObjectMeta: metav1.ObjectMeta{Name: "restricted-v2"}}, {ObjectMeta: metav1.ObjectMeta{Name: "mine"}, AllowHostPID: true}}
	if got := RiskyLevels(sccs); !maps.Equal(got, map[string]string{"mine": LevelPrivileged}) {
		t.Fatalf("risky = %v", got)
	}
}

func TestListSCCs(t *testing.T) {
	dyn := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(),
		map[schema.GroupVersionResource]string{SCCGVR: "SecurityContextConstraintsList"})
	obj := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "security.openshift.io/v1", "kind": "SecurityContextConstraints",
		"metadata": map[string]any{"name": "privileged"}, "allowPrivilegedContainer": true,
		"users": []any{"system:admin"}, "runAsUser": map[string]any{"type": "RunAsAny"},
	}}
	if _, err := dyn.Resource(SCCGVR).Create(t.Context(), obj, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	sccs, served, err := ListSCCs(t.Context(), dyn)
	if err != nil || !served || len(sccs) != 1 || !sccs[0].AllowPrivilegedContainer || sccs[0].Users[0] != "system:admin" {
		t.Fatalf("ListSCCs = %+v, %v, %v", sccs, served, err)
	}

	dyn.PrependReactor("list", SCCGVR.Resource, func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, apierrors.NewNotFound(SCCGVR.GroupResource(), "")
	})
	if _, served, err := ListSCCs(t.Context(), dyn); served || err != nil {
		t.Fatalf("not served: %v, %v", served, err)
	}
}
