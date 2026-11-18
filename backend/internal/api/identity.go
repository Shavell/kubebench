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
	"kubebench/internal/identity/ldapauth"
	"kubebench/internal/identity/oidcauth"
	"kubebench/internal/plugin"
	"kubebench/internal/reason"
	"kubebench/internal/store"
)

// External identities (docs/design-v0.5.md §3.6): the sign-in methods, the
// completion of an external sign-in shared by every provider, and its
// audit rows.

// identityState is the server's external-identity setup.
type identityState struct {
	service *identity.Service
	// oidc is the OpenID Connect provider; nil when not configured.
	oidc *oidcauth.Provider
	// logins seals pending OIDC sign-ins into the state cookie.
	logins *oidcauth.Sealer
	// passwords are the providers that check a username and password
	// (a directory), by source.
	passwords map[string]identity.PasswordProvider
}

// initIdentity sets up the configured providers. A provider that cannot be
// built (its CA bundle vanished since the configuration was checked) is
// logged and left out; local sign-in and the other providers keep working.
func (s *Server) initIdentity() {
	s.ident = identityState{
		service:   identity.NewService(s.store, s.cfg.Auth.RequireGroup),
		logins:    oidcauth.NewSealer(time.Now),
		passwords: map[string]identity.PasswordProvider{},
	}
	if cfg := s.cfg.Auth.OIDC; cfg != nil {
		if p, err := oidcauth.New(*cfg); err != nil {
			log.Printf("[auth] OIDC sign-in is disabled: %v", err)
		} else {
			s.ident.oidc = p
		}
	}
	if cfg := s.cfg.Auth.LDAP; cfg != nil {
		if p, err := ldapauth.New(*cfg); err != nil {
			log.Printf("[auth] directory sign-in is disabled: %v", err)
		} else {
			s.ident.passwords[p.Source()] = p
		}
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
// limit like local ones, by the provider's normalized name so the forms
// of one account (DOMAIN\user, User, user) share their attempts.
func (s *Server) signInExternal(w http.ResponseWriter, r *http.Request, source, username, password string) {
	p, ok := s.ident.passwords[source]
	if !ok {
		writeErr(w, badRequest(reasonUnknownSource, reason.Params{"source": source}))
		return
	}
	username = normalizeLogin(p, username)
	key := newLimiterKey(s.clientIP(r), source+"\x00"+username)
	if err := s.throttled(w, key); err != nil {
		s.auditSignIn(source, username, http.StatusTooManyRequests, identity.ClassRateLimited)
		writeErr(w, err)
		return
	}
	user, err := s.authenticateExternal(r, p, username, password)
	if err != nil {
		s.limiter.fail(key)
		refusal := signInError(err)
		s.auditSignIn(source, username, refusal.status, identity.ClassOf(err))
		writeErr(w, refusal)
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

// normalizeLogin is the name provider p searches a login by.
func normalizeLogin(p identity.Provider, login string) string {
	if n, ok := p.(identity.UsernameNormalizer); ok {
		return n.NormalizeUsername(login)
	}
	return strings.TrimSpace(login)
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

// signInReasons explain refused external sign-ins to the user by their
// failure class. The cause stays in the server log: it may describe the
// directory or the provider.
var signInReasons = map[string]reason.Code{
	identity.ClassInvalidCredentials: reasonSignInCredentials,
	identity.ClassAccountDisabled:    reasonSignInDisabled,
	identity.ClassDomainNotAllowed:   reasonSignInDomain,
	identity.ClassNoMatchingGroup:    reasonSignInNoGroup,
	identity.ClassUsernameConflict:   reasonSignInConflict,
	identity.ClassInvalidUsername:    reasonSignInUsername,
	identity.ClassInvalidRequest:     reasonSignInRequest,
	identity.ClassProviderError:      reasonSignInProviderError,
}

// signInError is the API error of a refused external sign-in; its reason
// (auth.signin.<class>) lets the sign-in screen translate the failure
// class. It logs the cause.
func signInError(err error) *apiError {
	class := identity.ClassOf(err)
	log.Printf("[auth] external sign-in refused: %v", err)
	rc, ok := signInReasons[class]
	if !ok {
		rc = reasonSignInRefused
	}
	switch class {
	case identity.ClassInvalidCredentials:
		return fail(http.StatusUnauthorized, plugin.CodeUnauthenticated, rc, nil)
	case identity.ClassProviderError:
		return fail(http.StatusBadGateway, plugin.CodeInternal, rc, nil)
	}
	return fail(http.StatusForbidden, plugin.CodeForbidden, rc, nil)
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
var errExternalPassword = badRequest(reasonExternalPassword, nil)
