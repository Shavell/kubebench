package updates

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// releasesJSON is a GitHub "list releases" answer, deliberately unsorted,
// with a draft, a non-SemVer tag and a prerelease newer than any release.
const releasesJSON = `[
  {"tag_name":"v0.5.0","draft":false,"prerelease":false,"html_url":"https://github.com/Shavell/kubebench/releases/tag/v0.5.0","body":"old","published_at":"2026-10-01T10:00:00Z"},
  {"tag_name":"v0.7.0","draft":true,"prerelease":false,"html_url":"https://github.com/x","body":"draft"},
  {"tag_name":"v0.6.0-rc.1","draft":false,"prerelease":true,"html_url":"https://github.com/Shavell/kubebench/releases/tag/v0.6.0-rc.1","body":"## Candidate","published_at":"2026-11-10T08:00:00Z"},
  {"tag_name":"nightly","draft":false,"prerelease":false,"html_url":"https://github.com/y","body":"nightly"},
  {"tag_name":"v0.5.1","draft":false,"prerelease":false,"html_url":"https://github.com/Shavell/kubebench/releases/tag/v0.5.1","body":"### Fixed\n- a bug","published_at":"2026-11-01T09:30:00Z"}
]`

// fakeGitHub serves a release list through a replaceable handler and
// counts the requests it gets.
type fakeGitHub struct {
	*httptest.Server
	requests atomic.Int32
	mu       sync.Mutex
	handler  http.HandlerFunc
	lastReq  *http.Request
}

func newFakeGitHub(t *testing.T, handler http.HandlerFunc) *fakeGitHub {
	t.Helper()
	f := &fakeGitHub{handler: handler}
	f.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.requests.Add(1)
		f.mu.Lock()
		h := f.handler
		f.lastReq = r.Clone(context.Background())
		f.mu.Unlock()
		h(w, r)
	}))
	t.Cleanup(f.Close)
	return f
}

func (f *fakeGitHub) setHandler(h http.HandlerFunc) {
	f.mu.Lock()
	f.handler = h
	f.mu.Unlock()
}

func (f *fakeGitHub) last() *http.Request {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.lastReq
}

func serveJSON(body, etag string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if etag != "" {
			if r.Header.Get("If-None-Match") == etag {
				w.WriteHeader(http.StatusNotModified)
				return
			}
			w.Header().Set("ETag", etag)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}
}

// clock is a settable time source.
type clock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *clock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *clock) advance(d time.Duration) {
	c.mu.Lock()
	c.now = c.now.Add(d)
	c.mu.Unlock()
}

func newChecker(url, version string) (*Checker, *clock) {
	clk := &clock{now: time.Date(2026, 11, 15, 12, 0, 0, 0, time.UTC)}
	return New(Options{URL: url, Version: version, Client: &http.Client{}, Now: clk.Now}), clk
}

func TestLatestRelease(t *testing.T) {
	gh := newFakeGitHub(t, serveJSON(releasesJSON, ""))
	c, _ := newChecker(gh.URL+"/repos/Shavell/kubebench/releases", "0.5.0")

	s := c.Status(context.Background(), ModeStable)
	if s.Error != "" || s.Latest != "0.5.1" || !s.Available || s.Prerelease || s.Current != "0.5.0" {
		t.Fatalf("stable status = %+v", s)
	}
	if s.URL != "https://github.com/Shavell/kubebench/releases/tag/v0.5.1" || s.Notes != "### Fixed\n- a bug" {
		t.Fatalf("release details = %q, %q", s.URL, s.Notes)
	}
	if s.PublishedAt == nil || !s.PublishedAt.Equal(time.Date(2026, 11, 1, 9, 30, 0, 0, time.UTC)) {
		t.Fatalf("publishedAt = %v", s.PublishedAt)
	}
	if s.CheckedAt == nil || s.Mode != ModeStable {
		t.Fatalf("checkedAt %v, mode %q", s.CheckedAt, s.Mode)
	}

	req := gh.last()
	if req.URL.Query().Get("per_page") != "20" || req.Header.Get("Accept") != "application/vnd.github+json" ||
		req.Header.Get("User-Agent") != "KubeBench/0.5.0" || req.Header.Get("X-GitHub-Api-Version") == "" {
		t.Fatalf("request %s with headers %v", req.URL, req.Header)
	}

	// The prerelease mode reuses the cached list.
	p := c.Status(context.Background(), ModePrerelease)
	if p.Latest != "0.6.0-rc.1" || !p.Prerelease || !p.Available || p.Notes != "## Candidate" {
		t.Fatalf("prerelease status = %+v", p)
	}
	if n := gh.requests.Load(); n != 1 {
		t.Fatalf("%d requests, want 1", n)
	}
}

