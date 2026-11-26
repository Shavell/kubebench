package kube

import (
	"fmt"
	"sort"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

// JSON shapes mirror src/data/types.ts on the frontend.

type NodeInfo struct {
	Name             string            `json:"name"`
	Roles            []string          `json:"roles"`
	Status           string            `json:"status"`
	Version          string            `json:"version"`
	OS               string            `json:"os"`
	KernelVersion    string            `json:"kernelVersion"`
	ContainerRuntime string            `json:"containerRuntime"`
	CPUCapacity      int64             `json:"cpuCapacity"`
	MemoryCapacityGi int64             `json:"memoryCapacityGi"`
	Age              string            `json:"age"`
	InternalIP       string            `json:"internalIP"`
	Labels           map[string]string `json:"labels"`
	Annotations      map[string]string `json:"annotations"`
	// Unschedulable: the node is cordoned.
	Unschedulable bool    `json:"unschedulable,omitempty"`
	Taints        []Taint `json:"taints,omitempty"`
}

type NamespaceInfo struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Age    string `json:"age"`
}

type EnvVar struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type ResourceAmounts struct {
	CPU    string `json:"cpu"`
	Memory string `json:"memory"`
}

type ContainerSpec struct {
	Name     string          `json:"name"`
	Image    string          `json:"image"`
	Command  []string        `json:"command,omitempty"`
	Args     []string        `json:"args,omitempty"`
	Env      []EnvVar        `json:"env"`
	Requests ResourceAmounts `json:"requests"`
	Limits   ResourceAmounts `json:"limits"`
	Ports    string          `json:"ports,omitempty"`
}

type PodInfo struct {
	Name            string            `json:"name"`
	Namespace       string            `json:"namespace"`
	Status          string            `json:"status"`
	ReadyContainers int               `json:"readyContainers"`
	TotalContainers int               `json:"totalContainers"`
	Restarts        int32             `json:"restarts"`
	Age             string            `json:"age"`
	Node            string            `json:"node"`
	PodIP           string            `json:"podIP"`
	ControlledBy    string            `json:"controlledBy,omitempty"`
	Labels          map[string]string `json:"labels"`
	Containers      []ContainerSpec   `json:"containers"`
}

type DeploymentInfo struct {
	Name          string            `json:"name"`
	Namespace     string            `json:"namespace"`
	ReadyReplicas int32             `json:"readyReplicas"`
	Replicas      int32             `json:"replicas"`
	UpToDate      int32             `json:"upToDate"`
	Available     int32             `json:"available"`
	Age           string            `json:"age"`
	Labels        map[string]string `json:"labels"`
	// Paused: rollouts are paused (kubectl rollout pause).
	Paused bool `json:"paused,omitempty"`
}

type ServiceInfo struct {
	Name       string `json:"name"`
	Namespace  string `json:"namespace"`
	Type       string `json:"type"`
	ClusterIP  string `json:"clusterIP"`
	ExternalIP string `json:"externalIP,omitempty"`
	Ports      string `json:"ports"`
	Age        string `json:"age"`
}

type EventInfo struct {
	Type    string `json:"type"`
	Reason  string `json:"reason"`
	Message string `json:"message"`
	Age     string `json:"age"`
	Count   int32  `json:"count"`
	Source  string `json:"source"`
}

func FormatAge(t time.Time) string {
	if t.IsZero() {
		return "—"
	}
	d := time.Since(t)
	minutes := int(d.Minutes())
	switch {
	case minutes < 1:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case minutes < 60:
		return fmt.Sprintf("%dm", minutes)
	case minutes < 48*60:
		return fmt.Sprintf("%dh", minutes/60)
	case minutes < 365*24*60:
		return fmt.Sprintf("%dd", minutes/(24*60))
	default:
		days := minutes / (24 * 60)
		return fmt.Sprintf("%dy%dmo", days/365, (days%365)/30)
	}
}

