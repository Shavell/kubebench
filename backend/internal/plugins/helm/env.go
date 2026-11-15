package helm

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/getter"
	"helm.sh/helm/v3/pkg/registry"
)

// environment is the Helm client state KubeBench keeps under
// $KUBEBENCH_DATA_DIR/helm, shared by every cluster:
//
//	repositories.yaml   chart repositories (credentials included, mode 0600)
//	cache/              repository indexes and downloaded charts
//	registry/config.json OCI registry credentials
//	uploads/<id>.tgz    uploaded charts, removed after uploadTTL
//
// It never reads the server account's own Helm configuration (~/.config/
// helm, HELM_* variables) and never runs Helm plugins: the getters are the
// built-in http(s) and oci ones only.
type environment struct {
	settings *cli.EnvSettings
	getters  getter.Providers
	registry *registry.Client
	uploads  string

	// repoMu serializes changes to repositories.yaml and the index cache.
	repoMu  sync.Mutex
	indexes indexCache
}

func newEnvironment(dir string) (*environment, error) {
	settings := &cli.EnvSettings{
		RepositoryConfig: filepath.Join(dir, "repositories.yaml"),
		RepositoryCache:  filepath.Join(dir, "cache"),
		RegistryConfig:   filepath.Join(dir, "registry", "config.json"),
	}
	uploads := filepath.Join(dir, "uploads")
	for _, d := range []string{settings.RepositoryCache, filepath.Dir(settings.RegistryConfig), uploads} {
		if err := os.MkdirAll(d, 0o700); err != nil {
			return nil, fmt.Errorf("create %s: %w", d, err)
		}
	}
	reg, err := registry.NewClient(
		registry.ClientOptCredentialsFile(settings.RegistryConfig),
		registry.ClientOptWriter(io.Discard),
		registry.ClientOptEnableCache(true),
	)
	if err != nil {
		return nil, fmt.Errorf("OCI registry client: %w", err)
	}
	return &environment{
		settings: settings,
		getters:  getter.Getters(),
		registry: reg,
		uploads:  uploads,
	}, nil
}
