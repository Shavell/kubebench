package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"kubebench/internal/cloudauth"
	"kubebench/internal/kube"
	"kubebench/internal/plugin"
	"kubebench/internal/reason"
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
	// Platform is the detected provider and version support (§7.12).
	Platform *platformView `json:"platform"`
	// Admins only: the cloud whose identity signs in to the cluster in
	// process (aws, gcp, azure; "" for other credentials) and the provider
	// of its stored per-cluster credentials ("" = the server's own
	// identity). The credentials themselves are never returned.
	CloudAuth        string `json:"cloudAuth,omitempty"`
	CloudCredentials string `json:"cloudCredentials,omitempty"`
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
		view := describeCluster(&clusters[i], c.user.ActiveClusterID)
		cluster := &clusters[i].Cluster
		view.Platform = s.platformOf(cluster, view.Server)
		if c.user.IsAdmin() {
			view.CloudAuth = cloudAuthOf(cluster)
			view.CloudCredentials = cloudauth.ProviderOf(cluster.Credentials)
		}
		out = append(out, view)
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
		// Per-cluster cloud credentials (§7.12), write-only.
		Credentials json.RawMessage `json:"credentials"`
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
			writeErr(w, fail(http.StatusForbidden, plugin.CodeForbidden, reasonPathDisabled, nil))
			return
		}
		if dir := s.requestWrittenDir(body.Path); dir != "" {
			writeErr(w, fail(http.StatusForbidden, plugin.CodeForbidden, reasonPathWritten,
				reason.Params{"path": body.Path, "dir": dir}))
			return
		}
		content, err := kube.ReadKubeconfig(body.Path)
		if err != nil {
			writeErr(w, badRequest(reasonReadKubeconfig, reason.Params{"path": body.Path, "detail": err.Error()}))
			return
		}
		cluster.Source, cluster.Kubeconfig = store.SourceKubeconfig, string(content)
	case body.Address != "":
		cluster.Source, cluster.Address = store.SourceAddress, body.Address
	default:
		writeErr(w, badRequest(reasonSourceRequired, nil))
		return
	}
	audit := map[string]string{"source": cluster.Source}
	c.setAudit("", auditJSON(audit))

	if cluster.Source == store.SourceKubeconfig {
		if err := s.checkKubeconfig(cluster.Kubeconfig, cluster.Context, body.Content != ""); err != nil {
			writeErr(w, err)
			return
		}
	}
	credentials, _, err := decodeCredentials(body.Credentials, &cluster)
	if err != nil {
		writeErr(w, err)
		return
	}
	if credentials != "" {
		cluster.Credentials = credentials
		audit["credentials"] = cloudauth.ProviderOf(credentials) // the provider only, never the values
		c.setAudit("", auditJSON(audit))
	}
	// A pasted kubeconfig may not lend itself the server's cloud identity;
	// one added by path is the server's own file.
	if body.Content != "" {
		if err := s.ambientIdentityError(&cluster); err != nil {
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

// requestWrittenDir is the directory of path's that KubeBench fills with
// content that requests supply (Helm repository indexes and chart uploads,
// other plugins' data, external plugins), or "" when path is outside all
// of them. A kubeconfig added by path is trusted as one of the server's
// own files (it may run exec plugins), which such a file is not.
func (s *Server) requestWrittenDir(path string) string {
	dirs := []string{s.cfg.PluginsDir}
	for _, p := range s.plugins {
		dirs = append(dirs, filepath.Join(s.cfg.DataDir, p.Manifest().ID))
	}
	target := resolvedPath(path)
	for _, dir := range dirs {
		if rel, err := filepath.Rel(resolvedPath(dir), target); err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return dir
		}
	}
	return ""
}

// resolvedPath is path made absolute with its symbolic links resolved, as
// far as they exist.
func resolvedPath(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		return filepath.Clean(path)
	}
	if real, err := filepath.EvalSymlinks(abs); err == nil {
		return real
	}
	return abs
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
		return badRequest(reasonBadKubeconfig, reason.Params{"detail": err.Error()})
	}
	if contextName == "" {
		contextName = kc.CurrentContext
	}
	ctx, ok := kc.Contexts[contextName]
	if !ok {
		return badRequest(reasonUnknownContext, reason.Params{"context": contextName})
	}
	// An auth provider KubeBench cannot use fails whatever the source;
	// say so now rather than when the cluster is first used.
	if err := kube.CheckAuthProvider(kc.AuthInfos[ctx.AuthInfo]); err != nil {
		return refuse(http.StatusBadRequest, plugin.CodeBadRequest, err)
	}
	if !pasted || s.cfg.AllowKubeconfigExec {
		return nil
	}
	// The clouds' exec plugins are not run but replaced in process
	// (§7.12), so they need no exception here; the identity they sign in
	// with is checked by ambientIdentityError.
	if refs := serverSideReferences(kc.Clusters[ctx.Cluster], kc.AuthInfos[ctx.AuthInfo]); len(refs) > 0 {
		return fail(http.StatusForbidden, plugin.CodeForbidden, reasonServerSideRefs,
			reason.Params{"references": strings.Join(refs, ", ")})
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
		if user.Exec != nil && cloudauth.ProviderFor(user) == "" {
			refs = append(refs, "an exec credential plugin")
		}
		// The oidc auth provider runs nothing: it refreshes the ID token
		// at the issuer the kubeconfig names, as the API server it names
		// is contacted too. Only its CA file would be read on the server.
		switch {
		case kube.IsOIDCAuthProvider(user):
			if user.AuthProvider.Config[kube.OIDCCAFileKey] != "" {
				refs = append(refs, "an idp-certificate-authority file")
			}
		case user.AuthProvider != nil:
			refs = append(refs, "an auth provider")
		}
	}
	sort.Strings(refs)
	return refs
}

