package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"kubebench/internal/plugin"
	"kubebench/internal/reason"
	"kubebench/internal/store"
)

// apiError is an error with the HTTP status and §3.3 code it is reported
// with, explained by a reason code (reasons.go). Handlers return it for
// validation and permission failures.
type apiError struct {
	status int
	code   string
	reason reason.Code
	params reason.Params
}

func (e *apiError) Error() string { return e.reason.Text(e.params) }

// Reason implements reason.Reasoned.
func (e *apiError) Reason() (reason.Code, reason.Params) { return e.reason, e.params }

func fail(status int, code string, rc reason.Code, params reason.Params) *apiError {
	return &apiError{status: status, code: code, reason: rc, params: params}
}

func badRequest(rc reason.Code, params reason.Params) *apiError {
	return fail(http.StatusBadRequest, plugin.CodeBadRequest, rc, params)
}

// refuse reports err with status and code under err's reason
// (reason.From): KubeBench's own explanation when it has one, else its
// message as it is.
func refuse(status int, code string, err error) *apiError {
	why := reason.From(err)
	return fail(status, code, why.Code, why.Params)
}

// pluginUnavailable is the refusal of a plugin route whose plugin st
// reports unavailable, explained by the plugin's own reason.
func pluginUnavailable(st plugin.Status) *apiError {
	code, ok := reason.Lookup(st.ReasonCode)
	switch {
	case ok:
		return fail(http.StatusServiceUnavailable, plugin.CodePluginUnavailable, code, st.ReasonParams)
	case st.Reason != "":
		return fail(http.StatusServiceUnavailable, plugin.CodePluginUnavailable, reason.Detail, reason.Params{"detail": st.Reason})
	}
	return fail(http.StatusServiceUnavailable, plugin.CodePluginUnavailable, reasonPluginUnavailable, nil)
}

// required is the error for missing query parameters.
func required(fields ...string) *apiError {
	return badRequest(reasonRequiredParams, reason.Params{"fields": strings.Join(fields, ", ")})
}

// hasCode reports whether err is an *apiError with the given code.
func hasCode(err error, code string) bool {
	var ae *apiError
	return errors.As(err, &ae) && ae.code == code
}

// writeErr reports a KubeBench-side error: an *apiError as itself, the
// store's sentinels as 404/409 and anything else as a 500. Errors from the
// cluster go through plugin.WriteK8sError instead.
func writeErr(w http.ResponseWriter, err error) {
	var ae *apiError
	switch {
	case errors.As(err, &ae):
		plugin.WriteError(w, ae.status, ae.code, ae)
	case errors.Is(err, store.ErrNotFound):
		plugin.WriteError(w, http.StatusNotFound, plugin.CodeNotFound, reason.Wrap(err, reasonNotFound, nil))
	case errors.Is(err, store.ErrLastAdmin):
		plugin.WriteError(w, http.StatusConflict, plugin.CodeConflict, reason.Wrap(err, reasonLastAdmin, nil))
	case errors.Is(err, store.ErrConflict):
		plugin.WriteError(w, http.StatusConflict, plugin.CodeConflict, err)
	default:
		plugin.WriteError(w, http.StatusInternalServerError, plugin.CodeInternal, err)
	}
}

// writeOK is the response of a mutation that returns no data.
func writeOK(w http.ResponseWriter) { plugin.WriteJSON(w, map[string]bool{"ok": true}) }

const maxJSONBody = 1 << 20

// decodeJSON reads a JSON request body of at most 1 MiB into v.
func decodeJSON(w http.ResponseWriter, r *http.Request, v any) error {
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxJSONBody)).Decode(v); err != nil {
		return badRequest(reason.InvalidJSON, reason.Params{"detail": err.Error()})
	}
	return nil
}
