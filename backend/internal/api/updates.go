package api

import (
	"net/http"

	"kubebench/internal/plugin"
	"kubebench/internal/reason"
	"kubebench/internal/store"
	"kubebench/internal/updates"
)

// updateStatus is the JSON of GET /api/updates (§7.11).
type updateStatus struct {
	updates.Status
	// DefaultMode is KUBEBENCH_UPDATE_MODE, the mode until an admin chooses.
	DefaultMode updates.Mode `json:"defaultMode"`
}

// updateMode is the admin's stored choice, else the configured default.
func (s *Server) updateMode() (updates.Mode, error) {
	stored, set, err := s.store.AppSetting(store.SettingUpdateMode)
	if err != nil {
		return "", err
	}
	if mode, ok := updates.ParseMode(stored); set && ok {
		return mode, nil
	}
	return s.defaultUpdateMode(), nil
}

func (s *Server) defaultUpdateMode() updates.Mode {
	if mode, ok := updates.ParseMode(s.cfg.UpdateMode); ok {
		return mode
	}
	return updates.ModeStable
}

func (s *Server) writeUpdateStatus(w http.ResponseWriter, st updates.Status) {
	plugin.WriteJSON(w, updateStatus{Status: st, DefaultMode: s.defaultUpdateMode()})
}

// handleUpdates reports the latest release, checking first when the cached
// result has expired.
func (s *Server) handleUpdates(w http.ResponseWriter, r *http.Request, _ *call) {
	mode, err := s.updateMode()
	if err != nil {
		writeErr(w, err)
		return
	}
	s.writeUpdateStatus(w, s.updates.Status(r.Context(), mode))
}

// handleCheckUpdates checks now (admins); refused while checks are off.
func (s *Server) handleCheckUpdates(w http.ResponseWriter, r *http.Request, _ *call) {
	mode, err := s.updateMode()
	if err != nil {
		writeErr(w, err)
		return
	}
	if mode == updates.ModeOff {
		writeErr(w, fail(http.StatusConflict, plugin.CodeConflict, reasonUpdatesOff, nil))
		return
	}
	s.writeUpdateStatus(w, s.updates.Check(r.Context(), mode))
}

// handleSetUpdateMode stores the admin's update mode and reports the
// status in that mode.
func (s *Server) handleSetUpdateMode(w http.ResponseWriter, r *http.Request, c *call) {
	var body struct {
		Mode string `json:"mode"`
	}
	if err := decodeJSON(w, r, &body); err != nil {
		writeErr(w, err)
		return
	}
	mode, ok := updates.ParseMode(body.Mode)
	if !ok {
		writeErr(w, badRequest(reason.FieldOneOf, reason.Params{"field": "mode", "values": "off, stable, prerelease"}))
		return
	}
	c.setAudit("setting/updates", auditJSON(map[string]string{"mode": string(mode)}))
	if err := s.store.SetAppSetting(store.SettingUpdateMode, string(mode)); err != nil {
		writeErr(w, err)
		return
	}
	s.writeUpdateStatus(w, s.updates.Status(r.Context(), mode))
}
