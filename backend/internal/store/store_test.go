package store

import (
	"errors"
	"testing"
	"time"
)

func mustUser(t *testing.T, s *Store, name, role string) *User {
	t.Helper()
	id, err := s.CreateUser(name, "password-123", role, false)
	if err != nil {
		t.Fatal(err)
	}
	u, err := s.GetUser(id)
	if err != nil {
		t.Fatal(err)
	}
	return u
}

func mustCluster(t *testing.T, s *Store, name string) int64 {
	t.Helper()
	id, err := s.AddCluster(Cluster{Name: name, Source: SourceKubeconfig})
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func TestDuplicateNamesConflict(t *testing.T) {
	s, _ := openTemp(t)
	mustUser(t, s, "alice", RoleUser)
	if _, err := s.CreateUser("alice", "password-123", RoleUser, false); !errors.Is(err, ErrConflict) {
		t.Fatalf("duplicate user: %v", err)
	}
	mustCluster(t, s, "prod")
	if _, err := s.AddCluster(Cluster{Name: "prod", Source: SourceKubeconfig}); !errors.Is(err, ErrConflict) {
		t.Fatalf("duplicate cluster: %v", err)
	}
	dev := mustCluster(t, s, "dev")
	name := "prod"
	if err := s.UpdateCluster(dev, ClusterUpdate{Name: &name}); !errors.Is(err, ErrConflict) {
		t.Fatalf("rename onto an existing name: %v", err)
	}
}

func TestDeletingTheActiveClusterResetsIt(t *testing.T) {
	s, _ := openTemp(t)
	alice := mustUser(t, s, "alice", RoleUser)
	prod := mustCluster(t, s, "prod")
	if err := s.SetAssignments(alice.ID, []Assignment{{ClusterID: prod, Role: ClusterRoleViewer}}); err != nil {
		t.Fatal(err)
	}
	if err := s.SetActiveCluster(alice.ID, prod); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteCluster(prod); err != nil {
		t.Fatal(err)
	}
	got, _ := s.GetUser(alice.ID)
	if got.ActiveClusterID != nil {
		t.Fatal("active cluster not reset")
	}
	if a, _ := s.Assignments(); len(a) != 0 {
		t.Fatalf("assignments not cascaded: %+v", a)
	}
	if err := s.DeleteCluster(prod); !errors.Is(err, ErrNotFound) {
		t.Fatalf("deleting a missing cluster: %v", err)
	}
}

func TestDeletingAUserCascades(t *testing.T) {
	s, _ := openTemp(t)
	mustUser(t, s, "admin", RoleAdmin)
	alice := mustUser(t, s, "alice", RoleUser)
	prod := mustCluster(t, s, "prod")
	if err := s.SetAssignments(alice.ID, []Assignment{{ClusterID: prod, Role: ClusterRoleOperator}}); err != nil {
		t.Fatal(err)
	}
	token, err := s.CreateSession(alice.ID, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteUser(alice.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.UserBySession(token); !errors.Is(err, ErrNotFound) {
		t.Fatalf("session survived its user: %v", err)
	}
	if a, _ := s.Assignments(); len(a) != 0 {
		t.Fatalf("assignments survived their user: %+v", a)
	}
}

func TestLastAdminIsProtected(t *testing.T) {
	s, _ := openTemp(t)
	root := mustUser(t, s, "root", RoleAdmin)
	if err := s.DeleteUser(root.ID); !errors.Is(err, ErrLastAdmin) {
		t.Fatalf("deleting the last admin: %v", err)
	}
	if err := s.SetUserRole(root.ID, RoleUser); !errors.Is(err, ErrLastAdmin) {
		t.Fatalf("demoting the last admin: %v", err)
	}
	if err := s.SetUserRole(root.ID, RoleAdmin); err != nil {
		t.Fatalf("a no-op role change must pass: %v", err)
	}
	second := mustUser(t, s, "second", RoleAdmin)
	if err := s.SetUserRole(root.ID, RoleUser); err != nil {
		t.Fatalf("demoting one of two admins: %v", err)
	}
	if err := s.DeleteUser(second.ID); !errors.Is(err, ErrLastAdmin) {
		t.Fatalf("the remaining admin must be protected: %v", err)
	}
	if err := s.DeleteUser(424242); !errors.Is(err, ErrNotFound) {
		t.Fatalf("deleting a missing user: %v", err)
	}
}

func TestAssignmentsAndRoles(t *testing.T) {
	s, _ := openTemp(t)
	admin := mustUser(t, s, "admin", RoleAdmin)
	alice := mustUser(t, s, "alice", RoleUser)
	prod, dev := mustCluster(t, s, "prod"), mustCluster(t, s, "dev")

	if err := s.SetAssignments(alice.ID, []Assignment{{ClusterID: prod, Role: "owner"}}); err == nil {
		t.Fatal("invalid role accepted")
	}
	if err := s.SetAssignments(alice.ID, []Assignment{{ClusterID: 999, Role: ClusterRoleViewer}}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown cluster: %v", err)
	}
	if err := s.SetAssignments(4242, nil); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown user: %v", err)
	}
	if err := s.SetAssignments(alice.ID, []Assignment{
		{ClusterID: prod, Role: ClusterRoleViewer},
		{ClusterID: dev, Role: ClusterRoleOperator},
	}); err != nil {
		t.Fatal(err)
	}

	if role, _ := s.ClusterRole(alice, prod); role != ClusterRoleViewer {
		t.Fatalf("alice on prod = %q", role)
	}
	if role, _ := s.ClusterRole(admin, prod); role != RoleAdmin {
		t.Fatalf("admin on prod = %q", role)
	}
	clusters, err := s.ListClusters(alice)
	if err != nil || len(clusters) != 2 || clusters[0].Role != ClusterRoleViewer || clusters[1].Role != ClusterRoleOperator {
		t.Fatalf("alice's clusters = %+v (err %v)", clusters, err)
	}

	if err := s.SetAssignments(alice.ID, []Assignment{{ClusterID: dev, Role: ClusterRoleViewer}}); err != nil {
		t.Fatal(err)
	}
	if role, _ := s.ClusterRole(alice, prod); role != "" {
		t.Fatalf("replaced assignment still grants %q", role)
	}
}

func TestClusterFlags(t *testing.T) {
	s, _ := openTemp(t)
	id := mustCluster(t, s, "prod")
	readOnly, env := true, "production"
	if err := s.UpdateCluster(id, ClusterUpdate{ReadOnly: &readOnly, Environment: &env}); err != nil {
		t.Fatal(err)
	}
	c, _ := s.GetCluster(id)
	if !c.ReadOnly || c.Environment != "production" || c.Name != "prod" {
		t.Fatalf("cluster = %+v", c)
	}
	if err := s.UpdateCluster(999, ClusterUpdate{ReadOnly: &readOnly}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("updating a missing cluster: %v", err)
	}
}

func TestPasswordsAndSessions(t *testing.T) {
	s, _ := openTemp(t)
	alice := mustUser(t, s, "alice", RoleUser)
	if _, err := s.Authenticate("alice", "wrong"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("wrong password: %v", err)
	}
	if _, err := s.Authenticate("nobody", "password-123"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown user: %v", err)
	}
	keep, _ := s.CreateSession(alice.ID, time.Hour)
	other, _ := s.CreateSession(alice.ID, time.Hour)

	if err := s.SetPassword(alice.ID, "new-password-1", true, keep); err != nil {
		t.Fatal(err)
	}
	if _, err := s.UserBySession(keep); err != nil {
		t.Fatalf("kept session revoked: %v", err)
	}
	if _, err := s.UserBySession(other); !errors.Is(err, ErrNotFound) {
		t.Fatal("other session survived a password change")
	}
	u, err := s.Authenticate("alice", "new-password-1")
	if err != nil || !u.MustChangePassword {
		t.Fatalf("after reset: %+v (err %v)", u, err)
	}
	if ok, _ := s.CheckPassword(alice.ID, "new-password-1"); !ok {
		t.Fatal("CheckPassword rejected the current password")
	}

	expired, _ := s.CreateSession(alice.ID, -time.Minute)
	if _, err := s.UserBySession(expired); !errors.Is(err, ErrNotFound) {
		t.Fatal("expired session accepted")
	}
	if n, err := s.PruneSessions(time.Now()); err != nil || n != 1 {
		t.Fatalf("PruneSessions removed %d (err %v)", n, err)
	}
}

func TestAuditLog(t *testing.T) {
	s, _ := openTemp(t)
	old := time.Now().Add(-100 * 24 * time.Hour)
	for i, e := range []AuditEntry{
		{Time: old, Username: "alice", Action: "DELETE /api/pods", Status: 200},
		{Username: "alice", Cluster: "prod", Action: "POST /api/deployments/scale", Target: "default/web", Status: 200, Detail: `{"replicas":"3"}`},
		{Username: "bob", Cluster: "dev", Action: "POST /api/deployments/restart", Status: 403},
		{Username: "alice", Cluster: "dev", Action: "exec.session", Status: 0},
	} {
		if err := s.InsertAudit(e); err != nil {
			t.Fatalf("row %d: %v", i, err)
		}
	}

	page, err := s.ListAudit(AuditQuery{Limit: 2})
	if err != nil || len(page) != 2 || page[0].Action != "exec.session" {
		t.Fatalf("first page = %+v (err %v)", page, err)
	}
	next, _ := s.ListAudit(AuditQuery{Limit: 2, Before: page[1].ID})
	if len(next) != 2 || next[1].Action != "DELETE /api/pods" {
		t.Fatalf("second page = %+v", next)
	}
	if rows, _ := s.ListAudit(AuditQuery{Limit: 10, User: "alice", Cluster: "dev"}); len(rows) != 1 {
		t.Fatalf("user+cluster filter = %+v", rows)
	}
	if rows, _ := s.ListAudit(AuditQuery{Limit: 10, Action: "DEPLOYMENTS"}); len(rows) != 2 {
		t.Fatalf("action filter = %+v", rows)
	}
	if rows, _ := s.ListAudit(AuditQuery{Limit: 10, Action: "%"}); len(rows) != 0 {
		t.Fatalf("LIKE wildcards must be literal: %+v", rows)
	}
	if rows, _ := s.ListAudit(AuditQuery{Limit: 10}); rows[2].Detail != `{"replicas":"3"}` || rows[2].Target != "default/web" {
		t.Fatalf("row fields = %+v", rows[2])
	}

	if n, err := s.PruneAudit(time.Now().Add(-90 * 24 * time.Hour)); err != nil || n != 1 {
		t.Fatalf("PruneAudit removed %d (err %v)", n, err)
	}
}

func TestPluginSettingsAndKV(t *testing.T) {
	s, _ := openTemp(t)
	if _, set, err := s.PluginEnabled("argocd"); err != nil || set {
		t.Fatalf("unset plugin reported as set (err %v)", err)
	}
	if err := s.SetPluginEnabled("argocd", false); err != nil {
		t.Fatal(err)
	}
	if err := s.SetPluginEnabled("topology", true); err != nil {
		t.Fatal(err)
	}
	if enabled, set, _ := s.PluginEnabled("argocd"); !set || enabled {
		t.Fatal("argocd should be explicitly disabled")
	}
	settings, _ := s.PluginSettings()
	if len(settings) != 2 || !settings["topology"] || settings["argocd"] {
		t.Fatalf("settings = %+v", settings)
	}

	kv, other := s.PluginKV("security"), s.PluginKV("argocd")
	for k, v := range map[string]string{"cluster/1/a": "x", "cluster/1/b": "y", "cluster/2/a": "z"} {
		if err := kv.Set(k, v); err != nil {
			t.Fatal(err)
		}
	}
	if err := kv.Set("cluster/1/a", "x2"); err != nil {
		t.Fatal(err)
	}
	if v, ok, _ := kv.Get("cluster/1/a"); !ok || v != "x2" {
		t.Fatalf("Get = %q, %v", v, ok)
	}
	if _, ok, _ := other.Get("cluster/1/a"); ok {
		t.Fatal("KV leaked across plugins")
	}
	if list, _ := kv.List("cluster/1/"); len(list) != 2 {
		t.Fatalf("List = %+v", list)
	}
	if err := kv.Delete("cluster/1/b"); err != nil {
		t.Fatal(err)
	}
	if all, _ := kv.List(""); len(all) != 2 {
		t.Fatalf("List(\"\") = %+v", all)
	}
	if err := kv.Set("", "v"); err == nil {
		t.Fatal("empty key accepted")
	}
}
