package api

import (
	"net/http"
	"strconv"

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
// prefers Prometheus. metrics-server has no network/fs history, so those
// metrics are unavailable without Prometheus.
func resolveSource(r *http.Request, rt *kube.Runtime) (*kube.PromSource, error) {
	wanted := r.URL.Query().Get("source")
	metric := r.URL.Query().Get("metric")
	prom := rt.Prom()
	switch wanted {
	case "prometheus":
		if prom == nil {
			return nil, unavailable("no Prometheus was found on this cluster")
		}
		return prom, nil
	case "metrics-server":
		prom = nil
	}
	if prom == nil && (metric == "network" || metric == "fs") {
		return nil, unavailable("metrics-server has no %s data; it needs Prometheus", metric)
	}
	return prom, nil
}

// unavailable is the error for a capability the active cluster lacks.
func unavailable(format string, args ...any) *apiError {
	return errorf(http.StatusServiceUnavailable, plugin.CodeCapabilityUnavailable, format, args...)
}

func (s *Server) handleMetricSources(w http.ResponseWriter, _ *http.Request, c *call) {
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

func (s *Server) handleClusterSeries(w http.ResponseWriter, r *http.Request, c *call) {
	prom, err := resolveSource(r, c.rt)
	if err != nil {
		writeErr(w, err)
		return
	}
	metric := r.URL.Query().Get("metric")
	minutes := minutesParam(r)
	if prom != nil {
		if points, err := prom.ClusterSeries(r.Context(), metric, minutes); err == nil {
			plugin.WriteJSON(w, points)
			return
		}
	}
	plugin.WriteJSON(w, c.rt.Sampler.ClusterSeries(metric, minutes))
}

func (s *Server) handleNodeSeries(w http.ResponseWriter, r *http.Request, c *call) {
	prom, err := resolveSource(r, c.rt)
	if err != nil {
		writeErr(w, err)
		return
	}
	q := r.URL.Query()
	metric, node := q.Get("metric"), q.Get("node")
	minutes := minutesParam(r)
	if prom != nil {
		if points, err := prom.NodeSeries(r.Context(), node, metric, minutes); err == nil {
			plugin.WriteJSON(w, points)
			return
		}
	}
	plugin.WriteJSON(w, c.rt.Sampler.NodeSeries(node, metric, minutes))
}

func (s *Server) handlePodSeries(w http.ResponseWriter, r *http.Request, c *call) {
	prom, err := resolveSource(r, c.rt)
	if err != nil {
		writeErr(w, err)
		return
	}
	q := r.URL.Query()
	metric, ns, pod := q.Get("metric"), q.Get("ns"), q.Get("pod")
	minutes := minutesParam(r)
	if prom != nil {
		if points, err := prom.PodSeries(r.Context(), ns, pod, metric, minutes); err == nil {
			plugin.WriteJSON(w, points)
			return
		}
	}
	plugin.WriteJSON(w, c.rt.Sampler.PodSeries(ns, pod, metric, minutes))
}
