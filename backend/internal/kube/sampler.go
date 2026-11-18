package kube

import (
	"context"
	"log"
	"maps"
	"math"
	"slices"
	"sync"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"kubebench/internal/reason"
)

const (
	sampleInterval = 15 * time.Second
	maxSamples     = 240 // ≈ 1h
)

type MetricPoint struct {
	Time  string  `json:"time"`
	Value float64 `json:"value"`
}

type usage struct{ cpu, memMi float64 }

// sample is one successful scrape of metrics.k8s.io with the capacity of
// the nodes at that time, so every percentage refers to the nodes that
// existed when it was measured.
type sample struct {
	time        string
	nodes       map[string]usage // usage of the nodes that reported metrics
	nodeCaps    map[string]usage // capacity of every node
	pods        map[string]usage // "namespace/name"
	cluster     usage            // sum of nodes
	clusterCaps usage            // capacity of the nodes that reported metrics
}

// Sampler polls metrics.k8s.io into a ring buffer so "last N minutes"
// series exist even without Prometheus. It only ever answers with what a
// scrape measured: while the metrics API does not work, every question
// gets the reason (Status) instead of zeros.
type Sampler struct {
	rt *Runtime

	mu     sync.RWMutex
	buffer []sample
	status error // why the latest scrape failed; nil after a successful one

	scraped     chan struct{} // closed after the first scrape attempt
	scrapedOnce sync.Once
	stop        chan struct{}
}

func newSampler(rt *Runtime) *Sampler {
	return &Sampler{rt: rt, scraped: make(chan struct{}), stop: make(chan struct{})}
}

// start begins polling in the background until Stop.
func (s *Sampler) start() { go s.loop() }

// Stop ends polling; call it once (Runtime.Close does).
func (s *Sampler) Stop() { close(s.stop) }

func (s *Sampler) loop() {
	s.tick()
	ticker := time.NewTicker(sampleInterval)
	defer ticker.Stop()
	for {
		select {
		case <-s.stop:
			return
		case <-ticker.C:
			s.tick()
		}
	}
}

func (s *Sampler) tick() {
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()
	sm, err := s.scrape(ctx, time.Now())
	s.record(sm, err)
}

// scrape reads node and pod metrics and the nodes' capacity.
func (s *Sampler) scrape(ctx context.Context, now time.Time) (sample, error) {
	nodeMetrics, err := s.rt.Metrics.MetricsV1beta1().NodeMetricses().List(ctx, metav1.ListOptions{})
	if err != nil {
		return sample{}, s.rt.explainMetricsAPI(ctx, err)
	}
	if len(nodeMetrics.Items) == 0 {
		return sample{}, reason.New(reasonMetricsEmpty, nil)
	}
	podMetrics, err := s.rt.Metrics.MetricsV1beta1().PodMetricses("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return sample{}, s.rt.explainMetricsAPI(ctx, err)
	}
	nodes, err := s.rt.CS.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return sample{}, reason.Wrap(err, reasonNodeCapacity, reason.Params{"detail": err.Error()})
	}

	sm := sample{
		time:     now.Format("15:04"),
		nodes:    map[string]usage{},
		nodeCaps: map[string]usage{},
		pods:     map[string]usage{},
	}
	for _, n := range nodes.Items {
		sm.nodeCaps[n.Name] = usage{cpu: CPUCores(n.Status.Capacity.Cpu()), memMi: MemoryMi(n.Status.Capacity.Memory())}
	}
	for _, m := range nodeMetrics.Items {
		capacity, known := sm.nodeCaps[m.Name]
		if !known {
			continue // a node that is gone (or not listed yet) has no capacity to relate to
		}
		u := usage{cpu: CPUCores(m.Usage.Cpu()), memMi: MemoryMi(m.Usage.Memory())}
		sm.nodes[m.Name] = u
		sm.cluster.cpu += u.cpu
		sm.cluster.memMi += u.memMi
		sm.clusterCaps.cpu += capacity.cpu
		sm.clusterCaps.memMi += capacity.memMi
	}
	if len(sm.nodes) == 0 {
		return sample{}, reason.New(reasonMetricsEmpty, nil)
	}
	for _, m := range podMetrics.Items {
		var u usage
		for _, c := range m.Containers {
			u.cpu += CPUCores(c.Usage.Cpu())
			u.memMi += MemoryMi(c.Usage.Memory())
		}
		sm.pods[m.Namespace+"/"+m.Name] = u
	}
	return sm, nil
}

