// kubebench serves the API and, in release builds, the embedded frontend
// of the VSCode-style Kubernetes workbench. Configuration comes from the
// environment (docs/design-v0.5.md §2, internal/config).
package main

import (
	"context"
	"crypto/tls"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"kubebench/internal/api"
	"kubebench/internal/config"
	"kubebench/internal/kube"
	"kubebench/internal/plugin"
	_ "kubebench/internal/plugins/all" // links the built-in backend plugins
	"kubebench/internal/store"
	"kubebench/internal/web"
)

// Build information, set with
// -ldflags "-X main.version=… -X main.commit=… -X main.date=…".
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

// shutdownGrace is how long in-flight requests may finish after SIGTERM;
// streams still open afterwards are cut.
const shutdownGrace = 10 * time.Second

// readTimeout bounds reading a whole request, body included, so a client
// that announces a body and never sends it cannot hold a connection and a
// goroutine (slowloris). It does not limit handlers: net/http lifts the
// read deadline once the body has been read to its end (at once for a
// request without one), so log streams, terminals and long operations run
// on. A handler that leaves its body unread keeps the deadline, which then
// bounds the server's read of the rest.
const readTimeout = time.Minute

func main() {
	showVersion := flag.Bool("version", false, "print the version and exit")
	flag.Usage = func() {
		out := flag.CommandLine.Output()
		fmt.Fprintln(out, "Usage: kubebench [--version] [healthcheck]")
		fmt.Fprintln(out, "\nWithout a command kubebench serves the workbench; healthcheck probes a running server.")
		fmt.Fprintln(out, "Settings are read from environment variables (PORT, KUBEBENCH_*, KUBECONFIG).")
		flag.PrintDefaults()
	}
	flag.Parse()
	if *showVersion {
		fmt.Printf("kubebench %s (%s, %s)\n", version, commit, date)
		return
	}
	if flag.NArg() > 1 || (flag.NArg() == 1 && flag.Arg(0) != "healthcheck") {
		flag.Usage()
		os.Exit(2)
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "kubebench: invalid configuration:\n%v\n", err)
		os.Exit(2)
	}
	if flag.Arg(0) == "healthcheck" {
		os.Exit(healthcheck(cfg))
	}
	if err := serve(cfg); err != nil {
		var cerr *configError
		if errors.As(err, &cerr) {
			fmt.Fprintf(os.Stderr, "kubebench: invalid configuration:\n%v\n", cerr)
			os.Exit(2)
		}
		log.Fatalf("kubebench: %v", err)
	}
}

// newHTTPServer is the HTTP server for handler on addr.
func newHTTPServer(addr string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       readTimeout,
		IdleTimeout:       2 * time.Minute,
		// No WriteTimeout: log streams (SSE) and terminals (WebSocket) stay
		// open far longer than any sane timeout.
		TLSConfig: &tls.Config{MinVersion: tls.VersionTLS12},
	}
}

// serve runs the server until SIGINT or SIGTERM, then shuts down
// gracefully.
func serve(cfg *config.Config) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	log.Printf("[kubebench] %s (%s, %s)", version, commit, date)

	// A certificate that cannot be loaded is a configuration error, found
	// before anything is created or started.
	var certificate *tls.Certificate
	if cfg.TLS() {
		cert, err := tls.LoadX509KeyPair(cfg.TLSCert, cfg.TLSKey)
		if err != nil {
			return &configError{key: "KUBEBENCH_TLS_CERT/KUBEBENCH_TLS_KEY", msg: err.Error()}
		}
		certificate = &cert
	}
	if err := prepareDirs(cfg); err != nil {
		return err
	}
	st, err := store.Open(cfg.DBPath)
	if err != nil {
		return fmt.Errorf("open database %s: %w", cfg.DBPath, err)
	}
	defer st.Close()
	log.Printf("[store] %s", cfg.DBPath)
	if err := bootstrap(cfg, st); err != nil {
		return err
	}
	if err := seedGroupMappings(cfg, st); err != nil {
		return err
	}
	logIdentityConfig(cfg)

	plugins := plugin.All()
	if err := plugin.StartAll(ctx, cfg.DataDir, plugins); err != nil {
		return err
	}

	build := api.BuildInfo{Version: version, Commit: commit, Date: date}
	apiServer := api.New(cfg, st, kube.NewManagerFor(kube.Options{Monitoring: kube.MonitoringOptions{
		Address:            cfg.Monitoring.Address,
		InsecureSkipVerify: cfg.Monitoring.InsecureSkipVerify,
	}}), build, plugins)
	spa, embedded := web.Handler()
	if embedded {
		log.Printf("[web] serving the embedded frontend")
	}
	srv := newHTTPServer(cfg.ListenAddr, apiServer.Handler(spa))
	if certificate != nil {
		srv.TLSConfig.Certificates = []tls.Certificate{*certificate}
	}

	go runJobs(ctx, cfg, st)

	served := make(chan error, 1)
	go func() {
		if cfg.TLS() {
			log.Printf("[api] listening on %s (https)", cfg.ListenAddr)
			served <- srv.ListenAndServeTLS("", "")
			return
		}
		log.Printf("[api] listening on %s", cfg.ListenAddr)
		served <- srv.ListenAndServe()
	}()

	select {
	case err := <-served:
		return err
	case <-ctx.Done():
	}
	log.Printf("[api] shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownGrace)
	defer cancel()
	// Streams and terminals first: http.Server.Shutdown would wait for the
	// streams until the deadline and leave the terminals' shells running.
	if err := apiServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("[api] terminals still open at the deadline: %v", err)
	}
	if err := srv.Shutdown(shutdownCtx); err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return srv.Close()
		}
		return err
	}
	return nil
}
