package kube

import (
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"kubebench/internal/store"
)

func testJWT(exp time.Time, sub string) string {
	enc := base64.RawURLEncoding.EncodeToString
	claims, _ := json.Marshal(map[string]any{"exp": exp.Unix(), "sub": sub})
	return enc([]byte(`{"alg":"RS256"}`)) + "." + enc(claims) + ".sig"
}

// fakeIssuer is an OIDC provider over TLS that answers refresh grants with
// a new ID token for the subject the refresh token names and, when
// rotate is set, a new refresh token.
type fakeIssuer struct {
	srv    *httptest.Server
	mu     sync.Mutex
	grants []string // refresh tokens presented
	rotate bool
}

func newFakeIssuer(t *testing.T) *fakeIssuer {
	f := &fakeIssuer{}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `{"issuer":%q,"token_endpoint":%q}`, f.srv.URL, f.srv.URL+"/token")
	})
	mux.HandleFunc("POST /token", func(w http.ResponseWriter, r *http.Request) {
		rt := r.FormValue("refresh_token")
		f.mu.Lock()
		f.grants = append(f.grants, rt)
		rotate := f.rotate
		f.mu.Unlock()
		sub, _, _ := strings.Cut(rt, "#")
		body := map[string]any{"access_token": "at", "token_type": "Bearer", "expires_in": 3600,
			"id_token": testJWT(time.Now().Add(time.Hour), sub)}
		if rotate {
			body["refresh_token"] = fmt.Sprintf("%s#%d", sub, len(f.grants))
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(body)
	})
	f.srv = httptest.NewTLSServer(mux)
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fakeIssuer) caData() string {
	p := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: f.srv.Certificate().Raw})
	return base64.StdEncoding.EncodeToString(p)
}

func (f *fakeIssuer) presented() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.grants...)
}

// subjectAPIServer answers every request with 200 and records the subject
// of the bearer ID token.
func subjectAPIServer(t *testing.T) (*httptest.Server, func() []string) {
	var mu sync.Mutex
	var subs []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tok := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		sub := "?"
		if parts := strings.Split(tok, "."); len(parts) == 3 {
			payload, _ := base64.RawURLEncoding.DecodeString(parts[1])
			var c struct{ Sub string }
			_ = json.Unmarshal(payload, &c)
			sub = c.Sub
		}
		mu.Lock()
		subs = append(subs, sub)
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"kind":"NamespaceList","apiVersion":"v1","items":[]}`)
	}))
	t.Cleanup(srv.Close)
	return srv, func() []string { mu.Lock(); defer mu.Unlock(); return append([]string(nil), subs...) }
}

func oidcKubeconfig(server, issuer, caData, idToken, refreshToken string) string {
	return fmt.Sprintf(`apiVersion: v1
kind: Config
clusters: [{name: k, cluster: {server: %q}}]
contexts: [{name: oidc, context: {cluster: k, user: u}}]
current-context: oidc
users:
- name: u
  user:
    auth-provider:
      name: oidc
      config:
        client-id: kubernetes
        idp-issuer-url: %q
        idp-certificate-authority-data: %s
        id-token: %q
        refresh-token: %q
