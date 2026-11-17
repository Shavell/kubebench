package notes

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"kubebench/internal/plugin"
	"kubebench/internal/store"
)

// Limits of §7.13.
const (
	maxTitle = 200      // characters
	maxBody  = 64 << 10 // bytes of Markdown
	maxTags  = 16
	// maxNotesPerUser bounds the notes one user authors (personal and shared).
	maxNotesPerUser = 1000
	// maxRequest bounds a request body: the note body plus its JSON framing.
	maxRequest = maxBody*2 + 16<<10
	// defaultLimit and maxLimit bound GET /notes.
	defaultLimit = 200
	maxLimit     = 500
	maxTerms     = 8
)

var (
	tagPattern       = regexp.MustCompile(`^[\p{L}\p{N}][\p{L}\p{N}._-]{0,39}$`)
	kindPattern      = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9]{0,62}$`)
	namespacePattern = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]{0,61}[a-z0-9])?$`)
)

// invalidErr is a request refused as 400 bad_request, naming the field.
type invalidErr struct{ field, msg string }

func (e *invalidErr) Error() string { return e.field + ": " + e.msg }

func invalidf(field, format string, args ...any) error {
	return &invalidErr{field: field, msg: fmt.Sprintf(format, args...)}
}

// writeErr reports err with the status of its kind.
func writeErr(w http.ResponseWriter, err error) {
	var bad *invalidErr
	switch {
	case errors.As(err, &bad):
		plugin.WriteError(w, http.StatusBadRequest, plugin.CodeBadRequest, err)
	case errors.Is(err, store.ErrNotFound):
		plugin.WriteError(w, http.StatusNotFound, plugin.CodeNotFound, errors.New("note not found"))
	case errors.Is(err, store.ErrNoteVersion):
		plugin.WriteError(w, http.StatusConflict, plugin.CodeConflict,
			errors.New("the note was changed since you opened it; reload it or overwrite the other change"))
	case errors.Is(err, store.ErrNoteLimit):
		plugin.WriteError(w, http.StatusConflict, plugin.CodeConflict,
			fmt.Errorf("you have reached the limit of %d notes; delete notes you no longer need", maxNotesPerUser))
	default:
		plugin.WriteError(w, http.StatusInternalServerError, plugin.CodeInternal, err)
	}
}

// decode reads a JSON body of at most maxRequest bytes into v.
func decode(w http.ResponseWriter, r *http.Request, v any) error {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxRequest))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			return invalidf("body", "the request is larger than %d bytes", maxRequest)
		}
		return invalidf("body", "invalid JSON: %v", err)
	}
	if dec.Decode(&struct{}{}) != io.EOF {
		return invalidf("body", "unexpected data after the JSON object")
	}
	return nil
}

// content is the editable part of create and update requests.
type content struct {
	Title string   `json:"title"`
	Body  string   `json:"body"`
	Tags  []string `json:"tags"`
}

// edit validates c and returns it normalized: the title trimmed, the tags
// lower-cased, deduplicated and sorted.
func (c content) edit() (store.NoteEdit, error) {
	title := strings.TrimSpace(c.Title)
	switch {
	case title == "":
		return store.NoteEdit{}, invalidf("title", "must not be empty")
	case utf8.RuneCountInString(title) > maxTitle:
		return store.NoteEdit{}, invalidf("title", "must be at most %d characters", maxTitle)
	case strings.IndexFunc(title, unicode.IsControl) >= 0:
		return store.NoteEdit{}, invalidf("title", "must be a single line of text")
	}
	if len(c.Body) > maxBody {
		return store.NoteEdit{}, invalidf("body", "must be at most %d KiB", maxBody>>10)
	}
	if !utf8.ValidString(c.Body) {
		return store.NoteEdit{}, invalidf("body", "must be UTF-8 text")
	}
	tags, err := normalizeTags(c.Tags)
	if err != nil {
		return store.NoteEdit{}, err
	}
	return store.NoteEdit{Title: title, Body: c.Body, Tags: tags}, nil
}

