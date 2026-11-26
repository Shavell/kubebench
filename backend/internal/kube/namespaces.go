package kube

import (
	"context"
	"os"
	"strings"

	authv1 "k8s.io/api/authorization/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	"kubebench/internal/store"
)

// Namespace-scoped credentials (docs/design-v0.5.md §7.16). A credential
// that may only use some namespaces (a tenant ServiceAccount bound to
// "edit" in one namespace) cannot list namespaces or use anything
// cluster-wide. The capability probe then looks at a bounded set of
// namespaces instead: the cluster's allowed namespaces (an admin setting),
// else the ones KubeBench can tell from the credential itself.

// MaxProbeNamespaces bounds the namespaces the capability probe checks one
// by one, and so the allowed namespaces an admin can set.
const MaxProbeNamespaces = 20

// inClusterNamespaceFile holds the namespace of KubeBench's own
// ServiceAccount when it runs in the cluster; a variable for tests.
var inClusterNamespaceFile = "/var/run/secrets/kubernetes.io/serviceaccount/namespace"

// ContextNamespace is the namespace the kubeconfig context of a cluster
// sets (as kubectl would use it), "" when it sets none or the cluster has
// no kubeconfig.
func ContextNamespace(c *store.Cluster) string {
	if c.Source != store.SourceKubeconfig || c.Kubeconfig == "" {
		return ""
	}
	kc, err := clientcmd.Load([]byte(c.Kubeconfig))
	if err != nil {
		return ""
	}
	name := c.Context
	if name == "" {
		name = kc.CurrentContext
	}
	if ctx, ok := kc.Contexts[name]; ok {
		return ctx.Namespace
	}
	return ""
}

// detectedNamespaces are the namespaces the credential evidently belongs
// to: the kubeconfig context's namespace, or KubeBench's own namespace when
// it runs in the cluster with its ServiceAccount.
func detectedNamespaces(c *store.Cluster) []string {
	if ns := ContextNamespace(c); ns != "" {
		return []string{ns}
	}
	if c.Source == store.SourceInCluster {
		if data, err := os.ReadFile(inClusterNamespaceFile); err == nil {
			if ns := strings.TrimSpace(string(data)); ns != "" {
				return []string{ns}
			}
		}
	}
	return nil
}

// candidateNamespaces are the namespaces the UI offers when the credential
// cannot list them: the allowed namespaces when set, else the detected
// ones; at most MaxProbeNamespaces, never nil.
func candidateNamespaces(c *store.Cluster) []string {
	names := c.Namespaces()
	if len(names) == 0 {
		names = detectedNamespaces(c)
	}
	if names == nil {
		names = []string{}
	}
	if len(names) > MaxProbeNamespaces {
		names = names[:MaxProbeNamespaces]
	}
	return names
}

// probeNamespaces fills the namespace part of caps: whether the credential
// may list namespaces, the namespaces to offer without that, the default
// namespace, and, for exec and Helm when they are not allowed cluster-wide,
// the namespaces where they are. helmClusterWide is the result of the
// cluster-wide Helm probe.
func probeNamespaces(ctx context.Context, cs kubernetes.Interface, c *store.Cluster, helmClusterWide bool, caps *Capabilities) {
	_, err := cs.CoreV1().Namespaces().List(ctx, metav1.ListOptions{Limit: 1})
	caps.ListNamespaces = err == nil
	caps.Namespaces = candidateNamespaces(c)
	caps.DefaultNamespace = ContextNamespace(c)
	if caps.DefaultNamespace == "" && !caps.ListNamespaces && len(caps.Namespaces) > 0 {
		caps.DefaultNamespace = caps.Namespaces[0]
	}
	// The namespaces worth checking one by one: those the UI offers, plus
	// the default namespace (when the admin's list leaves it out).
	probe := caps.Namespaces
	if caps.DefaultNamespace != "" && !contains(probe, caps.DefaultNamespace) && len(probe) < MaxProbeNamespaces {
		probe = append(append([]string{}, probe...), caps.DefaultNamespace)
	}

	exec := authv1.ResourceAttributes{Verb: "create", Resource: "pods", Subresource: "exec"}
	if allowed(ctx, cs, exec, "") {
		caps.Exec = true
	} else if names := allowedIn(ctx, cs, exec, probe); len(names) > 0 {
		caps.Exec, caps.ExecNamespaces = true, names
	}
	// Helm keeps releases in Secrets labelled owner=helm.
	if helmClusterWide {
		caps.Helm = true
	} else if names := allowedIn(ctx, cs, authv1.ResourceAttributes{Verb: "list", Resource: "secrets"}, probe); len(names) > 0 {
		caps.Helm, caps.HelmNamespaces = true, names
	}
}

// allowedIn returns the namespaces of names in which the credential may
// do what attrs describe.
func allowedIn(ctx context.Context, cs kubernetes.Interface, attrs authv1.ResourceAttributes, names []string) []string {
	var out []string
	for _, ns := range names {
		if allowed(ctx, cs, attrs, ns) {
			out = append(out, ns)
		}
	}
	return out
}

// allowed asks the API server (SelfSubjectAccessReview) whether the
// credential may do what attrs describe in namespace ("" = in all
// namespaces, or cluster-wide for cluster-scoped resources).
func allowed(ctx context.Context, cs kubernetes.Interface, attrs authv1.ResourceAttributes, namespace string) bool {
	attrs.Namespace = namespace
	review := &authv1.SelfSubjectAccessReview{Spec: authv1.SelfSubjectAccessReviewSpec{ResourceAttributes: &attrs}}
	res, err := cs.AuthorizationV1().SelfSubjectAccessReviews().Create(ctx, review, metav1.CreateOptions{})
	return err == nil && res.Status.Allowed
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