func TestPrereleaseByVersionOnly(t *testing.T) {
	// A tag with a prerelease part counts as one even when GitHub's flag is off.
	gh := newFakeGitHub(t, serveJSON(`[{"tag_name":"v1.0.0-beta.1","prerelease":false},{"tag_name":"v0.9.0"}]`, ""))
	c, _ := newChecker(gh.URL, "0.9.0")
	if s := c.Status(context.Background(), ModeStable); s.Latest != "0.9.0" || s.Available {
		t.Fatalf("stable status = %+v", s)
	}
	if s := c.Status(context.Background(), ModePrerelease); s.Latest != "1.0.0-beta.1" || !s.Prerelease || !s.Available {
		t.Fatalf("prerelease status = %+v", s)
	}
}

func TestCurrentIsNewestOrCandidate(t *testing.T) {
	gh := newFakeGitHub(t, serveJSON(releasesJSON, ""))
	for _, tc := range []struct {
		version   string
		mode      Mode
		available bool
	}{
		{"0.5.1", ModeStable, false},
		{"0.6.0-rc.1", ModePrerelease, false},
		{"0.5.1", ModePrerelease, true},
		{"0.5.1-rc.2", ModeStable, true}, // the release supersedes its candidate
		{"1.0.0", ModePrerelease, false},
	} {
		c, _ := newChecker(gh.URL, tc.version)
		if s := c.Status(context.Background(), tc.mode); s.Available != tc.available {
			t.Errorf("%s in %s mode: available = %v (latest %s)", tc.version, tc.mode, s.Available, s.Latest)
		}
	}
}

func TestDevBuildIsUnknown(t *testing.T) {
	gh := newFakeGitHub(t, serveJSON(releasesJSON, ""))
	for _, version := range []string{"dev", "0.5.0-3-gabc1234", "0.5.1-snapshot-abc1234"} {
		c, _ := newChecker(gh.URL, version)
		s := c.Status(context.Background(), ModeStable)
		if s.Current != "unknown" || s.Available || s.Latest != "0.5.1" {
			t.Errorf("%s: status = %+v", version, s)
		}
	}
}

func TestCacheAndETag(t *testing.T) {
	gh := newFakeGitHub(t, serveJSON(releasesJSON, `"v1"`))
	c, clk := newChecker(gh.URL, "0.5.0")
	first := c.Status(context.Background(), ModeStable)

	clk.advance(5 * time.Hour)
	if c.Status(context.Background(), ModeStable); gh.requests.Load() != 1 {
		t.Fatal("a result younger than 6 h must be reused")
	}

	clk.advance(time.Hour)
	s := c.Status(context.Background(), ModeStable)
	if gh.requests.Load() != 2 || gh.last().Header.Get("If-None-Match") != `"v1"` {
		t.Fatalf("expired cache: %d requests, If-None-Match %q", gh.requests.Load(), gh.last().Header.Get("If-None-Match"))
	}
	if s.Latest != "0.5.1" || s.Error != "" || !s.CheckedAt.After(*first.CheckedAt) {
		t.Fatalf("after 304: %+v", s)
	}

	// A forced check revalidates at once.
	c.Check(context.Background(), ModeStable)
	if gh.requests.Load() != 3 {
		t.Fatalf("Check made %d requests in total, want 3", gh.requests.Load())
	}
}

