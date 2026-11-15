package api

import (
	"errors"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"kubebench/internal/kube"
	"kubebench/internal/plugin"
	"kubebench/internal/store"
)

// clusterView is one inventory entry (src/data/types.ts ClusterEntry).
type clusterView struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Source      string `json:"source"`
	Active      bool   `json:"active"`
	Context     string `json:"context,omitempty"`
	Server      string `json:"server,omitempty"`
	Address     string `json:"address,omitempty"`
	Error       string `json:"error,omitempty"`
	ReadOnly    bool   `json:"readOnly"`
	Environment string `json:"environment"`
	Role        string `json:"role"` // the caller's effective role on the cluster
	// KubeconfigPath labels where the credentials come from (v0.4 field).
	KubeconfigPath string `json:"kubeconfigPath,omitempty"`
}

func describeCluster(uc *store.UserCluster, activeID *int64) clusterView {
	c := &uc.Cluster
	view := clusterView{
		ID: c.ID, Name: c.Name, Source: c.Source, Address: c.Address, Context: c.Context,
		Active:   activeID != nil && *activeID == c.ID,
		ReadOnly: c.ReadOnly, Environment: c.Environment, Role: uc.Role,
	}
	switch c.Source {
	case store.SourceInCluster:
		view.KubeconfigPath = "(in-cluster service account)"
		view.Server = "kubernetes.default.svc"
	case store.SourceAddress:
		view.Server = c.Address
	default:
		view.KubeconfigPath = "(stored kubeconfig)"
		kc, err := clientcmd.Load([]byte(c.Kubeconfig))
		if err != nil {
			view.Error = "unparseable kubeconfig"
			break
		}
		view.Context = c.Context
		if view.Context == "" {
			view.Context = kc.CurrentContext
		}
		if ctx, ok := kc.Contexts[view.Context]; ok {
			if cluster, ok := kc.Clusters[ctx.Cluster]; ok {
				view.Server = cluster.Server
			}
		}
	}
	return view
}

func (s *Server) handleListClusters(w http.ResponseWriter, _ *http.Request, c *call) {
	clusters, err := s.store.ListClusters(c.user)
	if err != nil {
		writeErr(w, err)
		return
	}
	out := make([]clusterView, 0, len(clusters))
	for i := range clusters {
		out = append(out, describeCluster(&clusters[i], c.user.ActiveClusterID))
	}
	plugin.WriteJSON(w, out)
}

// handleAddCluster stores a kubeconfig (pasted content or, when
// features.pathKubeconfig, a server-side path) or an address bookmark.
// JSON bodies and the v0.4 query parameters are both accepted.
func (s *Server) handleAddCluster(w http.ResponseWriter, r *http.Request, c *call) {
	var body struct {
		Name    string `json:"name"`
		Content string `json:"content"`
		Path    string `json:"path"`
		Address string `json:"address"`
		Context string `json:"context"`
	}
	if strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
		if err := decodeJSON(w, r, &body); err != nil {
			writeErr(w, err)
			return
		}
	} else {
		q := r.URL.Query()
		body.Name, body.Path, body.Address, body.Context = q.Get("name"), q.Get("path"), q.Get("address"), q.Get("context")
	}
	body.Name = strings.TrimSpace(body.Name)
	c.setAudit("cluster/"+body.Name, "")
	if err := validateClusterName(body.Name); err != nil {
		writeErr(w, err)
		return
	}

	cluster := store.Cluster{Name: body.Name, Context: body.Context}
	switch {
	case body.Content != "":
		cluster.Source, cluster.Kubeconfig = store.SourceKubeconfig, body.Content
	case body.Path != "":
		if !s.cfg.AllowPathKubeconfig {
			writeErr(w, errorf(http.StatusForbidden, plugin.CodeForbidden,
				"adding clusters by server-side path is disabled (KUBEBENCH_ALLOW_PATH_KUBECONFIG); paste the kubeconfig instead"))
			return
		}
		content, err := kube.ReadKubeconfig(body.Path)
		if err != nil {
			writeErr(w, badRequest("cannot read the kubeconfig %s: %v", body.Path, err))
			return
		}
		cluster.Source, cluster.Kubeconfig = store.SourceKubeconfig, string(content)
	case body.Address != "":
		cluster.Source, cluster.Address = store.SourceAddress, body.Address
	default:
		writeErr(w, badRequest("kubeconfig content, path or API address is required"))
		return
	}
	c.setAudit("", auditJSON(map[string]string{"source": cluster.Source}))

	if cluster.Source == store.SourceKubeconfig {
		if err := s.checkKubeconfig(cluster.Kubeconfig, cluster.Context, body.Content != ""); err != nil {
			writeErr(w, err)
			return
		}
	}
	id, err := s.store.AddCluster(cluster)
	if err != nil {
		writeErr(w, err)
		return
	}
	plugin.WriteJSON(w, map[string]any{"ok": true, "id": id})
}

