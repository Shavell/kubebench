package kube

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"

	authv1 "k8s.io/api/authorization/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"

	"kubebench/internal/store"
)

const tenantKubeconfig = `apiVersion: v1
kind: Config
current-context: alice
contexts:
- name: alice
  context: {cluster: c, user: alice, namespace: team-a}
- name: plain
  context: {cluster: c, user: alice}
clusters:
- name: c
  cluster: {server: "https://example.invalid"}
users:
- name: alice
  user: {token: t}
`

func TestContextNamespace(t *testing.T) {
	c := store.Cluster{Source: store.SourceKubeconfig, Kubeconfig: tenantKubeconfig}
	if ns := ContextNamespace(&c); ns != "team-a" {
		t.Fatalf("current context: %q", ns)
	}
	c.Context = "plain"
	if ns := ContextNamespace(&c); ns != "" {
		t.Fatalf("context without namespace: %q", ns)
	}
	if ns := ContextNamespace(&store.Cluster{Source: store.SourceKubeconfig, Kubeconfig: "{"}); ns != "" {
		t.Fatalf("unparseable: %q", ns)
	}
	if ns := ContextNamespace(&store.Cluster{Source: store.SourceAddress}); ns != "" {
		t.Fatalf("address: %q", ns)
	}
}

// tenantClientset denies everything cluster-wide (listing namespaces,
// SSARs without a namespace) and allows the SSARs in namespaces allowed.
func tenantClientset(allowedNS ...string) *fake.Clientset {
	cs := fake.NewClientset()
	cs.PrependReactor("list", "namespaces", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, apierrors.NewForbidden(schema.GroupResource{Resource: "namespaces"}, "", errors.New("RBAC"))
	})
	cs.PrependReactor("create", "selfsubjectaccessreviews", func(a k8stesting.Action) (bool, runtime.Object, error) {
		review := a.(k8stesting.CreateAction).GetObject().(*authv1.SelfSubjectAccessReview)
		review.Status.Allowed = slices.Contains(allowedNS, review.Spec.ResourceAttributes.Namespace)
		return true, review, nil
	})
	return cs
}

func TestProbeNamespacesTenant(t *testing.T) {
	c := store.Cluster{Source: store.SourceKubeconfig, Kubeconfig: tenantKubeconfig}
	var caps Capabilities
	probeNamespaces(context.Background(), tenantClientset("team-a"), &c, false, &caps)
	if caps.ListNamespaces || !slices.Equal(caps.Namespaces, []string{"team-a"}) || caps.DefaultNamespace != "team-a" {
		t.Fatalf("namespace access = %+v", caps)
	}
	if !caps.Exec || !slices.Equal(caps.ExecNamespaces, []string{"team-a"}) ||
		!caps.Helm || !slices.Equal(caps.HelmNamespaces, []string{"team-a"}) {
		t.Fatalf("exec/helm = %+v", caps)
	}

	// The admin's allowed namespaces replace the detected one; the default
	// stays the context's namespace, which is probed too.
	c.AllowedNamespaces = store.JoinNamespaces([]string{"team-b", "team-c"})
	caps = Capabilities{}
	probeNamespaces(context.Background(), tenantClientset("team-a", "team-c"), &c, false, &caps)
	if !slices.Equal(caps.Namespaces, []string{"team-b", "team-c"}) || caps.DefaultNamespace != "team-a" ||
		!slices.Equal(caps.ExecNamespaces, []string{"team-c", "team-a"}) {
		t.Fatalf("with allowed namespaces = %+v", caps)
	}

	// Without a context namespace the first allowed one is the default.
	c.Context = "plain"
	caps = Capabilities{}
	probeNamespaces(context.Background(), tenantClientset(), &c, false, &caps)
	if caps.DefaultNamespace != "team-b" || caps.Exec || caps.Helm || caps.ExecNamespaces != nil {
		t.Fatalf("no access = %+v", caps)
	}
}

func TestProbeNamespacesClusterWide(t *testing.T) {
	cs := fake.NewClientset()
	cs.PrependReactor("create", "selfsubjectaccessreviews", func(a k8stesting.Action) (bool, runtime.Object, error) {
		review := a.(k8stesting.CreateAction).GetObject().(*authv1.SelfSubjectAccessReview)
		review.Status.Allowed = true
		return true, review, nil
	})
	c := store.Cluster{Source: store.SourceKubeconfig, Kubeconfig: tenantKubeconfig, Context: "plain"}
	var caps Capabilities
	probeNamespaces(context.Background(), cs, &c, true, &caps)
	if !caps.ListNamespaces || caps.DefaultNamespace != "" || !caps.Exec || !caps.Helm ||
		caps.ExecNamespaces != nil || caps.HelmNamespaces != nil {
		t.Fatalf("cluster-wide = %+v", caps)
	}
}

func TestDetectedNamespacesInCluster(t *testing.T) {
	file := filepath.Join(t.TempDir(), "namespace")
	if err := os.WriteFile(file, []byte("kubebench\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	old := inClusterNamespaceFile
	inClusterNamespaceFile = file
	t.Cleanup(func() { inClusterNamespaceFile = old })
	got := candidateNamespaces(&store.Cluster{Source: store.SourceInCluster})
	if !slices.Equal(got, []string{"kubebench"}) {
		t.Fatalf("in cluster = %v", got)
	}
}

func TestCandidateNamespacesBounded(t *testing.T) {
	var names []string
	for i := range MaxProbeNamespaces + 5 {
		names = append(names, "ns"+string(rune('a'+i)))
	}
	got := candidateNamespaces(&store.Cluster{AllowedNamespaces: store.JoinNamespaces(names)})
	if len(got) != MaxProbeNamespaces {
		t.Fatalf("len = %d", len(got))
	}
}
