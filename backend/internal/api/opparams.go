package api

import (
	"errors"
	"net/http"
	"net/url"
	"strconv"

	"kubebench/internal/kube"
	"kubebench/internal/plugin"
)

// Helpers of the workload and node operations (§7.3): query parameters
// and the reporting of KubeBench's own refusals.

// requiredParam returns a query parameter that must be present.
func requiredParam(q url.Values, key string) (string, error) {
	v := q.Get(key)
	if v == "" {
		return "", badRequest("%s is required", key)
	}
	return v, nil
}

// boolFlag parses an optional true/false query parameter.
func boolFlag(q url.Values, key string, fallback bool) (bool, error) {
	raw := q.Get(key)
	if raw == "" {
		return fallback, nil
	}
	v, err := strconv.ParseBool(raw)
	if err != nil {
		return false, badRequest("%s must be true or false", key)
	}
	return v, nil
}

// boundedInt parses an optional integer query parameter in [lowest, highest].
func boundedInt(q url.Values, key string, fallback, lowest, highest int) (int, error) {
	raw := q.Get(key)
	if raw == "" {
		return fallback, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < lowest || n > highest {
		return 0, badRequest("%s must be a number from %d to %d", key, lowest, highest)
	}
	return n, nil
}

// writeOpError reports a failed operation: a refusal of KubeBench itself
// (kube.OpError) with its own status and code, anything else as an error of
// the cluster.
func writeOpError(w http.ResponseWriter, err error) {
	var op *kube.OpError
	if !errors.As(err, &op) {
		plugin.WriteK8sError(w, err)
		return
	}
	switch op.Kind {
	case kube.OpInvalid:
		plugin.WriteError(w, http.StatusBadRequest, plugin.CodeBadRequest, op)
	case kube.OpNotFound:
		plugin.WriteError(w, http.StatusNotFound, plugin.CodeNotFound, op)
	case kube.OpConflict:
		plugin.WriteError(w, http.StatusConflict, plugin.CodeConflict, op)
	case kube.OpProtected:
		plugin.WriteError(w, http.StatusForbidden, plugin.CodeForbidden, op)
	default:
		plugin.WriteError(w, http.StatusInternalServerError, plugin.CodeInternal, op)
	}
}

// respondOp answers an operation that returns no data.
func respondOp(w http.ResponseWriter, err error) {
	if err != nil {
		writeOpError(w, err)
		return
	}
	writeOK(w)
}

// writeOpResult answers with an operation's result, or its error.
func writeOpResult(w http.ResponseWriter, v any, err error) {
	if err != nil {
		writeOpError(w, err)
		return
	}
	plugin.WriteJSON(w, v)
}
