package access

import (
	"slices"
	"strings"

	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/util/validation"

	"kubebench/internal/reason"
)

// Subject is who a binding grants to: a User, Group or ServiceAccount.
// Namespace is set for ServiceAccounts only.
type Subject struct {
	Kind      string `json:"kind"`
	Name      string `json:"name"`
	Namespace string `json:"namespace,omitempty"`
}

func (s Subject) key() string { return s.Kind + "/" + s.Namespace + "/" + s.Name }

// Groups every authenticated request carries; ServiceAccounts also belong
// to system:serviceaccounts and system:serviceaccounts:<namespace>.
const (
	groupAuthenticated   = "system:authenticated"
	groupUnauthenticated = "system:unauthenticated"
	userAnonymous        = "system:anonymous"
	saGroupPrefix        = "system:serviceaccounts"
	saUserPrefix         = "system:serviceaccount:"
)

// identity is the user name and groups the API server sees for a subject,
// as when it authenticates that subject's own credentials.
type identity struct {
	User   string
	Groups []string
}

// identityOf returns the identity a subject authenticates as. A Group
// stands for an unnamed member: only the group itself is known.
func identityOf(s Subject) identity {
	switch s.Kind {
	case rbacv1.ServiceAccountKind:
		return identity{
			User:   saUserPrefix + s.Namespace + ":" + s.Name,
			Groups: []string{saGroupPrefix, saGroupPrefix + ":" + s.Namespace, groupAuthenticated},
		}
	case rbacv1.UserKind:
		if s.Name == userAnonymous {
			return identity{User: s.Name, Groups: []string{groupUnauthenticated}}
		}
		return identity{User: s.Name, Groups: []string{groupAuthenticated}}
	default:
		return identity{Groups: []string{s.Name}}
	}
}

// matches reports whether the binding subject bs (of a binding in
// namespace bindingNS, "" for a ClusterRoleBinding) covers id, and the
// group it does so through ("" when it names the identity's user).
// ServiceAccount subjects without a namespace default to the binding's,
// as in the API server's authorizer.
func (id identity) matches(bs rbacv1.Subject, bindingNS string) (ok bool, viaGroup string) {
	switch bs.Kind {
	case rbacv1.UserKind:
		return id.User != "" && bs.Name == id.User, ""
	case rbacv1.GroupKind:
		if slices.Contains(id.Groups, bs.Name) {
			return true, bs.Name
		}
	case rbacv1.ServiceAccountKind:
		ns := bs.Namespace
		if ns == "" {
			ns = bindingNS
		}
		return id.User != "" && id.User == saUserPrefix+ns+":"+bs.Name, ""
	}
	return false, ""
}

// subjectOf turns a binding subject into a Subject, defaulting a
// ServiceAccount's namespace to the binding's.
func subjectOf(bs rbacv1.Subject, bindingNS string) Subject {
	s := Subject{Kind: bs.Kind, Name: bs.Name}
	if bs.Kind == rbacv1.ServiceAccountKind {
		s.Namespace = bs.Namespace
		if s.Namespace == "" {
			s.Namespace = bindingNS
		}
	}
	return s
}

// validateSubject checks a subject named by a request.
func validateSubject(s Subject) error {
	switch s.Kind {
	case rbacv1.UserKind, rbacv1.GroupKind:
		if s.Namespace != "" {
			return invalidField("namespace", reasonOnlySANamespace, nil)
		}
	case rbacv1.ServiceAccountKind:
		if errs := validation.IsDNS1123Label(s.Namespace); len(errs) > 0 {
			return invalidField("namespace", reason.FieldInvalid, reason.Params{"detail": strings.Join(errs, "; ")})
		}
		if errs := validation.IsDNS1123Subdomain(s.Name); len(errs) > 0 {
			return invalidField("name", reason.FieldInvalid, reason.Params{"detail": strings.Join(errs, "; ")})
		}
	default:
		return invalidField("kind", reasonSubjectKind, reason.Params{"kind": s.Kind})
	}
	if strings.TrimSpace(s.Name) == "" {
		return invalidField("name", reason.FieldRequired, nil)
	}
	if len(s.Name) > maxNameLength {
		return invalidField("name", reason.FieldTooLong, reason.Params{"max": maxNameLength})
	}
	return nil
}

// maxNameLength bounds user and group names in requests; real ones are short.
const maxNameLength = 512
