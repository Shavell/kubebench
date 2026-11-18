package api

import (
	"context"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"kubebench/internal/identity"
	"kubebench/internal/plugin"
	"kubebench/internal/store"
)

// Settings > Authentication (docs/design-v0.5.md §3.6): the configured
// providers (without their secrets), the group mappings and the "who would
// I be" preview. Admins only; mapping changes are audited.

// providerCheckTimeout bounds the status check of a provider.
const providerCheckTimeout = 5 * time.Second

// providerSetting is one configuration value of a provider. Secrets are
// never sent: SecretSet only says whether one is configured.
type providerSetting struct {
	Key       string `json:"key"`
	Value     string `json:"value"`
	SecretSet *bool  `json:"secretSet,omitempty"`
}

type providerStatus struct {
	OK    bool   `json:"ok"`
	Error string `json:"error"`
}

type providerView struct {
	Source   string            `json:"source"`
	Name     string            `json:"name"`
	Settings []providerSetting `json:"settings"`
	Status   providerStatus    `json:"status"`
}

type authSettingsView struct {
	// PublicURL is the configured or, when unset, the derived URL.
	PublicURL           string         `json:"publicUrl"`
	PublicURLConfigured bool           `json:"publicUrlConfigured"`
	RequireGroup        bool           `json:"requireGroup"`
	Providers           []providerView `json:"providers"`
}

func (s *Server) handleAuthSettings(w http.ResponseWriter, r *http.Request, _ *call) {
	view := authSettingsView{
		PublicURL:           s.publicURL(r),
		PublicURLConfigured: s.cfg.PublicURL != "",
		RequireGroup:        s.ident.service.RequireGroup(),
		Providers:           []providerView{},
	}
	if p := s.ident.oidc; p != nil {
		view.Providers = append(view.Providers, s.oidcView(r))
	}
	plugin.WriteJSON(w, view)
}

func (s *Server) oidcView(r *http.Request) providerView {
	p := s.ident.oidc
	cfg := p.Config()
	secretSet := cfg.ClientSecret != ""
	v := providerView{
		Source: p.Source(),
		Name:   p.Name(),
		Settings: []providerSetting{
			{Key: "issuer", Value: cfg.Issuer},
			{Key: "clientId", Value: cfg.ClientID},
			{Key: "clientSecret", SecretSet: &secretSet},
			{Key: "redirectUrl", Value: s.publicURL(r) + oidcCallbackPath},
			{Key: "scopes", Value: strings.Join(cfg.Scopes, " ")},
			{Key: "usernameClaim", Value: cfg.UsernameClaim},
			{Key: "groupsClaim", Value: cfg.GroupsClaim},
			{Key: "allowedDomains", Value: strings.Join(cfg.AllowedDomains, ", ")},
			{Key: "caFile", Value: cfg.CAFile},
		},
	}
	ctx, cancel := context.WithTimeout(r.Context(), providerCheckTimeout)
	defer cancel()
	if err := p.Check(ctx); err != nil {
		v.Status.Error = err.Error()
	} else {
		v.Status.OK = true
	}
	return v
}

type mappingView struct {
	ID      int64  `json:"id"`
	Group   string `json:"group"`
	Source  string `json:"source"`
	Role    string `json:"role"`
	Cluster string `json:"cluster"`
	// ClusterExists is false for a mapping to a cluster the inventory does
	// not have (yet): it grants nothing until a cluster of that name exists.
	ClusterExists bool `json:"clusterExists"`
}

func (s *Server) handleListMappings(w http.ResponseWriter, _ *http.Request, _ *call) {
	mappings, err := s.store.ListGroupMappings()
	if err != nil {
		writeErr(w, err)
		return
	}
	known, err := s.clusterNames()
	if err != nil {
		writeErr(w, err)
		return
	}
	out := make([]mappingView, 0, len(mappings))
	for _, m := range mappings {
		out = append(out, mappingView{ID: m.ID, Group: m.Group, Source: m.Source, Role: m.Role, Cluster: m.Cluster,
			ClusterExists: m.Cluster == "" || known[m.Cluster]})
	}
	plugin.WriteJSON(w, out)
}

// clusterNames is the set of the inventory's cluster names.
func (s *Server) clusterNames() (map[string]bool, error) {
	clusters, err := s.store.ListClusters(&store.User{Role: store.RoleAdmin})
	if err != nil {
		return nil, err
	}
	names := make(map[string]bool, len(clusters))
	for _, c := range clusters {
		names[c.Name] = true
	}
	return names, nil
}

type mappingBody struct {
	Group   string `json:"group"`
	Source  string `json:"source"`
	Role    string `json:"role"`
	Cluster string `json:"cluster"`
}

// decodeMapping reads a mapping body and records it as the audit detail
// (groups, roles and cluster names are not secret).
func decodeMapping(w http.ResponseWriter, r *http.Request, c *call, id int64) (store.GroupMapping, error) {
	var body mappingBody
	if err := decodeJSON(w, r, &body); err != nil {
		return store.GroupMapping{}, err
	}
	m := store.GroupMapping{ID: id, Group: body.Group, Source: body.Source, Role: body.Role, Cluster: body.Cluster}.Normalize()
	c.setAudit("mapping/"+m.Group, auditJSON(map[string]string{"source": m.Source, "role": m.Role, "cluster": m.Cluster}))
	if err := m.Validate(); err != nil {
		return m, badRequest("%v", err)
	}
	return m, nil
}

