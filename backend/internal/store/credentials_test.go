package store

import "testing"

func TestClusterCredentials(t *testing.T) {
	s, _ := openTemp(t)
	id, err := s.AddCluster(Cluster{Name: "eks", Source: SourceKubeconfig, Kubeconfig: "k", Credentials: `{"provider":"aws"}`})
	if err != nil {
		t.Fatal(err)
	}
	c, err := s.GetCluster(id)
	if err != nil || c.Credentials != `{"provider":"aws"}` {
		t.Fatalf("stored = %+v, %v", c, err)
	}

	other := `{"provider":"gcp"}`
	if err := s.UpdateCluster(id, ClusterUpdate{Credentials: &other}); err != nil {
		t.Fatal(err)
	}
	if c, _ := s.ClusterByName("eks"); c.Credentials != other {
		t.Fatalf("after update = %q", c.Credentials)
	}
	none := ""
	if err := s.UpdateCluster(id, ClusterUpdate{Credentials: &none}); err != nil {
		t.Fatal(err)
	}
	if c, _ := s.GetCluster(id); c.Credentials != "" {
		t.Fatalf("after removal = %q", c.Credentials)
	}
}
