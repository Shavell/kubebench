package security

import (
	"context"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"kubebench/internal/reason"
)

// Limits that bound the cost of one scan on a large cluster. Lists are
// read in pages; a kind with more objects than maxPerKind is cut there and
// the report says so.
const (
	pageSize   = 500
	maxPerKind = 5000
)

// Inventory kinds, as the report's inventory names them.
const (
	kindNamespaces          = "namespaces"
	kindPods                = "pods"
	kindDeployments         = "deployments"
	kindStatefulSets        = "statefulsets"
	kindDaemonSets          = "daemonsets"
	kindReplicaSets         = "replicasets"
	kindJobs                = "jobs"
	kindCronJobs            = "cronjobs"
	kindServiceAccounts     = "serviceaccounts"
	kindRoles               = "roles"
	kindClusterRoles        = "clusterroles"
	kindRoleBindings        = "rolebindings"
	kindClusterRoleBindings = "clusterrolebindings"
	kindNetworkPolicies     = "networkpolicies"
)

// workloadKinds are the kinds whose pod templates the workload checks read.
var workloadKinds = []string{kindPods, kindDeployments, kindStatefulSets, kindDaemonSets, kindReplicaSets, kindJobs, kindCronJobs}

// inventory is everything a scan reads from the cluster, listed once.
type inventory struct {
	namespaces          []corev1.Namespace
	pods                []corev1.Pod
	deployments         []appsv1.Deployment
	statefulSets        []appsv1.StatefulSet
	daemonSets          []appsv1.DaemonSet
	replicaSets         []appsv1.ReplicaSet
	jobs                []batchv1.Job
	cronJobs            []batchv1.CronJob
	serviceAccounts     []corev1.ServiceAccount
	roles               []rbacv1.Role
	clusterRoles        []rbacv1.ClusterRole
	roleBindings        []rbacv1.RoleBinding
	clusterRoleBindings []rbacv1.ClusterRoleBinding
	networkPolicies     []networkingv1.NetworkPolicy

	// errs holds the kinds that could not be listed, truncated those cut
	// at maxPerKind.
	errs      map[string]*reason.Error
	truncated map[string]bool
}

// collect lists every kind the checks need, all namespaces at once. A kind
// that cannot be listed (RBAC, a missing API) is recorded in errs and the
// checks that need it are skipped; only an ended context stops it.
func collect(ctx context.Context, cs kubernetes.Interface) (*inventory, error) {
	inv := &inventory{errs: map[string]*reason.Error{}, truncated: map[string]bool{}}
	var err error
	if inv.namespaces, err = listKind(ctx, inv, kindNamespaces, func(o metav1.ListOptions) ([]corev1.Namespace, string, error) {
		l, err := cs.CoreV1().Namespaces().List(ctx, o)
		if err != nil {
			return nil, "", err
		}
		return l.Items, l.Continue, nil
	}); err != nil {
		return nil, err
	}
	if inv.pods, err = listKind(ctx, inv, kindPods, func(o metav1.ListOptions) ([]corev1.Pod, string, error) {
		l, err := cs.CoreV1().Pods("").List(ctx, o)
		if err != nil {
			return nil, "", err
		}
		return l.Items, l.Continue, nil
	}); err != nil {
		return nil, err
	}
	if inv.deployments, err = listKind(ctx, inv, kindDeployments, func(o metav1.ListOptions) ([]appsv1.Deployment, string, error) {
		l, err := cs.AppsV1().Deployments("").List(ctx, o)
		if err != nil {
			return nil, "", err
		}
		return l.Items, l.Continue, nil
	}); err != nil {
		return nil, err
	}
	if inv.statefulSets, err = listKind(ctx, inv, kindStatefulSets, func(o metav1.ListOptions) ([]appsv1.StatefulSet, string, error) {
		l, err := cs.AppsV1().StatefulSets("").List(ctx, o)
		if err != nil {
			return nil, "", err
		}
		return l.Items, l.Continue, nil
	}); err != nil {
		return nil, err
	}
	if inv.daemonSets, err = listKind(ctx, inv, kindDaemonSets, func(o metav1.ListOptions) ([]appsv1.DaemonSet, string, error) {
		l, err := cs.AppsV1().DaemonSets("").List(ctx, o)
		if err != nil {
			return nil, "", err
		}
		return l.Items, l.Continue, nil
	}); err != nil {
		return nil, err
	}
	if inv.replicaSets, err = listKind(ctx, inv, kindReplicaSets, func(o metav1.ListOptions) ([]appsv1.ReplicaSet, string, error) {
		l, err := cs.AppsV1().ReplicaSets("").List(ctx, o)
		if err != nil {
			return nil, "", err
		}
		return l.Items, l.Continue, nil
	}); err != nil {
		return nil, err
	}
	if inv.jobs, err = listKind(ctx, inv, kindJobs, func(o metav1.ListOptions) ([]batchv1.Job, string, error) {
		l, err := cs.BatchV1().Jobs("").List(ctx, o)
		if err != nil {
			return nil, "", err
		}
		return l.Items, l.Continue, nil
	}); err != nil {
		return nil, err
	}
	if inv.cronJobs, err = listKind(ctx, inv, kindCronJobs, func(o metav1.ListOptions) ([]batchv1.CronJob, string, error) {
		l, err := cs.BatchV1().CronJobs("").List(ctx, o)
		if err != nil {
			return nil, "", err
		}
		return l.Items, l.Continue, nil
	}); err != nil {
		return nil, err
	}
	if inv.serviceAccounts, err = listKind(ctx, inv, kindServiceAccounts, func(o metav1.ListOptions) ([]corev1.ServiceAccount, string, error) {
		l, err := cs.CoreV1().ServiceAccounts("").List(ctx, o)
		if err != nil {
			return nil, "", err
		}
		return l.Items, l.Continue, nil
	}); err != nil {
		return nil, err
	}
	if inv.roles, err = listKind(ctx, inv, kindRoles, func(o metav1.ListOptions) ([]rbacv1.Role, string, error) {
		l, err := cs.RbacV1().Roles("").List(ctx, o)
		if err != nil {
			return nil, "", err
		}
		return l.Items, l.Continue, nil
	}); err != nil {
		return nil, err
	}
	if inv.clusterRoles, err = listKind(ctx, inv, kindClusterRoles, func(o metav1.ListOptions) ([]rbacv1.ClusterRole, string, error) {
		l, err := cs.RbacV1().ClusterRoles().List(ctx, o)
		if err != nil {
			return nil, "", err
		}
		return l.Items, l.Continue, nil
	}); err != nil {
		return nil, err
	}
	if inv.roleBindings, err = listKind(ctx, inv, kindRoleBindings, func(o metav1.ListOptions) ([]rbacv1.RoleBinding, string, error) {
		l, err := cs.RbacV1().RoleBindings("").List(ctx, o)
		if err != nil {
			return nil, "", err
		}
		return l.Items, l.Continue, nil
	}); err != nil {
		return nil, err
	}
	if inv.clusterRoleBindings, err = listKind(ctx, inv, kindClusterRoleBindings, func(o metav1.ListOptions) ([]rbacv1.ClusterRoleBinding, string, error) {
		l, err := cs.RbacV1().ClusterRoleBindings().List(ctx, o)
		if err != nil {
			return nil, "", err
		}
		return l.Items, l.Continue, nil
	}); err != nil {
		return nil, err
	}
	if inv.networkPolicies, err = listKind(ctx, inv, kindNetworkPolicies, func(o metav1.ListOptions) ([]networkingv1.NetworkPolicy, string, error) {
		l, err := cs.NetworkingV1().NetworkPolicies("").List(ctx, o)
		if err != nil {
			return nil, "", err
		}
		return l.Items, l.Continue, nil
	}); err != nil {
		return nil, err
	}
	return inv, nil
}