// checkKubeconfig validates kubeconfig content for the given context ("" =
// current). Pasted content is whatever the admin's browser sent: unless
// KUBEBENCH_ALLOW_KUBECONFIG_EXEC is set, its credentials must be
// self-contained, so that it cannot make KubeBench run commands or read
// local files (such as its own service-account token or database) as the
// server's account. A kubeconfig added by path is one of the server's own
// files, which the path setting already trusts.
func (s *Server) checkKubeconfig(content, contextName string, pasted bool) error {
	kc, err := clientcmd.Load([]byte(content))
	if err != nil {
		return badRequest("invalid kubeconfig: %v", err)
	}
	if contextName == "" {
		contextName = kc.CurrentContext
	}
	ctx, ok := kc.Contexts[contextName]
	if !ok {
		return badRequest("context %q is not in the kubeconfig", contextName)
	}
	if !pasted || s.cfg.AllowKubeconfigExec {
		return nil
	}
	if refs := serverSideReferences(kc.Clusters[ctx.Cluster], kc.AuthInfos[ctx.AuthInfo]); len(refs) > 0 {
		return errorf(http.StatusForbidden, plugin.CodeForbidden,
			"the kubeconfig uses %s, which this server does not allow in pasted kubeconfigs (KUBEBENCH_ALLOW_KUBECONFIG_EXEC); embed the credentials as *-data fields or a token",
			strings.Join(refs, ", "))
	}
	return nil
}

// serverSideReferences lists what the cluster/user entries would read or
// run on the server.
func serverSideReferences(cluster *clientcmdapi.Cluster, user *clientcmdapi.AuthInfo) []string {
	var refs []string
	if cluster != nil && cluster.CertificateAuthority != "" {
		refs = append(refs, "a certificate-authority file")
	}
	if user != nil {
		if user.ClientCertificate != "" || user.ClientKey != "" {
			refs = append(refs, "client certificate files")
		}
		if user.TokenFile != "" {
			refs = append(refs, "a token file")
		}
		if user.Exec != nil {
			refs = append(refs, "an exec credential plugin")
		}
		if user.AuthProvider != nil {
			refs = append(refs, "an auth provider")
		}
	}
	sort.Strings(refs)
	return refs
}

func validateClusterName(name string) error {
	if name == "" || len(name) > 63 {
		return badRequest("a cluster name is 1-63 characters")
	}
	return nil
}

func (s *Server) handleDeleteCluster(w http.ResponseWriter, r *http.Request, _ *call) {
	name := r.URL.Query().Get("name")
	cluster, err := s.store.ClusterByName(name)
	if errors.Is(err, store.ErrNotFound) {
		writeErr(w, errorf(http.StatusNotFound, plugin.CodeNotFound, "cluster %q not found", name))
		return
	}
	if err != nil {
		writeErr(w, err)
		return
	}
	if err := s.store.DeleteCluster(cluster.ID); err != nil {
		writeErr(w, err)
		return
	}
	s.manager.Drop(cluster.ID)
	writeOK(w)
}

// handleUpdateCluster renames a cluster or changes its safety flags (§4.3).
// The cluster's runtime picks the new record up on its next use.
func (s *Server) handleUpdateCluster(w http.ResponseWriter, r *http.Request, c *call) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeErr(w, badRequest("invalid cluster id %q", r.PathValue("id")))
		return
	}
	cluster, err := s.store.GetCluster(id)
	if errors.Is(err, store.ErrNotFound) {
		writeErr(w, errorf(http.StatusNotFound, plugin.CodeNotFound, "cluster %d not found", id))
		return
	}
	if err != nil {
		writeErr(w, err)
		return
	}
	var body struct {
		Name        *string `json:"name"`
		ReadOnly    *bool   `json:"readOnly"`
		Environment *string `json:"environment"`
	}
	if err := decodeJSON(w, r, &body); err != nil {
		writeErr(w, err)
		return
	}
	update := store.ClusterUpdate{ReadOnly: body.ReadOnly, Environment: body.Environment}
	changes := map[string]any{}
	if body.Name != nil {
		name := strings.TrimSpace(*body.Name)
		update.Name, changes["name"] = &name, name
	}
	if body.ReadOnly != nil {
		changes["readOnly"] = *body.ReadOnly
	}
	if body.Environment != nil {
		changes["environment"] = *body.Environment
	}
	c.setAudit("cluster/"+cluster.Name, auditJSON(changes))
	if len(changes) == 0 {
		writeErr(w, badRequest("nothing to update: send name, readOnly and/or environment"))
		return
	}
	if update.Name != nil {
		if err := validateClusterName(*update.Name); err != nil {
			writeErr(w, err)
			return
		}
	}
	if update.Environment != nil && !store.ValidEnvironment(*update.Environment) {
		writeErr(w, badRequest("invalid environment %q: use development, staging, production or empty", *update.Environment))
		return
	}
	if err := s.store.UpdateCluster(id, update); err != nil {
		writeErr(w, err)
		return
	}
	writeOK(w)
}

func (s *Server) handleActivateCluster(w http.ResponseWriter, r *http.Request, c *call) {
	name := r.URL.Query().Get("name")
	cluster, err := s.store.ClusterByName(name)
	if errors.Is(err, store.ErrNotFound) {
		writeErr(w, errorf(http.StatusNotFound, plugin.CodeNotFound, "cluster %q not found", name))
		return
	}
	if err != nil {
		writeErr(w, err)
		return
	}
	if cluster.Source == store.SourceAddress {
		writeErr(w, badRequest("address-only entries cannot be activated"))
		return
	}
	role, err := s.store.ClusterRole(c.user, cluster.ID)
	if err != nil {
		writeErr(w, err)
		return
	}
	if role == "" {
		writeErr(w, errorf(http.StatusForbidden, plugin.CodeForbidden, "you have no access to cluster %q", name))
		return
	}
	if _, err := s.manager.Runtime(cluster); err != nil {
		writeErr(w, badRequest("cannot connect to cluster %q: %v", name, err))
		return
	}
	if err := s.store.SetActiveCluster(c.user.ID, cluster.ID); err != nil {
		writeErr(w, err)
		return
	}
	writeOK(w)
}
