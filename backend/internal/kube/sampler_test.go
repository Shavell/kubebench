package kube

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
	metricsv1beta1 "k8s.io/metrics/pkg/apis/metrics/v1beta1"
	metricsfake "k8s.io/metrics/pkg/client/clientset/versioned/fake"

	"kubebench/internal/reason"
	"kubebench/internal/store"
)

func TestSamplerSeries(t *testing.T) {
	s := newSampler(nil)
	s.record(sample{
		time:        "12:00",
		cluster:     usage{cpu: 3, memMi: 6144},
		clusterCaps: usage{cpu: 6, memMi: 12288},
		nodes:       map[string]usage{"b-node": {cpu: 1, memMi: 1024}},
		nodeCaps:    map[string]usage{"b-node": {cpu: 2, memMi: 4096}, "a-node": {cpu: 4, memMi: 8192}},
		pods:        map[string]usage{"shop/web": {cpu: 1, memMi: 2048}},
	}, nil)
	ctx := context.Background()
	value := func(points []MetricPoint, err error) float64 {
		t.Helper()
		if err != nil || len(points) != 1 {
			t.Fatalf("series = %v, %v", points, err)
		}
		return points[0].Value
	}

	if v := value(s.ClusterSeries(ctx, "cpu", 60)); v != 50 {
		t.Errorf("cluster cpu = %v", v)
	}
	if v := value(s.NodeSeries(ctx, "b-node", "memory", 60)); v != 25 {
		t.Errorf("node memory = %v", v)
	}
	// Pod percentages refer to one node, the same for every point and call.
	for range 20 {
		if v := value(s.PodSeries(ctx, "shop", "web", "cpu", 60)); v != 25 {
			t.Fatalf("pod cpu = %v, want 25 (of a-node's 4 cores)", v)
		}
	}
	// What was not measured has no points, never zeros.
	for name, series := range map[string]func() ([]MetricPoint, error){
		"unmeasured node": func() ([]MetricPoint, error) { return s.NodeSeries(ctx, "a-node", "cpu", 60) },
		"unknown node":    func() ([]MetricPoint, error) { return s.NodeSeries(ctx, "c-node", "cpu", 60) },
		"unknown pod":     func() ([]MetricPoint, error) { return s.PodSeries(ctx, "shop", "db", "memory", 60) },
	} {
		if points, err := series(); err != nil || len(points) != 0 {
			t.Errorf("%s: series = %v, %v; want no points", name, points, err)
		}
	}

	// metrics.k8s.io has no network or filesystem data: never answer with memory.
	for _, metric := range []string{"network", "fs"} {
		if _, err := s.ClusterSeries(ctx, metric, 60); err == nil {
			t.Errorf("cluster %s must be refused", metric)
		}
		if _, err := s.NodeSeries(ctx, "b-node", metric, 60); err == nil {
			t.Errorf("node %s must be refused", metric)
		}
		if _, err := s.PodSeries(ctx, "shop", "web", metric, 60); err == nil {
			t.Errorf("pod %s must be refused", metric)
		}
	}
}

// Until the first scrape finished, the sampler says so instead of
// answering with an empty history.
func TestSamplerPendingFirstScrape(t *testing.T) {
	s := newSampler(nil)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	if code := codeOf(s.Status(ctx)); code != "metrics.not_sampled" {
		t.Fatalf("status before the first scrape = %q", code)
	}
}

var apiServiceKinds = map[schema.GroupVersionResource]string{apiServiceGVR: "APIServiceList"}

// metricsCluster is a runtime whose metrics API answers with metricsErr
// (or two nodes and a pod when nil); apiService, when set, is the
// v1beta1.metrics.k8s.io APIService; served says whether discovery lists
// metrics.k8s.io/v1beta1.
func metricsCluster(t *testing.T, metricsErr error, served bool, apiService *unstructured.Unstructured) *Runtime {
	t.Helper()
	node := func(name, cpu, mem string) *corev1.Node {
		return &corev1.Node{
			ObjectMeta: metav1.ObjectMeta{Name: name},
			Status: corev1.NodeStatus{Capacity: corev1.ResourceList{
				corev1.ResourceCPU: resource.MustParse(cpu), corev1.ResourceMemory: resource.MustParse(mem),
			}},
		}
	}
	cs := fake.NewClientset(node("n1", "4", "8Gi"), node("n2", "4", "8Gi"))
	if served {
		cs.Resources = []*metav1.APIResourceList{{GroupVersion: metricsGroupVersion, APIResources: []metav1.APIResource{{Name: "nodes"}, {Name: "pods"}}}}
	}
	var objects []runtime.Object
	if apiService != nil {
		objects = append(objects, apiService)
	}
	dyn := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), apiServiceKinds, objects...)
	rt := NewRuntimeForTest(store.Cluster{ID: 1, Name: "test"}, cs, dyn)

	nodeUsage := func(name, cpu, mem string) *metricsv1beta1.NodeMetrics {
		return &metricsv1beta1.NodeMetrics{
			ObjectMeta: metav1.ObjectMeta{Name: name},
			Usage:      corev1.ResourceList{corev1.ResourceCPU: resource.MustParse(cpu), corev1.ResourceMemory: resource.MustParse(mem)},
		}
	}
	// The fake's object tracker files NodeMetrics under another resource
	// name than the client lists, so the lists are answered directly.
	metrics := metricsfake.NewSimpleClientset()
	metrics.PrependReactor("list", "nodes", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, &metricsv1beta1.NodeMetricsList{Items: []metricsv1beta1.NodeMetrics{
			*nodeUsage("n1", "2", "4Gi"), *nodeUsage("n2", "1", "2Gi"),
		}}, nil
	})
	metrics.PrependReactor("list", "pods", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, &metricsv1beta1.PodMetricsList{Items: []metricsv1beta1.PodMetrics{{
			ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "shop"},
			Containers: []metricsv1beta1.ContainerMetrics{{Name: "app", Usage: corev1.ResourceList{
				corev1.ResourceCPU: resource.MustParse("500m"), corev1.ResourceMemory: resource.MustParse("1Gi"),
			}}},
		}}}, nil
	})
	if metricsErr != nil {
		metrics.PrependReactor("list", "*", func(k8stesting.Action) (bool, runtime.Object, error) {
			return true, nil, metricsErr
		})
	}
	rt.Metrics = metrics
	return rt
}

