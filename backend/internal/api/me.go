package api

import (
	"net/http"
	"strings"

	"kubebench/internal/plugin"
	"kubebench/internal/store"
)

// maxSeenVersionLen bounds users.last_seen_version.
const maxSeenVersionLen = 64

// meDetail is GET /api/auth/me: the session's user and the release whose
// What's new they last closed (§7.11).
type meDetail struct {
	meView
	LastSeenVersion string `json:"lastSeenVersion"`
}

func (s *Server) handleMe(w http.ResponseWriter, _ *http.Request, c *call) {
	s.writeMe(w, c.user)
}

func (s *Server) writeMe(w http.ResponseWriter, u *store.User) {
	seen, err := s.store.LastSeenVersion(u.ID)
	if err != nil {
		writeErr(w, err)
		return
	}
	plugin.WriteJSON(w, meDetail{meView: viewMe(u), LastSeenVersion: seen})
}

// handleUpdateMe stores per-user state of the caller: the release whose
// What's new they closed.
func (s *Server) handleUpdateMe(w http.ResponseWriter, r *http.Request, c *call) {
	var body struct {
		LastSeenVersion *string `json:"lastSeenVersion"`
	}
	if err := decodeJSON(w, r, &body); err != nil {
		writeErr(w, err)
		return
	}
	if body.LastSeenVersion == nil {
		writeErr(w, badRequest("lastSeenVersion is required"))
		return
	}
	seen := strings.TrimSpace(*body.LastSeenVersion)
	if seen == "" || len(seen) > maxSeenVersionLen {
		writeErr(w, badRequest("lastSeenVersion must be 1 to %d characters", maxSeenVersionLen))
		return
	}
	if err := s.store.SetLastSeenVersion(c.user.ID, seen); err != nil {
		writeErr(w, err)
		return
	}
	s.writeMe(w, c.user)
}
