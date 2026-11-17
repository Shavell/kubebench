package notes

import (
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestNormalizeTags(t *testing.T) {
	got, err := normalizeTags([]string{" Runbook", "shop", "runbook", "çay-1"})
	if err != nil || !reflect.DeepEqual(got, []string{"runbook", "shop", "çay-1"}) {
		t.Fatalf("normalizeTags = %v, %v", got, err)
	}
	for _, bad := range []string{"", "-lead", "a b", "a,b", "x1234567890123456789012345678901234567890"} {
		if _, err := normalizeTags([]string{bad}); err == nil {
			t.Errorf("tag %q accepted", bad)
		}
	}
}

func TestGroupOf(t *testing.T) {
	for in, want := range map[string]string{"": "", "v1": "", "apps/v1": "apps", "argoproj.io/v1alpha1": "argoproj.io"} {
		if got, err := groupOf(in); err != nil || got != want {
			t.Errorf("groupOf(%q) = %q, %v", in, got, err)
		}
	}
	if _, err := groupOf("a/b/c"); err == nil {
		t.Error("a/b/c accepted")
	}
}

// A name without a namespace names a cluster-scoped object; filters that
// are not given match anything.
func TestMatch(t *testing.T) {
	m, err := match(httptest.NewRequest("GET", "/counts?kind=Node&name=worker-1", nil))
	if err != nil || m == nil || *m.Kind != "Node" || *m.Name != "worker-1" || m.Namespace == nil || *m.Namespace != "" || m.Group != nil {
		t.Fatalf("match = %+v, %v", m, err)
	}
	m, err = match(httptest.NewRequest("GET", "/counts?namespace=shop", nil))
	if err != nil || m == nil || m.Kind != nil || m.Name != nil || *m.Namespace != "shop" {
		t.Fatalf("match = %+v, %v", m, err)
	}
	if m, err := match(httptest.NewRequest("GET", "/counts", nil)); err != nil || m != nil {
		t.Fatalf("no filter: %+v, %v", m, err)
	}
	if _, err := match(httptest.NewRequest("GET", "/counts?namespace=Bad_NS", nil)); err == nil {
		t.Fatal("bad namespace accepted")
	}
}
