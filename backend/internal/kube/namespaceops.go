package kube

import (
	"context"
	"slices"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	metav1validation "k8s.io/apimachinery/pkg/apis/meta/v1/validation"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"kubebench/internal/reason"
)

// SystemNamespaces are the namespaces Kubernetes itself creates and needs.
var SystemNamespaces = []string{
	metav1.NamespaceDefault, metav1.NamespaceSystem, metav1.NamespacePublic, corev1.NamespaceNodeLease,
}

// CreateNamespace creates a namespace with optional labels. The name must
// be a DNS-1123 label.
func (r *Runtime) CreateNamespace(ctx context.Context, name string, labels map[string]string) error {
	if problems := validation.IsDNS1123Label(name); len(problems) > 0 {
		return opError(OpInvalid, reasonNamespaceName, reason.Params{"name": name, "detail": strings.Join(problems, "; ")})
	}
	if errs := metav1validation.ValidateLabels(labels, field.NewPath("labels")); len(errs) > 0 {
		return opError(OpInvalid, reasonNamespaceLabels, reason.Params{"detail": errs.ToAggregate().Error()})
	}
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: name, Labels: labels}}
	_, err := r.CS.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{FieldManager: FieldManager})
	return err
}

// DeleteNamespace deletes a namespace and everything in it. A system
// namespace and own, the namespace KubeBench runs in ("" when unknown), are
// refused unless force is set: deleting them breaks the cluster or
// KubeBench itself.
func (r *Runtime) DeleteNamespace(ctx context.Context, name, own string, force bool) error {
	if name == "" {
		return opError(OpInvalid, reasonNamespaceRequired, nil)
	}
	if !force {
		if slices.Contains(SystemNamespaces, name) {
			return opError(OpProtected, reasonNamespaceSystem, reason.Params{"name": name})
		}
		if name == own {
			return opError(OpProtected, reasonNamespaceOwn, reason.Params{"name": name})
		}
	}
	return r.CS.CoreV1().Namespaces().Delete(ctx, name, metav1.DeleteOptions{})
}
