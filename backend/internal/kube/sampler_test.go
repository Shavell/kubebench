package kube

import "testing"

func TestSamplerSeries(t *testing.T) {
	s := &Sampler{}
	s.caps.cpuCores, s.caps.memMi = 6, 12288
	s.caps.perNode = map[string]usage{"b-node": {cpu: 2, memMi: 4096}, "a-node": {cpu: 4, memMi: 8192}}
	s.buffer = []sample{{
		time:    "12:00",
		cluster: usage{cpu: 3, memMi: 6144},
		nodes:   map[string]usage{"b-node": {cpu: 1, memMi: 1024}},
		pods:    map[string]usage{"shop/web": {cpu: 1, memMi: 2048}},
	}}
	value := func(points []MetricPoint, err error) float64 {
		t.Helper()
		if err != nil || len(points) != 1 {
			t.Fatalf("series = %v, %v", points, err)
		}
		return points[0].Value
	}

	if v := value(s.ClusterSeries("cpu", 60)); v != 50 {
		t.Errorf("cluster cpu = %v", v)
	}
	if v := value(s.NodeSeries("b-node", "memory", 60)); v != 25 {
		t.Errorf("node memory = %v", v)
	}
	// Pod percentages refer to one node, the same for every point and call.
	for range 20 {
		if v := value(s.PodSeries("shop", "web", "cpu", 60)); v != 25 {
			t.Fatalf("pod cpu = %v, want 25 (of a-node's 4 cores)", v)
		}
	}

	// metrics.k8s.io has no network or filesystem data: never answer with memory.
	for _, metric := range []string{"network", "fs"} {
		if _, err := s.ClusterSeries(metric, 60); err == nil {
			t.Errorf("cluster %s must be refused", metric)
		}
		if _, err := s.NodeSeries("b-node", metric, 60); err == nil {
			t.Errorf("node %s must be refused", metric)
		}
		if _, err := s.PodSeries("shop", "web", metric, 60); err == nil {
			t.Errorf("pod %s must be refused", metric)
		}
	}
}
