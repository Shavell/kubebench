package kube

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"strconv"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PromSource serves utilization series from a Prometheus reached through
// the API server's service proxy — full history plus network/filesystem
// metrics that metrics-server cannot provide.
type PromSource struct {
	rt        *Runtime
	Namespace string `json:"namespace"`
	Service   string `json:"service"`
	Port      string `json:"port"`
}

var promCandidates = []string{
	"prometheus",
	"prometheus-server",
	"prometheus-operated",
	"prometheus-k8s",
	"prometheus-kube-prometheus-prometheus",
	"kube-prometheus-stack-prometheus",
}

func (r *Runtime) discoverProm() {
	defer close(r.promDone)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	services, err := r.CS.CoreV1().Services("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return
	}
	for _, svc := range services.Items {
		if !contains(promCandidates, svc.Name) {
			continue
		}
		for _, p := range svc.Spec.Ports {
			if p.Port != 80 && p.Port != 9090 {
				continue
			}
			prom := &PromSource{
				rt: r, Namespace: svc.Namespace, Service: svc.Name,
				Port: strconv.Itoa(int(p.Port)),
			}
			if _, err := prom.raw(ctx, "api/v1/query", map[string]string{"query": "up"}); err == nil {
				r.mu.Lock()
				r.prom = prom
				r.mu.Unlock()
				log.Printf("[prometheus] cluster=%s using %s/%s:%s",
					r.Cluster.Name, svc.Namespace, svc.Name, prom.Port)
				return
			}
		}
	}
	log.Printf("[prometheus] cluster=%s not found — metrics-server only", r.Cluster.Name)
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

func (p *PromSource) Location() string {
	return p.Namespace + "/" + p.Service
}

func (p *PromSource) raw(ctx context.Context, path string, params map[string]string) ([]byte, error) {
	return p.rt.CS.CoreV1().Services(p.Namespace).
		ProxyGet("http", p.Service, p.Port, path, params).
		DoRaw(ctx)
}

type promRangeResponse struct {
	Status string `json:"status"`
	Data   struct {
		Result []struct {
			Values [][2]any `json:"values"`
		} `json:"result"`
	} `json:"data"`
}

func (p *PromSource) rangeQuery(ctx context.Context, query string, minutes int) ([]MetricPoint, error) {
	end := time.Now().Unix()
	start := end - int64(minutes)*60
	step := max(15, minutes*60/120)
	body, err := p.raw(ctx, "api/v1/query_range", map[string]string{
		"query": query,
		"start": strconv.FormatInt(start, 10),
		"end":   strconv.FormatInt(end, 10),
		"step":  strconv.Itoa(step),
	})
	if err != nil {
		return nil, err
	}
	var res promRangeResponse
	if err := json.Unmarshal(body, &res); err != nil {
		return nil, err
	}
	if res.Status != "success" || len(res.Data.Result) == 0 {
		return []MetricPoint{}, nil
	}
	points := []MetricPoint{}
	for _, v := range res.Data.Result[0].Values {
		ts, _ := v[0].(float64)
		str, _ := v[1].(string)
		value, err := strconv.ParseFloat(str, 64)
		// PromQL yields NaN/Inf for 0/0 and the like; JSON cannot carry them.
		if err != nil || math.IsNaN(value) || math.IsInf(value, 0) {
			continue
		}
		points = append(points, MetricPoint{
			Time:  time.Unix(int64(ts), 0).Format("15:04"),
			Value: math.Round(value*10) / 10,
		})
	}
	return points, nil
}

const netDevices = `device!~"lo|veth.*"`

func (p *PromSource) ClusterSeries(ctx context.Context, metric string, minutes int) ([]MetricPoint, error) {
	queries := map[string]string{
		"cpu":     `100 * (1 - avg(rate(node_cpu_seconds_total{mode="idle"}[5m])))`,
		"memory":  `100 * (1 - sum(node_memory_MemAvailable_bytes) / sum(node_memory_MemTotal_bytes))`,
		"network": fmt.Sprintf(`sum(rate(node_network_receive_bytes_total{%[1]s}[5m]) + rate(node_network_transmit_bytes_total{%[1]s}[5m])) / 1e6`, netDevices),
		"fs":      `100 * (1 - sum(node_filesystem_avail_bytes{mountpoint="/"}) / sum(node_filesystem_size_bytes{mountpoint="/"}))`,
	}
	return p.rangeQuery(ctx, queries[metric], minutes)
}

func (p *PromSource) NodeSeries(ctx context.Context, node, metric string, minutes int) ([]MetricPoint, error) {
	sel := fmt.Sprintf(`kubernetes_node="%s"`, node)
	// kube-prometheus-stack labels node-exporter targets with "node"
	// via the service monitor relabeling; try both.
	altSel := fmt.Sprintf(`node="%s"`, node)
	queries := func(s string) map[string]string {
		return map[string]string{
			"cpu":     fmt.Sprintf(`100 * (1 - avg(rate(node_cpu_seconds_total{mode="idle",%s}[5m])))`, s),
			"memory":  fmt.Sprintf(`100 * (1 - sum(node_memory_MemAvailable_bytes{%s}) / sum(node_memory_MemTotal_bytes{%s}))`, s, s),
			"network": fmt.Sprintf(`sum(rate(node_network_receive_bytes_total{%s,%s}[5m]) + rate(node_network_transmit_bytes_total{%s,%s}[5m])) / 1e6`, netDevices, s, netDevices, s),
			"fs":      fmt.Sprintf(`100 * (1 - sum(node_filesystem_avail_bytes{mountpoint="/",%s}) / sum(node_filesystem_size_bytes{mountpoint="/",%s}))`, s, s),
		}
	}
	points, err := p.rangeQuery(ctx, queries(sel)[metric], minutes)
	if err == nil && len(points) == 0 {
		points, err = p.rangeQuery(ctx, queries(altSel)[metric], minutes)
	}
	if err == nil && len(points) == 0 {
		// Single-node clusters: fall back to instance-agnostic series.
		return p.ClusterSeries(ctx, metric, minutes)
	}
	return points, err
}

func (p *PromSource) PodSeries(ctx context.Context, namespace, pod, metric string, minutes int) ([]MetricPoint, error) {
	queries := map[string]string{
		"cpu":     fmt.Sprintf(`100 * sum(rate(container_cpu_usage_seconds_total{namespace="%s",pod="%s"}[5m])) / scalar(avg(machine_cpu_cores))`, namespace, pod),
		"memory":  fmt.Sprintf(`100 * sum(container_memory_working_set_bytes{namespace="%s",pod="%s"}) / scalar(avg(node_memory_MemTotal_bytes))`, namespace, pod),
		"network": fmt.Sprintf(`sum(rate(container_network_receive_bytes_total{namespace="%s",pod="%s"}[5m]) + rate(container_network_transmit_bytes_total{namespace="%s",pod="%s"}[5m])) / 1e6`, namespace, pod, namespace, pod),
	}
	q, ok := queries[metric]
	if !ok {
		return nil, fmt.Errorf(`metric %q not available for pods`, metric)
	}
	return p.rangeQuery(ctx, q, minutes)
}
