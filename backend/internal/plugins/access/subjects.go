package access

import (
	"net/http"
	"slices"
	"sort"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"

	"kubebench/internal/plugin"
)

// SubjectSummary is one row of GET /subjects: a subject some binding
// names, or a ServiceAccount no binding names.
type SubjectSummary struct {
	Subject
	// ClusterBindings counts the ClusterRoleBindings that name the subject.
	ClusterBindings int `json:"clusterBindings"`
	// Namespaces lists where RoleBindings name it.
	Namespaces []string `json:"namespaces"`
	// Roles are the roles those bindings refer to.
	Roles []Ref `json:"roles"`
	// Missing marks a ServiceAccount that bindings name but that does not exist.
	Missing bool `json:"missing,omitempty"`
}

// listSubjects serves GET /subjects.
func listSubjects(w http.ResponseWriter, r *http.Request, c *plugin.Context) {
	snap, err := loadSnapshot(r.Context(), c.Runtime.CS)
	if err != nil {
		plugin.WriteK8sError(w, err)
		return
	}
	plugin.WriteJSON(w, snap.subjects())
}

func (s *snapshot) subjects() []SubjectSummary {
	byKey := map[string]*SubjectSummary{}
	get := func(sub Subject) *SubjectSummary {
		if e := byKey[sub.key()]; e != nil {
			return e
		}
		e := &SubjectSummary{Subject: sub, Namespaces: []string{}, Roles: []Ref{}}
		byKey[sub.key()] = e
		return e
	}
	existing := map[string]bool{}
	for _, sa := range s.serviceAccounts {
		sub := Subject{Kind: rbacv1.ServiceAccountKind, Namespace: sa.Namespace, Name: sa.Name}
		existing[sub.key()] = true
		get(sub)
	}
	for i := range s.bindings {
		b := &s.bindings[i]
		seen := map[string]bool{}
		for _, bs := range b.Subjects {
			sub := subjectOf(bs, b.Scope)
			if seen[sub.key()] {
				continue
			}
			seen[sub.key()] = true
			e := get(sub)
			if b.Scope == "" {
				e.ClusterBindings++
			} else if !slices.Contains(e.Namespaces, b.Scope) {
				e.Namespaces = append(e.Namespaces, b.Scope)
			}
			if !slices.Contains(e.Roles, b.RoleRef) {
				e.Roles = append(e.Roles, b.RoleRef)
			}
			e.Missing = sub.Kind == rbacv1.ServiceAccountKind && !existing[sub.key()]
		}
	}
	out := make([]SubjectSummary, 0, len(byKey))
	for _, e := range byKey {
		sort.Strings(e.Namespaces)
		out = append(out, *e)
	}
	sort.Slice(out, func(i, j int) bool { return subjectLess(out[i].Subject, out[j].Subject) })
	return out
}

// SubjectDetail is GET /subjects/{kind}/{name}: who the subject
// authenticates as, the bindings that reach it and its effective rules.
type SubjectDetail struct {
	Subject Subject `json:"subject"`
	// User and Groups are the identity the API server sees; the bindings
	// below include those that name one of the groups.
	User   string   `json:"user,omitempty"`
	Groups []string `json:"groups"`
	// Missing marks a ServiceAccount that does not exist.
	Missing  bool         `json:"missing,omitempty"`
	Bindings []Path       `json:"bindings"`
	Rules    []ScopeRules `json:"rules"`
}

// getSubject serves GET /subjects/{kind}/{name}?namespace=.
func getSubject(w http.ResponseWriter, r *http.Request, c *plugin.Context) {
	sub := Subject{Kind: r.PathValue("kind"), Name: r.PathValue("name"), Namespace: r.URL.Query().Get("namespace")}
	if err := validateSubject(sub); err != nil {
		writeErr(w, err)
		return
	}
	snap, err := loadSnapshot(r.Context(), c.Runtime.CS)
	if err != nil {
		plugin.WriteK8sError(w, err)
		return
	}
	plugin.WriteJSON(w, snap.subjectDetail(sub))
}

func (s *snapshot) subjectDetail(sub Subject) SubjectDetail {
	id := identityOf(sub)
	paths := s.pathsOf(id)
	d := SubjectDetail{
		Subject: sub, User: id.User, Groups: id.Groups,
		Bindings: paths, Rules: s.effectiveRules(paths),
	}
	if d.Bindings == nil {
		d.Bindings = []Path{}
	}
	if sub.Kind == rbacv1.ServiceAccountKind {
		d.Missing = !slices.ContainsFunc(s.serviceAccounts, func(sa corev1.ServiceAccount) bool {
			return sa.Namespace == sub.Namespace && sa.Name == sub.Name
		})
	}
	return d
}

// whoCanResponse is GET /who-can.
type whoCanResponse struct {
	Request  Request         `json:"request"`
	Subjects []SubjectGrants `json:"subjects"`
}

// whoCan serves GET /who-can?verb=&group=&resource=&subresource=&namespace=&name=&path=.
func whoCan(w http.ResponseWriter, r *http.Request, c *plugin.Context) {
	req, err := requestFromQuery(r.URL.Query())
	if err != nil {
		writeErr(w, err)
		return
	}
	snap, err := loadSnapshot(r.Context(), c.Runtime.CS)
	if err != nil {
		plugin.WriteK8sError(w, err)
		return
	}
	plugin.WriteJSON(w, whoCanResponse{Request: req, Subjects: snap.whoCan(req)})
}
