package identity

import (
	"context"
	"errors"
	"fmt"
	"time"

	"kubebench/internal/store"
)

// Service turns identities that providers authenticated into KubeBench
// users (§3.6). It is shared by every provider.
type Service struct {
	store        *store.Store
	requireGroup bool
}

// NewService returns the sign-in service. With requireGroup, users whose
// groups grant nothing are refused instead of signing in without access.
func NewService(st *store.Store, requireGroup bool) *Service {
	return &Service{store: st, requireGroup: requireGroup}
}

// RequireGroup reports whether users without a granting group are refused.
func (s *Service) RequireGroup() bool { return s.requireGroup }

// SignIn creates or updates the user of an authenticated identity, with
// the global role and cluster roles its groups grant now. Mappings are
// re-applied at every sign-in and replace manual changes. A refusal is a
// *Failure; refused accounts lose their sessions.
func (s *Service) SignIn(id *Identity) (*store.User, error) {
	if !store.ValidExternalSource(id.Source) || id.Subject == "" {
		return nil, fmt.Errorf("provider returned an incomplete identity (source %q)", id.Source)
	}
	if !store.ValidUsername(id.Username) {
		return nil, s.refuse(id, Fail(ClassInvalidUsername,
			"username %q is not 1-64 letters, digits, '.', '_', '@' or '-'", id.Username))
	}
	mappings, err := s.store.ListGroupMappings()
	if err != nil {
		return nil, err
	}
	grants := Resolve(id.Source, id.Groups, mappings)
	if s.requireGroup && !grants.Any() {
		return nil, s.refuse(id, Fail(ClassNoMatchingGroup, "no group of %q is mapped to a role", id.Username))
	}
	user, err := s.store.SignInExternal(store.ExternalAccount{
		Source:       id.Source,
		Subject:      id.Subject,
		Username:     id.Username,
		DisplayName:  id.DisplayName,
		Email:        id.Email,
		Groups:       id.Groups,
		Role:         grants.Role(),
		ClusterRoles: grants.Clusters,
	})
	if errors.Is(err, store.ErrUsernameTaken) {
		return nil, Fail(ClassUsernameConflict, "username %q belongs to another account", id.Username)
	}
	return user, err
}

// refuse ends the sessions of a refused identity's account and returns f.
func (s *Service) refuse(id *Identity, f *Failure) *Failure {
	f.Subject = id.Subject
	s.Refused(id.Source, f)
	return f
}

// Refused records a refused sign-in: an account the provider knows
// (f.Subject set, e.g. one found disabled) loses its sessions, so access
// ends at the next sign-in attempt rather than when the session expires.
func (s *Service) Refused(source string, f *Failure) {
	if f == nil || f.Subject == "" {
		return
	}
	if _, err := s.store.RevokeExternalSessions(source, f.Subject); err != nil {
		f.Err = errors.Join(f.Err, fmt.Errorf("end sessions: %w", err))
	}
}

// Where the groups of a preview come from.
const (
	GroupsFromInput      = "input"      // given by the admin
	GroupsFromDirectory  = "directory"  // looked up at the provider
	GroupsFromLastSignIn = "lastSignIn" // the user's last sign-in
	GroupsFromNone       = "none"       // unknown user, no groups given
)

// Preview is what signing in would do for a user, computed without
// creating a user or a session ("who would I be").
type Preview struct {
	Source     string
	Username   string
	Groups     []string
	GroupsFrom string
	Grants     Grants
	// Allowed reports whether the sign-in would succeed; Reason is the
	// failure class when not.
	Allowed bool
	Reason  string
	// Existing is the account the username belongs to, if any.
	Existing   *store.User
	LastSignIn *time.Time
}

// Preview resolves the mappings for username at a provider of source.
// groups, when non-nil, are used as given; otherwise lookup (if the
// provider supports it) asks the provider, and else the groups of the
// user's last sign-in are used.
func (s *Service) Preview(ctx context.Context, source, username string, groups []string, lookup Lookuper) (*Preview, error) {
	p := &Preview{Source: source, Username: username, Groups: groups, GroupsFrom: GroupsFromInput}
	if !store.ValidUsername(username) {
		p.Reason = ClassInvalidUsername
		return p, nil
	}
	existing, err := s.store.UserByUsername(username)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		return nil, err
	}
	p.Existing = existing
	if groups == nil {
		if err := s.previewGroups(ctx, p, lookup); err != nil {
			return nil, err
		}
	}
	mappings, err := s.store.ListGroupMappings()
	if err != nil {
		return nil, err
	}
	p.Grants = Resolve(source, p.Groups, mappings)
	switch {
	case p.Reason != "":
	case existing != nil && existing.Source != source:
		p.Reason = ClassUsernameConflict
	case s.requireGroup && !p.Grants.Any():
		p.Reason = ClassNoMatchingGroup
	}
	p.Allowed = p.Reason == ""
	return p, nil
}

// previewGroups fills in the groups of a preview that was given none.
func (s *Service) previewGroups(ctx context.Context, p *Preview, lookup Lookuper) error {
	if lookup != nil {
		id, err := lookup.Lookup(ctx, p.Username)
		if err != nil {
			p.Reason = ClassOf(err)
			return nil
		}
		p.Groups, p.GroupsFrom = id.Groups, GroupsFromDirectory
		return nil
	}
	user, err := s.store.ExternalUserByUsername(p.Username)
	if errors.Is(err, store.ErrNotFound) || (err == nil && user.Source != p.Source) {
		p.Groups, p.GroupsFrom = []string{}, GroupsFromNone
		return nil
	}
	if err != nil {
		return err
	}
	p.Groups, p.GroupsFrom, p.LastSignIn = user.Groups, GroupsFromLastSignIn, user.LastSignIn
	return nil
}
