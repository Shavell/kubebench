package store

import (
	"database/sql"
	"errors"
	"slices"
	"strings"
	"time"
)

// Note visibilities (docs/design-v0.5.md §7.13).
const (
	NotePersonal = "personal"
	NoteShared   = "shared"
)

var (
	// ErrNoteVersion reports an update based on an outdated version.
	ErrNoteVersion = errors.New("the note was changed since it was read")
	// ErrNoteLimit reports that the author already has the maximum number of notes.
	ErrNoteLimit = errors.New("note limit reached")
)

// NoteObject is the Kubernetes object a note is about. Group is derived
// from APIVersion by the caller; objects are matched by group, kind,
// namespace and name, so every version of an API names the same object.
type NoteObject struct {
	APIVersion string
	Group      string
	Kind       string
	Namespace  string // "" for cluster-scoped objects
	Name       string
}

// Note is one notes row with its tags and the names of its authors.
type Note struct {
	ID         int64
	Visibility string // NotePersonal | NoteShared
	// OwnerID is the user a personal note belongs to; nil for shared notes.
	OwnerID *int64
	// ClusterID is nil for global (personal) notes.
	ClusterID   *int64
	ClusterName string
	// Object is nil for notes about the cluster as a whole (or global notes).
	Object *NoteObject
	Title  string
	Body   string
	Tags   []string
	Pinned bool
	// Version counts edits, starting at 1, for optimistic concurrency.
	Version int64
	// CreatorID is the author's user id; nil once the user was deleted.
	CreatorID *int64
	// CreatedBy and UpdatedBy are usernames; "" once the user was deleted.
	CreatedBy string
	CreatedAt time.Time
	UpdatedBy string
	UpdatedAt time.Time
}

// NoteMatch narrows notes to objects; nil fields match anything, so a
// match of Kind and Name alone finds the object in any API group.
type NoteMatch struct {
	Group, Kind, Namespace, Name *string
}

// NoteFilter selects the notes one user may see: their own personal notes
// and the shared notes of their active cluster.
type NoteFilter struct {
	UserID    int64
	ClusterID int64
	// Visibility limits the result to one visibility; "" lists both.
	Visibility string
	// AllClusters also lists the user's personal notes on other clusters
	// (global notes and those of ClusterID are always listed).
	AllClusters bool
	// Match limits the result to notes on objects of ClusterID.
	Match *NoteMatch
	// Tag lists only notes carrying this tag.
	Tag string
	// Terms must each occur in the title or the body (case-insensitive
	// for ASCII letters).
	Terms []string
	// Limit caps the number of notes (ListNotes); 0 means no limit.
	Limit int
	// WithBody loads bodies; otherwise Body is empty.
	WithBody bool
}

// TagCount is a tag and the number of notes carrying it.
type TagCount struct {
	Tag   string `json:"tag"`
	Count int    `json:"count"`
}

// ObjectNoteCount is the number of notes on one object.
type ObjectNoteCount struct {
	Object   NoteObject
	Personal int
	Shared   int
}

// where builds the condition of f over table alias n.
func (f NoteFilter) where() (string, []any) {
	personal := `(n.visibility = 'personal' AND n.owner_id = ?`
	args := []any{f.UserID}
	if !f.AllClusters || f.Match != nil {
		personal += ` AND (n.cluster_id IS NULL OR n.cluster_id = ?)`
		args = append(args, f.ClusterID)
	}
	personal += `)`
	shared := `(n.visibility = 'shared' AND n.cluster_id = ?)`
	var conds []string
	switch f.Visibility {
	case NotePersonal:
		conds = append(conds, personal)
	case NoteShared:
		conds, args = append(conds, shared), []any{f.ClusterID}
	default:
		conds, args = append(conds, "("+personal+" OR "+shared+")"), append(args, f.ClusterID)
	}
	if m := f.Match; m != nil {
		conds, args = append(conds, `n.cluster_id = ?`), append(args, f.ClusterID)
		conds = append(conds, `n.obj_kind <> ''`)
		for _, field := range []struct {
			column string
			value  *string
		}{{"n.obj_group", m.Group}, {"n.obj_kind", m.Kind}, {"n.obj_namespace", m.Namespace}, {"n.obj_name", m.Name}} {
			if field.value != nil {
				conds, args = append(conds, field.column+` = ?`), append(args, *field.value)
			}
		}
	}
	if f.Tag != "" {
		conds = append(conds, `EXISTS (SELECT 1 FROM note_tags t WHERE t.note_id = n.id AND t.tag = ?)`)
		args = append(args, f.Tag)
	}
	for _, term := range f.Terms {
		like := "%" + escapeLike(term) + "%"
		conds = append(conds, `(n.title LIKE ? ESCAPE '\' OR n.body LIKE ? ESCAPE '\')`)
		args = append(args, like, like)
	}
	return strings.Join(conds, " AND "), args
}