func validateClusterName(name string) error {
	if name == "" || len(name) > 63 {
		return badRequest(reasonClusterName, nil)
	}
	return nil
}

func (s *Server) handleDeleteCluster(w http.ResponseWriter, r *http.Request, _ *call) {
	name := r.URL.Query().Get("name")
	cluster, err := s.store.ClusterByName(name)
	if errors.Is(err, store.ErrNotFound) {
		writeErr(w, fail(http.StatusNotFound, plugin.CodeNotFound, reasonClusterNotFound, reason.Params{"name": name}))
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
	s.manager.Remove(cluster.ID)
	writeOK(w)
}

// handleUpdateCluster renames a cluster or changes its safety flags or
// credentials (§4.3). The cluster's runtime is closed, so its background
// work (sampler, live updates) stops using the old record at once; the
// next request builds one for the new record.
func (s *Server) handleUpdateCluster(w http.ResponseWriter, r *http.Request, c *call) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeErr(w, badRequest(reasonClusterID, reason.Params{"id": r.PathValue("id")}))
		return
	}
	cluster, err := s.store.GetCluster(id)
	if errors.Is(err, store.ErrNotFound) {
		writeErr(w, fail(http.StatusNotFound, plugin.CodeNotFound, reasonClusterIDMissing, reason.Params{"id": id}))
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
		// Per-cluster cloud credentials (§7.12): an object replaces them,
		// null removes them, absent leaves them.
		Credentials json.RawMessage `json:"credentials"`
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
	credentials, removeCredentials, credErr := decodeCredentials(body.Credentials, cluster)
	switch {
	case credentials != "":
		update.Credentials, changes["credentials"] = &credentials, cloudauth.ProviderOf(credentials)
	case removeCredentials:
		update.Credentials, changes["credentials"] = &credentials, nil
	case credErr != nil: // audited as an attempt to change the credentials
		changes["credentials"] = "invalid"
	}
	c.setAudit("cluster/"+cluster.Name, auditJSON(changes))
	if credErr != nil {
		writeErr(w, credErr)
		return
	}
	if len(changes) == 0 {
		writeErr(w, badRequest(reasonClusterNothing, nil))
		return
	}
	if update.Name != nil {
		if err := validateClusterName(*update.Name); err != nil {
			writeErr(w, err)
			return
		}
	}
	if update.Environment != nil && !store.ValidEnvironment(*update.Environment) {
		writeErr(w, badRequest(reasonEnvironment, reason.Params{"environment": *update.Environment}))
		return
	}
	if update.Credentials != nil {
		// Removing the credentials, or replacing them with an AWS profile,
		// would sign in with the server's own identity.
		changed := *cluster
		changed.Credentials = *update.Credentials
		if err := s.ambientIdentityError(&changed); err != nil {
			writeErr(w, err)
			return
		}
	}
	if err := s.store.UpdateCluster(id, update); err != nil {
		writeErr(w, err)
		return
	}
	s.manager.Drop(id)
	writeOK(w)
}

func (s *Server) handleActivateCluster(w http.ResponseWriter, r *http.Request, c *call) {
	name := r.URL.Query().Get("name")
	cluster, err := s.store.ClusterByName(name)
	if errors.Is(err, store.ErrNotFound) {
		writeErr(w, fail(http.StatusNotFound, plugin.CodeNotFound, reasonClusterNotFound, reason.Params{"name": name}))
		return
	}
	if err != nil {
		writeErr(w, err)
		return
	}
	if cluster.Source == store.SourceAddress {
		writeErr(w, badRequest(reasonAddressOnly, nil))
		return
	}
	role, err := s.store.ClusterRole(c.user, cluster.ID)
	if err != nil {
		writeErr(w, err)
		return
	}
	if role == "" {
		writeErr(w, fail(http.StatusForbidden, plugin.CodeForbidden, reasonNoAccess, reason.Params{"name": name}))
		return
	}
	if _, err := s.manager.Runtime(cluster); err != nil {
		writeErr(w, badRequest(reasonConnectFailed, reason.Params{"name": name, "detail": err.Error()}))
		return
	}
	if err := s.store.SetActiveCluster(c.user.ID, cluster.ID); err != nil {
		writeErr(w, err)
		return
	}
	writeOK(w)
}
