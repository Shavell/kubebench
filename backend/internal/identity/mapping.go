package identity

import (
	"slices"
	"strings"

	"kubebench/internal/store"
)

// Grants is what a user's groups grant through the group mappings.
type Grants struct {
	Admin bool
	// Clusters maps cluster names to store.ClusterRoleViewer or
	// store.ClusterRoleOperator; when several mappings name a cluster, the
	// strongest role wins.
	Clusters map[string]string
	// Matched are the IDs of the mappings that applied, in mapping order.
	Matched []int64
}

// Any reports whether the grants give any access.
func (g Grants) Any() bool { return g.Admin || len(g.Clusters) > 0 }

// Role is the global store role the grants give.
func (g Grants) Role() string {
	if g.Admin {
		return store.RoleAdmin
	}
	return store.RoleUser
}

// Resolve applies the mappings to the groups a provider of source reported.
// Groups match case-insensitively and exactly (LDAP DNs and IdP group
// names alike); a mapping with an empty source applies to every provider.
func Resolve(source string, groups []string, mappings []store.GroupMapping) Grants {
	g := Grants{Clusters: map[string]string{}}
	for _, m := range mappings {
		if m.Source != "" && m.Source != source {
			continue
		}
		if !slices.ContainsFunc(groups, func(group string) bool { return strings.EqualFold(group, m.Group) }) {
			continue
		}
		g.Matched = append(g.Matched, m.ID)
		if m.Role == store.RoleAdmin {
			g.Admin = true
			continue
		}
		if g.Clusters[m.Cluster] != store.ClusterRoleOperator {
			g.Clusters[m.Cluster] = m.Role
		}
	}
	return g
}
