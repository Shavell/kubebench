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

type sample struct {
	time    string
	nodes   map[string]usage
	pods    map[string]usage
	cluster usage
}

// Sampler polls metrics.k8s.io into a ring buffer so "last N minutes"
// series exist even without Prometheus.
type Sampler struct {
	rt     *Runtime
	mu     sync.RWMutex
	buffer []sample
	caps   struct {
		cpuCores float64
		memMi    float64
		perNode  map[string]usage
	}
	stop chan struct{}
}

func newSampler(rt *Runtime) *Sampler {
	return &Sampler{rt: rt, stop: make(chan struct{})}
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

	if s.caps.perNode == nil {
		nodes, err := s.rt.CS.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
		if err != nil {
			return
		}
		perNode := map[string]usage{}
		var cpu, mem float64
		for _, n := range nodes.Items {
			u := usage{cpu: CPUCores(n.Status.Capacity.Cpu()), memMi: MemoryMi(n.Status.Capacity.Memory())}
			perNode[n.Name] = u
			cpu += u.cpu
			mem += u.memMi
		}
		s.mu.Lock()
		s.caps.perNode, s.caps.cpuCores, s.caps.memMi = perNode, cpu, mem
		s.mu.Unlock()
	}

	nodeMetrics, err := s.rt.Metrics.MetricsV1beta1().NodeMetricses().List(ctx, metav1.ListOptions{})
	if err != nil {
		log.Printf("[sampler] %s: %v", s.rt.Cluster.Name, err)
		return
	}
	podMetrics, err := s.rt.Metrics.MetricsV1beta1().PodMetricses("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return
	}

	sm := sample{
		time:  time.Now().Format("15:04"),
		nodes: map[string]usage{},
		pods:  map[string]usage{},
	}
	for _, m := range nodeMetrics.Items {
		u := usage{cpu: CPUCores(m.Usage.Cpu()), memMi: MemoryMi(m.Usage.Memory())}
		sm.nodes[m.Name] = u
		sm.cluster.cpu += u.cpu
		sm.cluster.memMi += u.memMi
	}
	for _, m := range podMetrics.Items {
		var u usage
		for _, c := range m.Containers {
			u.cpu += CPUCores(c.Usage.Cpu())
			u.memMi += MemoryMi(c.Usage.Memory())
		}
		sm.pods[m.Namespace+"/"+m.Name] = u
	}

	s.mu.Lock()
	s.buffer = append(s.buffer, sm)
	if len(s.buffer) > maxSamples {
		s.buffer = s.buffer[1:]
	}
	s.mu.Unlock()
}

func pct(value, total float64) float64 {
	if total <= 0 {
		return 0
	}
	return math.Round(value/total*1000) / 10
}

func (s *Sampler) series(minutes int, pick func(sample) float64) []MetricPoint {
	s.mu.RLock()
	defer s.mu.RUnlock()
	count := max(1, minutes*60/int(sampleInterval.Seconds()))
	start := max(0, len(s.buffer)-count)
	points := []MetricPoint{}
	for _, sm := range s.buffer[start:] {
		points = append(points, MetricPoint{Time: sm.time, Value: pick(sm)})
	}
	return points
}

// percentOf is u's metric ("cpu" or "memory") as a percentage of capacity.
func (u usage) percentOf(metric string, capacity usage) float64 {
	if metric == "cpu" {
		return pct(u.cpu, capacity.cpu)
	}
	return pct(u.memMi, capacity.memMi)
}

// sampled refuses the metrics the sampler does not keep: metrics.k8s.io
// reports CPU and memory only.
func sampled(metric string) error {
	if metric != "cpu" && metric != "memory" {
		return reason.New(reasonNeedsPrometheus, reason.Params{"metric": metric})
	}
	return nil
}

// ClusterSeries is the cluster's CPU or memory use in percent.
func (s *Sampler) ClusterSeries(metric string, minutes int) ([]MetricPoint, error) {
	if err := sampled(metric); err != nil {
		return nil, err
	}
	return s.series(minutes, func(sm sample) float64 {
		return sm.cluster.percentOf(metric, usage{cpu: s.caps.cpuCores, memMi: s.caps.memMi})
	}), nil
}

// NodeSeries is a node's CPU or memory use in percent of its capacity.
func (s *Sampler) NodeSeries(node, metric string, minutes int) ([]MetricPoint, error) {
	if err := sampled(metric); err != nil {
		return nil, err
	}
	return s.series(minutes, func(sm sample) float64 {
		return sm.nodes[node].percentOf(metric, s.caps.perNode[node])
	}), nil
}

// PodSeries is a pod's CPU or memory use in percent of one node's capacity,
// like Lens: that of the first node by name, so every point refers to the
// same node.
func (s *Sampler) PodSeries(namespace, pod, metric string, minutes int) ([]MetricPoint, error) {
	if err := sampled(metric); err != nil {
		return nil, err
	}
	s.mu.RLock()
	var capacity usage
	if names := slices.Sorted(maps.Keys(s.caps.perNode)); len(names) > 0 {
		capacity = s.caps.perNode[names[0]]
	}
	s.mu.RUnlock()
	return s.series(minutes, func(sm sample) float64 {
		return sm.pods[namespace+"/"+pod].percentOf(metric, capacity)
	}), nil
}

type ClusterUsage struct {
	CPUUsagePercent    float64 `json:"cpuUsagePercent"`
	MemoryUsagePercent float64 `json:"memoryUsagePercent"`
	CPUCapacityCores   int     `json:"cpuCapacityCores"`
	MemoryCapacityGi   int     `json:"memoryCapacityGi"`
}

func (s *Sampler) LatestUsage() ClusterUsage {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := ClusterUsage{
		CPUCapacityCores: int(math.Round(s.caps.cpuCores)),
		MemoryCapacityGi: int(math.Round(s.caps.memMi / 1024)),
	}
	if len(s.buffer) > 0 {
		last := s.buffer[len(s.buffer)-1]
		out.CPUUsagePercent = pct(last.cluster.cpu, s.caps.cpuCores)
		out.MemoryUsagePercent = pct(last.cluster.memMi, s.caps.memMi)
	}
	return out
}