`, server, issuer, caData, idToken, refreshToken)
}

func listNamespaces(t *testing.T, c store.Cluster) {
	t.Helper()
	config, err := restConfigFor(&c)
	if err != nil {
		t.Fatal(err)
	}
	cs, err := kubernetes.NewForConfig(config)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := cs.CoreV1().Namespaces().List(t.Context(), metav1.ListOptions{}); err != nil {
		t.Fatal(err)
	}
}

func TestOIDCAuthProvider(t *testing.T) {
	issuer := newFakeIssuer(t)
	issuer.rotate = true
	api, subjects := subjectAPIServer(t)

	// A valid ID token is used as it is; no refresh.
	valid := store.Cluster{ID: 9001, Name: "valid", Source: store.SourceKubeconfig,
		Kubeconfig: oidcKubeconfig(api.URL, issuer.srv.URL, issuer.caData(), testJWT(time.Now().Add(time.Hour), "valid"), "valid#0")}
	listNamespaces(t, valid)

	// An expired one is refreshed at the issuer; two clusters on the same
	// API server and issuer keep their own identities.
	expired := testJWT(time.Now().Add(-time.Hour), "old")
	alice := store.Cluster{ID: 9002, Name: "alice", Source: store.SourceKubeconfig,
		Kubeconfig: oidcKubeconfig(api.URL, issuer.srv.URL, issuer.caData(), expired, "alice#0")}
	bob := store.Cluster{ID: 9003, Name: "bob", Source: store.SourceKubeconfig,
		Kubeconfig: oidcKubeconfig(api.URL, issuer.srv.URL, issuer.caData(), expired, "bob#0")}
	listNamespaces(t, alice)
	listNamespaces(t, bob)
	listNamespaces(t, alice)
	if got, want := strings.Join(subjects(), ","), "valid,alice,bob,alice"; got != want {
		t.Fatalf("subjects = %s, want %s", got, want)
	}
	if got := strings.Join(issuer.presented(), ","); got != "alice#0,bob#0" {
		t.Fatalf("refresh grants = %s", got)
	}

	// A rebuilt runtime of the same cluster keeps the refreshed tokens
	// (the stored refresh token is spent once the issuer rotated it) ...
	alice.Name = "alice-renamed"
	src := oidcSources.byCluster[alice.ID]
	src.mu.Lock()
	src.idToken = expired // force another refresh
	src.mu.Unlock()
	listNamespaces(t, alice)
	if got := issuer.presented(); got[len(got)-1] != "alice#1" {
		t.Fatalf("the rebuilt runtime refreshed with %s, want the rotated alice#1", got[len(got)-1])
	}
	// ... while a changed kubeconfig starts over with its own tokens.
	alice.Kubeconfig = oidcKubeconfig(api.URL, issuer.srv.URL, issuer.caData(), expired, "carol#0")
	listNamespaces(t, alice)
	if got := subjects(); got[len(got)-1] != "carol" {
		t.Fatalf("after a new kubeconfig the subject is %s", got[len(got)-1])
	}
}

func TestOIDCAuthProviderErrors(t *testing.T) {
	issuer := newFakeIssuer(t)
	api, _ := subjectAPIServer(t)
	expired := testJWT(time.Now().Add(-time.Hour), "x")

	c := store.Cluster{ID: 9101, Name: "no-refresh", Source: store.SourceKubeconfig,
		Kubeconfig: oidcKubeconfig(api.URL, issuer.srv.URL, issuer.caData(), expired, "")}
	config, err := restConfigFor(&c)
	if err != nil {
		t.Fatal(err)
	}
	cs, _ := kubernetes.NewForConfig(config)
	if _, err := cs.CoreV1().Namespaces().List(t.Context(), metav1.ListOptions{}); err == nil ||
		!strings.Contains(err.Error(), "no refresh-token") {
		t.Fatalf("expired token without refresh token: %v", err)
	}

	// Without the issuer's CA its certificate is not trusted.
	c = store.Cluster{ID: 9102, Name: "wrong-ca", Source: store.SourceKubeconfig,
		Kubeconfig: strings.Replace(oidcKubeconfig(api.URL, issuer.srv.URL, issuer.caData(), expired, "x#0"),
			"        idp-certificate-authority-data: "+issuer.caData()+"\n", "", 1)}
	config, err = restConfigFor(&c)
	if err != nil {
		t.Fatal(err)
	}
	cs, _ = kubernetes.NewForConfig(config)
	if _, err := cs.CoreV1().Namespaces().List(t.Context(), metav1.ListOptions{}); err == nil ||
		!strings.Contains(err.Error(), "certificate") {
		t.Fatalf("untrusted issuer: %v", err)
	}

	// Other auth providers are refused with a reason.
	c.Kubeconfig = strings.Replace(c.Kubeconfig, "name: oidc\n", "name: gcp\n", 1)
	if _, err := restConfigFor(&c); err == nil || !strings.Contains(err.Error(), `auth provider "gcp"`) {
		t.Fatalf("gcp provider: %v", err)
	}
}

func TestJWTExpiry(t *testing.T) {
	at := time.Unix(1_900_000_000, 0)
	if got, ok := jwtExpiry(testJWT(at, "s")); !ok || !got.Equal(at) {
		t.Fatalf("jwtExpiry = %v, %v", got, ok)
	}
	for _, tok := range []string{"", "a.b", "eyJhbGciOiJSUzI1NiJ9.e30.sig", "a.!!.c"} {
		if _, ok := jwtExpiry(tok); ok {
			t.Errorf("jwtExpiry(%q) found an expiry", tok)
		}
	}
}