func normalizeTags(in []string) ([]string, error) {
	tags := []string{}
	for _, raw := range in {
		tag := strings.ToLower(strings.TrimSpace(raw))
		if !tagPattern.MatchString(tag) {
			return nil, invalidf("tags", "%q is not a tag: 1-40 letters, digits, '.', '_' or '-', starting with a letter or digit", raw)
		}
		if !slices.Contains(tags, tag) {
			tags = append(tags, tag)
		}
	}
	if len(tags) > maxTags {
		return nil, invalidf("tags", "at most %d tags", maxTags)
	}
	slices.Sort(tags)
	return tags, nil
}

// object validates a reference to a Kubernetes object.
func (o ObjectRef) object() (*store.NoteObject, error) {
	if !kindPattern.MatchString(o.Kind) {
		return nil, invalidf("resource.kind", "%q is not a Kubernetes kind", o.Kind)
	}
	if err := checkName("resource.name", o.Name); err != nil {
		return nil, err
	}
	if o.Namespace != "" && !namespacePattern.MatchString(o.Namespace) {
		return nil, invalidf("resource.namespace", "%q is not a namespace name", o.Namespace)
	}
	group, err := groupOf(o.APIVersion)
	if err != nil {
		return nil, err
	}
	return &store.NoteObject{APIVersion: o.APIVersion, Group: group, Kind: o.Kind, Namespace: o.Namespace, Name: o.Name}, nil
}

// checkName accepts any object name Kubernetes accepts (RBAC names may
// hold ':'), refusing what no object can be called.
func checkName(field, name string) error {
	if name == "" || len(name) > 253 || strings.ContainsAny(name, "/%") ||
		strings.IndexFunc(name, func(r rune) bool { return unicode.IsControl(r) || unicode.IsSpace(r) }) >= 0 {
		return invalidf(field, "%q is not an object name", name)
	}
	return nil
}

// groupOf is the API group of apiVersion ("apps/v1" → "apps", "v1" → "").
func groupOf(apiVersion string) (string, error) {
	if apiVersion == "" {
		return "", nil
	}
	if len(apiVersion) > 253 || strings.Count(apiVersion, "/") > 1 ||
		strings.IndexFunc(apiVersion, func(r rune) bool { return unicode.IsControl(r) || unicode.IsSpace(r) }) >= 0 {
		return "", invalidf("resource.apiVersion", "%q is not an API version", apiVersion)
	}
	group, _, found := strings.Cut(apiVersion, "/")
	if !found {
		return "", nil
	}
	return group, nil
}

// noteID parses the {id} path value.
func noteID(r *http.Request) (int64, error) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		return 0, invalidf("id", "%q is not a note id", r.PathValue("id"))
	}
	return id, nil
}

// match reads the object filter of GET /notes and GET /counts: each of
// kind, namespace and name filters when given (a name without a namespace
// names a cluster-scoped object), and apiVersion filters by its group.
func match(r *http.Request) (*store.NoteMatch, error) {
	q := r.URL.Query()
	m := &store.NoteMatch{}
	set := false
	if kind := q.Get("kind"); kind != "" {
		if !kindPattern.MatchString(kind) {
			return nil, invalidf("kind", "%q is not a Kubernetes kind", kind)
		}
		m.Kind, set = &kind, true
	}
	if name := q.Get("name"); name != "" {
		if err := checkName("name", name); err != nil {
			return nil, err
		}
		m.Name, set = &name, true
	}
	// An object named without a namespace is cluster-scoped.
	if namespace := q.Get("namespace"); namespace != "" || m.Name != nil {
		if namespace != "" && !namespacePattern.MatchString(namespace) {
			return nil, invalidf("namespace", "%q is not a namespace name", namespace)
		}
		m.Namespace, set = &namespace, true
	}
	if apiVersion := q.Get("apiVersion"); apiVersion != "" {
		group, err := groupOf(apiVersion)
		if err != nil {
			return nil, err
		}
		m.Group, set = &group, true
	}
	if !set {
		return nil, nil
	}
	return m, nil
}
