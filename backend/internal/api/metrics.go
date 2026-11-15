package api

import (
	"context"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"kubebench/internal/kube"
	"kubebench/internal/plugin"
)

func minutesParam(r *http.Request) int {
	minutes, err := strconv.Atoi(r.URL.Query().Get("minutes"))
	if err != nil || minutes <= 0 {
		return 60
	}
	return minutes
}

// resolveSource implements source=auto|prometheus|metrics-server: auto
// prefers Prometheus when one was found, metrics-server never uses it.
// Runtime discovery is waited for (≤ probeWait), so a fresh runtime does not
// report "no Prometheus" merely because it is still looking.
func resolveSource(r *http.Request, rt *kube.Runtime) (*kube.PromSource, error) {
	wanted := r.URL.Query().Get("source")
	if wanted == "metrics-server" {
		return nil, nil
	}
	ctx, cancel := context.WithTimeout(r.Context(), probeWait)
	defer cancel()
	_ = rt.WaitPromDiscovered(ctx)
	prom := rt.Prom()
	if prom == nil && wanted == "prometheus" {
		return nil, unavailable("no Prometheus was found on this cluster")
	}
	return prom, nil
}

// writeSeries answers a series request for one of metrics from Prometheus
// or the metrics-server sampler (see resolveSource). A Prometheus query
// that fails is reported, never replaced with other data: only
// source=auto falls back to the sampler, which keeps CPU and memory only.
func writeSeries(w http.ResponseWriter, r *http.Request, rt *kube.Runtime, metrics []string,
	fromProm func(*kube.PromSource) ([]kube.MetricPoint, error), fromSampler func() ([]kube.MetricPoint, error)) {
	if metric := r.URL.Query().Get("metric"); !slices.Contains(metrics, metric) {
		writeErr(w, badRequest("metric must be one of %s", strings.Join(metrics, ", ")))
		return
	}
	prom, err := resolveSource(r, rt)
	if err != nil {
		writeErr(w, err)
		return
	}
	var promErr error
	if prom != nil {
		points, err := fromProm(prom)
		if err == nil {
			plugin.WriteJSON(w, points)
			return
		}
		promErr = fmt.Errorf("the query to Prometheus %s failed: %w", prom.Location(), err)
		if r.URL.Query().Get("source") == "prometheus" {
			writeErr(w, unavailable("%v", promErr))
			return
		}
	}
	points, err := fromSampler()
	if err != nil {
		if promErr != nil {
			err = promErr // what went wrong, rather than what the fallback lacks
		}
		writeErr(w, unavailable("%v", err))
		return
	}
	plugin.WriteJSON(w, points)
}

// unavailable is the error for a capability the active cluster lacks.
func unavailable(format string, args ...any) *apiError {
	return errorf(http.StatusServiceUnavailable, plugin.CodeCapabilityUnavailable, format, args...)
}

func (s *Server) handleMetricSources(w http.ResponseWriter, r *http.Request, c *call) {
	ctx, cancel := context.WithTimeout(r.Context(), probeWait)
	defer cancel()
	_ = c.rt.WaitPromDiscovered(ctx)
	prom := c.rt.Prom()
	out := map[string]any{
		"metricsServer": map[string]any{"available": true},
		"prometheus":    map[string]any{"available": prom != nil, "location": nil},
	}
	if prom != nil {
		out["prometheus"] = map[string]any{"available": true, "location": prom.Location()}
	}
	plugin.WriteJSON(w, out)
}

func (s *Server) handleOverview(w http.ResponseWriter, r *http.Request, c *call) {
	out, err := c.rt.Overview(r.Context())
	writeResult(w, out, err)
}

func (s *Server) handleAllocation(w http.ResponseWriter, r *http.Request, c *call) {
	out, err := c.rt.Allocation(r.Context())
	writeResult(w, out, err)
}

// Metrics of each series endpoint (src/data/types.ts MetricKind).
var (
	clusterMetrics = []string{"cpu", "memory", "network", "fs"}
	podMetrics     = []string{"cpu", "memory", "network"}
)

func (s *Server) handleClusterSeries(w http.ResponseWriter, r *http.Request, c *call) {
	metric, minutes := r.URL.Query().Get("metric"), minutesParam(r)
	writeSeries(w, r, c.rt, clusterMetrics,
		func(prom *kube.PromSource) ([]kube.MetricPoint, error) {
			return prom.ClusterSeries(r.Context(), metric, minutes)
		},
		func() ([]kube.MetricPoint, error) { return c.rt.Sampler.ClusterSeries(metric, minutes) })
}

func (s *Server) handleNodeSeries(w http.ResponseWriter, r *http.Request, c *call) {
	q := r.URL.Query()
	metric, node, minutes := q.Get("metric"), q.Get("node"), minutesParam(r)
	writeSeries(w, r, c.rt, clusterMetrics,
		func(prom *kube.PromSource) ([]kube.MetricPoint, error) {
			return prom.NodeSeries(r.Context(), node, metric, minutes)
		},
		func() ([]kube.MetricPoint, error) { return c.rt.Sampler.NodeSeries(node, metric, minutes) })
}

func (s *Server) handlePodSeries(w http.ResponseWriter, r *http.Request, c *call) {
	q := r.URL.Query()
	metric, ns, pod, minutes := q.Get("metric"), q.Get("ns"), q.Get("pod"), minutesParam(r)
	writeSeries(w, r, c.rt, podMetrics,
		func(prom *kube.PromSource) ([]kube.MetricPoint, error) {
			return prom.PodSeries(r.Context(), ns, pod, metric, minutes)
		},
		func() ([]kube.MetricPoint, error) { return c.rt.Sampler.PodSeries(ns, pod, metric, minutes) })
}
