package kube

import (
	"context"
	"math"
	"time"

	corev1 "k8s.io/api/core/v1"

	"kubebench/internal/reason"
)

// ResourceAllocation is one resource's allocatable amount with what pods
// request, limit and (when measured) use. Usage is null when no metrics
// source has a value: an unknown use is never reported as 0.
type ResourceAllocation struct {
	Unit        string   `json:"unit"`
	Allocatable float64  `json:"allocatable"`
	Requests    float64  `json:"requests"`
	Limits      float64  `json:"limits"`
	Usage       *float64 `json:"usage"`
}

type ClusterAllocation struct {
	CPU    ResourceAllocation `json:"cpu"`
	Memory ResourceAllocation `json:"memory"`
	// UsageUnavailable explains a null Usage.
	UsageUnavailable *reason.Message `json:"usageUnavailable,omitempty"`
}

// ClusterUsage is the cluster's capacity and its current use in percent.
// The percentages are null when no metrics source measured them, and
// UsageUnavailable says why.
type ClusterUsage struct {
	CPUUsagePercent    *float64        `json:"cpuUsagePercent"`
	MemoryUsagePercent *float64        `json:"memoryUsagePercent"`
	CPUCapacityCores   int             `json:"cpuCapacityCores"`
	MemoryCapacityGi   int             `json:"memoryCapacityGi"`
	UsageUnavailable   *reason.Message `json:"usageUnavailable,omitempty"`
}

type ClusterOverview struct {
	NodeCount      int `json:"nodeCount"`
	ReadyNodes     int `json:"readyNodes"`
	PodCount       int `json:"podCount"`
	RunningPods    int `json:"runningPods"`
	NamespaceCount int `json:"namespaceCount"`
	ClusterUsage
}

func round1(v float64) float64 { return math.Round(v*10) / 10 }

// usageWait bounds how long a request waits for the first scrape of the
// metrics API or for Prometheus discovery of a fresh runtime.
const usageWait = 3 * time.Second

var reasonPromUsage = reason.Define("metrics.prometheus_usage_failed", "metrics-server has no data and the query to Prometheus {{location}} failed: {{detail}}")

// measured is the cluster's current use: percentages and absolute amounts.
type measured struct {
	cpuPercent, memPercent float64
	cpuCores, memMi        float64
}

// measureUsage is the cluster's current use from the newest metrics-server
// sample, else (e.g. on OpenShift without metrics data) from Prometheus;
// capacity converts Prometheus' percentages into amounts. The error
// explains why neither source has a value.
func (r *Runtime) measureUsage(ctx context.Context, capacity usage) (measured, error) {
	ctx, cancel := context.WithTimeout(ctx, usageWait)
	defer cancel()
	used, caps, samplerErr := r.Sampler.latest(ctx)
	if samplerErr == nil {
		cpu, _ := pct(used.cpu, caps.cpu)
		mem, _ := pct(used.memMi, caps.memMi)
		return measured{cpuPercent: cpu, memPercent: mem, cpuCores: used.cpu, memMi: used.memMi}, nil
	}
	_ = r.WaitPromDiscovered(ctx)
	prom := r.Prom()
	if prom == nil {
		return measured{}, samplerErr
	}
	cpu, err := prom.clusterPercent(ctx, "cpu")
	if err == nil {
		var mem float64
		if mem, err = prom.clusterPercent(ctx, "memory"); err == nil {
			cpu, mem = round1(cpu), round1(mem)
			return measured{
				cpuPercent: cpu, memPercent: mem,
				cpuCores: cpu / 100 * capacity.cpu, memMi: mem / 100 * capacity.memMi,
			}, nil
		}
	}
	return measured{}, reason.Wrap(err, reasonPromUsage, reason.Params{"location": prom.Location(), "detail": err.Error()})
}

// nodeCapacity sums the capacity of nodes.
func nodeCapacity(nodes []corev1.Node) usage {
	var total usage
	for i := range nodes {
		total.cpu += CPUCores(nodes[i].Status.Capacity.Cpu())
		total.memMi += MemoryMi(nodes[i].Status.Capacity.Memory())
	}
	return total
}

func (r *Runtime) Overview(ctx context.Context) (*ClusterOverview, error) {
	nodes, _, err := r.nodeItems(ctx, 0)
	if err != nil {
		return nil, err
	}
	pods, _, err := r.podItems(ctx, "", 0)
	if err != nil {
		return nil, err
	}
	namespaces, _, err := r.namespaceItems(ctx, 0)
	if err != nil {
		return nil, err
	}
	capacity := nodeCapacity(nodes)
	out := &ClusterOverview{
		NodeCount:      len(nodes),
		PodCount:       len(pods),
		NamespaceCount: len(namespaces),
		ClusterUsage: ClusterUsage{
			CPUCapacityCores: int(math.Round(capacity.cpu)),
			MemoryCapacityGi: int(math.Round(capacity.memMi / 1024)),
		},
	}
	if m, err := r.measureUsage(ctx, capacity); err == nil {
		out.CPUUsagePercent, out.MemoryUsagePercent = &m.cpuPercent, &m.memPercent
	} else {
		why := reason.MessageOf(err)
		out.UsageUnavailable = &why
	}
	for i := range nodes {
		if MapNode(&nodes[i]).Status == "Ready" {
			out.ReadyNodes++
		}
	}
	for i := range pods {
		if pods[i].Status.Phase == corev1.PodRunning {
			out.RunningPods++
		}
	}
	return out, nil
}

func (r *Runtime) Allocation(ctx context.Context) (*ClusterAllocation, error) {
	nodes, _, err := r.nodeItems(ctx, 0)
	if err != nil {
		return nil, err
	}
	pods, _, err := r.podItems(ctx, "", 0)
	if err != nil {
		return nil, err
	}

	var allocCPU, allocMemMi float64
	for i := range nodes {
		alloc := nodes[i].Status.Allocatable
		allocCPU += CPUCores(alloc.Cpu())
		allocMemMi += MemoryMi(alloc.Memory())
	}

	var reqCPU, limCPU, reqMemMi, limMemMi float64
	for i := range pods {
		pod := &pods[i]
		if pod.Status.Phase == corev1.PodSucceeded || pod.Status.Phase == corev1.PodFailed {
			continue
		}
		for _, c := range pod.Spec.Containers {
			reqCPU += CPUCores(c.Resources.Requests.Cpu())
			limCPU += CPUCores(c.Resources.Limits.Cpu())
			reqMemMi += MemoryMi(c.Resources.Requests.Memory())
			limMemMi += MemoryMi(c.Resources.Limits.Memory())
		}
	}

	out := &ClusterAllocation{
		CPU: ResourceAllocation{
			Unit:        "cores",
			Allocatable: round1(allocCPU),
			Requests:    round1(reqCPU),
			Limits:      round1(limCPU),
		},
		Memory: ResourceAllocation{
			Unit:        "Gi",
			Allocatable: round1(allocMemMi / 1024),
			Requests:    round1(reqMemMi / 1024),
			Limits:      round1(limMemMi / 1024),
		},
	}
	if m, err := r.measureUsage(ctx, nodeCapacity(nodes)); err == nil {
		cpu, mem := round1(m.cpuCores), round1(m.memMi/1024)
		out.CPU.Usage, out.Memory.Usage = &cpu, &mem
	} else {
		why := reason.MessageOf(err)
		out.UsageUnavailable = &why
	}
	return out, nil
}
