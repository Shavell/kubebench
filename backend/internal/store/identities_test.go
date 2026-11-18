package store

import (
	"errors"
	"slices"
	"testing"
	"time"
)

func TestSignInExternalCreatesAndUpdates(t *testing.T) {
	s, _ := openTemp(t)
	prod, _ := s.AddCluster(Cluster{Name: "prod", Source: SourceAddress, Address: "https://prod"})
	dev, _ := s.AddCluster(Cluster{Name: "dev", Source: SourceAddress, Address: "https://dev"})

	u, err := s.SignInExternal(ExternalAccount{
		Source: UserSourceOIDC, Subject: "sub-1", Username: "alice", DisplayName: "Alice", Email: "a@x.test",
		Groups: []string{"ops"}, Role: RoleUser,
		ClusterRoles: map[string]string{"prod": ClusterRoleOperator, "missing": ClusterRoleViewer},
	})
	if err != nil {
		t.Fatal(err)
	}
	if u.Source != UserSourceOIDC || u.DisplayName != "Alice" || u.Email != "a@x.test" || u.MustChangePassword {
		t.Fatalf("created %+v", u)
	}
	if role, _ := s.ClusterRole(u, prod); role != ClusterRoleOperator {
		t.Fatalf("prod role %q", role)
	}

	// Same subject, new username and groups: the account is updated and
	// its assignments replaced.
	u2, err := s.SignInExternal(ExternalAccount{
		Source: UserSourceOIDC, Subject: "sub-1", Username: "alice2", Role: RoleAdmin,
		ClusterRoles: map[string]string{"dev": ClusterRoleViewer},
	})
	if err != nil {
		t.Fatal(err)
	}
	if u2.ID != u.ID || u2.Username != "alice2" || u2.Role != RoleAdmin {
		t.Fatalf("updated %+v", u2)
	}
	assignments, _ := s.Assignments()
	if len(assignments) != 1 || assignments[0].ClusterID != dev || assignments[0].Role != ClusterRoleViewer {
		t.Fatalf("assignments %+v", assignments)
	}

	ext, err := s.ExternalUserByUsername("alice2")
	if err != nil || ext.LastSignIn == nil || len(ext.Groups) != 0 {
		t.Fatalf("external user %+v, %v", ext, err)
	}
}

func TestSignInExternalNeverTakesOverAnAccount(t *testing.T) {
	s, _ := openTemp(t)
	if _, err := s.CreateUser("admin", "password-1", RoleAdmin, false); err != nil {
		t.Fatal(err)
	}
	_, err := s.SignInExternal(ExternalAccount{Source: UserSourceOIDC, Subject: "x", Username: "admin", Role: RoleAdmin})
	if !errors.Is(err, ErrUsernameTaken) {
		t.Fatalf("local name: %v", err)
	}
	if _, err := s.SignInExternal(ExternalAccount{Source: UserSourceOIDC, Subject: "a", Username: "alice", Role: RoleUser}); err != nil {
		t.Fatal(err)
	}
	// The same name from another subject or source is refused too.
	for _, a := range []ExternalAccount{
		{Source: UserSourceOIDC, Subject: "b", Username: "alice", Role: RoleUser},
		{Source: UserSourceLDAP, Subject: "a", Username: "alice", Role: RoleUser},
	} {
		if _, err := s.SignInExternal(a); !errors.Is(err, ErrUsernameTaken) {
			t.Fatalf("%+v: %v", a, err)
		}
	}
}

func TestExternalUsersHaveNoPassword(t *testing.T) {
	s, _ := openTemp(t)
	if _, err := s.SignInExternal(ExternalAccount{Source: UserSourceLDAP, Subject: "cn=a", Username: "alice", Role: RoleUser}); err != nil {
		t.Fatal(err)
	}
	for _, pw := range []string{"", "anything"} {
		if _, err := s.Authenticate("alice", pw); !errors.Is(err, ErrNotFound) {
			t.Fatalf("password %q: %v", pw, err)
		}
	}
	if _, err := s.ExternalUserByUsername("nobody"); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
}

