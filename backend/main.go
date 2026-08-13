// kubebench backend: serves the API (and, in production images, the
// embedded frontend) for the VSCode-style Kubernetes workbench.
package main

import (
	"log"
	"net/http"
	"os"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"kubebench/internal/api"
	"kubebench/internal/kube"
	"kubebench/internal/store"
	"kubebench/internal/web"
)

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func main() {
	dbPath := env("KUBEBENCH_DB", "kubebench.db")
	st, err := store.Open(dbPath)
	if err != nil {
		log.Fatalf("open db %s: %v", dbPath, err)
	}
	log.Printf("[store] %s", dbPath)

	bootstrap(st)

	server := &api.Server{Store: st, Manager: kube.NewManager()}
	mux := http.NewServeMux()
	server.Routes(mux)

	if handler, ok := web.Handler(); ok {
		mux.Handle("/", handler)
		log.Printf("[web] serving embedded frontend")
	}

	addr := ":" + env("PORT", "9876")
	log.Printf("[api] listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}

// bootstrap creates the initial admin user and seeds the cluster
// inventory from the environment on first run.
func bootstrap(st *store.Store) {
	if n, _ := st.CountUsers(); n == 0 {
		password := env("KUBEBENCH_ADMIN_PASSWORD", "admin")
		if _, err := st.CreateUser("admin", password, "admin"); err != nil {
			log.Fatalf("bootstrap admin: %v", err)
		}
		log.Printf("[auth] created initial user \"admin\" (set KUBEBENCH_ADMIN_PASSWORD to control the password)")
	}

	// In-cluster service account → register the local cluster.
	if _, err := rest.InClusterConfig(); err == nil {
		if _, err := st.ClusterByName("in-cluster"); err != nil {
			if _, err := st.AddCluster(store.Cluster{Name: "in-cluster", Source: "incluster"}); err == nil {
				log.Printf("[inventory] registered in-cluster service account")
			}
		}
	}

	// Local dev convenience: seed from KUBECONFIG.
	if path := os.Getenv("KUBECONFIG"); path != "" {
		content, err := os.ReadFile(path)
		if err != nil {
			log.Printf("[inventory] KUBECONFIG unreadable: %v", err)
			return
		}
		name := "kubeconfig"
		if cfg, err := clientcmd.Load(content); err == nil && cfg.CurrentContext != "" {
			name = cfg.CurrentContext
		}
		if _, err := st.ClusterByName(name); err != nil {
			if _, err := st.AddCluster(store.Cluster{
				Name: name, Source: "kubeconfig", Kubeconfig: string(content),
			}); err == nil {
				log.Printf("[inventory] seeded cluster %q from KUBECONFIG", name)
			}
		}
	}
}
