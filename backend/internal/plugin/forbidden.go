package plugin

import (
	"regexp"

	apierrors "k8s.io/apimachinery/pkg/api/errors"

	"kubebench/internal/reason"
)

// Reasons of a Kubernetes 403 (docs/design-v0.5.md §3.3, §7.16): the
// cluster's credential lacks the RBAC for a request. The UI shows them as
// one "no permission" state instead of the API server's raw message.
var (
	ReasonForbiddenNamespace = reason.Define("kubernetes.forbidden_namespace",
		"no permission to {{verb}} {{resource}} in namespace \"{{namespace}}\" (Kubernetes RBAC of {{user}})")
	ReasonForbiddenCluster = reason.Define("kubernetes.forbidden_cluster",
		"no permission to {{verb}} {{resource}} at the cluster scope (Kubernetes RBAC of {{user}})")
	// ReasonForbidden is a denial in another wording (an authorization
	// webhook's, say): the message as it is.
	ReasonForbidden = reason.Define("kubernetes.forbidden", "{{detail}}")
)

// forbiddenMessage matches the API server's RBAC denial, e.g. `User
// "system:serviceaccount:team-a:alice" cannot list resource "pods" in API
// group "" in the namespace "default"` (or `at the cluster scope`).
var forbiddenMessage = regexp.MustCompile(
	`User "([^"]*)" cannot (\S+) resource "([^"]+)" in API group "([^"]*)"(?: in the namespace "([^"]+)"| at the cluster scope)`)

// ForbiddenReason explains a Kubernetes 403 with one of the forbidden
// reasons, keeping err as the cause; other errors are returned as they
// are, as is an error that already has a reason.
func ForbiddenReason(err error) error {
	if err == nil || !apierrors.IsForbidden(err) {
		return err
	}
	if _, _, ok := reason.Of(err); ok {
		return err
	}
	m := forbiddenMessage.FindStringSubmatch(err.Error())
	if m == nil {
		return reason.Wrap(err, ReasonForbidden, reason.Params{"detail": err.Error()})
	}
	resource := m[3]
	if m[4] != "" {
		resource += "." + m[4]
	}
	params := reason.Params{"user": m[1], "verb": m[2], "resource": resource}
	if m[5] == "" {
		return reason.Wrap(err, ReasonForbiddenCluster, params)
	}
	params["namespace"] = m[5]
	return reason.Wrap(err, ReasonForbiddenNamespace, params)
}