func apiServiceObject(available, why, message string) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "apiregistration.k8s.io/v1", "kind": "APIService",
		"metadata": map[string]any{"name": metricsAPIService},
		"status": map[string]any{"conditions": []any{map[string]any{
			"type": "Available", "status": available, "reason": why, "message": message,
		}}},
	}}
}

func codeOf(err error) string {
	if code, _, ok := reason.Of(err); ok {
		return code.String()
	}
	if err != nil {
		return "unexplained: " + err.Error()
	}
	return ""
}

// The metrics API is available only after a scrape that measured nodes;
// a missing, unavailable, forbidden or failing API is reported with its
// reason, and then no metric is answered, not even from older samples.
func TestSamplerStatus(t *testing.T) {
	notFound := apierrors.NewNotFound(schema.GroupResource{Group: "metrics.k8s.io", Resource: "nodes"}, "")
	unavailable := apierrors.NewServiceUnavailable("the server is currently unable to handle the request")
	for _, tc := range []struct {
		name       string
		err        error
		served     bool
		apiService *unstructured.Unstructured
		code       string
		detail     string
	}{
		{name: "absent", err: notFound, code: "metrics.api_missing"},
		{name: "absent from discovery", err: unavailable, code: "metrics.api_missing"},
		{name: "present but unavailable", err: unavailable, served: true,
			apiService: apiServiceObject("False", "MissingEndpoints", "endpoints for service/metrics-server in \"kube-system\" have no addresses"),
			code:       "metrics.api_unavailable", detail: "MissingEndpoints: endpoints for service/metrics-server"},
		{name: "forbidden", err: apierrors.NewForbidden(schema.GroupResource{Group: "metrics.k8s.io", Resource: "nodes"}, "", errors.New("RBAC")),
			served: true, code: "metrics.api_forbidden"},
		{name: "erroring", err: apierrors.NewInternalError(errors.New("boom")), served: true,
			apiService: apiServiceObject("True", "Passed", "all checks passed"), code: "metrics.api_failed", detail: "boom"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rt := metricsCluster(t, tc.err, tc.served, tc.apiService)
			rt.SampleForTest(context.Background(), time.Date(2026, 11, 18, 12, 0, 0, 0, time.UTC))
			err := rt.Sampler.Status(context.Background())
			if code := codeOf(err); code != tc.code {
				t.Fatalf("status = %q (%v), want %s", code, err, tc.code)
			}
			if !strings.Contains(err.Error(), tc.detail) {
				t.Errorf("reason %q lacks %q", err, tc.detail)
			}
			if _, err := rt.Sampler.ClusterSeries(context.Background(), "cpu", 60); codeOf(err) != tc.code {
				t.Errorf("cluster series = %v, want the reason", err)
			}
			if _, _, err := rt.Sampler.latest(context.Background()); codeOf(err) != tc.code {
				t.Errorf("latest = %v, want the reason", err)
			}
		})
	}

	t.Run("working, then failing", func(t *testing.T) {
		rt := metricsCluster(t, nil, true, nil)
		at := time.Date(2026, 11, 18, 12, 0, 0, 0, time.UTC)
		rt.SampleForTest(context.Background(), at)
		if err := rt.Sampler.Status(context.Background()); err != nil {
			t.Fatalf("status = %v", err)
		}
		points, err := rt.Sampler.ClusterSeries(context.Background(), "cpu", 60)
		if err != nil || len(points) != 1 || points[0].Value != 37.5 || points[0].Time != "12:00" {
			t.Fatalf("cluster cpu = %v, %v; want 3 of 8 cores", points, err)
		}
		if points, _ := rt.Sampler.PodSeries(context.Background(), "shop", "web", "memory", 60); len(points) != 1 || points[0].Value != 12.5 {
			t.Fatalf("pod memory = %v; want 1 of n1's 8 Gi", points)
		}

		rt.Metrics.(*metricsfake.Clientset).PrependReactor("list", "*", func(k8stesting.Action) (bool, runtime.Object, error) {
			return true, nil, apierrors.NewNotFound(schema.GroupResource{Group: "metrics.k8s.io", Resource: "nodes"}, "")
		})
		rt.SampleForTest(context.Background(), at.Add(sampleInterval))
		if _, err := rt.Sampler.ClusterSeries(context.Background(), "cpu", 60); codeOf(err) != "metrics.api_missing" {
			t.Fatalf("after the API went away: %v", err)
		}
	})

	t.Run("no node metrics yet", func(t *testing.T) {
		rt := metricsCluster(t, nil, true, nil)
		rt.Metrics = metricsfake.NewSimpleClientset()
		rt.SampleForTest(context.Background(), time.Now())
		if code := codeOf(rt.Sampler.Status(context.Background())); code != "metrics.api_empty" {
			t.Fatalf("status = %q", code)
		}
	})
}
