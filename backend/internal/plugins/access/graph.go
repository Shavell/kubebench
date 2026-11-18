package access

import (
	"net/http"
	"slices"
	"sort"
	"strings"

	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/util/validation"

	"kubebench/internal/plugin"
	"kubebench/internal/reason"
)

// GraphNode is a subject, binding or role of the binding graph. ID is
// unique across all three columns.
type GraphNode struct {
	ID        string `json:"id"`
	Kind      string `json:"kind"`
	Namespace string `json:"namespace,omitempty"`
	Name      string `json:"name"`
	// Missing marks a role a binding refers to that does not exist, or a
	// ServiceAccount that does not exist.
	Missing bool `json:"missing,omitempty"`
}

// GraphEdge links a subject to a binding or a binding to its role.
type GraphEdge struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// Graph is GET /graph: three columns, subjects → bindings → roles.
type Graph struct {
	Subjects []GraphNode `json:"subjects"`
	Bindings []GraphNode `json:"bindings"`
	Roles    []GraphNode `json:"roles"`
	Edges    []GraphEdge `json:"edges"`
	// Truncated: more than maxGraphBindings bindings matched; the first
	// ones are shown.
	Truncated bool `json:"truncated"`
}

// maxGraphBindings keeps the drawing readable and the response small.
const maxGraphBindings = 300

// graph serves GET /graph?namespace=&system=.
func graph(w http.ResponseWriter, r *http.Request, c *plugin.Context) {
	ns := r.URL.Query().Get("namespace")
	if ns != "" {
		if errs := validation.IsDNS1123Label(ns); len(errs) > 0 {
			writeErr(w, invalidField("namespace", reason.FieldInvalid, reason.Params{"detail": strings.Join(errs, "; ")}))
			return
		}
	}
	snap, err := loadSnapshot(r.Context(), c.Runtime.CS)
	if err != nil {
		plugin.WriteK8sError(w, err)
		return
	}
	plugin.WriteJSON(w, snap.graph(ns, r.URL.Query().Get("system") == "true"))
}

// isSystem reports whether a name belongs to Kubernetes' own bootstrap
// RBAC (system:…), which most views hide by default.
func isSystem(name string) bool { return strings.HasPrefix(name, "system:") }

// graph builds the binding graph. With a namespace it holds that
// namespace's RoleBindings and the ClusterRoleBindings that name one of
// its ServiceAccounts; without, every binding. Unless withSystem is set,
// bindings that are or refer to system:… objects are left out.
func (s *snapshot) graph(ns string, withSystem bool) Graph {
	g := Graph{Subjects: []GraphNode{}, Bindings: []GraphNode{}, Roles: []GraphNode{}, Edges: []GraphEdge{}}
	subjects := map[string]bool{}
	roles := map[string]bool{}
	edges := map[GraphEdge]bool{}
	existingSA := map[string]bool{}
	for _, sa := range s.serviceAccounts {
		existingSA[sa.Namespace+"/"+sa.Name] = true
	}
	for i := range s.bindings {
		b := &s.bindings[i]
		if !withSystem && (isSystem(b.Ref.Name) || isSystem(b.RoleRef.Name)) {
			continue
		}
		if ns != "" && b.Scope != ns && !(b.Scope == "" && namesServiceAccountIn(b, ns)) {
			continue
		}
		if len(g.Bindings) == maxGraphBindings {
			g.Truncated = true
			break
		}
		bID := "b:" + b.Ref.key()
		g.Bindings = append(g.Bindings, GraphNode{ID: bID, Kind: b.Ref.Kind, Namespace: b.Ref.Namespace, Name: b.Ref.Name})

		rID := "r:" + b.RoleRef.key()
		if !roles[rID] {
			roles[rID] = true
			g.Roles = append(g.Roles, GraphNode{
				ID: rID, Kind: b.RoleRef.Kind, Namespace: b.RoleRef.Namespace, Name: b.RoleRef.Name,
				Missing: s.roleOf(b) == nil,
			})
		}
		g.Edges = append(g.Edges, GraphEdge{From: bID, To: rID})

		for _, bs := range b.Subjects {
			sub := subjectOf(bs, b.Scope)
			sID := "s:" + sub.key()
			edge := GraphEdge{From: sID, To: bID}
			if edges[edge] {
				continue // the binding names the subject twice
			}
			edges[edge] = true
			if !subjects[sID] {
				subjects[sID] = true
				g.Subjects = append(g.Subjects, GraphNode{
					ID: sID, Kind: sub.Kind, Namespace: sub.Namespace, Name: sub.Name,
					Missing: sub.Kind == rbacv1.ServiceAccountKind && !existingSA[sub.Namespace+"/"+sub.Name],
				})
			}
			g.Edges = append(g.Edges, edge)
		}
	}
	sort.Slice(g.Subjects, func(i, j int) bool {
		a, b := g.Subjects[i], g.Subjects[j]
		return subjectLess(Subject{a.Kind, a.Name, a.Namespace}, Subject{b.Kind, b.Name, b.Namespace})
	})
	sort.Slice(g.Roles, func(i, j int) bool { return g.Roles[i].ID < g.Roles[j].ID })
	return g
}

func namesServiceAccountIn(b *binding, ns string) bool {
	return slices.ContainsFunc(b.Subjects, func(bs rbacv1.Subject) bool {
		return bs.Kind == rbacv1.ServiceAccountKind && bs.Namespace == ns
	})
}
