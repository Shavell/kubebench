package argocd

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"kubebench/internal/plugin"
	"kubebench/internal/reason"
)

// requestError is a failure the plugin decides itself, reported with its
// own status and §3.3 code rather than as a Kubernetes error.
type requestError struct {
	status int
	code   string
	reason reason.Code
	params reason.Params
}

func (e *requestError) Error() string { return e.reason.Text(e.params) }

// Reason implements reason.Reasoned.
func (e *requestError) Reason() (reason.Code, reason.Params) { return e.reason, e.params }

func badRequest(rc reason.Code, params reason.Params) error {
	return &requestError{http.StatusBadRequest, plugin.CodeBadRequest, rc, params}
}

func conflict(rc reason.Code, params reason.Params) error {
	return &requestError{http.StatusConflict, plugin.CodeConflict, rc, params}
}

func notFound(rc reason.Code, params reason.Params) error {
	return &requestError{http.StatusNotFound, plugin.CodeNotFound, rc, params}
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
		return badRequest(reason.InvalidJSON, reason.Params{"detail": err.Error()})
	}
	if dec.More() {
		return badRequest(reasonSeveralValues, nil)
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
	return false, badRequest(reason.FieldBoolean, reason.Params{"field": name})
}