func (s *Server) handleCreateMapping(w http.ResponseWriter, r *http.Request, c *call) {
	m, err := decodeMapping(w, r, c, 0)
	if err != nil {
		writeErr(w, err)
		return
	}
	id, err := s.store.CreateGroupMapping(m)
	if err != nil {
		writeErr(w, err)
		return
	}
	plugin.WriteJSON(w, map[string]int64{"id": id})
}

func (s *Server) handleUpdateMapping(w http.ResponseWriter, r *http.Request, c *call) {
	id, err := mappingID(r)
	if err != nil {
		writeErr(w, err)
		return
	}
	m, err := decodeMapping(w, r, c, id)
	if err != nil {
		writeErr(w, err)
		return
	}
	if err := s.store.UpdateGroupMapping(m); err != nil {
		writeErr(w, err)
		return
	}
	writeOK(w)
}

func (s *Server) handleDeleteMapping(w http.ResponseWriter, r *http.Request, c *call) {
	id, err := mappingID(r)
	if err != nil {
		writeErr(w, err)
		return
	}
	m, err := s.store.GetGroupMapping(id)
	if err != nil {
		writeErr(w, err)
		return
	}
	c.setAudit("mapping/"+m.Group, auditJSON(map[string]string{"source": m.Source, "role": m.Role, "cluster": m.Cluster}))
	if err := s.store.DeleteGroupMapping(id); err != nil {
		writeErr(w, err)
		return
	}
	writeOK(w)
}

func mappingID(r *http.Request) (int64, error) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		return 0, badRequest("invalid mapping id %q", r.PathValue("id"))
	}
	return id, nil
}

type previewGrant struct {
	Cluster string `json:"cluster"`
	Role    string `json:"role"`
	Exists  bool   `json:"exists"`
}

type previewView struct {
	Source     string         `json:"source"`
	Username   string         `json:"username"`
	Groups     []string       `json:"groups"`
	GroupsFrom string         `json:"groupsFrom"`
	Admin      bool           `json:"admin"`
	Clusters   []previewGrant `json:"clusters"`
	Matched    []int64        `json:"matched"`
	Allowed    bool           `json:"allowed"`
	Reason     string         `json:"reason"`
	// ExistingSource is the source of the account that has the username
	// ("" when there is none).
	ExistingSource string     `json:"existingSource"`
	LastSignIn     *time.Time `json:"lastSignIn"`
}

// handlePreviewSignIn shows what signing in would do for a user ("who
// would I be") without creating a user or a session.
func (s *Server) handlePreviewSignIn(w http.ResponseWriter, r *http.Request, c *call) {
	var body struct {
		Source   string    `json:"source"`
		Username string    `json:"username"`
		Groups   *[]string `json:"groups"`
	}
	if err := decodeJSON(w, r, &body); err != nil {
		writeErr(w, err)
		return
	}
	body.Username = strings.TrimSpace(body.Username)
	c.setAudit("user/"+body.Username, auditJSON(map[string]string{"source": body.Source}))
	if !store.ValidExternalSource(body.Source) {
		writeErr(w, badRequest("source must be oidc or ldap"))
		return
	}
	var groups []string
	if body.Groups != nil {
		groups = []string{}
		for _, g := range *body.Groups {
			if g = strings.TrimSpace(g); g != "" {
				groups = append(groups, g)
			}
		}
	}
	var lookup identity.Lookuper
	if p, ok := s.ident.passwords[body.Source].(identity.Lookuper); ok {
		lookup = p
	}
	p, err := s.ident.service.Preview(r.Context(), body.Source, body.Username, groups, lookup)
	if err != nil {
		writeErr(w, err)
		return
	}
	known, err := s.clusterNames()
	if err != nil {
		writeErr(w, err)
		return
	}
	view := previewView{
		Source: p.Source, Username: p.Username, Groups: nonNilStrings(p.Groups), GroupsFrom: p.GroupsFrom,
		Admin: p.Grants.Admin, Clusters: []previewGrant{}, Matched: p.Grants.Matched,
		Allowed: p.Allowed, Reason: p.Reason, LastSignIn: p.LastSignIn,
	}
	if view.Matched == nil {
		view.Matched = []int64{}
	}
	for cluster, role := range p.Grants.Clusters {
		view.Clusters = append(view.Clusters, previewGrant{Cluster: cluster, Role: role, Exists: known[cluster]})
	}
	slices.SortFunc(view.Clusters, func(a, b previewGrant) int { return strings.Compare(a.Cluster, b.Cluster) })
	if p.Existing != nil {
		view.ExistingSource = p.Existing.Source
	}
	plugin.WriteJSON(w, view)
}

func nonNilStrings(list []string) []string {
	if list == nil {
		return []string{}
	}
	return list
}
