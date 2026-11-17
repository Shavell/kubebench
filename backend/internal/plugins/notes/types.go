package notes

import (
	"time"

	"kubebench/internal/store"
)

// Visibilities as they appear in paths and JSON.
const (
	personal = store.NotePersonal
	shared   = store.NoteShared
)

// ObjectRef names the Kubernetes object a note is about
// (src/plugins/notes/data/types.ts NoteObjectRef).
type ObjectRef struct {
	APIVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`
	Namespace  string `json:"namespace"`
	Name       string `json:"name"`
}

// Note is the JSON shape of a note (§7.13).
type Note struct {
	ID          int64      `json:"id"`
	Visibility  string     `json:"visibility"`
	ClusterID   *int64     `json:"clusterId"`
	ClusterName string     `json:"clusterName"`
	Resource    *ObjectRef `json:"resource"`
	Title       string     `json:"title"`
	Body        string     `json:"body"`
	Tags        []string   `json:"tags"`
	Pinned      bool       `json:"pinned"`
	Version     int64      `json:"version"`
	// CreatedBy and UpdatedBy are "" once the user was deleted.
	CreatedBy string    `json:"createdBy"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedBy string    `json:"updatedBy"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// NoteList answers GET /notes.
type NoteList struct {
	Items []Note `json:"items"`
	// Truncated reports that more notes match than the limit returned.
	Truncated bool `json:"truncated"`
}

// ObjectCount is the number of notes on one object, by visibility.
type ObjectCount struct {
	ObjectRef
	Personal int `json:"personal"`
	Shared   int `json:"shared"`
}

func noteView(n *store.Note) Note {
	v := Note{
		ID: n.ID, Visibility: n.Visibility, ClusterID: n.ClusterID, ClusterName: n.ClusterName,
		Title: n.Title, Body: n.Body, Tags: n.Tags, Pinned: n.Pinned, Version: n.Version,
		CreatedBy: n.CreatedBy, CreatedAt: n.CreatedAt.UTC(), UpdatedBy: n.UpdatedBy, UpdatedAt: n.UpdatedAt.UTC(),
	}
	if o := n.Object; o != nil {
		v.Resource = &ObjectRef{APIVersion: o.APIVersion, Kind: o.Kind, Namespace: o.Namespace, Name: o.Name}
	}
	return v
}
