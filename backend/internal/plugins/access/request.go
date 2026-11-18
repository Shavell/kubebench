package access

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"k8s.io/apimachinery/pkg/util/validation"

	"kubebench/internal/plugin"
	"kubebench/internal/reason"
)

// invalid is a request refused before anything reaches the cluster:
// 400 bad_request naming the field.
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

// writeErr reports a validation error as 400 and anything else as the
// cluster's error.
func writeErr(w http.ResponseWriter, err error) {
	var bad *invalid
	if errors.As(err, &bad) {
		plugin.WriteError(w, http.StatusBadRequest, plugin.CodeBadRequest, err)
		return
	}
	plugin.WriteK8sError(w, err)
}

// maxBody bounds a review body; real ones are a few hundred bytes.
const maxBody = 16 << 10

// decodeBody reads a JSON body into v, refusing unknown fields and
// trailing data.
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

// requestFromQuery reads the access question of GET /who-can.
func requestFromQuery(q url.Values) (Request, error) {
	r := Request{
		Verb:        strings.TrimSpace(q.Get("verb")),
		Group:       strings.TrimSpace(q.Get("group")),
		Resource:    strings.TrimSpace(q.Get("resource")),
		Subresource: strings.TrimSpace(q.Get("subresource")),
		Namespace:   strings.TrimSpace(q.Get("namespace")),
		Name:        strings.TrimSpace(q.Get("name")),
		Path:        strings.TrimSpace(q.Get("path")),
	}
	return r, validateRequest(r)
}

// validateRequest checks that r asks one well-formed question. Verbs and
// resources are matched literally, so "*" asks who has a wildcard grant.
func validateRequest(r Request) error {
	if r.Verb == "" {
		return invalidField("verb", reason.FieldRequired, nil)
	}
	if !r.IsResource() {
		if r.Resource != "" || r.Subresource != "" || r.Group != "" || r.Namespace != "" || r.Name != "" {
			return invalidField("path", reasonNonResourceFields, nil)
		}
		if !strings.HasPrefix(r.Path, "/") {
			return invalidField("path", reasonPathSlash, nil)
		}
		return checkLength("path", r.Path)
	}
	if r.Resource == "" {
		return invalidField("resource", reasonResourceRequired, nil)
	}
	if r.Namespace != "" {
		if errs := validation.IsDNS1123Label(r.Namespace); len(errs) > 0 {
			return invalidField("namespace", reason.FieldInvalid, reason.Params{"detail": strings.Join(errs, "; ")})
		}
	}
	for _, f := range []struct{ field, value string }{
		{"verb", r.Verb}, {"group", r.Group}, {"resource", r.Resource}, {"subresource", r.Subresource},
	} {
		if err := checkLength(f.field, f.value); err != nil {
			return err
		}
		if strings.ContainsAny(f.value, "/ ") {
			return invalidField(f.field, reasonNoSpaces, nil)
		}
	}
	return checkLength("name", r.Name)
}

func checkLength(field, v string) error {
	if len(v) > maxNameLength {
		return invalidField(field, reason.FieldTooLong, reason.Params{"max": maxNameLength})
	}
	return nil
}