func TestLastLocalAdminIsKept(t *testing.T) {
	s, _ := openTemp(t)
	local, _ := s.CreateUser("admin", "password-1", RoleAdmin, false)
	ext, err := s.SignInExternal(ExternalAccount{Source: UserSourceOIDC, Subject: "a", Username: "boss", Role: RoleAdmin})
	if err != nil {
		t.Fatal(err)
	}
	// An external admin does not count as the break-glass admin.
	if err := s.SetUserRole(local, RoleUser); !errors.Is(err, ErrLastAdmin) {
		t.Fatalf("demote: %v", err)
	}
	if err := s.DeleteUser(local); !errors.Is(err, ErrLastAdmin) {
		t.Fatalf("delete: %v", err)
	}
	// External admins can always be demoted or deleted.
	if err := s.SetUserRole(ext.ID, RoleUser); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteUser(ext.ID); err != nil {
		t.Fatal(err)
	}
}

func TestRevokeExternalSessionsAndIDToken(t *testing.T) {
	s, _ := openTemp(t)
	u, _ := s.SignInExternal(ExternalAccount{Source: UserSourceOIDC, Subject: "a", Username: "alice", Role: RoleUser})
	token, err := s.CreateExternalSession(u.ID, time.Hour, "id-token")
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := s.SessionIDToken(token); got != "id-token" {
		t.Fatalf("id token %q", got)
	}
	if ok, err := s.RevokeExternalSessions(UserSourceOIDC, "a"); !ok || err != nil {
		t.Fatalf("revoke: %v %v", ok, err)
	}
	if _, err := s.UserBySession(token); !errors.Is(err, ErrNotFound) {
		t.Fatal("session survived")
	}
	if ok, _ := s.RevokeExternalSessions(UserSourceOIDC, "unknown"); ok {
		t.Fatal("unknown subject reported")
	}
}

func TestGroupMappings(t *testing.T) {
	s, _ := openTemp(t)
	id, err := s.CreateGroupMapping(GroupMapping{Group: " kb-admins ", Role: RoleAdmin})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateGroupMapping(GroupMapping{Group: "KB-ADMINS", Role: RoleAdmin}); !errors.Is(err, ErrConflict) {
		t.Fatalf("duplicate: %v", err)
	}
	for _, bad := range []GroupMapping{
		{Group: "", Role: RoleAdmin},
		{Group: "g", Role: RoleAdmin, Cluster: "prod"},
		{Group: "g", Role: ClusterRoleViewer},
		{Group: "g", Role: "owner", Cluster: "prod"},
		{Group: "g", Role: RoleAdmin, Source: "saml"},
	} {
		if _, err := s.CreateGroupMapping(bad); err == nil {
			t.Errorf("%+v accepted", bad)
		}
	}
	if err := s.UpdateGroupMapping(GroupMapping{ID: id, Group: "ops", Role: ClusterRoleOperator, Cluster: "prod", Source: UserSourceLDAP}); err != nil {
		t.Fatal(err)
	}
	list, _ := s.ListGroupMappings()
	if len(list) != 1 || list[0].Group != "ops" || list[0].Cluster != "prod" || list[0].Source != UserSourceLDAP {
		t.Fatalf("list %+v", list)
	}
	if err := s.DeleteGroupMapping(id); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteGroupMapping(id); !errors.Is(err, ErrNotFound) {
		t.Fatalf("second delete: %v", err)
	}
}

func TestSeedGroupMappingsOnce(t *testing.T) {
	s, _ := openTemp(t)
	seeds := []GroupMapping{{Group: "kb-admins", Role: RoleAdmin}, {Group: "ops", Role: ClusterRoleViewer, Cluster: "prod"}}
	if ok, err := s.SeedGroupMappings(seeds); !ok || err != nil {
		t.Fatalf("first seed: %v %v", ok, err)
	}
	list, _ := s.ListGroupMappings()
	if err := s.DeleteGroupMapping(list[0].ID); err != nil {
		t.Fatal(err)
	}
	if ok, err := s.SeedGroupMappings(seeds); ok || err != nil {
		t.Fatalf("second seed: %v %v", ok, err)
	}
	after, _ := s.ListGroupMappings()
	if len(after) != 1 {
		t.Fatalf("a deleted seed came back: %+v", after)
	}
	if _, err := s.SeedGroupMappings([]GroupMapping{{Group: "x", Role: "owner"}}); err == nil {
		t.Fatal("invalid seed accepted")
	}
	groups := []string{}
	for _, m := range after {
		groups = append(groups, m.Group)
	}
	if !slices.Equal(groups, []string{"ops"}) {
		t.Fatalf("groups %v", groups)
	}
}
