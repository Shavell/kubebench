// Package updates checks whether a newer KubeBench release is published
// (docs/design-v0.5.md §7.11). It reads GitHub's "list releases" JSON, or
// a mirror of it, lazily and at most every few hours, and never installs
// anything.
package updates

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Masterminds/semver/v3"
)

const (
	// cacheTTL is how long a successful check is reused.
	cacheTTL = 6 * time.Hour
	// failureRetry is the earliest retry after a failed check.
	failureRetry = 15 * time.Minute
	// requestTimeout bounds one request, connecting included.
	requestTimeout = 10 * time.Second
	// maxBody caps the release list read from the server.
	maxBody = 4 << 20
	// perPage is how many releases one request asks for.
	perPage = "20"
)

// Status is the outcome of the latest check for one mode, the JSON of
// GET /api/updates without the server's default mode.
type Status struct {
	Mode Mode `json:"mode"`
	// Current is the build version, or "unknown" for builds that are no
	// release (see ParseBuildVersion).
	Current string `json:"current"`
	// Latest is the newest release the mode admits; empty when there is none.
	Latest      string     `json:"latest"`
	Available   bool       `json:"available"`
	Prerelease  bool       `json:"prerelease"`
	PublishedAt *time.Time `json:"publishedAt"`
	// URL is the release page, or the releases page when there is no release.
	URL string `json:"url"`
	// Notes are the release notes as published: Markdown source.
	Notes string `json:"notes"`
	// CheckedAt is the time of the last successful check.
	CheckedAt *time.Time `json:"checkedAt"`
	// Error is why the last check failed; empty after a success.
	Error string `json:"error"`
}

// Options configure a Checker.
type Options struct {
	// URL is the releases endpoint (KUBEBENCH_UPDATE_URL).
	URL string
	// Version is the running build's version (main.version).
	Version string
	// Client defaults to one with a 10 s timeout that takes its proxy
	// from HTTPS_PROXY, HTTP_PROXY and NO_PROXY.
	Client *http.Client
	// Now defaults to time.Now.
	Now func() time.Time
}

// release is one published release the check considers.
type release struct {
	version     *semver.Version
	prerelease  bool
	url         string
	notes       string
	publishedAt *time.Time
}

// Checker caches the release list. It is safe for concurrent use.
type Checker struct {
	url          string
	releasesPage string
	userAgent    string
	current      *semver.Version
	client       *http.Client
	now          func() time.Time

	// fetching serializes requests, so concurrent callers share one.
	fetching sync.Mutex

	mu        sync.Mutex // guards the fields below
	releases  []release  // newest first
	etag      string
	checkedAt time.Time // zero until a check succeeded
	lastError string
	next      time.Time // when a lazy check is due
}

// New returns a Checker; it makes no request until one is asked for.
func New(o Options) *Checker {
	c := &Checker{
		url:          withPerPage(o.URL),
		releasesPage: releasesPage(o.URL),
		userAgent:    "KubeBench/" + o.Version,
		current:      ParseBuildVersion(o.Version),
		client:       o.Client,
		now:          o.Now,
	}
	if c.client == nil {
		// DefaultTransport takes its proxy from the environment.
		c.client = &http.Client{Timeout: requestTimeout, Transport: http.DefaultTransport.(*http.Transport).Clone()}
	}
	if c.now == nil {
		c.now = time.Now
	}
	return c
}

// Status reports the latest check for mode, checking first when the
// cached result has expired. Mode off never makes a request.
func (c *Checker) Status(ctx context.Context, mode Mode) Status {
	if mode != ModeOff {
		c.refresh(ctx, false)
	}
	return c.status(mode)
}

// Check checks now, whatever the cache holds (the request still carries
// the cached ETag), and reports the result for mode. Mode off never makes
// a request.
func (c *Checker) Check(ctx context.Context, mode Mode) Status {
	if mode != ModeOff {
		c.refresh(ctx, true)
	}
	return c.status(mode)
}

// refresh checks when forced or due. The request does not end with ctx:
// a caller that goes away leaves the result to the next one.
func (c *Checker) refresh(ctx context.Context, force bool) {
	c.fetching.Lock()
	defer c.fetching.Unlock()
	c.mu.Lock()
	due := force || !c.now().Before(c.next)
	c.mu.Unlock()
	if !due {
		return
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), requestTimeout)
	defer cancel()
	c.fetch(ctx)
}

// fetch requests the release list once and records the outcome.
func (c *Checker) fetch(ctx context.Context) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.url, nil)
	if err != nil {
		c.failed(err.Error(), failureRetry)
		return
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("User-Agent", c.userAgent)
	c.mu.Lock()
	if c.etag != "" {
		req.Header.Set("If-None-Match", c.etag)
	}
	c.mu.Unlock()

	resp, err := c.client.Do(req)
	if err != nil {
		c.failed(fmt.Sprintf("cannot reach %s: %v", req.URL.Host, transportError(err)), failureRetry)
		return
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode == http.StatusOK:
		releases, err := decodeReleases(io.LimitReader(resp.Body, maxBody+1))
		if err != nil {
			c.failed(fmt.Sprintf("unexpected response from %s: %v", req.URL.Host, err), failureRetry)
			return
		}
		c.succeeded(releases, resp.Header.Get("ETag"), true)
	case resp.StatusCode == http.StatusNotModified:
		c.succeeded(nil, "", false)
	case resp.StatusCode == http.StatusNotFound:
		// GitHub answers 404 for a repository without public releases yet.
		c.succeeded([]release{}, "", true)
	default:
		if wait, limited := rateLimited(resp, c.now()); limited {
			c.failed(fmt.Sprintf("%s rate limit exceeded; the next check is after %s",
				req.URL.Host, c.now().Add(wait).UTC().Format("15:04 UTC")), wait)
			return
		}
		c.failed(fmt.Sprintf("%s answered %s%s", req.URL.Host, resp.Status, serverMessage(resp.Body)), failureRetry)
	}
}

