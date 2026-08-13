package kube

import (
	"context"
	"math"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ResourceAllocation struct {
	Unit        string  `json:"unit"`
	Allocatable float64 `json:"allocatable"`
	Requests    float64 `json:"requests"`
	Limits      float64 `json:"limits"`
	Usage       float64 `json:"usage"`
}

type ClusterAllocation struct {
	CPU    ResourceAllocation `json:"cpu"`
	Memory ResourceAllocation `json:"memory"`
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

func (r *Runtime) Overview(ctx context.Context) (*ClusterOverview, error) {
	nodes, err := r.CS.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	pods, err := r.CS.CoreV1().Pods("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	namespaces, err := r.CS.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	out := &ClusterOverview{
		NodeCount:      len(nodes.Items),
		PodCount:       len(pods.Items),
		NamespaceCount: len(namespaces.Items),
		ClusterUsage:   r.Sampler.LatestUsage(),
	}
	for i := range nodes.Items {
		if MapNode(&nodes.Items[i]).Status == "Ready" {
			out.ReadyNodes++
		}
	}
	for i := range pods.Items {
		if pods.Items[i].Status.Phase == corev1.PodRunning {
			out.RunningPods++
		}
	}
	return out, nil
}

func (r *Runtime) Allocation(ctx context.Context) (*ClusterAllocation, error) {
	nodes, err := r.CS.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	pods, err := r.CS.CoreV1().Pods("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var allocCPU, allocMemMi float64
	for i := range nodes.Items {
		alloc := nodes.Items[i].Status.Allocatable
		allocCPU += CPUCores(alloc.Cpu())
		allocMemMi += MemoryMi(alloc.Memory())
	}

	var reqCPU, limCPU, reqMemMi, limMemMi float64
	for i := range pods.Items {
		pod := &pods.Items[i]
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

	usage := r.Sampler.LatestUsage()
	return &ClusterAllocation{
		CPU: ResourceAllocation{
			Unit:        "cores",
			Allocatable: round1(allocCPU),
			Requests:    round1(reqCPU),
			Limits:      round1(limCPU),
			Usage:       round1(usage.CPUUsagePercent / 100 * float64(usage.CPUCapacityCores)),
		},
		Memory: ResourceAllocation{
			Unit:        "Gi",
			Allocatable: round1(allocMemMi / 1024),
			Requests:    round1(reqMemMi / 1024),
			Limits:      round1(limMemMi / 1024),
			Usage:       round1(usage.MemoryUsagePercent / 100 * float64(usage.MemoryCapacityGi)),
		},
	}, nil
}
