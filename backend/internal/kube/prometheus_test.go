package kube

import (
	"context"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/fake"

	"kubebench/internal/store"
)

// scriptedProm answers queries containing a key with its body, others
// with an empty result, and records the queries.
type scriptedProm struct {
	answers map[string]string
	queries []string
}

const emptyResult = `{"status":"success","data":{"resultType":"matrix","result":[]}}`

func (s *scriptedProm) get(_ context.Context, _ string, params map[string]string) ([]byte, error) {
	s.queries = append(s.queries, params["query"])
	for key, body := range s.answers {
		if strings.Contains(params["query"], key) {
			return []byte(body), nil
		}
	}
	return []byte(emptyResult), nil
}

func promOn(t *testing.T, api promAPI, nodes ...string) *PromSource {
	t.Helper()
	var objects []runtime.Object
	for _, n := range nodes {
		objects = append(objects, &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: n}})
	}
	rt := NewRuntimeForTest(store.Cluster{ID: 1, Name: "test"}, fake.NewClientset(objects...), dynamicfake.NewSimpleDynamicClient(runtime.NewScheme()))
	return &PromSource{rt: rt, location: "monitoring/prometheus", api: api}
}

const oneSample = `{"status":"success","data":{"resultType":"matrix","result":[{"values":[[1763467200,"12.34"]]}]}}`

// Node series find the node by the label its node-exporter series carry;
// OpenShift monitoring names it in instance.
func TestPromNodeSeriesLabels(t *testing.T) {
	api := &scriptedProm{answers: map[string]string{`instance="crc"`: oneSample}}
	points, err := promOn(t, api, "crc", "other").NodeSeries(context.Background(), "crc", "cpu", 15)
	if err != nil || len(points) != 1 || points[0].Value != 12.3 {
		t.Fatalf("points = %v, %v", points, err)
	}
	if len(api.queries) != 3 || !strings.Contains(api.queries[0], `kubernetes_node="crc"`) || !strings.Contains(api.queries[1], `node="crc"`) {
		t.Errorf("queries = %q", api.queries)
	}
}

// A node whose series cannot be told apart has no series on a cluster of
// several nodes — the cluster's would be someone else's — and the
// cluster's on a single-node cluster.
func TestPromNodeSeriesWithoutNodeLabel(t *testing.T) {
	clusterWide := `100 * (1 - avg(rate(node_cpu_seconds_total{mode="idle"}[5m])))`
	api := &scriptedProm{answers: map[string]string{clusterWide: oneSample}}
	if points, err := promOn(t, api, "a", "b").NodeSeries(context.Background(), "a", "cpu", 15); err != nil || len(points) != 0 {
		t.Fatalf("multi-node: %v, %v", points, err)
	}
	if points, err := promOn(t, api, "a").NodeSeries(context.Background(), "a", "cpu", 15); err != nil || len(points) != 1 {
		t.Fatalf("single node: %v, %v", points, err)
	}
}

func TestPromQueries(t *testing.T) {
	api := &scriptedProm{}
	prom := promOn(t, api)
	for _, metric := range []string{"cpu", "memory"} {
		if _, err := prom.PodSeries(context.Background(), "shop", "web", metric, 15); err != nil {
			t.Fatal(err)
		}
	}
	for _, q := range api.queries {
		if !strings.Contains(q, `container!="",container!="POD"`) {
			t.Errorf("pod query %q counts the pod cgroup with its containers", q)
		}
	}
	if q := clusterQueries("")["fs"]; !strings.Contains(q, `mountpoint=~"/|/sysroot"`) {
		t.Errorf("fs query %q misses RHCOS' root", q)
	}

	// An error answer is an error, not an empty series.
	api.answers = map[string]string{"": `{"status":"error","errorType":"bad_data","error":"parse error"}`}
	if _, err := prom.ClusterSeries(context.Background(), "cpu", 15); err == nil || !strings.Contains(err.Error(), "parse error") {
		t.Fatalf("error answer: %v", err)
	}
}
