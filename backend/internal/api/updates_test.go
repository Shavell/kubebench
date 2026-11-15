package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"kubebench/internal/store"
)

// releasesServer is a fake GitHub releases endpoint with one release newer
// than the test build (1.2.3) and counts its requests.
func releasesServer(t *testing.T) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var requests atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"tag_name":"v1.3.0","html_url":"https://github.com/Shavell/kube-bench/releases/tag/v1.3.0","body":"## Added\n- things","published_at":"2026-11-01T00:00:00Z"}]`))
	}))
	t.Cleanup(srv.Close)
	return srv, &requests
}

type updateStatusBody struct {
	Mode        string  `json:"mode"`
	DefaultMode string  `json:"defaultMode"`
	Current     string  `json:"current"`
	Latest      string  `json:"latest"`
	Available   bool    `json:"available"`
	Notes       string  `json:"notes"`
	URL         string  `json:"url"`
	CheckedAt   *string `json:"checkedAt"`
	Error       string  `json:"error"`
}

func TestUpdatesStatusForEverySession(t *testing.T) {
	gh, requests := releasesServer(t)
	e := newTestEnv(t, map[string]string{"KUBEBENCH_UPDATE_URL": gh.URL})

	expect(t, e.do("GET", "/api/updates", "", nil), http.StatusUnauthorized, "unauthenticated")
	rec := e.do("GET", "/api/updates", "bob", nil)
	expect(t, rec, http.StatusOK, "")
	s := decode[updateStatusBody](t, rec)
	if s.Mode != "stable" || s.DefaultMode != "stable" || s.Current != "1.2.3" || s.Latest != "1.3.0" || !s.Available {
		t.Fatalf("status = %+v", s)
	}
	if s.Notes != "## Added\n- things" || s.CheckedAt == nil || s.Error != "" {
		t.Fatalf("status = %+v", s)
	}
	// A forced password change does not hide the version either.
	if err := e.store.SetMustChangePassword(e.users["alice"], true); err != nil {
		t.Fatal(err)
	}
	expect(t, e.do("GET", "/api/updates", "alice", nil), http.StatusOK, "")
	if n := requests.Load(); n != 1 {
		t.Fatalf("%d requests to the releases URL, want 1 (cached)", n)
	}
}

func TestUpdateModeIsAdminOnlyAndAudited(t *testing.T) {
	gh, requests := releasesServer(t)
	e := newTestEnv(t, map[string]string{"KUBEBENCH_UPDATE_URL": gh.URL})

	expect(t, e.do("PUT", "/api/settings/updates", "alice", map[string]string{"mode": "off"}), http.StatusForbidden, "admin_required")
	expectAudit(t, e, "PUT /api/settings/updates", "setting", http.StatusForbidden, "")
	expect(t, e.do("PUT", "/api/settings/updates", "admin", map[string]string{"mode": "weekly"}), http.StatusBadRequest, "bad_request")

	rec := e.do("PUT", "/api/settings/updates", "admin", map[string]string{"mode": "off"})
	expect(t, rec, http.StatusOK, "")
	if s := decode[updateStatusBody](t, rec); s.Mode != "off" || s.DefaultMode != "stable" || s.Latest != "" {
		t.Fatalf("status = %+v", s)
	}
	expectAudit(t, e, "PUT /api/settings/updates", "setting/updates", http.StatusOK, `{"mode":"off"}`)
	if mode, set, err := e.store.AppSetting(store.SettingUpdateMode); err != nil || !set || mode != "off" {
		t.Fatalf("stored mode = %q, %v, %v", mode, set, err)
	}

	// Off: no request, and a forced check is refused.
	if s := decode[updateStatusBody](t, e.do("GET", "/api/updates", "bob", nil)); s.Mode != "off" {
		t.Fatalf("status = %+v", s)
	}
	expect(t, e.do("POST", "/api/updates/check", "admin", nil), http.StatusConflict, "conflict")
	if n := requests.Load(); n != 0 {
		t.Fatalf("mode off made %d requests", n)
	}

	expect(t, e.do("PUT", "/api/settings/updates", "admin", map[string]string{"mode": "prerelease"}), http.StatusOK, "")
	if s := decode[updateStatusBody](t, e.do("GET", "/api/updates", "bob", nil)); s.Mode != "prerelease" || s.Latest != "1.3.0" {
		t.Fatalf("status = %+v", s)
	}
}

func TestCheckUpdatesNow(t *testing.T) {
	gh, requests := releasesServer(t)
	e := newTestEnv(t, map[string]string{"KUBEBENCH_UPDATE_URL": gh.URL})

	expect(t, e.do("POST", "/api/updates/check", "alice", nil), http.StatusForbidden, "admin_required")
	expect(t, e.do("POST", "/api/updates/check", "bob", nil), http.StatusForbidden, "admin_required")
	if n := requests.Load(); n != 0 {
		t.Fatalf("a refused check made %d requests", n)
	}
	for range 2 {
		rec := e.do("POST", "/api/updates/check", "admin", nil)
		expect(t, rec, http.StatusOK, "")
		if s := decode[updateStatusBody](t, rec); !s.Available {
			t.Fatalf("status = %+v", s)
		}
	}
	if n := requests.Load(); n != 2 {
		t.Fatalf("%d requests, want one per forced check", n)
	}
	expectAudit(t, e, "POST /api/updates/check", "update", http.StatusOK, "")
}

func TestConfiguredUpdateModeOff(t *testing.T) {
	gh, requests := releasesServer(t)
	e := newTestEnv(t, map[string]string{"KUBEBENCH_UPDATE_URL": gh.URL, "KUBEBENCH_UPDATE_MODE": "off"})
	s := decode[updateStatusBody](t, e.do("GET", "/api/updates", "admin", nil))
	if s.Mode != "off" || s.DefaultMode != "off" || s.Current != "1.2.3" || s.CheckedAt != nil {
		t.Fatalf("status = %+v", s)
	}
	if n := requests.Load(); n != 0 {
		t.Fatalf("mode off made %d requests", n)
	}
	// An admin may still turn checks on.
	expect(t, e.do("PUT", "/api/settings/updates", "admin", map[string]string{"mode": "stable"}), http.StatusOK, "")
	if n := requests.Load(); n != 1 {
		t.Fatalf("%d requests after turning checks on, want 1", n)
	}
}

func TestLastSeenVersion(t *testing.T) {
	e := newTestEnv(t, nil)
	type me struct {
		Username        string `json:"username"`
		LastSeenVersion string `json:"lastSeenVersion"`
	}
	if got := decode[me](t, e.do("GET", "/api/auth/me", "bob", nil)); got.Username != "bob" || got.LastSeenVersion != "" {
		t.Fatalf("me = %+v", got)
	}
	expect(t, e.do("PATCH", "/api/auth/me", "", map[string]string{"lastSeenVersion": "1.2.3"}), http.StatusUnauthorized, "unauthenticated")
	expect(t, e.do("PATCH", "/api/auth/me", "bob", map[string]string{}), http.StatusBadRequest, "bad_request")
	expect(t, e.do("PATCH", "/api/auth/me", "bob", map[string]string{"lastSeenVersion": " "}), http.StatusBadRequest, "bad_request")
	expect(t, e.do("PATCH", "/api/auth/me", "bob", map[string]string{"lastSeenVersion": strings.Repeat("1", 65)}), http.StatusBadRequest, "bad_request")

	rec := e.do("PATCH", "/api/auth/me", "bob", map[string]string{"lastSeenVersion": "1.2.3"})
	expect(t, rec, http.StatusOK, "")
	if got := decode[me](t, rec); got.LastSeenVersion != "1.2.3" {
		t.Fatalf("me = %+v", got)
	}
	if got := decode[me](t, e.do("GET", "/api/auth/me", "bob", nil)); got.LastSeenVersion != "1.2.3" {
		t.Fatalf("me = %+v", got)
	}
	// Per user, and not audited (it is the user's own UI state).
	if got := decode[me](t, e.do("GET", "/api/auth/me", "alice", nil)); got.LastSeenVersion != "" {
		t.Fatalf("alice = %+v", got)
	}
	if rows := e.audit(); len(rows) != 0 {
		t.Fatalf("audit rows = %+v", rows)
	}
}
