package identity

import (
	"context"
	"errors"
	"maps"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"kubebench/internal/store"
)

func openStore(t *testing.T) *store.Store {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "kubebench.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	return st
}

func TestResolve(t *testing.T) {
	mappings := []store.GroupMapping{
		{ID: 1, Group: "KB-Admins", Role: store.RoleAdmin},
		{ID: 2, Group: "devs", Role: store.ClusterRoleViewer, Cluster: "prod"},
		{ID: 3, Group: "ops", Role: store.ClusterRoleOperator, Cluster: "prod"},
		{ID: 4, Group: "devs", Role: store.ClusterRoleOperator, Cluster: "dev"},
		{ID: 5, Group: "ldap-only", Source: store.UserSourceLDAP, Role: store.RoleAdmin},
	}
	cases := []struct {
		name     string
		source   string
		groups   []string
		admin    bool
		clusters map[string]string
		matched  []int64
	}{
		{"no groups", "oidc", nil, false, map[string]string{}, nil},
		{"case-insensitive admin", "oidc", []string{"kb-admins"}, true, map[string]string{}, []int64{1}},
		{"strongest role wins", "oidc", []string{"ops", "devs"},
			false, map[string]string{"prod": "operator", "dev": "operator"}, []int64{2, 3, 4}},
		{"viewer stays below operator", "oidc", []string{"devs"},
			false, map[string]string{"prod": "viewer", "dev": "operator"}, []int64{2, 4}},
		{"source-scoped mapping skipped", "oidc", []string{"ldap-only"}, false, map[string]string{}, nil},
		{"source-scoped mapping applies", "ldap", []string{"ldap-only"}, true, map[string]string{}, []int64{5}},
		{"no substring matches", "oidc", []string{"dev", "admins"}, false, map[string]string{}, nil},
	}
	for _, tc := range cases {
		g := Resolve(tc.source, tc.groups, mappings)
		if g.Admin != tc.admin || !maps.Equal(g.Clusters, tc.clusters) || !slices.Equal(g.Matched, tc.matched) {
			t.Errorf("%s: got %+v", tc.name, g)
		}
		if g.Any() != (tc.admin || len(tc.clusters) > 0) {
			t.Errorf("%s: Any = %v", tc.name, g.Any())
		}
	}
}

func seed(t *testing.T, st *store.Store) {
	t.Helper()
	if _, err := st.AddCluster(store.Cluster{Name: "prod", Source: store.SourceAddress, Address: "https://prod"}); err != nil {
		t.Fatal(err)
	}
	for _, m := range []store.GroupMapping{
		{Group: "kb-admins", Role: store.RoleAdmin},
		{Group: "ops", Role: store.ClusterRoleOperator, Cluster: "prod"},
	} {
		if _, err := st.CreateGroupMapping(m); err != nil {
			t.Fatal(err)
		}
	}
}

func TestSignInAppliesMappingsAtEverySignIn(t *testing.T) {
	st := openStore(t)
	seed(t, st)
	svc := NewService(st, false)

	id := &Identity{Source: store.UserSourceOIDC, Subject: "s1", Username: "alice", Groups: []string{"kb-admins"}}
	u, err := svc.SignIn(id)
	if err != nil {
		t.Fatal(err)
	}
	if u.Role != store.RoleAdmin || u.Source != store.UserSourceOIDC {
		t.Fatalf("first sign-in: %+v", u)
	}
	// A manual edit is overwritten by the next sign-in.
	if err := st.SetUserRole(u.ID, store.RoleUser); err != nil {
		t.Fatal(err)
	}
	id.Groups = []string{"ops"}
	u, err = svc.SignIn(id)
	if err != nil {
		t.Fatal(err)
	}
	role, err := st.ClusterRole(u, 1)
	if err != nil {
		t.Fatal(err)
	}
	if u.Role != store.RoleUser || role != store.ClusterRoleOperator {
		t.Fatalf("second sign-in: role %s, cluster role %q", u.Role, role)
	}
	// Without a granting group the user signs in and sees nothing.
	id.Groups = nil
	u, err = svc.SignIn(id)
	if err != nil {
		t.Fatal(err)
	}
	if role, _ := st.ClusterRole(u, 1); role != "" || u.Role != store.RoleUser {
		t.Fatalf("third sign-in: role %s, cluster role %q", u.Role, role)
	}
}

func TestRequireGroupRefusesAndEndsSessions(t *testing.T) {
	st := openStore(t)
	seed(t, st)
	svc := NewService(st, true)
	id := &Identity{Source: store.UserSourceOIDC, Subject: "s1", Username: "alice", Groups: []string{"ops"}}
	u, err := svc.SignIn(id)
	if err != nil {
		t.Fatal(err)
	}
	token, err := st.CreateSession(u.ID, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	id.Groups = []string{"other"}
	_, err = svc.SignIn(id)
	if ClassOf(err) != ClassNoMatchingGroup {
		t.Fatalf("err = %v", err)
	}
	if _, err := st.UserBySession(token); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("session survived the refusal: %v", err)
	}
}

