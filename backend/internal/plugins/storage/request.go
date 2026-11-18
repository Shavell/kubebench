package storage

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"kubebench/internal/plugin"
	"kubebench/internal/reason"
)

// maxBody bounds a create or resize body; real ones are a few hundred bytes.
const maxBody = 256 << 10

// fieldManager names KubeBench in the objects' managedFields.
const fieldManager = "kubebench"

// invalid is a request the server refuses before it reaches the cluster:
// 400 bad_request naming the field, e.g. "size: must be greater than zero".
type invalid struct{ err *reason.Error }

func (e *invalid) Error() string { return e.err.Error() }

// Reason implements reason.Reasoned.
func (e *invalid) Reason() (reason.Code, reason.Params) { return e.err.Reason() }

// invalidField is the refusal of field ("" for the body as a whole),
// explained by code with params; the field is the {{field}} parameter.
func invalidField(field string, code reason.Code, params reason.Params) error {
	if params == nil {
		params = reason.Params{}
	}
	if field != "" {
		params["field"] = field
	}
	return &invalid{reason.New(code, params)}
}

// conflictErr is a request the cluster's current state rules out, e.g.
// expanding a claim whose StorageClass does not allow it: 409 conflict.
type conflictErr struct{ err *reason.Error }

func (e *conflictErr) Error() string { return e.err.Error() }

// Reason implements reason.Reasoned.
func (e *conflictErr) Reason() (reason.Code, reason.Params) { return e.err.Reason() }

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
			return invalidField("", reason.BodyTooLarge, reason.Params{"size": fmt.Sprintf("%d KiB", maxBody>>10)})
		}
		return invalidField("", reason.InvalidJSON, reason.Params{"detail": err.Error()})
	}
	if _, err := dec.Token(); err != io.EOF {
		return invalidField("", reason.TrailingData, nil)
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

// auditCreate names the created object in the router's row for the request:
// the URL only gives its kind, the body names it. Dry runs are not audited
// (§3.4), so SetAudit does nothing for them.
func auditCreate(c *plugin.Context, kind, target string, detail map[string]any) {
	if c.SetAudit == nil {
		return
	}
	c.SetAudit(kind+"/"+target, auditJSON(detail))
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
