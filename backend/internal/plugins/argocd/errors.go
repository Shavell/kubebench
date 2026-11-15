package argocd

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"kubebench/internal/plugin"
)

// requestError is a failure the plugin decides itself, reported with its
// own status and §3.3 code rather than as a Kubernetes error.
type requestError struct {
	status int
	code   string
	msg    string
}

func (e *requestError) Error() string { return e.msg }

func badRequest(format string, args ...any) error {
	return &requestError{http.StatusBadRequest, plugin.CodeBadRequest, fmt.Sprintf(format, args...)}
}

func conflict(format string, args ...any) error {
	return &requestError{http.StatusConflict, plugin.CodeConflict, fmt.Sprintf(format, args...)}
}

func notFound(format string, args ...any) error {
	return &requestError{http.StatusNotFound, plugin.CodeNotFound, fmt.Sprintf(format, args...)}
}

// writeError reports a requestError with its status, anything else as a
// Kubernetes error.
func writeError(w http.ResponseWriter, err error) {
	var re *requestError
	if errors.As(err, &re) {
		plugin.WriteError(w, re.status, re.code, re)
		return
	}
	plugin.WriteK8sError(w, err)
}

// maxBody bounds request bodies; the largest, a selective sync, names a
// few hundred resources at most.
const maxBody = 1 << 20

// decodeBody reads an optional JSON body into v: an empty body leaves v at
// its zero value, unknown fields are refused.
func decodeBody(r *http.Request, v any) error {
	dec := json.NewDecoder(http.MaxBytesReader(nil, r.Body, maxBody))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil && !errors.Is(err, io.EOF) {
		return badRequest("invalid request body: %v", err)
	}
	if dec.More() {
		return badRequest("invalid request body: more than one JSON value")
	}
	return nil
}

// boolParam reads an optional true/false query parameter.
func boolParam(r *http.Request, name string, fallback bool) (bool, error) {
	switch r.URL.Query().Get(name) {
	case "":
		return fallback, nil
	case "true":
		return true, nil
	case "false":
		return false, nil
	}
	return false, badRequest("%s must be true or false", name)
}
