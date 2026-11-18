package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"kubebench/internal/plugin"
	"kubebench/internal/reason"
)

// External plugins are UI bundles an admin installs as
// $KUBEBENCH_PLUGINS_DIR/<id>/plugin.json plus their files (§6.8).
const (
	pluginManifestFile    = "plugin.json"
	maxPluginManifest     = 64 << 10
	defaultPluginCategory = "tools"
)

var (
	// entryPattern keeps the entry a plain .js file in the plugin's directory.
	entryPattern    = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*\.js$`)
	categoryPattern = regexp.MustCompile(`^[a-z][a-z-]{0,31}$`)
)

// pluginFileTypes are the files served from a plugin directory; anything
// else (HTML in particular) is not served under the app's origin.
var pluginFileTypes = map[string]string{
	".js":   "text/javascript; charset=utf-8",
	".mjs":  "text/javascript; charset=utf-8",
	".css":  "text/css; charset=utf-8",
	".json": "application/json",
	".map":  "application/json",
	".svg":  "image/svg+xml",
	".png":  "image/png",
}

type externalPlugin struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Version     string `json:"version"`
	Description string `json:"description"`
	Category    string `json:"category"`
	Entry       string `json:"entry"`
}

// externalPlugins lists the valid plugins in the plugins directory;
// invalid ones are logged and skipped.
func (s *Server) externalPlugins() []externalPlugin {
	entries, err := os.ReadDir(s.cfg.PluginsDir)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			log.Printf("[plugins] %v", err)
		}
		return nil
	}
	var out []externalPlugin
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		p, err := s.loadExternalPlugin(e.Name())
		if err != nil {
			log.Printf("[plugins] skipping %s: %v", filepath.Join(s.cfg.PluginsDir, e.Name()), err)
			continue
		}
		out = append(out, *p)
	}
	return out
}

// loadExternalPlugin reads and validates <plugins dir>/<id>/plugin.json.
func (s *Server) loadExternalPlugin(id string) (*externalPlugin, error) {
	if !plugin.ValidID(id) {
		return nil, fmt.Errorf("%q is not a valid plugin id", id)
	}
	for _, builtin := range s.plugins {
		if builtin.Manifest().ID == id {
			return nil, fmt.Errorf("the id %q belongs to a built-in plugin", id)
		}
	}
	root, err := os.OpenRoot(filepath.Join(s.cfg.PluginsDir, id))
	if err != nil {
		return nil, err
	}
	defer root.Close()
	f, err := root.Open(pluginManifestFile)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, maxPluginManifest+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxPluginManifest {
		return nil, fmt.Errorf("%s is larger than %d bytes", pluginManifestFile, maxPluginManifest)
	}
	var p externalPlugin
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("%s: %w", pluginManifestFile, err)
	}
	if p.ID != id {
		return nil, fmt.Errorf("%s declares id %q but lives in directory %q", pluginManifestFile, p.ID, id)
	}
	if !entryPattern.MatchString(p.Entry) {
		return nil, fmt.Errorf("entry %q must be a .js file name in the plugin directory", p.Entry)
	}
	if info, err := root.Stat(p.Entry); err != nil || !info.Mode().IsRegular() {
		return nil, fmt.Errorf("entry %q is missing", p.Entry)
	}
	if p.Name == "" {
		p.Name = id
	}
	if !categoryPattern.MatchString(p.Category) {
		p.Category = defaultPluginCategory
	}
	return &p, nil
}

// handlePluginFile serves GET /plugins/{id}/{file...} from an enabled
// external plugin. os.Root keeps every lookup, symlinks included, inside
// the plugin's directory; directories are never listed.
func (s *Server) handlePluginFile(w http.ResponseWriter, r *http.Request, _ *call) {
	id, name := r.PathValue("id"), r.PathValue("file")
	notFound := fail(http.StatusNotFound, plugin.CodeNotFound, reasonNoPluginFile, reason.Params{"path": r.URL.Path})
	contentType, served := pluginFileTypes[strings.ToLower(path.Ext(name))]
	if !served || !fs.ValidPath(name) {
		writeErr(w, notFound)
		return
	}
	if _, err := s.loadExternalPlugin(id); err != nil {
		writeErr(w, notFound)
		return
	}
	if enabled, _, err := s.store.PluginEnabled(id); err != nil {
		writeErr(w, err)
		return
	} else if !enabled {
		writeErr(w, fail(http.StatusNotFound, plugin.CodePluginDisabled, reasonPluginDisabled, reason.Params{"id": id}))
		return
	}
	root, err := os.OpenRoot(filepath.Join(s.cfg.PluginsDir, id))
	if err != nil {
		writeErr(w, notFound)
		return
	}
	defer root.Close()
	f, err := root.Open(name)
	if err != nil {
		writeErr(w, notFound)
		return
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() {
		writeErr(w, notFound)
		return
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "no-cache") // plugins are updated in place
	http.ServeContent(w, r, "", info.ModTime(), f)
}
