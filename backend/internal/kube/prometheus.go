package kube

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"slices"
	"strconv"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"kubebench/internal/reason"
)

// PromSource serves utilization series from a Prometheus-compatible query
// API — full history plus network/filesystem metrics that metrics-server
// cannot provide. It is reached either through the API server's service
// proxy (a plain Prometheus in the cluster) or directly over HTTPS with
// KubeBench's cluster credential (OpenShift monitoring's Thanos Querier,
// see thanos.go).
type PromSource struct {
	rt       *Runtime
	location string
	api      promAPI
}

// promAPI performs a GET on the query API, e.g. path "api/v1/query".
type promAPI interface {
	get(ctx context.Context, path string, params map[string]string) ([]byte, error)
}

// serviceProxy reaches an http Prometheus Service through the API server.
type serviceProxy struct {
	cs                       kubernetes.Interface
	namespace, service, port string
}

func (s serviceProxy) get(ctx context.Context, path string, params map[string]string) ([]byte, error) {
	return s.cs.CoreV1().Services(s.namespace).ProxyGet("http", s.service, s.port, path, params).DoRaw(ctx)
}

var promCandidates = []string{
	"prometheus",
	"prometheus-server",
	"prometheus-operated",
	"prometheus-k8s",
	"prometheus-kube-prometheus-prometheus",
	"kube-prometheus-stack-prometheus",
}

// promRetry is how often a runtime without Prometheus looks again, so
// that e.g. a newly bound cluster-monitoring-view or a newly installed
// Prometheus is used without rebuilding the runtime.
const promRetry = 5 * time.Minute

// watchProm runs discovery now and, while nothing was found, every
// promRetry until stop is closed.
func (r *Runtime) watchProm(stop <-chan struct{}) {
	r.discoverProm()
	ticker := time.NewTicker(promRetry)
	defer ticker.Stop()
	for r.Prom() == nil {
		select {
		case <-stop:
			return
		case <-ticker.C:
			r.findProm()
		}
	}
}

// discoverProm runs findProm and marks discovery finished
// (WaitPromDiscovered).
func (r *Runtime) discoverProm() {
	defer r.promOnce.Do(func() { close(r.promDone) })
	r.findProm()
}

// findProm looks for a query API: OpenShift monitoring's Thanos Querier
// first (it aggregates the platform Prometheus pair), then a Prometheus
// Service with a well-known name. When nothing works, promErr keeps why
// OpenShift monitoring could not be used, if it exists.
func (r *Runtime) findProm() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	thanos, thanosErr := r.discoverThanos(ctx)
	if thanos != nil {
		r.setProm(thanos, nil)
		return
	}
	if prom := r.discoverPromService(ctx); prom != nil {
		r.setProm(prom, nil)
		return
	}
	r.setProm(nil, thanosErr)
	outcome := "not found — metrics-server only"
	if thanosErr != nil {
		outcome = "OpenShift monitoring unusable: " + thanosErr.Error()
	}
	// Rediscovery repeats the outcome every promRetry; log changes only.
	if outcome != r.promLogged {
		r.promLogged = outcome
		log.Printf("[prometheus] cluster=%s %s", r.Cluster.Name, outcome)
	}
}

func (r *Runtime) setProm(prom *PromSource, err error) {
	r.mu.Lock()
	r.prom, r.promErr = prom, err
	r.mu.Unlock()
}

// discoverPromService finds a Prometheus Service reachable over the
// service proxy that answers a query.
func (r *Runtime) discoverPromService(ctx context.Context) *PromSource {
	services, err := r.CS.CoreV1().Services("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil
	}
	for _, svc := range services.Items {
		if !slices.Contains(promCandidates, svc.Name) {
			continue
		}
		for _, p := range svc.Spec.Ports {
			if p.Port != 80 && p.Port != 9090 {
				continue
			}
			prom := r.proxiedProm(svc.Namespace, svc.Name, strconv.Itoa(int(p.Port)))
			if _, err := prom.api.get(ctx, "api/v1/query", map[string]string{"query": "up"}); err == nil {
				log.Printf("[prometheus] cluster=%s using %s:%d", r.Cluster.Name, prom.location, p.Port)
				return prom
			}
		}
	}
	return nil
}

func (r *Runtime) proxiedProm(namespace, service, port string) *PromSource {
	return &PromSource{
		rt: r, location: namespace + "/" + service,
		api: serviceProxy{cs: r.CS, namespace: namespace, service: service, port: port},
	}
}

// Location names the Service the source queries, "namespace/name".
func (p *PromSource) Location() string { return p.location }

// promResponse is the envelope of the Prometheus HTTP API.
type promResponse struct {
	Status    string `json:"status"`
	ErrorType string `json:"errorType"`
	Error     string `json:"error"`
	Data      struct {
		Result []struct {
			Value  [2]any   `json:"value"`
			Values [][2]any `json:"values"`
		} `json:"result"`
	} `json:"data"`
}

func (p *PromSource) query(ctx context.Context, path string, params map[string]string) (*promResponse, error) {
	body, err := p.api.get(ctx, path, params)
	if err != nil {
		return nil, err
	}
	var res promResponse
	if err := json.Unmarshal(body, &res); err != nil {
		return nil, err
	}
	if res.Status != "success" {
		return nil, fmt.Errorf("%s: %s", res.ErrorType, res.Error)
	}
	return &res, nil
}

// sampleValue parses a [time, "value"] pair. PromQL yields NaN/Inf for
// 0/0 and the like, which are no measurement (and JSON cannot carry them).
func sampleValue(v [2]any) (time.Time, float64, bool) {
	ts, _ := v[0].(float64)
	str, _ := v[1].(string)
	value, err := strconv.ParseFloat(str, 64)
	if err != nil || math.IsNaN(value) || math.IsInf(value, 0) {
		return time.Time{}, 0, false
	}
	return time.Unix(int64(ts), 0), value, true
}