// succeeded records a successful check; replace false keeps the cached
// list and ETag (304 Not Modified).
func (c *Checker) succeeded(releases []release, etag string, replace bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if replace {
		c.releases, c.etag = releases, etag
	}
	c.checkedAt = c.now()
	c.lastError = ""
	c.next = c.checkedAt.Add(cacheTTL)
}

// failed records a failed check, keeping the last good result.
func (c *Checker) failed(reason string, retryIn time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lastError = reason
	c.next = c.now().Add(retryIn)
}

func (c *Checker) status(mode Mode) Status {
	s := Status{Mode: mode, Current: "unknown", URL: c.releasesPage}
	if c.current != nil {
		s.Current = c.current.String()
	}
	if mode == ModeOff {
		return s
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	s.Error = c.lastError
	if !c.checkedAt.IsZero() {
		checked := c.checkedAt.UTC()
		s.CheckedAt = &checked
	}
	for _, r := range c.releases {
		if r.prerelease && mode != ModePrerelease {
			continue
		}
		s.Latest, s.Prerelease, s.Notes, s.PublishedAt = r.version.String(), r.prerelease, r.notes, r.publishedAt
		if r.url != "" {
			s.URL = r.url
		}
		s.Available = c.current != nil && r.version.GreaterThan(c.current)
		break
	}
	return s
}

// githubRelease is the part of GitHub's release JSON the check reads.
type githubRelease struct {
	TagName     string     `json:"tag_name"`
	Draft       bool       `json:"draft"`
	Prerelease  bool       `json:"prerelease"`
	HTMLURL     string     `json:"html_url"`
	Body        string     `json:"body"`
	PublishedAt *time.Time `json:"published_at"`
}

// decodeReleases parses a release list, dropping drafts and tags that are
// not SemVer, newest version first.
func decodeReleases(body io.Reader) ([]release, error) {
	data, err := io.ReadAll(body)
	if err != nil {
		return nil, err
	}
	if len(data) > maxBody {
		return nil, fmt.Errorf("the release list exceeds %d MiB", maxBody>>20)
	}
	var list []githubRelease
	if err := json.Unmarshal(data, &list); err != nil {
		return nil, fmt.Errorf("not a release list: %w", err)
	}
	out := make([]release, 0, len(list))
	for _, r := range list {
		v := parseVersion(r.TagName)
		if r.Draft || v == nil {
			continue
		}
		out = append(out, release{
			version:     v,
			prerelease:  r.Prerelease || v.Prerelease() != "",
			url:         safeURL(r.HTMLURL),
			notes:       r.Body,
			publishedAt: r.PublishedAt,
		})
	}
	slices.SortStableFunc(out, func(a, b release) int { return b.version.Compare(a.version) })
	return out, nil
}

// rateLimited reports whether resp is a rate-limit refusal and how long to
// wait: Retry-After, else until X-RateLimit-Reset, within [1 min, cacheTTL].
func rateLimited(resp *http.Response, now time.Time) (time.Duration, bool) {
	if resp.StatusCode != http.StatusForbidden && resp.StatusCode != http.StatusTooManyRequests {
		return 0, false
	}
	retryAfter := resp.Header.Get("Retry-After")
	if retryAfter == "" && resp.Header.Get("X-RateLimit-Remaining") != "0" {
		return 0, false
	}
	wait := failureRetry
	if secs, err := strconv.Atoi(retryAfter); err == nil {
		wait = time.Duration(secs) * time.Second
	} else if reset, err := strconv.ParseInt(resp.Header.Get("X-RateLimit-Reset"), 10, 64); err == nil {
		wait = time.Unix(reset, 0).Sub(now)
	}
	return min(max(wait, time.Minute), cacheTTL), true
}

// serverMessage is ": <message>" from a GitHub error body, if it has one.
func serverMessage(body io.Reader) string {
	var e struct {
		Message string `json:"message"`
	}
	if json.NewDecoder(io.LimitReader(body, 64<<10)).Decode(&e) != nil || e.Message == "" {
		return ""
	}
	const maxLen = 200
	if len(e.Message) > maxLen {
		e.Message = e.Message[:maxLen] + "…"
	}
	return ": " + e.Message
}

// transportError drops the "Get <url>:" wrapper the client adds.
func transportError(err error) error {
	var uerr *url.Error
	if errors.As(err, &uerr) {
		if uerr.Timeout() {
			return fmt.Errorf("no answer within %s", requestTimeout)
		}
		return uerr.Err
	}
	return err
}

// withPerPage asks for perPage releases unless the URL already says.
func withPerPage(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	q := u.Query()
	if q.Get("per_page") == "" {
		q.Set("per_page", perPage)
		u.RawQuery = q.Encode()
	}
	return u.String()
}

// releasesPage is the web page listing the releases of a GitHub API
// endpoint (https://api.github.com/repos/<owner>/<repo>/releases); empty
// for other URLs.
func releasesPage(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.Host != "api.github.com" {
		return ""
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) != 4 || parts[0] != "repos" || parts[3] != "releases" {
		return ""
	}
	return "https://github.com/" + parts[1] + "/" + parts[2] + "/releases"
}

// safeURL keeps http(s) links only: the UI renders it as a link.
func safeURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
		return ""
	}
	return u.String()
}
