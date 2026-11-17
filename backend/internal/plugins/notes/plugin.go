// Package notes is the built-in "notes" backend plugin
// (docs/design-v0.5.md §7.13): Markdown notes that users keep about a
// cluster, about one object of it, or (personal notes only) globally.
//
// Personal notes belong to their author alone; shared notes are read by
// everyone with a role on their cluster and written by its operators and
// admins. Notes live in KubeBench's database (tables of store migration
// v5), never in the cluster, so a read-only cluster does not block them.
// The routes split by visibility so that the route table states the gate:
// personal routes are AccessRead and act on the caller's own notes only,
// shared routes are AccessContribute and audited by the router.
package notes

import (
	"context"
	"net/http"

	"kubebench/internal/kube"
	"kubebench/internal/plugin"
)

// ID is the plugin id; its routes live under /api/plugins/notes/.
const ID = "notes"

func init() { plugin.Register(&Plugin{}) }

// Plugin implements plugin.Plugin.
type Plugin struct{}

func (*Plugin) Manifest() plugin.Manifest {
	return plugin.Manifest{
		ID:          ID,
		Name:        "Notes",
		Version:     "0.6.0",
		Description: "Personal and shared Markdown notes on clusters and their objects",
		Category:    "tools",
	}
}

func (*Plugin) Routes(r plugin.Router) {
	r.Handle(http.MethodGet, "/notes", plugin.AccessRead, listNotes)
	r.Handle(http.MethodGet, "/notes/{id}", plugin.AccessRead, getNote)
	r.Handle(http.MethodGet, "/tags", plugin.AccessRead, listTags)
	r.Handle(http.MethodGet, "/counts", plugin.AccessRead, countNotes)

	for _, v := range []struct {
		visibility string
		access     plugin.Access
	}{
		{personal, plugin.AccessRead},
		{shared, plugin.AccessContribute},
	} {
		base := "/notes/" + v.visibility
		r.Handle(http.MethodPost, base, v.access, createNote(v.visibility))
		r.Handle(http.MethodPut, base+"/{id}", v.access, updateNote(v.visibility))
		r.Handle(http.MethodPut, base+"/{id}/pin", v.access, pinNote(v.visibility))
		r.Handle(http.MethodDelete, base+"/{id}", v.access, deleteNote(v.visibility))
	}
}

// Detect: notes need nothing from the cluster, so they are always available.
func (*Plugin) Detect(context.Context, *kube.Runtime) plugin.Status {
	return plugin.Status{Available: true}
}
