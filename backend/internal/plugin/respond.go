package plugin

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"

	"kubebench/internal/logsafe"
	"kubebench/internal/reason"
)

// Machine-readable error codes (docs/design-v0.5.md §3.3). The frontend
// branches on them, so their spelling is part of the API.
const (
	CodeUnauthenticated        = "unauthenticated"
	CodeSessionExpired         = "session_expired"
	CodeForbidden              = "forbidden"
	CodeForbiddenRole          = "forbidden_role"
	CodeClusterReadOnly        = "cluster_read_only"
	CodeAdminRequired          = "admin_required"
	CodeNoCluster              = "no_cluster"
	CodeNotFound               = "not_found"
	CodeBadRequest             = "bad_request"
	CodeConflict               = "conflict"
	CodeRateLimited            = "rate_limited"
	CodePluginDisabled         = "plugin_disabled"
	CodePluginUnavailable      = "plugin_unavailable"
	CodeCapabilityUnavailable  = "capability_unavailable"
	CodeRedactedSecret         = "redacted_secret"
	CodePasswordChangeRequired = "password_change_required"
	// CodeKubernetes marks an error returned by (or while reaching) the
	// cluster API; the HTTP status mirrors the Kubernetes status.
	CodeKubernetes = "kubernetes"
	// CodeInternal marks a KubeBench-side failure (e.g. the database).
	CodeInternal = "internal"
)

// ErrorBody is the JSON shape of every error response. Reason and Params
// are set when KubeBench itself explains the error (a reason.Reasoned
// error): the UI shows the reason's translation and falls back to Error,
// the English text.
type ErrorBody struct {
	Error  string        `json:"error"`
	Code   string        `json:"code"`
	Reason string        `json:"reason,omitempty"`
	Params reason.Params `json:"params,omitempty"`
}

// WriteJSON writes v as a 200 JSON response. v is encoded before anything
// is sent, so an unencodable value still yields a proper 500.
func WriteJSON(w http.ResponseWriter, v any) {
	body, err := json.Marshal(v)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, CodeInternal, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(append(body, '\n'))
}

// WriteError writes {"error": err.Error(), "code": code} with status, plus
// "reason" and "params" when err carries a reason (reason.Of); a nil err
// uses the status text. Server errors are logged.
func WriteError(w http.ResponseWriter, status int, code string, err error) {
	body := ErrorBody{Error: http.StatusText(status), Code: code}
	if err != nil {
		body.Error = err.Error()
	}
	if rc, params, ok := reason.Of(err); ok {
		body.Reason, body.Params = rc.String(), params
	}
	msg := body.Error
	if status >= http.StatusInternalServerError {
		// The message can quote request input: escape line breaks.
		log.Printf("[api] %d %s: %s", status, logsafe.Text(code), logsafe.Text(msg))
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

// WriteK8sError reports a client-go error with code "kubernetes". Kubernetes
// API errors keep their status so RBAC denials and missing objects show up
// verbatim; failures to reach the cluster become 502/504.
//
// A 401 from the cluster means KubeBench's stored credential was rejected.
// It is reported as 502: a 401 would tell the browser that its own
// KubeBench session ended.
func WriteK8sError(w http.ResponseWriter, err error) {
	status, code := http.StatusBadGateway, CodeKubernetes
	var apiStatus apierrors.APIStatus
	switch {
	case errors.As(err, &apiStatus) && apiStatus.Status().Code > 0:
		if status = int(apiStatus.Status().Code); status == http.StatusUnauthorized {
			status = http.StatusBadGateway
		}
	case meta.IsNoMatchError(err):
		status, code = http.StatusNotFound, CodeNotFound
	case errors.Is(err, context.DeadlineExceeded):
		status = http.StatusGatewayTimeout
	}
	WriteError(w, status, code, err)
}