// listKind reads a kind page by page, up to maxPerKind objects. Errors are
// recorded in inv; it returns an error only once ctx has ended, which
// stops the whole scan.
func listKind[T any](ctx context.Context, inv *inventory, kind string, list func(metav1.ListOptions) ([]T, string, error)) ([]T, error) {
	var out []T
	opts := metav1.ListOptions{Limit: pageSize}
	for {
		page, next, err := list(opts)
		if err != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			inv.errs[kind] = reason.Wrap(err, reasonCannotList, reason.Params{"kind": kind, "detail": err.Error()})
			return nil, nil
		}
		out = append(out, page...)
		if len(out) >= maxPerKind {
			if len(out) > maxPerKind || next != "" {
				inv.truncated[kind] = true
			}
			return out[:maxPerKind], nil
		}
		if next == "" {
			return out, nil
		}
		opts.Continue = next
	}
}

// listed reports whether every one of kinds could be listed.
func (inv *inventory) listed(kinds ...string) bool {
	for _, k := range kinds {
		if inv.errs[k] != nil {
			return false
		}
	}
	return true
}

// anyListed reports whether at least one of kinds could be listed.
func (inv *inventory) anyListed(kinds ...string) bool {
	for _, k := range kinds {
		if inv.errs[k] == nil {
			return true
		}
	}
	return false
}

// failure is the reason the first of kinds that failed could not be listed.
func (inv *inventory) failure(kinds ...string) *reason.Error {
	for _, k := range kinds {
		if err := inv.errs[k]; err != nil {
			return err
		}
	}
	return reason.New(reasonNotListed, nil)
}

// counts is the report's inventory: the objects listed, by kind.
func (inv *inventory) counts() map[string]int {
	return map[string]int{
		kindNamespaces: len(inv.namespaces), kindPods: len(inv.pods),
		kindDeployments: len(inv.deployments), kindStatefulSets: len(inv.statefulSets),
		kindDaemonSets: len(inv.daemonSets), kindReplicaSets: len(inv.replicaSets),
		kindJobs: len(inv.jobs), kindCronJobs: len(inv.cronJobs),
		kindServiceAccounts: len(inv.serviceAccounts), kindRoles: len(inv.roles),
		kindClusterRoles: len(inv.clusterRoles), kindRoleBindings: len(inv.roleBindings),
		kindClusterRoleBindings: len(inv.clusterRoleBindings), kindNetworkPolicies: len(inv.networkPolicies),
	}
}

// allKinds lists every inventory kind in a fixed order, for messages.
var allKinds = []string{
	kindNamespaces, kindPods, kindDeployments, kindStatefulSets, kindDaemonSets, kindReplicaSets,
	kindJobs, kindCronJobs, kindServiceAccounts, kindRoles, kindClusterRoles, kindRoleBindings,
	kindClusterRoleBindings, kindNetworkPolicies,
}