func MapNode(n *corev1.Node) NodeInfo {
	roles := []string{}
	for label := range n.Labels {
		if rest, ok := strings.CutPrefix(label, "node-role.kubernetes.io/"); ok && rest != "" {
			roles = append(roles, rest)
		}
	}
	sort.Strings(roles)
	if len(roles) == 0 {
		roles = []string{"worker"}
	}
	status := "NotReady"
	for _, c := range n.Status.Conditions {
		if c.Type == corev1.NodeReady && c.Status == corev1.ConditionTrue {
			status = "Ready"
		}
	}
	internalIP := "—"
	for _, a := range n.Status.Addresses {
		if a.Type == corev1.NodeInternalIP {
			internalIP = a.Address
		}
	}
	memory := n.Status.Capacity.Memory().Value() / (1 << 30)
	return NodeInfo{
		Name:             n.Name,
		Roles:            roles,
		Status:           status,
		Version:          n.Status.NodeInfo.KubeletVersion,
		OS:               n.Status.NodeInfo.OSImage,
		KernelVersion:    n.Status.NodeInfo.KernelVersion,
		ContainerRuntime: n.Status.NodeInfo.ContainerRuntimeVersion,
		CPUCapacity:      n.Status.Capacity.Cpu().Value(),
		MemoryCapacityGi: memory,
		Age:              FormatAge(n.CreationTimestamp.Time),
		InternalIP:       internalIP,
		Labels:           orEmpty(n.Labels),
		Annotations:      orEmpty(n.Annotations),
		Unschedulable:    n.Spec.Unschedulable,
		Taints:           mapTaints(n.Spec.Taints),
	}
}

func MapNamespace(ns *corev1.Namespace) NamespaceInfo {
	return NamespaceInfo{
		Name:   ns.Name,
		Status: string(ns.Status.Phase),
		Age:    FormatAge(ns.CreationTimestamp.Time),
	}
}

func quantityString(list corev1.ResourceList, name corev1.ResourceName) string {
	if q, ok := list[name]; ok {
		return q.String()
	}
	return "—"
}

func mapContainer(c corev1.Container) ContainerSpec {
	env := []EnvVar{}
	for _, e := range c.Env {
		value := e.Value
		if value == "" && e.ValueFrom != nil {
			switch {
			case e.ValueFrom.FieldRef != nil:
				value = "(fieldRef)"
			case e.ValueFrom.SecretKeyRef != nil:
				value = "(secretKeyRef)"
			case e.ValueFrom.ConfigMapKeyRef != nil:
				value = "(configMapKeyRef)"
			default:
				value = "(valueFrom)"
			}
		}
		env = append(env, EnvVar{Name: e.Name, Value: value})
	}
	ports := []string{}
	for _, p := range c.Ports {
		proto := string(p.Protocol)
		if proto == "" {
			proto = "TCP"
		}
		ports = append(ports, fmt.Sprintf("%d/%s", p.ContainerPort, proto))
	}
	return ContainerSpec{
		Name:    c.Name,
		Image:   c.Image,
		Command: c.Command,
		Args:    c.Args,
		Env:     env,
		Requests: ResourceAmounts{
			CPU:    quantityString(c.Resources.Requests, corev1.ResourceCPU),
			Memory: quantityString(c.Resources.Requests, corev1.ResourceMemory),
		},
		Limits: ResourceAmounts{
			CPU:    quantityString(c.Resources.Limits, corev1.ResourceCPU),
			Memory: quantityString(c.Resources.Limits, corev1.ResourceMemory),
		},
		Ports: strings.Join(ports, ", "),
	}
}