func (p *PromSource) rangeQuery(ctx context.Context, query string, minutes int) ([]MetricPoint, error) {
	end := time.Now().Unix()
	start := end - int64(minutes)*60
	step := max(15, minutes*60/120)
	res, err := p.query(ctx, "api/v1/query_range", map[string]string{
		"query": query,
		"start": strconv.FormatInt(start, 10),
		"end":   strconv.FormatInt(end, 10),
		"step":  strconv.Itoa(step),
	})
	if err != nil {
		return nil, err
	}
	points := []MetricPoint{}
	if len(res.Data.Result) == 0 {
		return points, nil
	}
	for _, v := range res.Data.Result[0].Values {
		if ts, value, ok := sampleValue(v); ok {
			points = append(points, MetricPoint{Time: ts.Format("15:04"), Value: math.Round(value*10) / 10})
		}
	}
	return points, nil
}

// errNoValue is instant's error for a query without a (finite) result.
var errNoValue = errors.New("the query returned no value")

func (p *PromSource) instant(ctx context.Context, query string) (float64, error) {
	res, err := p.query(ctx, "api/v1/query", map[string]string{"query": query})
	if err != nil {
		return 0, err
	}
	if len(res.Data.Result) == 0 {
		return 0, errNoValue
	}
	_, value, ok := sampleValue(res.Data.Result[0].Value)
	if !ok {
		return 0, errNoValue
	}
	return value, nil
}

const (
	netDevices = `device!~"lo|veth.*"`
	// The root filesystem; RHCOS (OpenShift) mounts it at /sysroot.
	rootFS = `mountpoint=~"/|/sysroot"`
	// cAdvisor also reports the pod's own cgroup (container="") and, with
	// dockershim, the sandbox ("POD"): summing those would count twice.
	realContainers = `container!="",container!="POD"`
)

// clusterQueries are the cluster-wide series; sel narrows them to one node.
func clusterQueries(sel string) map[string]string {
	and := func(matchers ...string) string {
		out := ""
		for _, m := range matchers {
			if m == "" {
				continue
			}
			if out != "" {
				out += ","
			}
			out += m
		}
		return out
	}
	return map[string]string{
		"cpu":     fmt.Sprintf(`100 * (1 - avg(rate(node_cpu_seconds_total{%s}[5m])))`, and(`mode="idle"`, sel)),
		"memory":  fmt.Sprintf(`100 * (1 - sum(node_memory_MemAvailable_bytes{%[1]s}) / sum(node_memory_MemTotal_bytes{%[1]s}))`, sel),
		"network": fmt.Sprintf(`sum(rate(node_network_receive_bytes_total{%[1]s}[5m]) + rate(node_network_transmit_bytes_total{%[1]s}[5m])) / 1e6`, and(netDevices, sel)),
		"fs":      fmt.Sprintf(`100 * (1 - sum(node_filesystem_avail_bytes{%[1]s}) / sum(node_filesystem_size_bytes{%[1]s}))`, and(rootFS, sel)),
	}
}

func (p *PromSource) ClusterSeries(ctx context.Context, metric string, minutes int) ([]MetricPoint, error) {
	return p.rangeQuery(ctx, clusterQueries("")[metric], minutes)
}

// nodeSelectors are the labels node-exporter series name their node by:
// kubernetes_node (the Prometheus chart's relabeling), node
// (kube-prometheus-stack) and instance (OpenShift monitoring).
var nodeSelectors = []string{"kubernetes_node", "node", "instance"}

func (p *PromSource) NodeSeries(ctx context.Context, node, metric string, minutes int) ([]MetricPoint, error) {
	for _, label := range nodeSelectors {
		points, err := p.rangeQuery(ctx, clusterQueries(fmt.Sprintf(`%s=%q`, label, node))[metric], minutes)
		if err != nil || len(points) > 0 {
			return points, err
		}
	}
	// Node-exporter targets without a node label: on a single-node cluster
	// the cluster's series are that node's; on others there is no series.
	nodes, err := p.rt.CS.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err == nil && len(nodes.Items) == 1 && nodes.Items[0].Name == node {
		return p.ClusterSeries(ctx, metric, minutes)
	}
	return []MetricPoint{}, nil
}

func (p *PromSource) PodSeries(ctx context.Context, namespace, pod, metric string, minutes int) ([]MetricPoint, error) {
	sel := fmt.Sprintf(`namespace=%q,pod=%q`, namespace, pod)
	queries := map[string]string{
		"cpu":     fmt.Sprintf(`100 * sum(rate(container_cpu_usage_seconds_total{%s,%s}[5m])) / scalar(avg(machine_cpu_cores))`, sel, realContainers),
		"memory":  fmt.Sprintf(`100 * sum(container_memory_working_set_bytes{%s,%s}) / scalar(avg(node_memory_MemTotal_bytes))`, sel, realContainers),
		"network": fmt.Sprintf(`sum(rate(container_network_receive_bytes_total{%[1]s}[5m]) + rate(container_network_transmit_bytes_total{%[1]s}[5m])) / 1e6`, sel),
	}
	q, ok := queries[metric]
	if !ok {
		return nil, reason.New(reasonPodMetric, reason.Params{"metric": metric})
	}
	return p.rangeQuery(ctx, q, minutes)
}

// clusterPercent is the cluster's current CPU or memory use in percent.
func (p *PromSource) clusterPercent(ctx context.Context, metric string) (float64, error) {
	return p.instant(ctx, clusterQueries("")[metric])
}
