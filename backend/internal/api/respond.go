package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"kubebench/internal/plugin"
	"kubebench/internal/store"
)

// apiError is an error with the HTTP status and §3.3 code it is reported
// with. Handlers return it for validation and permission failures.
type apiError struct {
	status int
	code   string
	msg    string
}

func (e *apiError) Error() string { return e.msg }

func errorf(status int, code, format string, args ...any) *apiError {
	return &apiError{status: status, code: code, msg: fmt.Sprintf(format, args...)}
}

func badRequest(format string, args ...any) *apiError {
	return errorf(http.StatusBadRequest, plugin.CodeBadRequest, format, args...)
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
		plugin.WriteError(w, http.StatusNotFound, plugin.CodeNotFound, err)
	case errors.Is(err, store.ErrConflict), errors.Is(err, store.ErrLastAdmin):
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
		return badRequest("invalid JSON body: %v", err)
	}
	return nil
}