func MapPod(p *corev1.Pod) PodInfo {
	ready := 0
	var restarts int32
	phase := string(p.Status.Phase)
	for _, cs := range p.Status.ContainerStatuses {
		if cs.Ready {
			ready++
		}
		restarts += cs.RestartCount
		if w := cs.State.Waiting; w != nil {
			switch w.Reason {
			case "CrashLoopBackOff", "ImagePullBackOff", "ContainerCreating":
				phase = w.Reason
			case "ErrImagePull":
				phase = "ImagePullBackOff"
			}
		}
	}
	controlledBy := ""
	if len(p.OwnerReferences) > 0 {
		o := p.OwnerReferences[0]
		controlledBy = o.Kind + "/" + o.Name
	}
	containers := []ContainerSpec{}
	for _, c := range p.Spec.Containers {
		containers = append(containers, mapContainer(c))
	}
	return PodInfo{
		Name:            p.Name,
		Namespace:       p.Namespace,
		Status:          phase,
		ReadyContainers: ready,
		TotalContainers: len(p.Spec.Containers),
		Restarts:        restarts,
		Age:             FormatAge(p.CreationTimestamp.Time),
		Node:            p.Spec.NodeName,
		PodIP:           p.Status.PodIP,
		ControlledBy:    controlledBy,
		Labels:          orEmpty(p.Labels),
		Containers:      containers,
	}
}

func MapDeployment(d *appsv1.Deployment) DeploymentInfo {
	var labels map[string]string
	if d.Spec.Selector != nil {
		labels = d.Spec.Selector.MatchLabels
	}
	if len(labels) == 0 {
		labels = d.Labels
	}
	replicas := int32(0)
	if d.Spec.Replicas != nil {
		replicas = *d.Spec.Replicas
	}
	return DeploymentInfo{
		Name:          d.Name,
		Namespace:     d.Namespace,
		ReadyReplicas: d.Status.ReadyReplicas,
		Replicas:      replicas,
		UpToDate:      d.Status.UpdatedReplicas,
		Available:     d.Status.AvailableReplicas,
		Age:           FormatAge(d.CreationTimestamp.Time),
		Labels:        orEmpty(labels),
		Paused:        d.Spec.Paused,
	}
}

func MapService(s *corev1.Service) ServiceInfo {
	ports := []string{}
	for _, p := range s.Spec.Ports {
		if p.NodePort > 0 {
			ports = append(ports, fmt.Sprintf("%d:%d/%s", p.Port, p.NodePort, p.Protocol))
		} else {
			ports = append(ports, fmt.Sprintf("%d/%s", p.Port, p.Protocol))
		}
	}
	externalIP := ""
	if ingress := s.Status.LoadBalancer.Ingress; len(ingress) > 0 {
		externalIP = ingress[0].IP
		if externalIP == "" {
			externalIP = ingress[0].Hostname
		}
	}
	return ServiceInfo{
		Name:       s.Name,
		Namespace:  s.Namespace,
		Type:       string(s.Spec.Type),
		ClusterIP:  s.Spec.ClusterIP,
		ExternalIP: externalIP,
		Ports:      strings.Join(ports, ","),
		Age:        FormatAge(s.CreationTimestamp.Time),
	}
}

// eventLastSeen is when the event last occurred: the series' last
// observation for an events.k8s.io-style series, else lastTimestamp, else
// eventTime, else when it was recorded.
func eventLastSeen(e *corev1.Event) time.Time {
	if e.Series != nil && !e.Series.LastObservedTime.IsZero() {
		return e.Series.LastObservedTime.Time
	}
	for _, t := range []time.Time{e.LastTimestamp.Time, e.EventTime.Time} {
		if !t.IsZero() {
			return t
		}
	}
	return e.CreationTimestamp.Time
}

func MapEvent(e *corev1.Event) EventInfo {
	when := eventLastSeen(e)
	count := e.Count
	if e.Series != nil && e.Series.Count > count {
		count = e.Series.Count
	}
	if count == 0 {
		count = 1
	}
	source := e.Source.Component
	if source == "" {
		source = e.ReportingController
	}
	return EventInfo{
		Type:    e.Type,
		Reason:  e.Reason,
		Message: e.Message,
		Age:     FormatAge(when),
		Count:   count,
		Source:  source,
	}
}

func CPUCores(q *resource.Quantity) float64 {
	return float64(q.MilliValue()) / 1000
}

func MemoryMi(q *resource.Quantity) float64 {
	return float64(q.Value()) / (1 << 20)
}

func orEmpty(m map[string]string) map[string]string {
	if m == nil {
		return map[string]string{}
	}
	return m
}
