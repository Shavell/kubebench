package main

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"os"
	"time"

	"kubebench/internal/config"
)

// healthcheck asks the local server for GET /healthz and returns the exit
// code (0 healthy, 1 not). Docker's HEALTHCHECK runs it because the
// distroless image has neither a shell nor curl.
func healthcheck(cfg *config.Config) int {
	client := &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			// The certificate names the public host, not the loopback address
			// probed here; the check is about liveness, not identity.
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}
	resp, err := client.Get(healthURL(cfg))
	if err != nil {
		fmt.Fprintf(os.Stderr, "healthcheck: %v\n", err)
		return 1
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		fmt.Fprintf(os.Stderr, "healthcheck: %s\n", resp.Status)
		return 1
	}
	return 0
}

// healthURL targets the listen address, using loopback when the server
// listens on all interfaces.
func healthURL(cfg *config.Config) string {
	host, port, _ := net.SplitHostPort(cfg.ListenAddr) // validated by config
	if ip := net.ParseIP(host); host == "" || (ip != nil && ip.IsUnspecified()) {
		host = "127.0.0.1"
	}
	scheme := "http"
	if cfg.TLS() {
		scheme = "https"
	}
	return scheme + "://" + net.JoinHostPort(host, port) + "/healthz"
}
