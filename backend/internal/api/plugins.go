package api

import (
	"net/http"

	"kubebench/internal/kube"
	"kubebench/internal/plugin"
	"kubebench/internal/reason"
	"kubebench/internal/store"
)

type pluginView struct {
	ID          string        `json:"id"`
	Name        string        `json:"name"`
	Version     string        `json:"version"`
	Description string        `json:"description"`
	Category    string        `json:"category"`
	Builtin     bool          `json:"builtin"`
	External    bool          `json:"external"`
	Enabled     bool          `json:"enabled"`
	Status      plugin.Status `json:"status"`
	Entry       string        `json:"entry"` // script URL of an external plugin
}

type pluginSetting struct {
	Enabled bool `json:"enabled"`
}

type pluginCatalog struct {
	Plugins []pluginView `json:"plugins"`
	// Settings holds the admin's choices for any plugin id, including
	// frontend-only plugins the backend does not know.
	Settings map[string]pluginSetting `json:"settings"`
}

// handleListPlugins lists the backend and external plugins with their
// availability on the caller's active cluster (§4.6).
func (s *Server) handleListPlugins(w http.ResponseWriter, _ *http.Request, c *call) {
	settings, err := s.store.PluginSettings()
	if err != nil {
		writeErr(w, err)
		return
	}
	catalog := pluginCatalog{Plugins: []pluginView{}, Settings: map[string]pluginSetting{}}
	for id, enabled := range settings {
		catalog.Settings[id] = pluginSetting{Enabled: enabled}
	}

	rt, clusterErr := s.statusRuntime(c.user)
	for _, p := range s.plugins {
		m := p.Manifest()
		enabled, set := settings[m.ID]
		view := pluginView{
			ID: m.ID, Name: m.Name, Version: m.Version, Description: m.Description, Category: m.Category,
			Builtin: true, Enabled: enabled || !set, // built-in plugins default to enabled
		}
		if clusterErr != nil {
			view.Status = plugin.UnavailableFrom(clusterErr)
		} else {
			view.Status = s.detect.status(p, rt)
		}
		catalog.Plugins = append(catalog.Plugins, view)
	}
	for _, ext := range s.externalPlugins() {
		catalog.Plugins = append(catalog.Plugins, pluginView{
			ID: ext.ID, Name: ext.Name, Version: ext.Version, Description: ext.Description, Category: ext.Category,
			External: true,
			Enabled:  settings[ext.ID], // external plugins stay off until an admin enables them
			Status:   plugin.Status{Available: true},
			Entry:    "/plugins/" + ext.ID + "/" + ext.Entry,
		})
	}
	plugin.WriteJSON(w, catalog)
}

// statusRuntime is the runtime plugin availability is reported for.
func (s *Server) statusRuntime(user *store.User) (*kube.Runtime, error) {
	cluster, _, err := s.activeCluster(user)
	if err != nil {
		return nil, err
	}
	return s.manager.Runtime(cluster)
}

// handleSetPluginEnabled stores the admin's choice for any plugin id.
func (s *Server) handleSetPluginEnabled(w http.ResponseWriter, r *http.Request, c *call) {
	id := r.PathValue("id")
	if !plugin.ValidID(id) {
		writeErr(w, badRequest(reasonPluginID, reason.Params{"id": id}))
		return
	}
	var body struct {
		Enabled *bool `json:"enabled"`
	}
	if err := decodeJSON(w, r, &body); err != nil {
		writeErr(w, err)
		return
	}
	if body.Enabled == nil {
		writeErr(w, badRequest(reason.FieldRequired, reason.Params{"field": "enabled"}))
		return
	}
	c.setAudit("", auditJSON(map[string]bool{"enabled": *body.Enabled}))
	if err := s.store.SetPluginEnabled(id, *body.Enabled); err != nil {
		writeErr(w, err)
		return
	}
	writeOK(w)
}

// checkPluginEnabled refuses requests to a plugin an admin disabled.
func (s *Server) checkPluginEnabled(p plugin.Plugin) error {
	id := p.Manifest().ID
	enabled, set, err := s.store.PluginEnabled(id)
	if err != nil {
		return err
	}
	if set && !enabled {
		return fail(http.StatusNotFound, plugin.CodePluginDisabled, reasonPluginDisabled, reason.Params{"id": id})
	}
	return nil
}
