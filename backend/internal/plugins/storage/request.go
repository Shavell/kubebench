package storage

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"kubebench/internal/plugin"
)

// maxBody bounds a create or resize body; real ones are a few hundred bytes.
const maxBody = 256 << 10

// fieldManager names KubeBench in the objects' managedFields.
const fieldManager = "kubebench"

// invalid is a request the server refuses before it reaches the cluster:
// 400 bad_request naming the field, e.g. "size: must be greater than zero".
type invalid struct {
	field, msg string
}

func (e *invalid) Error() string {
	if e.field == "" {
		return e.msg
	}
	return e.field + ": " + e.msg
}

func invalidf(field, format string, args ...any) error {
	return &invalid{field: field, msg: fmt.Sprintf(format, args...)}
}

// conflictErr is a request the cluster's current state rules out, e.g.
// expanding a claim whose StorageClass does not allow it: 409 conflict.
type conflictErr struct{ msg string }

func (e *conflictErr) Error() string { return e.msg }

func conflictf(format string, args ...any) error {
	return &conflictErr{msg: fmt.Sprintf(format, args...)}
}

// writeErr reports err with the status of its kind; anything that is not a
// validation or state refusal came from the cluster.
func writeErr(w http.ResponseWriter, err error) {
	var bad *invalid
	var conflict *conflictErr
	switch {
	case errors.As(err, &bad):
		plugin.WriteError(w, http.StatusBadRequest, plugin.CodeBadRequest, err)
	case errors.As(err, &conflict):
		plugin.WriteError(w, http.StatusConflict, plugin.CodeConflict, err)
	default:
		plugin.WriteK8sError(w, err)
	}
}

// decodeBody reads a JSON body into v, refusing unknown fields (a typo
// would otherwise be ignored silently) and trailing data.
func decodeBody(w http.ResponseWriter, r *http.Request, v any) error {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBody))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			return invalidf("", "the request body is larger than %d KiB", maxBody>>10)
		}
		return invalidf("", "invalid JSON body: %v", err)
	}
	if _, err := dec.Token(); err != io.EOF {
		return invalidf("", "invalid JSON body: unexpected data after the object")
	}
	return nil
}

// createOptions is a server-side dry run whenever the request asks for one
// (plugin.IsDryRun), the condition under which the router let it through
// with read access only.
func createOptions(r *http.Request) metav1.CreateOptions {
	opts := metav1.CreateOptions{FieldManager: fieldManager, FieldValidation: "Strict"}
	if plugin.IsDryRun(r) {
		opts.DryRun = []string{metav1.DryRunAll}
	}
	return opts
}

// CreateResult answers a create: the object the server stored (or, on a
// dry run, would store) and advice that did not stop it.
type CreateResult struct {
	APIVersion string   `json:"apiVersion"`
	Kind       string   `json:"kind"`
	Namespace  string   `json:"namespace,omitempty"`
	Name       string   `json:"name"`
	DryRun     bool     `json:"dryRun"`
	Warnings   []string `json:"warnings,omitempty"`
}

// auditCreate adds the row that names a created object, which the router's
// own row cannot: its target comes from the URL, and the object is named in
// the body. Dry runs are not audited (§3.4).
func auditCreate(r *http.Request, c *plugin.Context, kind, target string, detail map[string]any) {
	if plugin.IsDryRun(r) || c.Audit == nil {
		return
	}
	c.Audit("storage."+kind+".create", kind+"/"+target, auditJSON(detail))
}

func auditJSON(v map[string]any) string {
	if len(v) == 0 {
		return ""
	}
	b, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return string(b)
}
