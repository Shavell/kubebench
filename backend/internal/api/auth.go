package api

import (
	"errors"
	"math"
	"net/http"
	"strconv"
	"strings"

	"kubebench/internal/config"
	"kubebench/internal/password"
	"kubebench/internal/plugin"
	"kubebench/internal/reason"
	"kubebench/internal/store"
)

const sessionCookie = "kubebench_session"

// validatePassword enforces the password policy for new passwords.
func validatePassword(pw string) error {
	if err := password.Check(pw); err != nil {
		return refuse(http.StatusBadRequest, plugin.CodeBadRequest, err)
	}
	return nil
}

type meView struct {
	ID                 int64  `json:"id"`
	Username           string `json:"username"`
	Role               string `json:"role"`
	MustChangePassword bool   `json:"mustChangePassword"`
	// Source is where the account comes from: local, oidc or ldap (§3.6).
	Source      string `json:"source"`
	DisplayName string `json:"displayName"`
}

func viewMe(u *store.User) meView {
	return meView{ID: u.ID, Username: u.Username, Role: u.Role, MustChangePassword: u.MustChangePassword,
		Source: u.Source, DisplayName: u.DisplayName}
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request, _ *call) {
	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
		// Source names a password provider (a directory, §3.6); empty or
		// "local" signs in with a local account.
		Source string `json:"source"`
	}
	if err := decodeJSON(w, r, &body); err != nil {
		writeErr(w, err)
		return
	}
	if body.Source != "" && body.Source != store.UserSourceLocal {
		s.signInExternal(w, r, body.Source, body.Username, body.Password)
		return
	}
	key := newLimiterKey(s.clientIP(r), body.Username)
	if err := s.throttled(w, key); err != nil {
		writeErr(w, err)
		return
	}
	user, err := s.store.Authenticate(body.Username, body.Password)
	if errors.Is(err, store.ErrNotFound) {
		s.limiter.fail(key)
		writeErr(w, fail(http.StatusUnauthorized, plugin.CodeUnauthenticated, reasonInvalidCredentials, nil))
		return
	}
	if err != nil {
		writeErr(w, err)
		return
	}
	s.limiter.reset(key)
	// A user still signing in with the well-known bootstrap password (for
	// example an install upgraded from v0.4, which had no forced change)
	// must pick a new one before doing anything else (§3.5).
	if body.Password == config.DefaultAdminPassword && !user.MustChangePassword {
		if err := s.store.SetMustChangePassword(user.ID, true); err != nil {
			writeErr(w, err)
			return
		}
		user.MustChangePassword = true
	}
	token, err := s.store.CreateSession(user.ID, s.cfg.SessionTTL)
	if err != nil {
		writeErr(w, err)
		return
	}
	http.SetCookie(w, s.sessionCookie(r, token, int(s.cfg.SessionTTL.Seconds())))
	plugin.WriteJSON(w, viewMe(user))
}

// throttled refuses an attempt for a key with too many recent failures.
func (s *Server) throttled(w http.ResponseWriter, key limiterKey) error {
	wait := s.limiter.blocked(key)
	if wait <= 0 {
		return nil
	}
	w.Header().Set("Retry-After", strconv.Itoa(int(math.Ceil(wait.Seconds()))))
	return fail(http.StatusTooManyRequests, plugin.CodeRateLimited, reasonTooManyAttempts,
		reason.Params{"minutes": int(math.Ceil(wait.Minutes()))})
}

// handleLogout ends the session. For an OIDC session whose provider
// supports RP-initiated logout, logoutUrl is where the browser goes next
// to end the provider's session too (§3.6).
func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request, c *call) {
	logoutURL := s.oidcLogoutURL(r, c.user, c.token)
	if err := s.store.DeleteSession(c.token); err != nil {
		writeErr(w, err)
		return
	}
	http.SetCookie(w, s.sessionCookie(r, "", -1))
	resp := map[string]any{"ok": true}
	if logoutURL != "" {
		resp["logoutUrl"] = logoutURL
	}
	plugin.WriteJSON(w, resp)
}

// handleChangePassword sets the caller's own password (§4.2). It clears a
// forced change and signs out the caller's other sessions.
func (s *Server) handleChangePassword(w http.ResponseWriter, r *http.Request, c *call) {
	var body struct {
		CurrentPassword string `json:"currentPassword"`
		NewPassword     string `json:"newPassword"`
	}
	if err := decodeJSON(w, r, &body); err != nil {
		writeErr(w, err)
		return
	}
	if c.user.IsExternal() {
		writeErr(w, errExternalPassword)
		return
	}
	if err := validatePassword(body.NewPassword); err != nil {
		writeErr(w, err)
		return
	}
	// Guessing the current password is throttled like logins.
	key := newLimiterKey(s.clientIP(r), c.user.Username)
	if err := s.throttled(w, key); err != nil {
		writeErr(w, err)
		return
	}
	ok, err := s.store.CheckPassword(c.user.ID, body.CurrentPassword)
	if err != nil {
		writeErr(w, err)
		return
	}
	if !ok {
		s.limiter.fail(key)
		writeErr(w, fail(http.StatusForbidden, plugin.CodeForbidden, reasonWrongPassword, nil))
		return
	}
	if body.NewPassword == body.CurrentPassword {
		writeErr(w, badRequest(reasonSamePassword, nil))
		return
	}
	s.limiter.reset(key)
	if err := s.store.SetPassword(c.user.ID, body.NewPassword, false, c.token); err != nil {
		writeErr(w, err)
		return
	}
	writeOK(w)
}

// sessionCookie builds the session cookie; maxAge < 0 deletes it.
func (s *Server) sessionCookie(r *http.Request, value string, maxAge int) *http.Cookie {
	return &http.Cookie{
		Name:     sessionCookie,
		Value:    value,
		Path:     "/",
		MaxAge:   maxAge,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   s.secureRequest(r),
	}
}

// secureRequest decides the cookie's Secure flag (KUBEBENCH_SECURE_COOKIES):
// in auto mode a request counts as secure when it arrived over TLS or, from
// a trusted proxy, with X-Forwarded-Proto: https.
func (s *Server) secureRequest(r *http.Request) bool {
	switch s.cfg.SecureCookies {
	case config.CookiesSecure:
		return true
	case config.CookiesPlain:
		return false
	}
	if r.TLS != nil {
		return true
	}
	return s.cfg.TrustProxy && strings.EqualFold(firstHeaderValue(r.Header.Get("X-Forwarded-Proto")), "https")
}
