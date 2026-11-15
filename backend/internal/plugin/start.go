package plugin

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

// Host is what KubeBench hands a plugin at start-up.
type Host struct {
	// DataDir is the plugin's own writable directory,
	// $KUBEBENCH_DATA_DIR/<plugin id>. It exists (mode 0700) when Start
	// runs and keeps its content across restarts.
	DataDir string
}

// Starter is implemented by plugins that prepare state before serving,
// e.g. files under their data directory or a background clean-up. Start
// runs once, before the server accepts requests; ctx ends when the server
// shuts down, and background work started from Start must stop with it.
// An error stops KubeBench from starting.
type Starter interface {
	Start(ctx context.Context, host Host) error
}

// StartAll starts every plugin in plugins that implements Starter, after
// creating its data directory under dataDir.
func StartAll(ctx context.Context, dataDir string, plugins []Plugin) error {
	for _, p := range plugins {
		s, ok := p.(Starter)
		if !ok {
			continue
		}
		id := p.Manifest().ID
		dir := filepath.Join(dataDir, id)
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return fmt.Errorf("plugin %s: create %s: %w", id, dir, err)
		}
		if err := s.Start(ctx, Host{DataDir: dir}); err != nil {
			return fmt.Errorf("plugin %s: %w", id, err)
		}
	}
	return nil
}
