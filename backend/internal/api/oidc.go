package api

import (
	"crypto/subtle"
	"errors"
	"log"
	"net/http"
	"net/url"
	"strings"

	"kubebench/internal/identity"
	"kubebench/internal/plugin"
	"kubebench/internal/store"
)

// OpenID Connect sign-in (docs/design-v0.5.md §3.6): /login sends the
// browser to the provider, the provider sends it back to /callback.
const (
	oidcLoginPath    = "/api/auth/oidc/login"
	oidcCallbackPath = "/api/auth/oidc/callback"
	// oidcStateCookie binds a sign-in to the browser that started it, so a
	// callback URL planted on another browser (login CSRF) is refused.
	oidcStateCookie = "kubebench_oidc_state"
	// signInErrorParam carries the failure class of a refused sign-in to
	// the sign-in screen.
	signInErrorParam = "signInError"
)

// oidcCallbackKey is the login limiter's username for OIDC callbacks,
// whose user is unknown until the provider's token is verified: failed
// callbacks are throttled per client IP. The NUL keeps it apart from
// every username.
const oidcCallbackKey = "\x00oidc-callback"

func (s *Server) handleOIDCLogin(w http.ResponseWriter, r *http.Request, _ *call) {
	if s.ident.oidc == nil {
		writeErr(w, errorf(http.StatusNotFound, plugin.CodeNotFound, "OIDC sign-in is not configured"))
		return
	}
	base := s.publicURL(r)
	login, authURL, err := s.ident.oidc.Start(r.Context(), base+oidcCallbackPath)
	if err != nil {
		log.Printf("[auth] OIDC sign-in cannot start: %v", err)
		s.auditSignIn(store.UserSourceOIDC, "", http.StatusBadGateway, identity.ClassOf(err))
		redirectSignInError(w, r, base, identity.ClassOf(err))
		return
	}
	s.ident.logins.Put(login)
	http.SetCookie(w, &http.Cookie{
		Name:     oidcStateCookie,
		Value:    login.State,
		Path:     "/api/auth/oidc/",
		MaxAge:   600,
		HttpOnly: true,
		// The callback is a top-level navigation from the provider, which
		// a Lax cookie accompanies.
		SameSite: http.SameSiteLaxMode,
		Secure:   s.secureRequest(r),
	})
	http.Redirect(w, r, authURL, http.StatusFound)
}

// handleOIDCCallback completes a sign-in the provider sent back: it checks
// the state against the browser's cookie and the pending sign-in, redeems
// the code and signs the user in. Every outcome is audited.
func (s *Server) handleOIDCCallback(w http.ResponseWriter, r *http.Request, _ *call) {
	if s.ident.oidc == nil {
		writeErr(w, errorf(http.StatusNotFound, plugin.CodeNotFound, "OIDC sign-in is not configured"))
		return
	}
	base := s.publicURL(r)
	http.SetCookie(w, &http.Cookie{Name: oidcStateCookie, Path: "/api/auth/oidc/", MaxAge: -1,
		HttpOnly: true, SameSite: http.SameSiteLaxMode, Secure: s.secureRequest(r)})
	key := newLimiterKey(s.clientIP(r), oidcCallbackKey)
	if err := s.throttled(w, key); err != nil {
		s.auditSignIn(store.UserSourceOIDC, "", http.StatusTooManyRequests, identity.ClassRateLimited)
		redirectSignInError(w, r, base, identity.ClassRateLimited)
		return
	}
	user, username, idToken, err := s.finishOIDC(r)
	if err != nil {
		s.limiter.fail(key)
		status, _ := signInError(err)
		s.auditSignIn(store.UserSourceOIDC, username, status, identity.ClassOf(err))
		redirectSignInError(w, r, base, identity.ClassOf(err))
		return
	}
	s.limiter.reset(key)
	token, err := s.store.CreateExternalSession(user.ID, s.cfg.SessionTTL, idToken)
	if err != nil {
		log.Printf("[auth] create the session of %q: %v", user.Username, err)
		redirectSignInError(w, r, base, identity.ClassProviderError)
		return
	}
	s.auditSignIn(store.UserSourceOIDC, user.Username, http.StatusOK, "")
	http.SetCookie(w, s.sessionCookie(r, token, int(s.cfg.SessionTTL.Seconds())))
	http.Redirect(w, r, base+"/", http.StatusFound)
}

// finishOIDC validates the callback and signs its user in. username is
// the one the provider named, also when the sign-in is refused afterwards
// (for the audit row).
func (s *Server) finishOIDC(r *http.Request) (user *store.User, username, idToken string, err error) {
	q := r.URL.Query()
	if e := q.Get("error"); e != "" {
		return nil, "", "", identity.Fail(identity.ClassProviderError, "the provider refused the sign-in: %s %s", e, q.Get("error_description"))
	}
	state := q.Get("state")
	cookie, err := r.Cookie(oidcStateCookie)
	if err != nil || state == "" || subtle.ConstantTimeCompare([]byte(cookie.Value), []byte(state)) != 1 {
		return nil, "", "", identity.Fail(identity.ClassInvalidRequest, "the callback's state does not match this browser's sign-in")
	}
	login, ok := s.ident.logins.Take(state)
	if !ok {
		return nil, "", "", identity.Fail(identity.ClassInvalidRequest, "the sign-in expired or was already completed")
	}
	id, idToken, err := s.ident.oidc.Finish(r.Context(), login, q.Get("code"))
	if err != nil {
		if f, ok := errors.AsType[*identity.Failure](err); ok {
			s.ident.service.Refused(store.UserSourceOIDC, f)
		}
		return nil, "", "", err
	}
	user, err = s.ident.service.SignIn(id)
	return user, id.Username, idToken, err
}

// redirectSignInError returns the browser to the sign-in screen with the
// failure class.
func redirectSignInError(w http.ResponseWriter, r *http.Request, base, class string) {
	http.Redirect(w, r, base+"/?"+signInErrorParam+"="+url.QueryEscape(class), http.StatusFound)
}

// publicURL is the URL users reach KubeBench at: KUBEBENCH_PUBLIC_URL or,
// when unset, the request's scheme and host (behind a trusted proxy the
// forwarded ones).
func (s *Server) publicURL(r *http.Request) string {
	if s.cfg.PublicURL != "" {
		return s.cfg.PublicURL
	}
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	host := r.Host
	if s.cfg.TrustProxy {
		if proto := strings.ToLower(firstHeaderValue(r.Header.Get("X-Forwarded-Proto"))); proto == "https" || proto == "http" {
			scheme = proto
		}
		if forwarded := firstHeaderValue(r.Header.Get("X-Forwarded-Host")); forwarded != "" {
			host = forwarded
		}
	}
	return scheme + "://" + host
}

// oidcLogoutURL is where the browser ends the provider's session after
// signing out of an OIDC session; "" when there is none to end.
func (s *Server) oidcLogoutURL(r *http.Request, user *store.User, sessionToken string) string {
	if s.ident.oidc == nil || user.Source != store.UserSourceOIDC {
		return ""
	}
	idToken, err := s.store.SessionIDToken(sessionToken)
	if err != nil {
		log.Printf("[auth] read the ID token of a session: %v", err)
	}
	return s.ident.oidc.LogoutURL(r.Context(), idToken, s.publicURL(r)+"/")
}
