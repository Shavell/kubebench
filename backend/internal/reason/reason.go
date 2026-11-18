// Package reason gives KubeBench's own explanations (error messages,
// unavailable reasons, refused operations) stable codes with parameters,
// so the UI can show them in the user's language (docs/design-v0.5.md
// §3.3). Every code is defined once, with its English text, by the package
// that emits it; the English text is what the API sends as the fallback
// message and must equal the frontend's English string for the code
// (src/core/reasons/locales/en.json, checked by a test).
//
// Texts are templates: {{name}} is replaced with the parameter name, the
// interpolation syntax of i18next. Messages that come from elsewhere (the
// Kubernetes API, a Helm chart, the network) are not translated; they are
// passed as a parameter, usually "detail".
package reason

import (
	"errors"
	"fmt"
	"maps"
	"regexp"
	"strings"
	"sync"
)

// Code names one explanation, e.g. "cluster.read_only".
type Code struct{ id string }

// Params fill a code's placeholders. Values are strings or numbers.
type Params map[string]any

var (
	mu    sync.RWMutex
	texts = map[string]string{}

	// A code is a lower-case group and name: "argocd.not_installed".
	idPattern          = regexp.MustCompile(`^[a-z][a-z0-9]*(\.[a-z][a-z0-9_]*)+$`)
	placeholderPattern = regexp.MustCompile(`\{\{(\w+)\}\}`)
)

// Define registers the code id with its English text. It is called from
// package-level variable declarations; a malformed or duplicate id is a
// programming error and panics.
func Define(id, english string) Code {
	if !idPattern.MatchString(id) {
		panic(fmt.Sprintf("reason: malformed code %q", id))
	}
	if strings.TrimSpace(english) == "" {
		panic(fmt.Sprintf("reason: code %q has no text", id))
	}
	mu.Lock()
	defer mu.Unlock()
	if _, dup := texts[id]; dup {
		panic(fmt.Sprintf("reason: code %q is defined twice", id))
	}
	texts[id] = english
	return Code{id: id}
}

// Lookup returns the defined code id, e.g. one received as a string.
func Lookup(id string) (Code, bool) {
	mu.RLock()
	defer mu.RUnlock()
	_, ok := texts[id]
	if !ok {
		return Code{}, false
	}
	return Code{id: id}, true
}

// String is the code as sent in the API ("" for the zero Code).
func (c Code) String() string { return c.id }

// IsZero reports whether c is the zero Code (no reason).
func (c Code) IsZero() bool { return c.id == "" }

// Text is the English text of c with params filled in.
func (c Code) Text(params Params) string {
	mu.RLock()
	template := texts[c.id]
	mu.RUnlock()
	return Render(template, params)
}

// Render fills the {{name}} placeholders of template from params; a
// placeholder without a parameter is left as it is.
func Render(template string, params Params) string {
	return placeholderPattern.ReplaceAllStringFunc(template, func(m string) string {
		value, ok := params[m[2:len(m)-2]]
		if !ok {
			return m
		}
		return fmt.Sprint(value)
	})
}

// Placeholders are the parameter names template uses, in order.
func Placeholders(template string) []string {
	var names []string
	for _, m := range placeholderPattern.FindAllStringSubmatch(template, -1) {
		names = append(names, m[1])
	}
	return names
}

// Catalog returns every defined code with its English text.
func Catalog() map[string]string {
	mu.RLock()
	defer mu.RUnlock()
	return maps.Clone(texts)
}

// Error is an error explained by a reason code. Its message is the
// English text; Err, if set, is the underlying cause (for errors.Is).
type Error struct {
	Code   Code
	Params Params
	Err    error
}

// New returns the error of code with params.
func New(code Code, params Params) *Error { return &Error{Code: code, Params: params} }

// Wrap returns the error of code with params, caused by err.
func Wrap(err error, code Code, params Params) *Error {
	return &Error{Code: code, Params: params, Err: err}
}

func (e *Error) Error() string { return e.Code.Text(e.Params) }

func (e *Error) Unwrap() error { return e.Err }

// Reason implements Reasoned.
func (e *Error) Reason() (Code, Params) { return e.Code, e.Params }

// Ref is a reason as JSON, for lists of explanations next to their
// English texts: {"reason": "<code>", "params": {...}}.
type Ref struct {
	Code   string `json:"reason"`
	Params Params `json:"params,omitempty"`
}

// Ref returns e's code and parameters as a Ref.
func (e *Error) Ref() Ref { return Ref{Code: e.Code.String(), Params: e.Params} }

// Split returns the English texts of errs and, index by index, their
// reasons: a list of advice as a response sends it ("warnings" with
// "warningReasons"). Both are nil for no errs.
func Split(errs []*Error) (texts []string, refs []Ref) {
	for _, e := range errs {
		texts = append(texts, e.Error())
		refs = append(refs, e.Ref())
	}
	return texts, refs
}

// Detail is the reason of an error that has none of its own, such as an
// answer of the Kubernetes API: its message is shown as it is.
var Detail = Define("error.detail", "{{detail}}")

// From returns err's reason as an *Error: the reason of the first error in
// its chain that has one, else Detail with err's message.
func From(err error) *Error {
	if code, params, ok := Of(err); ok {
		return &Error{Code: code, Params: params, Err: err}
	}
	return Wrap(err, Detail, Params{"detail": err.Error()})
}

// Reasoned is an error that carries a reason code; the API reports its
// code and parameters next to the message (§3.3).
type Reasoned interface {
	error
	Reason() (Code, Params)
}

// Of returns the reason of the first error in err's chain that has one.
func Of(err error) (Code, Params, bool) {
	var r Reasoned
	if err == nil || !errors.As(err, &r) {
		return Code{}, nil, false
	}
	code, params := r.Reason()
	return code, params, !code.IsZero()
}
