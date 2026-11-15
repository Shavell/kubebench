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
)

// invalid is a request refused before anything reaches the cluster:
// 400 bad_request naming the field.
type invalid struct{ field, msg string }

func (e *invalid) Error() string {
	if e.field == "" {
		return e.msg
	}
	return e.field + ": " + e.msg
}

func invalidf(field, format string, args ...any) error {
	return &invalid{field: field, msg: fmt.Sprintf(format, args...)}
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
			return invalidf("", "the request body is larger than %d KiB", maxBody>>10)
		}
		return invalidf("", "invalid JSON body: %v", err)
	}
	if _, err := dec.Token(); err != io.EOF {
		return invalidf("", "invalid JSON body: unexpected data after the object")
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
		return invalidf("verb", "is required")
	}
	if !r.IsResource() {
		if r.Resource != "" || r.Subresource != "" || r.Group != "" || r.Namespace != "" || r.Name != "" {
			return invalidf("path", "a non-resource URL takes no group, resource, namespace or name")
		}
		if !strings.HasPrefix(r.Path, "/") {
			return invalidf("path", "must start with /")
		}
		return checkLength("path", r.Path)
	}
	if r.Resource == "" {
		return invalidf("resource", "is required (or path, for a non-resource URL)")
	}
	if r.Namespace != "" {
		if errs := validation.IsDNS1123Label(r.Namespace); len(errs) > 0 {
			return invalidf("namespace", "%s", strings.Join(errs, "; "))
		}
	}
	for _, f := range []struct{ field, value string }{
		{"verb", r.Verb}, {"group", r.Group}, {"resource", r.Resource}, {"subresource", r.Subresource},
	} {
		if err := checkLength(f.field, f.value); err != nil {
			return err
		}
		if strings.ContainsAny(f.value, "/ ") {
			return invalidf(f.field, "must not contain spaces or slashes")
		}
	}
	return checkLength("name", r.Name)
}

func checkLength(field, v string) error {
	if len(v) > maxNameLength {
		return invalidf(field, "must be at most %d characters", maxNameLength)
	}
	return nil
}
