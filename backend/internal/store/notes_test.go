package store

import (
	"database/sql"
	"errors"
	"path/filepath"
	"reflect"
	"testing"
)

// notesFixture is a store with two clusters and three users.
type notesFixture struct {
	s               *Store
	alpha, beta     int64 // clusters
	ann, ben, admin int64 // users
}

func newNotesFixture(t *testing.T) notesFixture {
	t.Helper()
	s, _ := openTemp(t)
	f := notesFixture{s: s}
	var err error
	for _, c := range []struct {
		name string
		id   *int64
	}{{"alpha", &f.alpha}, {"beta", &f.beta}} {
		if *c.id, err = s.AddCluster(Cluster{Name: c.name, Source: SourceKubeconfig, Kubeconfig: "k"}); err != nil {
			t.Fatal(err)
		}
	}
	for _, u := range []struct {
		name, role string
		id         *int64
	}{{"ann", RoleUser, &f.ann}, {"ben", RoleUser, &f.ben}, {"root", RoleAdmin, &f.admin}} {
		if *u.id, err = s.CreateUser(u.name, "password-123", u.role, false); err != nil {
			t.Fatal(err)
		}
	}
	return f
}

func ptr[T any](v T) *T { return &v }

// add creates a note and returns its id.
func (f notesFixture) add(t *testing.T, n Note, author int64) int64 {
	t.Helper()
	id, err := f.s.CreateNote(n, author, 1000)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func personalNote(owner int64, cluster *int64, title string) Note {
	return Note{Visibility: NotePersonal, OwnerID: &owner, ClusterID: cluster, Title: title}
}

func sharedNote(cluster int64, title string) Note {
	return Note{Visibility: NoteShared, ClusterID: &cluster, Title: title}
}

func titles(t *testing.T, s *Store, f NoteFilter) []string {
	t.Helper()
	notes, _, err := s.ListNotes(f)
	if err != nil {
		t.Fatal(err)
	}
	out := []string{}
	for _, n := range notes {
		out = append(out, n.Title)
	}
	return out
}

// TestUpgradeFromV4AddsNotes migrates a v0.5 database (schema v4): its
// users and clusters stay, and notes can be written right away.
func TestUpgradeFromV4AddsNotes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "v4.db")
	old, err := sql.Open("sqlite", path+"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)")
	if err != nil {
		t.Fatal(err)
	}
	tx, err := old.Begin()
	if err != nil {
		t.Fatal(err)
	}
	for _, step := range migrations[:4] {
		if err := step(tx); err != nil {
			t.Fatal(err)
		}
	}
	for _, stmt := range []string{
		`PRAGMA user_version = 4`,
		`INSERT INTO users (id, username, password_hash, role) VALUES (1, 'admin', 'h', 'admin')`,
		`INSERT INTO clusters (id, name, source, kubeconfig) VALUES (7, 'prod', 'kubeconfig', 'k')`,
	} {
		if _, err := tx.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	old.Close()

	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if v := schemaVersion(t, s); v != len(migrations) || v < 5 {
		t.Fatalf("user_version = %d, want %d", v, len(migrations))
	}
	id, err := s.CreateNote(Note{Visibility: NoteShared, ClusterID: ptr(int64(7)), Title: "after the upgrade"}, 1, 10)
	if err != nil {
		t.Fatal(err)
	}
	n, err := s.GetNote(id)
	if err != nil || n.ClusterName != "prod" || n.CreatedBy != "admin" || n.Version != 1 {
		t.Fatalf("GetNote = %+v, %v", n, err)
	}
}

func TestNoteConstraints(t *testing.T) {
	f := newNotesFixture(t)
	for name, n := range map[string]Note{
		"shared without cluster":      {Visibility: NoteShared, Title: "x"},
		"shared with owner":           {Visibility: NoteShared, ClusterID: &f.alpha, OwnerID: &f.ann, Title: "x"},
		"personal without owner":      {Visibility: NotePersonal, ClusterID: &f.alpha, Title: "x"},
		"object without cluster":      {Visibility: NotePersonal, OwnerID: &f.ann, Object: &NoteObject{Kind: "Pod", Name: "p"}, Title: "x"},
		"unknown visibility":          {Visibility: "public", ClusterID: &f.alpha, Title: "x"},
		"cluster that does not exist": {Visibility: NoteShared, ClusterID: ptr(int64(99)), Title: "x"},
	} {
		if _, err := f.s.CreateNote(n, f.ann, 1000); err == nil {
			t.Errorf("%s: stored", name)
		}
	}
}

