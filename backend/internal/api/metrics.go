package api

import (
	"context"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"kubebench/internal/kube"
	"kubebench/internal/plugin"
	"kubebench/internal/reason"
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
		if err := rt.PromProblem(); err != nil {
			return nil, unavailable(err)
		}
		return nil, fail(http.StatusServiceUnavailable, plugin.CodeCapabilityUnavailable, reasonNoPrometheus, nil)
	}
	return prom, nil
}

// writeSeries answers a series request for one of metrics from Prometheus
// or the metrics-server sampler (see resolveSource). A Prometheus query
// that fails is reported, never replaced with other data: only
// source=auto falls back to the sampler, which keeps CPU and memory only.
func writeSeries(w http.ResponseWriter, r *http.Request, rt *kube.Runtime, metrics []string,
	fromProm func(*kube.PromSource) ([]kube.MetricPoint, error), fromSampler func(context.Context) ([]kube.MetricPoint, error)) {
	if metric := r.URL.Query().Get("metric"); !slices.Contains(metrics, metric) {
		writeErr(w, badRequest(reason.FieldOneOf, reason.Params{"field": "metric", "values": strings.Join(metrics, ", ")}))
		return
	}
	prom, err := resolveSource(r, rt)
	if err != nil {
		writeErr(w, err)
		return
	}
	var promErr error
	if prom == nil && r.URL.Query().Get("source") != "metrics-server" {
		// OpenShift monitoring that KubeBench may not use explains a
		// missing series better than what metrics-server lacks.
		promErr = rt.PromProblem()
	}
	if prom != nil {
		points, err := fromProm(prom)
		if err == nil {
			plugin.WriteJSON(w, points)
			return
		}
		promErr = reason.Wrap(err, reasonPromQuery, reason.Params{"location": prom.Location(), "detail": err.Error()})
		if r.URL.Query().Get("source") == "prometheus" {
			writeErr(w, unavailable(promErr))
			return
		}
	}
	ctx, cancel := context.WithTimeout(r.Context(), probeWait)
	defer cancel()
	points, err := fromSampler(ctx)
	if err != nil {
		if promErr != nil {
			err = promErr // what went wrong, rather than what the fallback lacks
		}
		writeErr(w, unavailable(err))
		return
	}
	plugin.WriteJSON(w, points)
}

// unavailable is the error for a capability the active cluster lacks.
func unavailable(err error) *apiError {
	return refuse(http.StatusServiceUnavailable, plugin.CodeCapabilityUnavailable, err)
}

// sourceStatus is one metrics source in GET /api/metrics/sources. An
// unavailable source carries the reason as {error, reason, params}.
type sourceStatus struct {
	Available bool            `json:"available"`
	Location  *string         `json:"location"`
	Reason    *reason.Message `json:"unavailable,omitempty"`
}

func unavailableSource(err error) sourceStatus {
	why := reason.MessageOf(err)
	return sourceStatus{Reason: &why}
}

// handleMetricSources reports what each source can serve now:
// metrics-server by the outcome of the sampler's latest scrape of the
// metrics API, Prometheus by discovery (with why OpenShift monitoring is
// unusable, when it exists).
func (s *Server) handleMetricSources(w http.ResponseWriter, r *http.Request, c *call) {
	ctx, cancel := context.WithTimeout(r.Context(), probeWait)
	defer cancel()
	metricsServer := sourceStatus{Available: true}
	if err := c.rt.Sampler.Status(ctx); err != nil {
		metricsServer = unavailableSource(err)
	}
	_ = c.rt.WaitPromDiscovered(ctx)
	prometheus := unavailableSource(reason.New(reasonNoPrometheus, nil))
	if prom := c.rt.Prom(); prom != nil {
		location := prom.Location()
		prometheus = sourceStatus{Available: true, Location: &location}
	} else if err := c.rt.PromProblem(); err != nil {
		prometheus = unavailableSource(err)
	}
	plugin.WriteJSON(w, map[string]sourceStatus{"metricsServer": metricsServer, "prometheus": prometheus})
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
		func(ctx context.Context) ([]kube.MetricPoint, error) {
			return c.rt.Sampler.ClusterSeries(ctx, metric, minutes)
		})
}

func (s *Server) handleNodeSeries(w http.ResponseWriter, r *http.Request, c *call) {
	q := r.URL.Query()
	metric, node, minutes := q.Get("metric"), q.Get("node"), minutesParam(r)
	writeSeries(w, r, c.rt, clusterMetrics,
		func(prom *kube.PromSource) ([]kube.MetricPoint, error) {
			return prom.NodeSeries(r.Context(), node, metric, minutes)
		},
		func(ctx context.Context) ([]kube.MetricPoint, error) {
			return c.rt.Sampler.NodeSeries(ctx, node, metric, minutes)
		})
}

func (s *Server) handlePodSeries(w http.ResponseWriter, r *http.Request, c *call) {
	q := r.URL.Query()
	metric, ns, pod, minutes := q.Get("metric"), q.Get("ns"), q.Get("pod"), minutesParam(r)
	writeSeries(w, r, c.rt, podMetrics,
		func(prom *kube.PromSource) ([]kube.MetricPoint, error) {
			return prom.PodSeries(r.Context(), ns, pod, metric, minutes)
		},
		func(ctx context.Context) ([]kube.MetricPoint, error) {
			return c.rt.Sampler.PodSeries(ctx, ns, pod, metric, minutes)
		})
}
