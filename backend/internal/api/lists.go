package api

import (
	"net/http"
	"net/url"
	"strconv"

	"kubebench/internal/kube"
	"kubebench/internal/plugin"
	"kubebench/internal/reason"
	"kubebench/internal/watch"
)

// headerListRemaining marks a list cut at its limit parameter (design §4.4,
// "Lists"): how many objects were left out, or "unknown" when the cluster
// did not say. The body stays the plain JSON array.
const headerListRemaining = "X-KubeBench-List-Remaining"

// listLimit parses the optional limit parameter of the list routes: at
// least 1; absent = 0, no limit.
func listLimit(q url.Values) (int, error) {
	raw := q.Get("limit")
	if raw == "" {
		return 0, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 {
		return 0, badRequest(reason.FieldMin, reason.Params{"field": "limit", "min": 1})
	}
	return n, nil
}

// writeList answers a list read with listLimit: the array, and when it
// was cut, headerListRemaining.
func writeList(w http.ResponseWriter, v any, cut *kube.Truncated, err error) {
	if err != nil {
		plugin.WriteK8sError(w, err)
		return
	}
	if cut != nil {
		remaining := "unknown"
		if cut.Remaining >= 0 {
			remaining = strconv.FormatInt(cut.Remaining, 10)
		}
		w.Header().Set(headerListRemaining, remaining)
	}
	plugin.WriteJSON(w, v)
}

// listHandler serves a list route with the limit parameter.
func listHandler[T any](list func(r *http.Request, c *call, limit int) ([]T, *kube.Truncated, error)) func(http.ResponseWriter, *http.Request, *call) {
	return func(w http.ResponseWriter, r *http.Request, c *call) {
		limit, err := listLimit(r.URL.Query())
		if err != nil {
			writeErr(w, err)
			return
		}
		out, cut, err := list(r, c, limit)
		writeList(w, out, cut, err)
	}
}

// handleCount counts the objects of one resource (design §4.4, "Lists"):
// topic names it as live updates do ("pods", "deployments.apps"), ns
// optionally one namespace. Views that show only how many objects exist
// (the explorer's badges) read this instead of the whole list.
func (s *Server) handleCount(w http.ResponseWriter, r *http.Request, c *call) {
	q := r.URL.Query()
	topic := q.Get("topic")
	if topic == "" {
		writeErr(w, required("topic"))
		return
	}
	if !watch.ValidTopic(topic) {
		writeErr(w, badRequest(reasonTopic, reason.Params{"topic": topic}))
		return
	}
	n, err := c.rt.CountObjects(r.Context(), topic, q.Get("ns"))
	writeResult(w, map[string]int{"count": n}, err)
}