// record keeps a successful sample, or the reason the scrape failed. A
// change of state is logged once instead of on every tick.
func (s *Sampler) record(sm sample, err error) {
	s.mu.Lock()
	changed := (err == nil) != (s.status == nil) || (err != nil && err.Error() != s.status.Error())
	s.status = err
	if err == nil {
		s.buffer = append(s.buffer, sm)
		if len(s.buffer) > maxSamples {
			s.buffer = s.buffer[1:]
		}
	}
	s.mu.Unlock()
	s.scrapedOnce.Do(func() { close(s.scraped) })
	if changed && s.rt != nil {
		if err != nil {
			log.Printf("[sampler] cluster=%s metrics unavailable: %v", s.rt.Cluster.Name, err)
		} else {
			log.Printf("[sampler] cluster=%s metrics available", s.rt.Cluster.Name)
		}
	}
}

// Status is nil when the latest scrape of the metrics API succeeded, else
// the reason it has no data. It waits for the first scrape until ctx ends.
func (s *Sampler) Status(ctx context.Context) error {
	if err := wait(ctx, s.scraped); err != nil {
		return reason.New(reasonMetricsPending, nil)
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.status
}

// pct is value as a percentage of total; ok is false without a capacity.
func pct(value, total float64) (float64, bool) {
	if total <= 0 {
		return 0, false
	}
	return math.Round(value/total*1000) / 10, true
}

// series picks a point from each of the newest samples of the window;
// pick reports false for a sample that has no value (the node or pod was
// not measured then), which is left out rather than drawn as zero.
func (s *Sampler) series(ctx context.Context, metric string, minutes int, pick func(sample) (float64, bool)) ([]MetricPoint, error) {
	if metric != "cpu" && metric != "memory" {
		// metrics.k8s.io reports CPU and memory only.
		return nil, reason.New(reasonNeedsPrometheus, reason.Params{"metric": metric})
	}
	if err := s.Status(ctx); err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	count := max(1, minutes*60/int(sampleInterval.Seconds()))
	start := max(0, len(s.buffer)-count)
	points := []MetricPoint{}
	for _, sm := range s.buffer[start:] {
		if v, ok := pick(sm); ok {
			points = append(points, MetricPoint{Time: sm.time, Value: v})
		}
	}
	return points, nil
}

// percentOf is u's metric ("cpu" or "memory") as a percentage of capacity.
func (u usage) percentOf(metric string, capacity usage) (float64, bool) {
	if metric == "cpu" {
		return pct(u.cpu, capacity.cpu)
	}
	return pct(u.memMi, capacity.memMi)
}

// ClusterSeries is the cluster's CPU or memory use in percent of the
// capacity of the nodes that reported metrics.
func (s *Sampler) ClusterSeries(ctx context.Context, metric string, minutes int) ([]MetricPoint, error) {
	return s.series(ctx, metric, minutes, func(sm sample) (float64, bool) {
		return sm.cluster.percentOf(metric, sm.clusterCaps)
	})
}

// NodeSeries is a node's CPU or memory use in percent of its capacity.
func (s *Sampler) NodeSeries(ctx context.Context, node, metric string, minutes int) ([]MetricPoint, error) {
	return s.series(ctx, metric, minutes, func(sm sample) (float64, bool) {
		u, measured := sm.nodes[node]
		if !measured {
			return 0, false
		}
		return u.percentOf(metric, sm.nodeCaps[node])
	})
}

// PodSeries is a pod's CPU or memory use in percent of one node's capacity,
// like Lens: that of the first node by name, so every point of a sample
// refers to the same node.
func (s *Sampler) PodSeries(ctx context.Context, namespace, pod, metric string, minutes int) ([]MetricPoint, error) {
	return s.series(ctx, metric, minutes, func(sm sample) (float64, bool) {
		u, measured := sm.pods[namespace+"/"+pod]
		names := slices.Sorted(maps.Keys(sm.nodeCaps))
		if !measured || len(names) == 0 {
			return 0, false
		}
		return u.percentOf(metric, sm.nodeCaps[names[0]])
	})
}

// latest is the cluster's use in the newest sample, or the reason there is
// none.
func (s *Sampler) latest(ctx context.Context) (used, capacity usage, err error) {
	if err := s.Status(ctx); err != nil {
		return usage{}, usage{}, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if len(s.buffer) == 0 {
		return usage{}, usage{}, reason.New(reasonMetricsPending, nil)
	}
	last := s.buffer[len(s.buffer)-1]
	return last.cluster, last.clusterCaps, nil
}
