package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"kubebench/internal/config"
)

type testCert struct {
	cert *x509.Certificate
	key  *ecdsa.PrivateKey
	der  []byte
}

// newTestCert makes a certificate for names (DNS names or IP addresses),
// signed by parent, or self-signed when parent is nil.
func newTestCert(t *testing.T, parent *testCert, isCA bool, names ...string) *testCert {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	serial, _ := rand.Int(rand.Reader, big.NewInt(1<<62))
	tmpl := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: "kubebench test"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA:                  isCA,
	}
	for _, n := range names {
		if ip := net.ParseIP(n); ip != nil {
			tmpl.IPAddresses = append(tmpl.IPAddresses, ip)
		} else {
			tmpl.DNSNames = append(tmpl.DNSNames, n)
		}
	}
	signer, signerKey := tmpl, key
	if parent != nil {
		signer, signerKey = parent.cert, parent.key
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, signer, &key.PublicKey, signerKey)
	if err != nil {
		t.Fatal(err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	return &testCert{cert: cert, key: key, der: der}
}

func pemOf(chain ...*testCert) string {
	var b strings.Builder
	for _, c := range chain {
		_ = pem.Encode(&b, &pem.Block{Type: "CERTIFICATE", Bytes: c.der})
	}
	return b.String()
}

// tlsHealthServer serves /healthz over TLS with chain (leaf first) and
// returns the configuration of a KubeBench listening there whose
// KUBEBENCH_TLS_CERT holds certPEM.
func tlsHealthServer(t *testing.T, certPEM string, chain ...*testCert) *config.Config {
	t.Helper()
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	served := tls.Certificate{PrivateKey: chain[0].key}
	for _, c := range chain {
		served.Certificate = append(served.Certificate, c.der)
	}
	srv.TLS = &tls.Config{Certificates: []tls.Certificate{served}}
	srv.StartTLS()
	t.Cleanup(srv.Close)
	_, port, _ := net.SplitHostPort(srv.Listener.Addr().String())
	certFile := filepath.Join(t.TempDir(), "tls.crt")
	writeTestFile(t, certFile, certPEM)
	return &config.Config{ListenAddr: ":" + port, TLSCert: certFile, TLSKey: "unused"}
}

// Over TLS the healthcheck verifies the server's certificate against the
// configured one instead of skipping verification.
func TestHealthcheckVerifiesTheConfiguredCertificate(t *testing.T) {
	selfSigned := newTestCert(t, nil, false, "kubebench.example.com")
	wildcard := newTestCert(t, nil, false, "*.example.com")
	ipOnly := newTestCert(t, nil, false, "203.0.113.7")
	ca := newTestCert(t, nil, true)
	issued := newTestCert(t, ca, false, "kubebench.example.com")
	other := newTestCert(t, nil, false, "kubebench.example.com")
	noName := newTestCert(t, nil, false)

	for _, tc := range []struct {
		name       string
		configured string
		served     []*testCert
		want       int
	}{
		{"self-signed", pemOf(selfSigned), []*testCert{selfSigned}, 0},
		{"wildcard", pemOf(wildcard), []*testCert{wildcard}, 0},
		{"IP address only", pemOf(ipOnly), []*testCert{ipOnly}, 0},
		{"issued by a CA, with its chain", pemOf(issued, ca), []*testCert{issued, ca}, 0},
		{"another certificate for the same name", pemOf(other), []*testCert{selfSigned}, 1},
		{"a certificate issued by the configured one's CA", pemOf(issued, ca), []*testCert{newTestCert(t, ca, false, "kubebench.example.com"), ca}, 1},
		{"no subject alternative name", pemOf(noName), []*testCert{noName}, 1},
		{"not PEM", "garbage", []*testCert{selfSigned}, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := tlsHealthServer(t, tc.configured, tc.served...)
			if got := healthcheck(cfg); got != tc.want {
				t.Fatalf("healthcheck = %d, want %d", got, tc.want)
			}
		})
	}

	cfg := tlsHealthServer(t, pemOf(selfSigned), selfSigned)
	cfg.TLSCert = filepath.Join(t.TempDir(), "missing.crt")
	if got := healthcheck(cfg); got != 1 {
		t.Fatalf("missing certificate file: healthcheck = %d, want 1", got)
	}
}
