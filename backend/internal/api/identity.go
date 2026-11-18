package api

import (
	"errors"
	"log"
	"maps"
	"net/http"
	"slices"
	"strings"
	"time"

	"kubebench/internal/identity"
	"kubebench/internal/identity/oidcauth"
	"kubebench/internal/plugin"
	"kubebench/internal/store"
)

// External identities (docs/design-v0.5.md §3.6): the sign-in methods, the
// completion of an external sign-in shared by every provider, and its
// audit rows.

// identityState is the server's external-identity setup.
type identityState struct {
	service *identity.Service
	// oidc is the OpenID Connect provider; nil when not configured.
	oidc   *oidcauth.Provider
	logins *oidcauth.Logins
	// passwords are the providers that check a username and password
	// (a directory), by source.
	passwords map[string]identity.PasswordProvider
}

// initIdentity sets up the configured providers. A provider that cannot be
// built (its CA bundle vanished since the configuration was checked) is
// logged and left out; local sign-in keeps working.
func (s *Server) initIdentity() {
	s.ident = identityState{
		service:   identity.NewService(s.store, s.cfg.Auth.RequireGroup),
		logins:    oidcauth.NewLogins(time.Now),
		passwords: map[string]identity.PasswordProvider{},
	}
	if cfg := s.cfg.Auth.OIDC; cfg != nil {
		p, err := oidcauth.New(*cfg)
		if err != nil {
			log.Printf("[auth] OIDC sign-in is disabled: %v", err)
			return
		}
		s.ident.oidc = p
	}
}

// providers lists the configured external providers.
func (s *Server) providers() []identity.Provider {
	var out []identity.Provider
	if s.ident.oidc != nil {
		out = append(out, s.ident.oidc)
	}
	for _, source := range slices.Sorted(maps.Keys(s.ident.passwords)) {
		out = append(out, s.ident.passwords[source])
	}
	return out
}

// Sign-in method types of GET /api/auth/methods.
const (
	methodRedirect = "redirect" // the browser goes to loginUrl (OIDC)
	methodPassword = "password" // POST /api/auth/login with the source
)

type signInMethod struct {
	Source   string `json:"source"`
	Name     string `json:"name"`
	Type     string `json:"type"`
	LoginURL string `json:"loginUrl,omitempty"`
}

// handleAuthMethods tells the sign-in screen how users may sign in. Local
// accounts always can: the local admin is the break-glass account.
func (s *Server) handleAuthMethods(w http.ResponseWriter, _ *http.Request, _ *call) {
	methods := []signInMethod{}
	for _, p := range s.providers() {
		m := signInMethod{Source: p.Source(), Name: p.Name(), Type: methodPassword}
		if p.Source() == store.UserSourceOIDC {
			m.Type, m.LoginURL = methodRedirect, oidcLoginPath
		}
		methods = append(methods, m)
	}
	plugin.WriteJSON(w, map[string]any{"local": true, "providers": methods})
}

// signInExternal authenticates through a password provider for
// POST /api/auth/login {"source": …}. Failures count against the login
// limit like local ones.
func (s *Server) signInExternal(w http.ResponseWriter, r *http.Request, source, username, password string) {
	p, ok := s.ident.passwords[source]
	if !ok {
		writeErr(w, badRequest("unknown sign-in source %q", source))
		return
	}
	key := newLimiterKey(s.clientIP(r), source+"\x00"+username)
	if err := s.throttled(w, key); err != nil {
		s.auditSignIn(source, username, http.StatusTooManyRequests, identity.ClassRateLimited)
		writeErr(w, err)
		return
	}
	user, err := s.authenticateExternal(r, p, username, password)
	if err != nil {
		s.limiter.fail(key)
		status, apiErr := signInError(err)
		s.auditSignIn(source, username, status, identity.ClassOf(err))
		writeErr(w, apiErr)
		return
	}
	s.limiter.reset(key)
	token, err := s.store.CreateSession(user.ID, s.cfg.SessionTTL)
	if err != nil {
		writeErr(w, err)
		return
	}
	s.auditSignIn(source, user.Username, http.StatusOK, "")
	http.SetCookie(w, s.sessionCookie(r, token, int(s.cfg.SessionTTL.Seconds())))
	plugin.WriteJSON(w, viewMe(user))
}

func (s *Server) authenticateExternal(r *http.Request, p identity.PasswordProvider, username, password string) (*store.User, error) {
	id, err := p.Authenticate(r.Context(), username, password)
	if err != nil {
		if f, ok := errors.AsType[*identity.Failure](err); ok {
			s.ident.service.Refused(p.Source(), f)
		}
		return nil, err
	}
	return s.ident.service.SignIn(id)
}

// signInMessages explain refused external sign-ins to the user. The cause
// stays in the server log: it may describe the directory or the provider.
var signInMessages = map[string]string{
	identity.ClassInvalidCredentials: "invalid username or password",
	identity.ClassAccountDisabled:    "this account is disabled",
	identity.ClassDomainNotAllowed:   "this email domain may not sign in",
	identity.ClassNoMatchingGroup:    "no group of this account grants access to KubeBench",
	identity.ClassUsernameConflict:   "the username belongs to another KubeBench account",
	identity.ClassInvalidUsername:    "the account's username cannot be used in KubeBench",
	identity.ClassInvalidRequest:     "the sign-in could not be verified; try again",
	identity.ClassProviderError:      "the identity provider could not be reached",
}

// signInError is the status and API error of a refused external sign-in;
// it logs the cause.
func signInError(err error) (int, error) {
	class := identity.ClassOf(err)
	log.Printf("[auth] external sign-in refused: %v", err)
	switch class {
	case identity.ClassInvalidCredentials:
		return http.StatusUnauthorized, errorf(http.StatusUnauthorized, plugin.CodeUnauthenticated, "%s", signInMessages[class])
	case identity.ClassProviderError:
		return http.StatusBadGateway, errorf(http.StatusBadGateway, plugin.CodeInternal, "%s", signInMessages[class])
	}
	msg, ok := signInMessages[class]
	if !ok {
		msg = "the sign-in was refused"
	}
	return http.StatusForbidden, errorf(http.StatusForbidden, plugin.CodeForbidden, "%s", msg)
}

// Audit action of external sign-ins.
const auditSignInAction = "auth.signin"

// auditSignIn records an external sign-in: the source and, for a refused
// one, the failure class — never what the user typed or the provider's
// tokens. username is what the user or the provider named; "" when the
// sign-in failed before the provider named one.
func (s *Server) auditSignIn(source, username string, status int, class string) {
	detail := map[string]string{"source": source}
	if class != "" {
		detail["reason"] = class
	}
	target := ""
	if username != "" {
		target = "user/" + strings.TrimSpace(username)
	}
	s.writeAudit(store.AuditEntry{
		Username: username,
		Action:   auditSignInAction,
		Target:   target,
		Status:   status,
		Detail:   auditJSON(detail),
	})
}

// errExternalPassword refuses password changes of external users, whose
// password belongs to their identity provider.
var errExternalPassword = errorf(http.StatusBadRequest, plugin.CodeBadRequest,
	"this account signs in through an identity provider; its password is managed there")