func TestRateLimitKeepsLastResult(t *testing.T) {
	gh := newFakeGitHub(t, serveJSON(releasesJSON, ""))
	c, clk := newChecker(gh.URL, "0.5.0")
	c.Status(context.Background(), ModeStable)

	reset := clk.Now().Add(cacheTTL + 30*time.Minute)
	gh.setHandler(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-RateLimit-Remaining", "0")
		w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(reset.Unix(), 10))
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"message":"API rate limit exceeded for 203.0.113.7."}`))
	})
	clk.advance(cacheTTL)
	s := c.Status(context.Background(), ModeStable)
	if !strings.Contains(s.Error, "rate limit exceeded") || !strings.Contains(s.Error, "18:30 UTC") {
		t.Fatalf("error = %q", s.Error)
	}
	if s.Latest != "0.5.1" || !s.Available {
		t.Fatalf("the last good result must survive: %+v", s)
	}

	clk.advance(29 * time.Minute)
	c.Status(context.Background(), ModeStable)
	if gh.requests.Load() != 2 {
		t.Fatal("no request may be made before the rate limit resets")
	}
	gh.setHandler(serveJSON(releasesJSON, ""))
	clk.advance(time.Minute)
	if s := c.Status(context.Background(), ModeStable); gh.requests.Load() != 3 || s.Error != "" {
		t.Fatalf("after the reset: %d requests, error %q", gh.requests.Load(), s.Error)
	}
}

func TestRetryAfter(t *testing.T) {
	resp := &http.Response{StatusCode: http.StatusTooManyRequests, Header: http.Header{"Retry-After": {"120"}}}
	if wait, ok := rateLimited(resp, time.Now()); !ok || wait != 2*time.Minute {
		t.Fatalf("Retry-After 120: %v, %v", wait, ok)
	}
	// A 403 without rate-limit headers is some other refusal.
	if _, ok := rateLimited(&http.Response{StatusCode: http.StatusForbidden, Header: http.Header{}}, time.Now()); ok {
		t.Fatal("a plain 403 is no rate limit")
	}
}

func TestOtherHTTPError(t *testing.T) {
	gh := newFakeGitHub(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(`{"message":"upstream down"}`))
	})
	c, _ := newChecker(gh.URL, "0.5.0")
	if s := c.Status(context.Background(), ModeStable); !strings.Contains(s.Error, "502 Bad Gateway: upstream down") || s.CheckedAt != nil {
		t.Fatalf("status = %+v", s)
	}
}

func TestNetworkError(t *testing.T) {
	gh := newFakeGitHub(t, serveJSON(releasesJSON, ""))
	url := gh.URL
	gh.Close()
	c, _ := newChecker(url, "0.5.0")
	s := c.Status(context.Background(), ModeStable)
	if !strings.HasPrefix(s.Error, "cannot reach 127.0.0.1:") || s.Latest != "" || s.CheckedAt != nil {
		t.Fatalf("status = %+v", s)
	}
}

// failingTransport fails every request like an unreachable network.
type failingTransport struct{ calls atomic.Int32 }

func (f *failingTransport) RoundTrip(*http.Request) (*http.Response, error) {
	f.calls.Add(1)
	return nil, errors.New("dial tcp: lookup api.github.com: no such host")
}

func TestFailedCheckRetriesAfter15Minutes(t *testing.T) {
	transport := &failingTransport{}
	clk := &clock{now: time.Date(2026, 11, 15, 12, 0, 0, 0, time.UTC)}
	c := New(Options{URL: "https://api.github.com/repos/Shavell/kubebench/releases", Version: "0.5.0",
		Client: &http.Client{Transport: transport}, Now: clk.Now})
	s := c.Status(context.Background(), ModeStable)
	if s.Error != "cannot reach api.github.com: dial tcp: lookup api.github.com: no such host" {
		t.Fatalf("error = %q", s.Error)
	}
	clk.advance(failureRetry - time.Second)
	c.Status(context.Background(), ModeStable)
	if transport.calls.Load() != 1 {
		t.Fatal("a failed check must not be repeated within 15 minutes")
	}
	clk.advance(time.Second)
	c.Status(context.Background(), ModeStable)
	if transport.calls.Load() != 2 {
		t.Fatal("a failed check must be retried after 15 minutes")
	}
}

func TestMalformedJSON(t *testing.T) {
	gh := newFakeGitHub(t, serveJSON(`{"tag_name": "v1.0.0"`, ""))
	c, _ := newChecker(gh.URL, "0.5.0")
	if s := c.Status(context.Background(), ModeStable); !strings.Contains(s.Error, "unexpected response from") || s.Latest != "" {
		t.Fatalf("status = %+v", s)
	}
}

func TestNoReleasesPublished(t *testing.T) {
	for name, handler := range map[string]http.HandlerFunc{
		"empty list": serveJSON(`[]`, ""),
		"404": func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, `{"message":"Not Found"}`, http.StatusNotFound)
		},
	} {
		gh := newFakeGitHub(t, handler)
		c, _ := newChecker(gh.URL, "0.5.0")
		s := c.Status(context.Background(), ModeStable)
		if s.Error != "" || s.Latest != "" || s.Available || s.CheckedAt == nil {
			t.Errorf("%s: status = %+v", name, s)
		}
	}
}

func TestModeOffMakesNoRequest(t *testing.T) {
	gh := newFakeGitHub(t, serveJSON(releasesJSON, ""))
	c, _ := newChecker(gh.URL, "0.5.0")
	s := c.Status(context.Background(), ModeOff)
	c.Check(context.Background(), ModeOff)
	if gh.requests.Load() != 0 {
		t.Fatalf("mode off made %d requests", gh.requests.Load())
	}
	if s.Mode != ModeOff || s.Current != "0.5.0" || s.Latest != "" || s.CheckedAt != nil {
		t.Fatalf("status = %+v", s)
	}
}

func TestConcurrentCallersShareOneRequest(t *testing.T) {
	release := make(chan struct{})
	gh := newFakeGitHub(t, func(w http.ResponseWriter, r *http.Request) {
		<-release
		serveJSON(releasesJSON, "")(w, r)
	})
	c, _ := newChecker(gh.URL, "0.5.0")
	var wg sync.WaitGroup
	for range 5 {
		wg.Go(func() {
			if s := c.Status(context.Background(), ModeStable); s.Latest != "0.5.1" {
				t.Errorf("status = %+v", s)
			}
		})
	}
	time.Sleep(50 * time.Millisecond) // let every caller queue up
	close(release)
	wg.Wait()
	if n := gh.requests.Load(); n != 1 {
		t.Fatalf("%d requests, want 1", n)
	}
}

func TestCheckOutlivesItsCaller(t *testing.T) {
	gh := newFakeGitHub(t, serveJSON(releasesJSON, ""))
	c, _ := newChecker(gh.URL, "0.5.0")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if s := c.Status(ctx, ModeStable); s.Error != "" || s.Latest != "0.5.1" {
		t.Fatalf("a cancelled caller must not abort the shared check: %+v", s)
	}
}

func TestReleasesPage(t *testing.T) {
	c := New(Options{URL: "https://api.github.com/repos/Shavell/kubebench/releases", Version: "0.5.0"})
	if s := c.status(ModeStable); s.URL != "https://github.com/Shavell/kubebench/releases" {
		t.Fatalf("url = %q", s.URL)
	}
	if got := releasesPage("https://mirror.internal/kubebench/releases.json"); got != "" {
		t.Fatalf("a mirror has no known releases page: %q", got)
	}
	if got := withPerPage("https://mirror.internal/r?per_page=5"); got != "https://mirror.internal/r?per_page=5" {
		t.Fatalf("withPerPage kept %q", got)
	}
}

func TestUnsafeReleaseLinkDropped(t *testing.T) {
	gh := newFakeGitHub(t, serveJSON(`[{"tag_name":"v0.6.0","html_url":"javascript:alert(1)"}]`, ""))
	c, _ := newChecker(gh.URL, "0.5.0")
	if s := c.Status(context.Background(), ModeStable); s.URL != "" || s.Latest != "0.6.0" {
		t.Fatalf("status = %+v", s)
	}
}
