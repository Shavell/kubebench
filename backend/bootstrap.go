package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"k8s.io/client-go/tools/clientcmd"

	"kubebench/internal/config"
	"kubebench/internal/kube"
	"kubebench/internal/store"
)

// prepareDirs creates the database and data directories. The plugins
// directory is created when possible; it may also be a read-only mount.
func prepareDirs(cfg *config.Config) error {
	for _, dir := range []string{filepath.Dir(cfg.DBPath), cfg.DataDir} {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			return fmt.Errorf("create %s: %w", dir, err)
		}
	}
	if err := os.MkdirAll(cfg.PluginsDir, 0o750); err != nil {
		log.Printf("[plugins] %s: %v", cfg.PluginsDir, err)
	}
	return nil
}

// bootstrap fills a new database: the first admin, then the inventory
// seeded from the in-cluster service account and KUBECONFIG. A database
// that already has users is left alone, so a removed seed cluster stays
// removed.
func bootstrap(cfg *config.Config, st *store.Store) error {
	n, err := st.CountUsers()
	if err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	// The well-known default password must be replaced at first sign-in.
	if _, err := st.CreateUser("admin", cfg.AdminPassword, store.RoleAdmin, cfg.AdminPasswordIsDefault); err != nil {
		return fmt.Errorf("create the initial admin: %w", err)
	}
	if cfg.AdminPasswordIsDefault {
		log.Printf(`[auth] created user "admin" with the default password; it must be changed at first sign-in`)
	} else {
		log.Printf(`[auth] created user "admin" with KUBEBENCH_ADMIN_PASSWORD`)
	}

	if cfg.InCluster {
		seedCluster(st, store.Cluster{Name: "in-cluster", Source: store.SourceInCluster}, "the in-cluster service account")
	}
	if cfg.Kubeconfig != "" {
		seedKubeconfig(st, cfg.Kubeconfig)
	}
	return nil
}

// seedKubeconfig registers the KUBECONFIG file, named after its current
// context. Problems are logged: the inventory can still be filled in the UI.
func seedKubeconfig(st *store.Store, path string) {
	content, err := kube.ReadKubeconfig(path)
	if err != nil {
		log.Printf("[inventory] KUBECONFIG %s: %v", path, err)
		return
	}
	name := "kubeconfig"
	if kc, err := clientcmd.Load(content); err == nil && kc.CurrentContext != "" {
		name = kc.CurrentContext
	}
	seedCluster(st, store.Cluster{Name: name, Source: store.SourceKubeconfig, Kubeconfig: string(content)}, path)
}

func seedCluster(st *store.Store, c store.Cluster, from string) {
	if _, err := st.AddCluster(c); err != nil {
		log.Printf("[inventory] cannot add %q from %s: %v", c.Name, from, err)
		return
	}
	log.Printf("[inventory] added cluster %q from %s", c.Name, from)
}