// TestNoteVisibility: a user lists their personal notes (global, on the
// active cluster, on others with AllClusters) and the active cluster's
// shared notes — never someone else's personal notes.
func TestNoteVisibility(t *testing.T) {
	f := newNotesFixture(t)
	f.add(t, personalNote(f.ann, nil, "ann global"), f.ann)
	f.add(t, personalNote(f.ann, &f.alpha, "ann alpha"), f.ann)
	f.add(t, personalNote(f.ann, &f.beta, "ann beta"), f.ann)
	f.add(t, personalNote(f.ben, &f.alpha, "ben alpha"), f.ben)
	f.add(t, personalNote(f.admin, &f.alpha, "admin alpha"), f.admin)
	f.add(t, sharedNote(f.alpha, "shared alpha"), f.ben)
	f.add(t, sharedNote(f.beta, "shared beta"), f.ann)

	same := func(got []string, want ...string) {
		t.Helper()
		if !sameSet(got, want) {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
	ann := NoteFilter{UserID: f.ann, ClusterID: f.alpha}
	same(titles(t, f.s, ann), "ann global", "ann alpha", "shared alpha")
	all := ann
	all.AllClusters = true
	same(titles(t, f.s, all), "ann global", "ann alpha", "ann beta", "shared alpha")
	only := ann
	only.Visibility = NoteShared
	same(titles(t, f.s, only), "shared alpha")
	only.Visibility = NotePersonal
	same(titles(t, f.s, only), "ann global", "ann alpha")
	same(titles(t, f.s, NoteFilter{UserID: f.admin, ClusterID: f.beta}), "shared beta")
}

func sameSet(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	seen := map[string]int{}
	for _, s := range a {
		seen[s]++
	}
	for _, s := range b {
		seen[s]--
	}
	for _, n := range seen {
		if n != 0 {
			return false
		}
	}
	return true
}

func TestNoteObjectsTagsAndSearch(t *testing.T) {
	f := newNotesFixture(t)
	web := &NoteObject{APIVersion: "apps/v1", Group: "apps", Kind: "Deployment", Namespace: "shop", Name: "web"}
	n := sharedNote(f.alpha, "Scaling web")
	n.Object, n.Body, n.Tags = web, "Keep **two** replicas: 100% of traffic", []string{"runbook", "shop"}
	f.add(t, n, f.ann)
	p := personalNote(f.ann, &f.alpha, "My web reminder")
	p.Object, p.Tags = &NoteObject{APIVersion: "apps/v1beta2", Group: "apps", Kind: "Deployment", Namespace: "shop", Name: "web"}, []string{"shop"}
	f.add(t, p, f.ann)
	node := personalNote(f.ann, &f.alpha, "Node disk")
	node.Object, node.Body = &NoteObject{APIVersion: "v1", Kind: "Node", Name: "worker-1"}, "replace the disk"
	f.add(t, node, f.ann)
	f.add(t, personalNote(f.ann, nil, "Unrelated"), f.ann)

	base := NoteFilter{UserID: f.ann, ClusterID: f.alpha}
	with := func(mod func(*NoteFilter)) NoteFilter { x := base; mod(&x); return x }

	// Every version of the API names the same Deployment.
	deploy := &NoteMatch{Group: ptr("apps"), Kind: ptr("Deployment"), Namespace: ptr("shop"), Name: ptr("web")}
	if got := titles(t, f.s, with(func(x *NoteFilter) { x.Match = deploy })); !sameSet(got, []string{"Scaling web", "My web reminder"}) {
		t.Fatalf("object match: %v", got)
	}
	if got := titles(t, f.s, with(func(x *NoteFilter) { x.Match = &NoteMatch{Group: ptr("extensions"), Kind: ptr("Deployment")} })); len(got) != 0 {
		t.Fatalf("another group matched: %v", got)
	}
	if got := titles(t, f.s, with(func(x *NoteFilter) { x.Tag = "runbook" })); !sameSet(got, []string{"Scaling web"}) {
		t.Fatalf("tag: %v", got)
	}
	if got := titles(t, f.s, with(func(x *NoteFilter) { x.Terms = []string{"REPLICAS", "two"} })); !sameSet(got, []string{"Scaling web"}) {
		t.Fatalf("search terms are ANDed and case-insensitive: %v", got)
	}
	// LIKE wildcards in a term are literal.
	if got := titles(t, f.s, with(func(x *NoteFilter) { x.Terms = []string{"100%"} })); !sameSet(got, []string{"Scaling web"}) {
		t.Fatalf("literal %%: %v", got)
	}
	if got := titles(t, f.s, with(func(x *NoteFilter) { x.Terms = []string{"1_0"} })); len(got) != 0 {
		t.Fatalf("_ must not be a wildcard: %v", got)
	}

	tags, err := f.s.NoteTags(base)
	if err != nil || !reflect.DeepEqual(tags, []TagCount{{"shop", 2}, {"runbook", 1}}) {
		t.Fatalf("NoteTags = %v, %v", tags, err)
	}
	counts, err := f.s.NoteCounts(with(func(x *NoteFilter) { x.Match = &NoteMatch{Namespace: ptr("shop")} }))
	if err != nil || len(counts) != 1 || counts[0].Personal != 1 || counts[0].Shared != 1 || counts[0].Object.Name != "web" {
		t.Fatalf("NoteCounts = %+v, %v", counts, err)
	}
	counts, err = f.s.NoteCounts(base)
	if err != nil || len(counts) != 2 {
		t.Fatalf("all counts = %+v, %v", counts, err)
	}

	// Bodies only on request; tags sorted.
	notes, _, err := f.s.ListNotes(with(func(x *NoteFilter) { x.Tag = "runbook" }))
	if err != nil || notes[0].Body != "" || !reflect.DeepEqual(notes[0].Tags, []string{"runbook", "shop"}) {
		t.Fatalf("ListNotes = %+v, %v", notes, err)
	}
}

func TestNoteOrderAndLimit(t *testing.T) {
	f := newNotesFixture(t)
	first := f.add(t, sharedNote(f.alpha, "first"), f.ann)
	f.add(t, sharedNote(f.alpha, "second"), f.ann)
	f.add(t, sharedNote(f.alpha, "third"), f.ann)
	if err := f.s.SetNotePinned(first, true); err != nil {
		t.Fatal(err)
	}
	notes, truncated, err := f.s.ListNotes(NoteFilter{UserID: f.ann, ClusterID: f.alpha, Limit: 2})
	if err != nil || !truncated || len(notes) != 2 || notes[0].Title != "first" || notes[1].Title != "third" {
		t.Fatalf("pinned first, then newest: %+v truncated=%v err=%v", notes, truncated, err)
	}
	if n, _ := f.s.GetNote(first); n.Version != 1 {
		t.Fatalf("pinning is not an edit: version %d", n.Version)
	}

	if _, err := f.s.CreateNote(sharedNote(f.alpha, "over"), f.ann, 3); !errors.Is(err, ErrNoteLimit) {
		t.Fatalf("limit: %v", err)
	}
	// The limit counts the author's notes, not anyone else's.
	if _, err := f.s.CreateNote(sharedNote(f.alpha, "ben's"), f.ben, 3); err != nil {
		t.Fatal(err)
	}
}

func TestNoteUpdateConflict(t *testing.T) {
	f := newNotesFixture(t)
	n := sharedNote(f.alpha, "draft")
	n.Tags = []string{"a"}
	id := f.add(t, n, f.ann)
	if err := f.s.UpdateNote(id, 1, NoteEdit{Title: "v2", Body: "b", Tags: []string{"b", "c"}}, f.ben); err != nil {
		t.Fatal(err)
	}
	got, err := f.s.GetNote(id)
	if err != nil || got.Version != 2 || got.Title != "v2" || got.UpdatedBy != "ben" || got.CreatedBy != "ann" ||
		!reflect.DeepEqual(got.Tags, []string{"b", "c"}) || got.UpdatedAt.Before(got.CreatedAt) {
		t.Fatalf("after update: %+v, %v", got, err)
	}
	if err := f.s.UpdateNote(id, 1, NoteEdit{Title: "stale"}, f.ann); !errors.Is(err, ErrNoteVersion) {
		t.Fatalf("stale version: %v", err)
	}
	if err := f.s.UpdateNote(999, 1, NoteEdit{Title: "x"}, f.ann); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing note: %v", err)
	}
	if err := f.s.DeleteNote(id); err != nil {
		t.Fatal(err)
	}
	if err := f.s.DeleteNote(id); !errors.Is(err, ErrNotFound) {
		t.Fatalf("second delete: %v", err)
	}
	var tags int
	if err := f.s.db.QueryRow(`SELECT count(*) FROM note_tags`).Scan(&tags); err != nil || tags != 0 {
		t.Fatalf("tags left behind: %d, %v", tags, err)
	}
}

// TestNoteCascades: deleting a cluster deletes its shared notes and the
// personal notes placed on it; deleting a user deletes their personal
// notes and keeps their shared ones, with an unknown author.
func TestNoteCascades(t *testing.T) {
	f := newNotesFixture(t)
	annGlobal := f.add(t, personalNote(f.ann, nil, "ann global"), f.ann)
	annAlpha := f.add(t, personalNote(f.ann, &f.alpha, "ann alpha"), f.ann)
	annShared := f.add(t, sharedNote(f.alpha, "ann shared"), f.ann)
	benBeta := f.add(t, personalNote(f.ben, &f.beta, "ben beta"), f.ben)
	betaShared := f.add(t, sharedNote(f.beta, "beta shared"), f.ann)
	if err := f.s.UpdateNote(annShared, 1, NoteEdit{Title: "edited by ann"}, f.ann); err != nil {
		t.Fatal(err)
	}

	if err := f.s.DeleteCluster(f.beta); err != nil {
		t.Fatal(err)
	}
	for _, id := range []int64{benBeta, betaShared} {
		if _, err := f.s.GetNote(id); !errors.Is(err, ErrNotFound) {
			t.Fatalf("note %d survived its cluster: %v", id, err)
		}
	}

	if err := f.s.DeleteUser(f.ann); err != nil {
		t.Fatal(err)
	}
	for _, id := range []int64{annGlobal, annAlpha} {
		if _, err := f.s.GetNote(id); !errors.Is(err, ErrNotFound) {
			t.Fatalf("personal note %d survived its owner: %v", id, err)
		}
	}
	n, err := f.s.GetNote(annShared)
	if err != nil || n.CreatedBy != "" || n.UpdatedBy != "" || n.CreatorID != nil || n.Title != "edited by ann" {
		t.Fatalf("shared note after its author was deleted: %+v, %v", n, err)
	}
}
