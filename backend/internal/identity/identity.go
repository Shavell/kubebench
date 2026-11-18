// Package identity is KubeBench's external-identity layer
// (docs/design-v0.5.md §3.6): identity providers (OIDC, LDAP) authenticate
// a user and report who they are; this package maps their groups to
// KubeBench roles and turns them into store users, created at the first
// sign-in and updated at every one after.
package identity

import (
	"context"
	"errors"
	"fmt"
)

// Identity is a user as an identity provider reports them.
type Identity struct {
	Source      string // store.UserSourceOIDC | store.UserSourceLDAP
	Subject     string // stable ID at the source (OIDC sub, directory entry ID)
	Username    string
	DisplayName string
	Email       string
	Groups      []string
}

// Provider is an identity provider users can sign in with.
type Provider interface {
	// Source is the store user source of its users (oidc, ldap).
	Source() string
	// Name is the provider's display name ("Sign in with <Name>").
	Name() string
}

// PasswordProvider authenticates a username and password, e.g. against a
// directory. It returns a *Failure for refused sign-ins.
type PasswordProvider interface {
	Provider
	Authenticate(ctx context.Context, username, password string) (*Identity, error)
}

// Lookuper finds a user without their password (with a service account),
// for the admins' "who would I be" preview. Optional.
type Lookuper interface {
	Lookup(ctx context.Context, username string) (*Identity, error)
}

// Failure classes: why an external sign-in was refused. They are recorded
// in the audit log and shown on the sign-in screen; the details stay in
// the server log.
const (
	ClassInvalidCredentials = "invalid_credentials"
	ClassAccountDisabled    = "account_disabled"
	ClassDomainNotAllowed   = "domain_not_allowed"
	ClassNoMatchingGroup    = "no_matching_group"
	ClassUsernameConflict   = "username_conflict"
	ClassInvalidUsername    = "invalid_username"
	// ClassInvalidRequest is a callback that does not belong to a sign-in
	// this browser started (state, nonce or PKCE mismatch, expired).
	ClassInvalidRequest = "invalid_request"
	// ClassProviderError is a provider that cannot be reached or answered
	// with an error.
	ClassProviderError = "provider_error"
	ClassRateLimited   = "rate_limited"
)

// Failure is a refused sign-in: a class for the audit log and the user,
// and the cause for the server log.
type Failure struct {
	Class string
	Err   error
	// Subject identifies the refused account when the provider knows it
	// (an account found disabled), so its sessions can be ended.
	Subject string
}

func (f *Failure) Error() string {
	if f.Err == nil {
		return f.Class
	}
	return f.Class + ": " + f.Err.Error()
}

func (f *Failure) Unwrap() error { return f.Err }

// Fail returns a *Failure of class with a formatted cause.
func Fail(class, format string, args ...any) *Failure {
	return &Failure{Class: class, Err: fmt.Errorf(format, args...)}
}

// ClassOf is the failure class of err; errors that are not a *Failure are
// provider errors.
func ClassOf(err error) string {
	if f, ok := errors.AsType[*Failure](err); ok {
		return f.Class
	}
	return ClassProviderError
}
