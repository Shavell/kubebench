package notes

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"kubebench/internal/plugin"
	"kubebench/internal/store"
	"kubebench/internal/watch"
)

// filter is what the caller may see: their own personal notes and the
// shared notes of their active cluster.
func filter(c *plugin.Context) store.NoteFilter {
	return store.NoteFilter{UserID: c.User.ID, ClusterID: c.Runtime.Cluster.ID}
}

// queryFilter reads the scope, cluster, object, tag and search filters
// shared by GET /notes and GET /tags.
func queryFilter(r *http.Request, c *plugin.Context) (store.NoteFilter, error) {
	f := filter(c)
	q := r.URL.Query()
	switch scope := q.Get("scope"); scope {
	case "", "all":
	case personal, shared:
		f.Visibility = scope
	default:
		return f, invalidf("scope", "must be all, personal or shared")
	}
	switch clusters := q.Get("clusters"); clusters {
	case "", "current":
	case "all":
		f.AllClusters = true
	default:
		return f, invalidf("clusters", "must be current or all")
	}
	m, err := match(r)
	if err != nil {
		return f, err
	}
	f.Match = m
	if tag := q.Get("tag"); tag != "" {
		tags, err := normalizeTags([]string{tag})
		if err != nil {
			return f, err
		}
		f.Tag = tags[0]
	}
	f.Terms = strings.Fields(q.Get("q"))
	if len(f.Terms) > maxTerms {
		return f, invalidf("q", "at most %d search terms", maxTerms)
	}
	return f, nil
}

func listNotes(w http.ResponseWriter, r *http.Request, c *plugin.Context) {
	f, err := queryFilter(r, c)
	if err != nil {
		writeErr(w, err)
		return
	}
	q := r.URL.Query()
	f.Limit = defaultLimit
	if raw := q.Get("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > maxLimit {
			writeErr(w, invalidf("limit", "must be 1-%d", maxLimit))
			return
		}
		f.Limit = n
	}
	switch q.Get("body") {
	case "", "false":
	case "true":
		f.WithBody = true
	default:
		writeErr(w, invalidf("body", "must be true or false"))
		return
	}
	notes, truncated, err := c.Store.ListNotes(f)
	if err != nil {
		writeErr(w, err)
		return
	}
	out := NoteList{Items: make([]Note, 0, len(notes)), Truncated: truncated}
	for i := range notes {
		out.Items = append(out.Items, noteView(&notes[i]))
	}
	plugin.WriteJSON(w, out)
}

func listTags(w http.ResponseWriter, r *http.Request, c *plugin.Context) {
	f, err := queryFilter(r, c)
	if err != nil {
		writeErr(w, err)
		return
	}
	tags, err := c.Store.NoteTags(f)
	if err != nil {
		writeErr(w, err)
		return
	}
	plugin.WriteJSON(w, map[string]any{"tags": tags})
}

func countNotes(w http.ResponseWriter, r *http.Request, c *plugin.Context) {
	m, err := match(r)
	if err != nil {
		writeErr(w, err)
		return
	}
	f := filter(c)
	f.Match = m
	counts, err := c.Store.NoteCounts(f)
	if err != nil {
		writeErr(w, err)
		return
	}
	items := make([]ObjectCount, 0, len(counts))
	for _, n := range counts {
		o := n.Object
		items = append(items, ObjectCount{
			ObjectRef: ObjectRef{APIVersion: o.APIVersion, Kind: o.Kind, Namespace: o.Namespace, Name: o.Name},
			Personal:  n.Personal, Shared: n.Shared,
		})
	}
	plugin.WriteJSON(w, map[string]any{"items": items})
}

// visible reports whether the caller may see n: a personal note only by
// its owner (admins included: personal means personal), a shared note
// only while its cluster is the active one.
func visible(c *plugin.Context, n *store.Note) bool {
	if n.Visibility == personal {
		return n.OwnerID != nil && *n.OwnerID == c.User.ID
	}
	return n.ClusterID != nil && *n.ClusterID == c.Runtime.Cluster.ID
}

// load returns note {id} if the caller may see it and, when visibility is
// set, it has that visibility. Anything else is ErrNotFound, so that ids
// reveal nothing about notes of others.
func load(r *http.Request, c *plugin.Context, visibility string) (*store.Note, error) {
	id, err := noteID(r)
	if err != nil {
		return nil, err
	}
	n, err := c.Store.GetNote(id)
	if err != nil {
		return nil, err
	}
	if !visible(c, n) || (visibility != "" && n.Visibility != visibility) {
		return nil, store.ErrNotFound
	}
	return n, nil
}

func getNote(w http.ResponseWriter, r *http.Request, c *plugin.Context) {
	n, err := load(r, c, "")
	if err != nil {
		writeErr(w, err)
		return
	}
	plugin.WriteJSON(w, noteView(n))
}

// respond answers with note id as it is stored now.
func respond(w http.ResponseWriter, c *plugin.Context, id int64) {
	n, err := c.Store.GetNote(id)
	if err != nil {
		writeErr(w, err)
		return
	}
	plugin.WriteJSON(w, noteView(n))
}

