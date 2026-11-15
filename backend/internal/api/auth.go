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
	"kubebench/internal/store"
)

const sessionCookie = "kubebench_session"

// validatePassword enforces the password policy for new passwords.
func validatePassword(pw string) error {
	if err := password.Check(pw); err != nil {
		return badRequest("the password %v", err)
	}
	return nil
}

type meView struct {
	ID                 int64  `json:"id"`
	Username           string `json:"username"`
	Role               string `json:"role"`
	MustChangePassword bool   `json:"mustChangePassword"`
}

func viewMe(u *store.User) meView {
	return meView{ID: u.ID, Username: u.Username, Role: u.Role, MustChangePassword: u.MustChangePassword}
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request, _ *call) {
	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := decodeJSON(w, r, &body); err != nil {
		writeErr(w, err)
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
		writeErr(w, errorf(http.StatusUnauthorized, plugin.CodeUnauthenticated, "invalid username or password"))
		return
	}
	if err != nil {
		writeErr(w, err)
		return
	}
	s.limiter.reset(key)
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
	return errorf(http.StatusTooManyRequests, plugin.CodeRateLimited,
		"too many failed attempts; try again in %d minutes", int(math.Ceil(wait.Minutes())))
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request, c *call) {
	if err := s.store.DeleteSession(c.token); err != nil {
		writeErr(w, err)
		return
	}
	http.SetCookie(w, s.sessionCookie(r, "", -1))
	writeOK(w)
}

func (s *Server) handleMe(w http.ResponseWriter, _ *http.Request, c *call) {
	plugin.WriteJSON(w, viewMe(c.user))
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
		writeErr(w, errorf(http.StatusForbidden, plugin.CodeForbidden, "the current password is incorrect"))
		return
	}
	if body.NewPassword == body.CurrentPassword {
		writeErr(w, badRequest("the new password must differ from the current one"))
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
