package api

import (
	"errors"
	"net/http"

	"kubebench/internal/kube"
	"kubebench/internal/plugin"
	"kubebench/internal/reason"
	"kubebench/internal/store"
)

// permissions is the caller's effective permission set on the active
// cluster (src/data/types.ts SessionPermissions).
type permissions struct {
	Role     string `json:"role"` // admin | operator | viewer
	CanRead  bool   `json:"canRead"`
	CanWrite bool   `json:"canWrite"`
	CanExec  bool   `json:"canExec"`
	IsAdmin  bool   `json:"isAdmin"`
	ReadOnly bool   `json:"readOnly"`
}

// permissionsFor derives the permissions from the same rule the router
// enforces (mutationDenied). Without a cluster nothing can be read, and a
// non-admin counts as a viewer.
func permissionsFor(user *store.User, role string, cluster *store.Cluster) permissions {
	p := permissions{Role: role, IsAdmin: user.IsAdmin()}
	if cluster == nil {
		p.Role = store.ClusterRoleViewer
		if user.IsAdmin() {
			p.Role = store.RoleAdmin
		}
		return p
	}
	p.CanRead = true
	p.ReadOnly = cluster.ReadOnly
	p.CanWrite = mutationDenied(role, cluster) == nil
	p.CanExec = p.CanWrite
	return p
}

// mutationDenied explains why role may not write or exec on cluster, or
// returns nil. A read-only cluster refuses everyone, admins included, and
// is reported first, as the UI does.
func mutationDenied(role string, cluster *store.Cluster) error {
	if cluster.ReadOnly {
		return fail(http.StatusForbidden, plugin.CodeClusterReadOnly, reasonReadOnly, reason.Params{"cluster": cluster.Name})
	}
	if role != store.RoleAdmin && role != store.ClusterRoleOperator {
		return fail(http.StatusForbidden, plugin.CodeForbiddenRole, reasonRoleNoChange, reason.Params{"cluster": cluster.Name, "role": role})
	}
	return nil
}

// contributionDenied explains why role may not change data that every
// user of cluster sees (accessContribute), or returns nil. Unlike
// mutationDenied it ignores the read-only flag: such data lives in
// KubeBench, not in the cluster.
func contributionDenied(role string, cluster *store.Cluster) error {
	if role != store.RoleAdmin && role != store.ClusterRoleOperator {
		return fail(http.StatusForbidden, plugin.CodeForbiddenRole, reasonRoleNoContribute, reason.Params{"cluster": cluster.Name, "role": role})
	}
	return nil
}

// browseDenied explains why role may not open the service browser on
// cluster (§7.14), or returns nil: it reaches into the cluster network
// with KubeBench's credential, so it needs the operator or admin role. A
// read-only cluster does not refuse it; the proxy then sends only GET and
// HEAD requests.
func browseDenied(role string, cluster *store.Cluster) error {
	if role != store.RoleAdmin && role != store.ClusterRoleOperator {
		return fail(http.StatusForbidden, plugin.CodeForbiddenRole, reasonRoleNoBrowse, reason.Params{"cluster": cluster.Name, "role": role})
	}
	return nil
}

// activeCluster returns the caller's active cluster and their role on it.
// It never chooses a cluster by itself (only /api/info does, see
// selectCluster), so a request cannot act on a cluster other than the one
// the UI shows.
func (s *Server) activeCluster(user *store.User) (*store.Cluster, string, error) {
	if user.ActiveClusterID == nil {
		return nil, "", fail(http.StatusConflict, plugin.CodeNoCluster, reasonNoActive, nil)
	}
	cluster, err := s.store.GetCluster(*user.ActiveClusterID)
	if errors.Is(err, store.ErrNotFound) {
		return nil, "", fail(http.StatusConflict, plugin.CodeNoCluster, reasonActiveRemoved, nil)
	}
	if err != nil {
		return nil, "", err
	}
	role, err := s.store.ClusterRole(user, cluster.ID)
	if err != nil {
		return nil, "", err
	}
	if role == "" {
		return nil, "", fail(http.StatusConflict, plugin.CodeNoCluster, reasonAccessRevoked, reason.Params{"cluster": cluster.Name})
	}
	return cluster, role, nil
}

// resolveCluster fills c with the active cluster, the role and the runtime.
func (s *Server) resolveCluster(c *call) error {
	cluster, role, err := s.activeCluster(c.user)
	if err != nil {
		return err
	}
	rt, err := s.manager.Runtime(cluster)
	if errors.Is(err, kube.ErrClusterRemoved) {
		return fail(http.StatusConflict, plugin.CodeNoCluster, reasonActiveRemoved, nil)
	}
	if err != nil {
		return fail(http.StatusServiceUnavailable, plugin.CodeNoCluster, reasonClusterDown, reason.Params{"cluster": cluster.Name, "detail": err.Error()})
	}
	c.cluster, c.role, c.rt = cluster, role, rt
	return nil
}

// selectCluster is activeCluster for /api/info: when no usable cluster is
// active it activates the first cluster the caller may use. The app reads
// /api/info at boot and shows the cluster it reports, so switching here
// cannot surprise a request in flight.
func (s *Server) selectCluster(user *store.User) (*store.Cluster, string, error) {
	cluster, role, err := s.activeCluster(user)
	if err == nil || !hasCode(err, plugin.CodeNoCluster) {
		return cluster, role, err
	}
	clusters, err := s.store.ListClusters(user)
	if err != nil {
		return nil, "", err
	}
	for i := range clusters {
		uc := &clusters[i]
		if uc.Source == store.SourceAddress {
			continue // bookmarks cannot be activated
		}
		if err := s.store.SetActiveCluster(user.ID, uc.ID); err != nil {
			return nil, "", err
		}
		user.ActiveClusterID = &uc.ID
		return &uc.Cluster, uc.Role, nil
	}
	if user.IsAdmin() {
		return nil, "", fail(http.StatusConflict, plugin.CodeNoCluster, reasonNoneAdded, nil)
	}
	return nil, "", fail(http.StatusConflict, plugin.CodeNoCluster, reasonNoneAssigned, nil)
}