func TestRefusedDisabledAccountEndsSessions(t *testing.T) {
	st := openStore(t)
	svc := NewService(st, false)
	u, err := svc.SignIn(&Identity{Source: store.UserSourceLDAP, Subject: "cn=bob", Username: "bob"})
	if err != nil {
		t.Fatal(err)
	}
	token, _ := st.CreateSession(u.ID, time.Hour)
	svc.Refused(store.UserSourceLDAP, &Failure{Class: ClassAccountDisabled, Subject: "cn=bob"})
	if _, err := st.UserBySession(token); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("session survived: %v", err)
	}
}

func TestSignInRefusals(t *testing.T) {
	st := openStore(t)
	svc := NewService(st, false)
	if _, err := st.CreateUser("admin", "password-1", store.RoleAdmin, false); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		id    Identity
		class string
	}{
		{Identity{Source: "oidc", Subject: "s1", Username: "admin"}, ClassUsernameConflict},
		{Identity{Source: "oidc", Subject: "s2", Username: "bad name"}, ClassInvalidUsername},
	}
	for _, tc := range cases {
		if _, err := svc.SignIn(&tc.id); ClassOf(err) != tc.class {
			t.Errorf("%s: err = %v, want %s", tc.id.Username, err, tc.class)
		}
	}
	if _, err := svc.SignIn(&Identity{Source: "oidc", Username: "x"}); err == nil {
		t.Error("an identity without a subject was accepted")
	}
}

type fakeLookup struct {
	groups   []string
	username string
}

func (f fakeLookup) Lookup(context.Context, string) (*Identity, error) {
	if f.groups == nil {
		return nil, Fail(ClassInvalidCredentials, "no such user")
	}
	return &Identity{Username: f.username, Groups: f.groups}, nil
}

func TestPreview(t *testing.T) {
	st := openStore(t)
	seed(t, st)
	svc := NewService(st, true)
	ctx := context.Background()
	if _, err := svc.SignIn(&Identity{Source: "oidc", Subject: "s1", Username: "alice", Groups: []string{"ops"}}); err != nil {
		t.Fatal(err)
	}
	usersBefore, _ := st.ListUsers()

	p, err := svc.Preview(ctx, "oidc", "alice", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if p.GroupsFrom != GroupsFromLastSignIn || !p.Allowed || p.Grants.Clusters["prod"] != "operator" || p.LastSignIn == nil {
		t.Fatalf("last sign-in preview: %+v", p)
	}
	p, _ = svc.Preview(ctx, "oidc", "newbie", []string{"kb-admins"}, nil)
	if p.GroupsFrom != GroupsFromInput || !p.Grants.Admin || !p.Allowed {
		t.Fatalf("input preview: %+v", p)
	}
	p, _ = svc.Preview(ctx, "oidc", "nobody", nil, nil)
	if p.GroupsFrom != GroupsFromNone || p.Allowed || p.Reason != ClassNoMatchingGroup {
		t.Fatalf("unknown user preview: %+v", p)
	}
	p, _ = svc.Preview(ctx, "ldap", "carol", nil, fakeLookup{groups: []string{"kb-admins"}})
	if p.GroupsFrom != GroupsFromDirectory || !p.Grants.Admin {
		t.Fatalf("directory preview: %+v", p)
	}
	// The directory names the account a login form belongs to.
	p, _ = svc.Preview(ctx, "ldap", `CORP\Carol`, nil, fakeLookup{username: "carol", groups: []string{"ops"}})
	if p.Username != "carol" || !p.Allowed || p.Grants.Clusters["prod"] != "operator" {
		t.Fatalf("directory preview of DOMAIN\\user: %+v", p)
	}
	p, _ = svc.Preview(ctx, "ldap", "ghost", nil, fakeLookup{})
	if p.Allowed || p.Reason != ClassInvalidCredentials || p.GroupsFrom != GroupsFromDirectory || p.Groups == nil {
		t.Fatalf("directory miss: %+v", p)
	}
	p, _ = svc.Preview(ctx, "ldap", "alice", []string{"kb-admins"}, nil)
	if p.Allowed || p.Reason != ClassUsernameConflict {
		t.Fatalf("other source's user: %+v", p)
	}
	usersAfter, _ := st.ListUsers()
	if len(usersAfter) != len(usersBefore) {
		t.Fatal("a preview created a user")
	}
}
