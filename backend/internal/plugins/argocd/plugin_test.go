package argocd

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"kubebench/internal/kube"
	"kubebench/internal/plugin"
	"kubebench/internal/store"
)

func TestDetect(t *testing.T) {
	tests := []struct {
		name      string
		resources []*metav1.APIResourceList
		available bool
	}{
		{name: "no Argo CD"},
		{
			name:      "Argo CD installed",
			resources: []*metav1.APIResourceList{{GroupVersion: groupVersion, APIResources: []metav1.APIResource{{Name: "applications"}, {Name: "appprojects"}}}},
			available: true,
		},
		{
			name:      "only ApplicationSets",
			resources: []*metav1.APIResourceList{{GroupVersion: groupVersion, APIResources: []metav1.APIResource{{Name: "applicationsets"}}}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cs := fake.NewClientset()
			cs.Resources = tt.resources
			st := (&Plugin{}).Detect(t.Context(), kube.NewRuntimeForTest(store.Cluster{ID: 1}, cs, nil))
			if st.Available != tt.available || (!st.Available && st.Reason == "") {
				t.Fatalf("status = %+v", st)
			}
		})
	}
}

type recordingRouter map[string]plugin.Access

func (r recordingRouter) Handle(method, path string, access plugin.Access, _ plugin.HandlerFunc) {
	r[method+" "+path] = access
}

// Every mutation is a write route, so the router refuses viewers and
// read-only clusters before the handler sends anything to the cluster;
// none is DryRunnable, because Argo CD's dry-run sync still patches the
// Application.
func TestRouteAccessLevels(t *testing.T) {
	routes := recordingRouter{}
	(&Plugin{}).Routes(routes)
	want := map[string]plugin.Access{
		"GET /applications":                       plugin.AccessRead,
		"GET /applications/{ns}/{name}":           plugin.AccessRead,
		"POST /applications/{ns}/{name}/sync":     plugin.AccessWrite,
		"POST /applications/{ns}/{name}/refresh":  plugin.AccessWrite,
		"POST /applications/{ns}/{name}/rollback": plugin.AccessWrite,
		"DELETE /applications/{ns}/{name}":        plugin.AccessWrite,
		"GET /projects":                           plugin.AccessRead,
		"GET /applicationsets":                    plugin.AccessRead,
	}
	if len(routes) != len(want) {
		t.Fatalf("routes = %v", routes)
	}
	for route, access := range want {
		if routes[route] != access {
			t.Errorf("%s: access %d, want %d", route, routes[route], access)
		}
	}
}