// audit refines the router's row of a shared-note request (personal
// requests are not audited, so SetAudit does nothing for them). The row
// names the note and its object, never its text.
func audit(c *plugin.Context, id int64, detail map[string]any) {
	if c.SetAudit == nil {
		return
	}
	d := ""
	if len(detail) > 0 {
		b, _ := json.Marshal(detail)
		d = string(b)
	}
	c.SetAudit("note/"+strconv.FormatInt(id, 10), d)
}

// Topic is the live-update topic of notes (§7.13): a change to a shared
// note reaches the streams of its cluster's users, a change to a personal
// note only its owner's streams.
const Topic = "kubebench:notes"

// publish announces a change to n on the live-update streams that may see it.
func publish(c *plugin.Context, n *store.Note) {
	if c.Publish == nil {
		return
	}
	switch {
	case n.Visibility == personal && n.OwnerID != nil:
		c.Publish(Topic, watch.Audience{UserID: *n.OwnerID})
	case n.Visibility == shared && n.ClusterID != nil:
		c.Publish(Topic, watch.Audience{ClusterID: *n.ClusterID})
	}
}

func objectDetail(n *store.Note) map[string]any {
	o := n.Object
	if o == nil {
		return nil
	}
	ref := o.Kind + "/"
	if o.Namespace != "" {
		ref += o.Namespace + "/"
	}
	return map[string]any{"resource": ref + o.Name}
}

type createRequest struct {
	content
	Pinned bool `json:"pinned"`
	// Global places a personal note on no cluster.
	Global   bool       `json:"global"`
	Resource *ObjectRef `json:"resource"`
}

func createNote(visibility string) plugin.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request, c *plugin.Context) {
		var req createRequest
		if err := decode(w, r, &req); err != nil {
			writeErr(w, err)
			return
		}
		e, err := req.edit()
		if err != nil {
			writeErr(w, err)
			return
		}
		n := store.Note{Visibility: visibility, Title: e.Title, Body: e.Body, Tags: e.Tags, Pinned: req.Pinned}
		switch {
		case req.Global && visibility == shared:
			writeErr(w, invalidf("global", "only personal notes can be global"))
			return
		case req.Global && req.Resource != nil:
			writeErr(w, invalidf("global", "a note on an object belongs to its cluster"))
			return
		case !req.Global:
			cluster := c.Runtime.Cluster.ID
			n.ClusterID = &cluster
		}
		if req.Resource != nil {
			if n.Object, err = req.Resource.object(); err != nil {
				writeErr(w, err)
				return
			}
		}
		if visibility == personal {
			owner := c.User.ID
			n.OwnerID = &owner
		}
		id, err := c.Store.CreateNote(n, c.User.ID, maxNotesPerUser)
		if err != nil {
			writeErr(w, err)
			return
		}
		audit(c, id, objectDetail(&n))
		publish(c, &n)
		respond(w, c, id)
	}
}

type updateRequest struct {
	content
	// Version is the version the edit is based on.
	Version int64 `json:"version"`
}

func updateNote(visibility string) plugin.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request, c *plugin.Context) {
		n, err := load(r, c, visibility)
		if err != nil {
			writeErr(w, err)
			return
		}
		audit(c, n.ID, objectDetail(n))
		var req updateRequest
		if err := decode(w, r, &req); err != nil {
			writeErr(w, err)
			return
		}
		if req.Version < 1 {
			writeErr(w, invalidf("version", "must name the version the edit is based on"))
			return
		}
		e, err := req.edit()
		if err != nil {
			writeErr(w, err)
			return
		}
		if err := c.Store.UpdateNote(n.ID, req.Version, e, c.User.ID); err != nil {
			writeErr(w, err)
			return
		}
		publish(c, n)
		respond(w, c, n.ID)
	}
}

func pinNote(visibility string) plugin.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request, c *plugin.Context) {
		n, err := load(r, c, visibility)
		if err != nil {
			writeErr(w, err)
			return
		}
		var req struct {
			Pinned *bool `json:"pinned"`
		}
		if err := decode(w, r, &req); err != nil {
			writeErr(w, err)
			return
		}
		if req.Pinned == nil {
			writeErr(w, invalidf("pinned", "must be true or false"))
			return
		}
		audit(c, n.ID, map[string]any{"pinned": *req.Pinned})
		if err := c.Store.SetNotePinned(n.ID, *req.Pinned); err != nil {
			writeErr(w, err)
			return
		}
		publish(c, n)
		respond(w, c, n.ID)
	}
}

func deleteNote(visibility string) plugin.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request, c *plugin.Context) {
		n, err := load(r, c, visibility)
		if err != nil {
			writeErr(w, err)
			return
		}
		audit(c, n.ID, objectDetail(n))
		// Operators share the editing of shared notes, but removing one is
		// for its author or an administrator.
		if visibility == shared && c.Role != store.RoleAdmin && (n.CreatorID == nil || *n.CreatorID != c.User.ID) {
			plugin.WriteError(w, http.StatusForbidden, plugin.CodeForbidden,
				errors.New("only its author or an administrator can delete a shared note"))
			return
		}
		if err := c.Store.DeleteNote(n.ID); err != nil {
			writeErr(w, err)
			return
		}
		publish(c, n)
		plugin.WriteJSON(w, map[string]any{"deleted": n.ID})
	}
}
