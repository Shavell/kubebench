package api

import (
	"errors"
	"net/http"
	"strconv"

	"kubebench/internal/kube"
)

func minutesParam(r *http.Request) int {
	minutes, err := strconv.Atoi(r.URL.Query().Get("minutes"))
	if err != nil || minutes <= 0 {
		return 60
	}
	return minutes
}

// resolveSource implements source=auto|prometheus|metrics-server:
// auto prefers Prometheus; metrics-server has no network/fs data (501 so
// the frontend falls back to mock series).
func resolveSource(r *http.Request, rt *kube.Runtime) (*kube.PromSource, error) {
	wanted := r.URL.Query().Get("source")
	metric := r.URL.Query().Get("metric")
	prom := rt.Prom()
	switch wanted {
	case "prometheus":
		if prom == nil {
			return nil, errors.New("prometheus source not available")
		}
		return prom, nil
	case "metrics-server":
		prom = nil
	}
	if prom == nil && (metric == "network" || metric == "fs") {
		return nil, errors.New("metrics-server has no " + metric + " data")
	}
	return prom, nil
}

func (s *Server) handleMetricSources(w http.ResponseWriter, _ *http.Request, rt *kube.Runtime) {
	prom := rt.Prom()
	out := map[string]any{
		"metricsServer": map[string]any{"available": true},
		"prometheus":    map[string]any{"available": prom != nil, "location": nil},
	}
	if prom != nil {
		out["prometheus"] = map[string]any{"available": true, "location": prom.Location()}
	}
	writeJSON(w, out)
}

func (s *Server) handleOverview(w http.ResponseWriter, r *http.Request, rt *kube.Runtime) {
	out, err := rt.Overview(r.Context())
	if err != nil {
		writeK8sErr(w, err)
		return
	}
	writeJSON(w, out)
}

func (s *Server) handleAllocation(w http.ResponseWriter, r *http.Request, rt *kube.Runtime) {
	out, err := rt.Allocation(r.Context())
	if err != nil {
		writeK8sErr(w, err)
		return
	}
	writeJSON(w, out)
}

func (s *Server) handleClusterSeries(w http.ResponseWriter, r *http.Request, rt *kube.Runtime) {
	prom, err := resolveSource(r, rt)
	if err != nil {
		writeErr(w, http.StatusNotImplemented, err)
		return
	}
	metric := r.URL.Query().Get("metric")
	minutes := minutesParam(r)
	if prom != nil {
		points, err := prom.ClusterSeries(r.Context(), metric, minutes)
		if err == nil {
			writeJSON(w, points)
			return
		}
	}
	writeJSON(w, rt.Sampler.ClusterSeries(metric, minutes))
}

func (s *Server) handleNodeSeries(w http.ResponseWriter, r *http.Request, rt *kube.Runtime) {
	prom, err := resolveSource(r, rt)
	if err != nil {
		writeErr(w, http.StatusNotImplemented, err)
		return
	}
	q := r.URL.Query()
	metric, node := q.Get("metric"), q.Get("node")
	minutes := minutesParam(r)
	if prom != nil {
		points, err := prom.NodeSeries(r.Context(), node, metric, minutes)
		if err == nil {
			writeJSON(w, points)
			return
		}
	}
	writeJSON(w, rt.Sampler.NodeSeries(node, metric, minutes))
}

func (s *Server) handlePodSeries(w http.ResponseWriter, r *http.Request, rt *kube.Runtime) {
	prom, err := resolveSource(r, rt)
	if err != nil {
		writeErr(w, http.StatusNotImplemented, err)
		return
	}
	q := r.URL.Query()
	metric, ns, pod := q.Get("metric"), q.Get("ns"), q.Get("pod")
	minutes := minutesParam(r)
	if prom != nil {
		points, err := prom.PodSeries(r.Context(), ns, pod, metric, minutes)
		if err == nil {
			writeJSON(w, points)
			return
		}
	}
	writeJSON(w, rt.Sampler.PodSeries(ns, pod, metric, minutes))
}
