package main

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"kubebench/internal/config"
)

// healthcheck asks the local server for GET /healthz and returns the exit
// code (0 healthy, 1 not). Docker's HEALTHCHECK runs it because the
// distroless image has neither a shell nor curl.
func healthcheck(cfg *config.Config) int {
	transport := &http.Transport{}
	if cfg.TLS() {
		tlsConfig, err := healthTLSConfig(cfg.TLSCert)
		if err != nil {
			fmt.Fprintf(os.Stderr, "healthcheck: %v\n", err)
			return 1
		}
		transport.TLSClientConfig = tlsConfig
	}
	client := &http.Client{Timeout: 5 * time.Second, Transport: transport}
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

// healthTLSConfig verifies the server's certificate against the one it is
// configured with (KUBEBENCH_TLS_CERT): that certificate is the only
// trusted root, so a self-signed certificate works as well as one a CA
// issued, and nothing else is accepted. The certificate names the public
// host rather than the loopback address probed, so the name verified is
// one the certificate itself holds.
func healthTLSConfig(certFile string) (*tls.Config, error) {
	data, err := os.ReadFile(certFile)
	if err != nil {
		return nil, err
	}
	var leaf *x509.Certificate
	for block, rest := pem.Decode(data); block != nil && leaf == nil; block, rest = pem.Decode(rest) {
		if block.Type != "CERTIFICATE" {
			continue
		}
		if leaf, err = x509.ParseCertificate(block.Bytes); err != nil {
			return nil, fmt.Errorf("%s: %w", certFile, err)
		}
	}
	if leaf == nil {
		return nil, fmt.Errorf("%s: no PEM certificate", certFile)
	}
	name, err := certificateName(leaf)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", certFile, err)
	}
	roots := x509.NewCertPool()
	roots.AddCert(leaf)
	return &tls.Config{RootCAs: roots, ServerName: name, MinVersion: tls.VersionTLS12}, nil
}

// certificateName is a host name the certificate is valid for: its first
// DNS name (a wildcard stands for one label of our choosing), else its
// first IP address.
func certificateName(cert *x509.Certificate) (string, error) {
	if len(cert.DNSNames) > 0 {
		name := cert.DNSNames[0]
		if rest, ok := strings.CutPrefix(name, "*."); ok {
			name = "healthcheck." + rest
		}
		return name, nil
	}
	if len(cert.IPAddresses) > 0 {
		return cert.IPAddresses[0].String(), nil
	}
	return "", errors.New("the certificate names no host (no subject alternative name)")
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
