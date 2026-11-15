package helm

import (
	"errors"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"sync"
	"time"

	"helm.sh/helm/v3/pkg/helmpath"
	"helm.sh/helm/v3/pkg/repo"
)

var repoNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)

// Repo is a chart repository as GET /repos lists it. Credentials are never
// part of it: Authenticated only says that some are stored.
type Repo struct {
	Name          string `json:"name"`
	URL           string `json:"url"`
	Authenticated bool   `json:"authenticated"`
	ChartCount    int    `json:"chartCount"`
	// Updated is when the index was last downloaded (RFC 3339).
	Updated string `json:"updated"`
	// Error says why the cached index cannot be read, e.g. it was never
	// downloaded; such a repository needs an update.
	Error string `json:"error,omitempty"`
}

// RepoInput is POST /repos.
type RepoInput struct {
	Name     string `json:"name"`
	URL      string `json:"url"`
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
}

// RepoUpdate is the outcome of updating one repository's index.
type RepoUpdate struct {
	Name  string `json:"name"`
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
}

func (in RepoInput) validate() error {
	if !repoNamePattern.MatchString(in.Name) {
		return badRequest("name must be 1-64 letters, digits, '.', '_' or '-', starting with a letter or digit")
	}
	u, err := url.Parse(in.URL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return badRequest("url must be an http or https URL (OCI registries need no repository: install from oci://…)")
	}
	if u.User != nil {
		// The URL is shown to every user and recorded in the audit log.
		return badRequest("put credentials in username and password, not in the URL")
	}
	if (in.Username == "") != (in.Password == "") {
		return badRequest("give both username and password, or neither")
	}
	return nil
}

// loadRepoFile reads repositories.yaml; a missing file is an empty one.
func (e *environment) loadRepoFile() (*repo.File, error) {
	f, err := repo.LoadFile(e.settings.RepositoryConfig)
	if errors.Is(err, fs.ErrNotExist) {
		return repo.NewFile(), nil
	}
	return f, err
}

func (e *environment) hasRepo(name string) bool {
	f, err := e.loadRepoFile()
	return err == nil && f.Has(name)
}

func (e *environment) indexPath(name string) string {
	return filepath.Join(e.settings.RepositoryCache, helmpath.CacheIndexFile(name))
}

// listRepos lists the added repositories with their cached indexes.
func (e *environment) listRepos() ([]Repo, error) {
	f, err := e.loadRepoFile()
	if err != nil {
		return nil, err
	}
	out := make([]Repo, 0, len(f.Repositories))
	for _, entry := range f.Repositories {
		r := Repo{Name: entry.Name, URL: entry.URL, Authenticated: entry.Username != ""}
		if idx, updated, err := e.indexes.load(e.indexPath(entry.Name)); err != nil {
			r.Error = indexError(err)
		} else {
			r.ChartCount = len(idx.Entries)
			r.Updated = timestamp(updated)
		}
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func indexError(err error) string {
	if errors.Is(err, fs.ErrNotExist) {
		return "the index has not been downloaded; update the repository"
	}
	return err.Error()
}

// addRepo downloads the repository's index, which proves the URL and
// credentials work, then records the repository.
func (e *environment) addRepo(in RepoInput) (*Repo, error) {
	if err := in.validate(); err != nil {
		return nil, err
	}
	e.repoMu.Lock()
	defer e.repoMu.Unlock()
	f, err := e.loadRepoFile()
	if err != nil {
		return nil, err
	}
	if f.Has(in.Name) {
		return nil, conflict("repository %q already exists", in.Name)
	}
	entry := &repo.Entry{Name: in.Name, URL: in.URL, Username: in.Username, Password: in.Password}
	if err := e.downloadIndex(entry); err != nil {
		return nil, badRequest("could not download the index of %s: %v", in.URL, err)
	}
	f.Update(entry)
	if err := f.WriteFile(e.settings.RepositoryConfig, 0o600); err != nil {
		return nil, err
	}
	repos, err := e.listRepos()
	if err != nil {
		return nil, err
	}
	for i := range repos {
		if repos[i].Name == in.Name {
			return &repos[i], nil
		}
	}
	return nil, errors.New("the added repository is not listed")
}

// removeRepo forgets a repository and its cached index.
func (e *environment) removeRepo(name string) error {
	e.repoMu.Lock()
	defer e.repoMu.Unlock()
	f, err := e.loadRepoFile()
	if err != nil {
		return err
	}
	if !f.Remove(name) {
		return notFound("repository %q does not exist", name)
	}
	if err := f.WriteFile(e.settings.RepositoryConfig, 0o600); err != nil {
		return err
	}
	for _, file := range []string{helmpath.CacheIndexFile(name), helmpath.CacheChartsFile(name)} {
		if err := os.Remove(filepath.Join(e.settings.RepositoryCache, file)); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
	}
	e.indexes.forget(e.indexPath(name))
	return nil
}

// updateRepos downloads every repository's index again, like helm repo
// update; one failing repository does not stop the others.
func (e *environment) updateRepos() ([]RepoUpdate, error) {
	e.repoMu.Lock()
	defer e.repoMu.Unlock()
	f, err := e.loadRepoFile()
	if err != nil {
		return nil, err
	}
	out := make([]RepoUpdate, 0, len(f.Repositories))
	for _, entry := range f.Repositories {
		u := RepoUpdate{Name: entry.Name, OK: true}
		if err := e.downloadIndex(entry); err != nil {
			u.OK, u.Error = false, err.Error()
		}
		out = append(out, u)
	}
	return out, nil
}

func (e *environment) downloadIndex(entry *repo.Entry) error {
	r, err := repo.NewChartRepository(entry, e.getters)
	if err != nil {
		return err
	}
	r.CachePath = e.settings.RepositoryCache
	_, err = r.DownloadIndexFile()
	return err
}

// indexCache keeps parsed repository indexes, which can be megabytes of
// YAML, until their file changes.
type indexCache struct {
	mu      sync.Mutex
	entries map[string]cachedIndex
}

type cachedIndex struct {
	modTime time.Time
	size    int64
	index   *repo.IndexFile
}

// load returns the index at path and when it was written.
func (c *indexCache) load(path string) (*repo.IndexFile, time.Time, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, time.Time{}, err
	}
	c.mu.Lock()
	cached, ok := c.entries[path]
	c.mu.Unlock()
	if ok && cached.modTime.Equal(info.ModTime()) && cached.size == info.Size() {
		return cached.index, info.ModTime(), nil
	}
	idx, err := repo.LoadIndexFile(path)
	if err != nil {
		return nil, time.Time{}, err
	}
	c.mu.Lock()
	if c.entries == nil {
		c.entries = map[string]cachedIndex{}
	}
	c.entries[path] = cachedIndex{modTime: info.ModTime(), size: info.Size(), index: idx}
	c.mu.Unlock()
	return idx, info.ModTime(), nil
}

func (c *indexCache) forget(path string) {
	c.mu.Lock()
	delete(c.entries, path)
	c.mu.Unlock()
}
