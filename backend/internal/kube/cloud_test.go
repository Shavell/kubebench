package kube

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/version"
	fakediscovery "k8s.io/client-go/discovery/fake"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"

	"kubebench/internal/cloud"
	"kubebench/internal/store"
)

func TestDetectCloud(t *testing.T) {
	cs := fake.NewClientset(&corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: "ip-10-0-1-2", Labels: map[string]string{"eks.amazonaws.com/nodegroup": "default"}},
		Spec:       corev1.NodeSpec{ProviderID: "aws:///eu-west-1a/i-0abc"},
	})
	cs.Discovery().(*fakediscovery.FakeDiscovery).FakedServerVersion = &version.Info{GitVersion: "v1.34.2-eks-b8e4e3a"}
	rt := NewRuntimeForTest(store.Cluster{ID: 1, Name: "eks"}, cs, nil)

	if d, ok := rt.Cloud(); !ok || d.Provider != cloud.ProviderKubernetes {
		t.Fatalf("before detection = %+v, %v", d, ok)
	}
	rt.DetectCloudForTest()
	d, ok := rt.Cloud()
	if !ok || d != (cloud.Detection{Provider: cloud.ProviderEKS, Cloud: cloud.CloudAWS, Version: "v1.34.2-eks-b8e4e3a"}) {
		t.Fatalf("detection = %+v, %v", d, ok)
	}
}

// A fresh runtime reports no detection until the first one finished.
func TestCloudPending(t *testing.T) {
	rt := assemble(store.Cluster{ID: 1}, &rest.Config{Host: "https://10.0.0.1:6443"}, fake.NewClientset(), nil, nil)
	if _, ok := rt.Cloud(); ok {
		t.Fatal("detection reported before it ran")
	}
	go rt.detectCloudOnce()
	if err := rt.WaitCloud(t.Context()); err != nil {
		t.Fatal(err)
	}
	if d, ok := rt.Cloud(); !ok || d.Provider != cloud.ProviderKubernetes {
		t.Fatalf("detection = %+v, %v", d, ok)
	}
}

const eksKubeconfig = `apiVersion: v1
kind: Config
clusters:
- name: arn:aws:eks:eu-west-1:123456789012:cluster/prod
  cluster:
    server: https://ABCDEF.gr7.eu-west-1.eks.amazonaws.com
contexts:
- name: prod
  context:
    cluster: arn:aws:eks:eu-west-1:123456789012:cluster/prod
    user: arn:aws:eks:eu-west-1:123456789012:cluster/prod
current-context: prod
users:
- name: arn:aws:eks:eu-west-1:123456789012:cluster/prod
  user:
    exec:
      apiVersion: client.authentication.k8s.io/v1beta1
      command: aws
      args: [--region, eu-west-1, eks, get-token, --cluster-name, prod, --output, json]
`

// The cloud's exec plugin is replaced by an in-process token provider:
// client-go gets no exec provider to run.
func TestRestConfigForCloudKubeconfig(t *testing.T) {
	config, err := restConfigFor(&store.Cluster{Source: store.SourceKubeconfig, Kubeconfig: eksKubeconfig})
	if err != nil {
		t.Fatal(err)
	}
	if config.ExecProvider != nil || config.WrapTransport == nil {
		t.Fatalf("exec provider %v, wrap transport set %v", config.ExecProvider, config.WrapTransport != nil)
	}

	// Credentials of another cloud are refused.
	if _, err := restConfigFor(&store.Cluster{Source: store.SourceKubeconfig, Kubeconfig: eksKubeconfig,
		Credentials: `{"provider":"gcp","serviceAccountJson":"{}"}`}); err == nil {
		t.Fatal("GCP credentials accepted for an EKS kubeconfig")
	}
}