const noteSelect = `SELECT n.id, n.visibility, n.owner_id, n.cluster_id, COALESCE(c.name, ''),
  n.obj_api_version, n.obj_group, n.obj_kind, n.obj_namespace, n.obj_name,
  n.title, %BODY%, COALESCE((SELECT group_concat(tag, ',') FROM note_tags t WHERE t.note_id = n.id), ''),
  n.pinned, n.version, n.created_by, COALESCE(cu.username, ''), n.created_at, COALESCE(uu.username, ''), n.updated_at
FROM notes n
LEFT JOIN clusters c ON c.id = n.cluster_id
LEFT JOIN users cu ON cu.id = n.created_by
LEFT JOIN users uu ON uu.id = n.updated_by`

func noteQuery(withBody bool) string {
	body := `''`
	if withBody {
		body = `n.body`
	}
	return strings.Replace(noteSelect, "%BODY%", body, 1)
}

func scanNote(row interface{ Scan(...any) error }) (*Note, error) {
	var n Note
	var obj NoteObject
	var tags string
	err := row.Scan(&n.ID, &n.Visibility, &n.OwnerID, &n.ClusterID, &n.ClusterName,
		&obj.APIVersion, &obj.Group, &obj.Kind, &obj.Namespace, &obj.Name,
		&n.Title, &n.Body, &tags, &n.Pinned, &n.Version, &n.CreatorID, &n.CreatedBy, &n.CreatedAt, &n.UpdatedBy, &n.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if obj.Kind != "" {
		n.Object = &obj
	}
	n.Tags = []string{}
	if tags != "" {
		n.Tags = strings.Split(tags, ",")
		slices.Sort(n.Tags)
	}
	return &n, nil
}

// ListNotes returns the notes f selects, pinned first, then the most
// recently updated; truncated reports that f.Limit cut the list.
func (s *Store) ListNotes(f NoteFilter) (notes []Note, truncated bool, err error) {
	where, args := f.where()
	query := noteQuery(f.WithBody) + ` WHERE ` + where + ` ORDER BY n.pinned DESC, n.updated_at DESC, n.id DESC`
	if f.Limit > 0 {
		query += ` LIMIT ?`
		args = append(args, f.Limit+1)
	}
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()
	notes = []Note{}
	for rows.Next() {
		n, err := scanNote(rows)
		if err != nil {
			return nil, false, err
		}
		notes = append(notes, *n)
	}
	if err := rows.Err(); err != nil {
		return nil, false, err
	}
	if f.Limit > 0 && len(notes) > f.Limit {
		return notes[:f.Limit], true, nil
	}
	return notes, false, nil
}

// GetNote returns a note with its body, whoever may see it: callers
// decide about access.
func (s *Store) GetNote(id int64) (*Note, error) {
	return scanNote(s.db.QueryRow(noteQuery(true)+` WHERE n.id = ?`, id))
}

// CreateNote stores n, authored by userID, and returns its id. Version,
// authors and times are set here. It fails with ErrNoteLimit when the
// user already authored limit notes.
func (s *Store) CreateNote(n Note, userID int64, limit int) (int64, error) {
	var id int64
	err := s.withTx(func(tx *sql.Tx) error {
		var count int
		if err := tx.QueryRow(`SELECT count(*) FROM notes WHERE created_by = ?`, userID).Scan(&count); err != nil {
			return err
		}
		if count >= limit {
			return ErrNoteLimit
		}
		obj := NoteObject{}
		if n.Object != nil {
			obj = *n.Object
		}
		t := now()
		res, err := tx.Exec(`INSERT INTO notes (visibility, owner_id, cluster_id,
  obj_api_version, obj_group, obj_kind, obj_namespace, obj_name,
  title, body, pinned, version, created_by, created_at, updated_by, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 1, ?, ?, ?, ?)`,
			n.Visibility, n.OwnerID, n.ClusterID,
			obj.APIVersion, obj.Group, obj.Kind, obj.Namespace, obj.Name,
			n.Title, n.Body, boolInt(n.Pinned), userID, t, userID, t)
		if err != nil {
			return err
		}
		if id, err = res.LastInsertId(); err != nil {
			return err
		}
		return setNoteTags(tx, id, n.Tags)
	})
	return id, err
}

// NoteEdit is the editable content of a note.
type NoteEdit struct {
	Title string
	Body  string
	Tags  []string
}

// UpdateNote replaces the content of note id if it is still at version,
// recording userID as the last editor. It fails with ErrNotFound or
// ErrNoteVersion.
func (s *Store) UpdateNote(id, version int64, e NoteEdit, userID int64) error {
	return s.withTx(func(tx *sql.Tx) error {
		res, err := tx.Exec(`UPDATE notes SET title = ?, body = ?, version = version + 1, updated_by = ?, updated_at = ?
WHERE id = ? AND version = ?`, e.Title, e.Body, userID, now(), id, version)
		if err != nil {
			return err
		}
		if n, err := res.RowsAffected(); err != nil || n == 0 {
			if err != nil {
				return err
			}
			var exists bool
			if err := tx.QueryRow(`SELECT EXISTS (SELECT 1 FROM notes WHERE id = ?)`, id).Scan(&exists); err != nil {
				return err
			}
			if !exists {
				return ErrNotFound
			}
			return ErrNoteVersion
		}
		if _, err := tx.Exec(`DELETE FROM note_tags WHERE note_id = ?`, id); err != nil {
			return err
		}
		return setNoteTags(tx, id, e.Tags)
	})
}

func setNoteTags(tx *sql.Tx, id int64, tags []string) error {
	for _, tag := range tags {
		if _, err := tx.Exec(`INSERT OR IGNORE INTO note_tags (note_id, tag) VALUES (?, ?)`, id, tag); err != nil {
			return err
		}
	}
	return nil
}

// SetNotePinned pins or unpins a note without counting it as an edit.
func (s *Store) SetNotePinned(id int64, pinned bool) error {
	return affectedOne(s.db.Exec(`UPDATE notes SET pinned = ? WHERE id = ?`, boolInt(pinned), id))
}

// DeleteNote deletes a note and its tags.
func (s *Store) DeleteNote(id int64) error {
	return affectedOne(s.db.Exec(`DELETE FROM notes WHERE id = ?`, id))
}

func affectedOne(res sql.Result, err error) error {
	if err != nil {
		return err
	}
	if n, err := res.RowsAffected(); err != nil || n == 0 {
		if err != nil {
			return err
		}
		return ErrNotFound
	}
	return nil
}

// NoteTags counts the tags of the notes f selects (Limit and WithBody do
// not apply), most used first.
func (s *Store) NoteTags(f NoteFilter) ([]TagCount, error) {
	where, args := f.where()
	rows, err := s.db.Query(`SELECT t.tag, count(*) FROM note_tags t JOIN notes n ON n.id = t.note_id
WHERE `+where+` GROUP BY t.tag ORDER BY count(*) DESC, t.tag`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []TagCount{}
	for rows.Next() {
		var tc TagCount
		if err := rows.Scan(&tc.Tag, &tc.Count); err != nil {
			return nil, err
		}
		out = append(out, tc)
	}
	return out, rows.Err()
}

// NoteCounts counts the notes f selects per object; f.Match must be set.
func (s *Store) NoteCounts(f NoteFilter) ([]ObjectNoteCount, error) {
	if f.Match == nil {
		f.Match = &NoteMatch{}
	}
	where, args := f.where()
	rows, err := s.db.Query(`SELECT max(n.obj_api_version), n.obj_group, n.obj_kind, n.obj_namespace, n.obj_name,
  sum(n.visibility = 'personal'), sum(n.visibility = 'shared')
FROM notes n WHERE `+where+`
GROUP BY n.obj_group, n.obj_kind, n.obj_namespace, n.obj_name
ORDER BY n.obj_kind, n.obj_namespace, n.obj_name`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ObjectNoteCount{}
	for rows.Next() {
		var c ObjectNoteCount
		o := &c.Object
		if err := rows.Scan(&o.APIVersion, &o.Group, &o.Kind, &o.Namespace, &o.Name, &c.Personal, &c.Shared); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}
