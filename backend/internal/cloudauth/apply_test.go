package cloudauth

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"golang.org/x/oauth2"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

// clock is a settable time source.
type clock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *clock) now() time.Time { c.mu.Lock(); defer c.mu.Unlock(); return c.t }
func (c *clock) advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

// fakeGoogle hands out numbered tokens valid for an hour.
type fakeGoogle struct {
	clock *clock
	mu    sync.Mutex
	calls int
}

func (f *fakeGoogle) Token() (*oauth2.Token, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	return &oauth2.Token{AccessToken: "ya29.token-" + string(rune('0'+f.calls)), Expiry: f.clock.now().Add(time.Hour)}, nil
}

// fakeAzure records the scopes it was asked for.
type fakeAzure struct {
	clock  *clock
	scopes []string
}

func (f *fakeAzure) GetToken(_ context.Context, opts policy.TokenRequestOptions) (azcore.AccessToken, error) {
	f.scopes = append(f.scopes, opts.Scopes...)
	return azcore.AccessToken{Token: "eyJ.aks", ExpiresOn: f.clock.now().Add(time.Hour)}, nil
}

// apiServer answers with the Authorization header it received; a request
// to /unauthorized gets 401.
func apiServer(t *testing.T) *httptest.Server {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/unauthorized" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_, _ = w.Write([]byte(r.Header.Get("Authorization")))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func get(t *testing.T, client *http.Client, url string) string {
	t.Helper()
	resp, err := client.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var b strings.Builder
	buf := make([]byte, 4096)
	for {
		n, err := resp.Body.Read(buf)
		b.Write(buf[:n])
		if err != nil {
			break
		}
	}
	return b.String()
}

func clientFor(t *testing.T, s Sources, user *clientcmdapi.AuthInfo, creds *Credentials) *http.Client {
	t.Helper()
	wrap, err := s.Apply(user, creds)
	if err != nil {
		t.Fatal(err)
	}
	if wrap == nil {
		t.Fatal("the stanza was not recognised")
	}
	if user.Exec != nil {
		t.Fatal("the exec stanza was left in place: client-go would run it")
	}
	return &http.Client{Transport: wrap(http.DefaultTransport)}
}

func TestApplyEKS(t *testing.T) {
	clk := &clock{t: signedAt}
	var gotSpec *ExecSpec
	var gotCreds *Credentials
	retrieves := 0
	s := Sources{
		AWS: func(_ context.Context, spec *ExecSpec, creds *Credentials) (aws.CredentialsProvider, string, error) {
			gotSpec, gotCreds = spec, creds
			static := credentials.NewStaticCredentialsProvider(creds.AccessKeyID, creds.SecretAccessKey, "")
			return aws.CredentialsProviderFunc(func(ctx context.Context) (aws.Credentials, error) {
				retrieves++
				return static.Retrieve(ctx)
			}), "eu-west-1", nil
		},
		Now: clk.now,
	}
	user := &clientcmdapi.AuthInfo{Exec: stanza("aws", []string{"eks", "get-token", "--cluster-name", "prod"})}
	creds := &Credentials{Provider: AWS, AccessKeyID: "AKIDEXAMPLE", SecretAccessKey: "secret"}
	client := clientFor(t, s, user, creds)
	srv := apiServer(t)

	auth := get(t, client, srv.URL)
	token, ok := strings.CutPrefix(auth, "Bearer ")
	if !ok {
		t.Fatalf("Authorization = %q", auth)
	}
	u := decodeEKSToken(t, token)
	if u.Host != "sts.eu-west-1.amazonaws.com" || u.Query().Get("X-Amz-Date") != "20261115T120000Z" {
		t.Fatalf("token url = %s", u)
	}
	if gotSpec.ClusterID != "prod" || gotCreds != creds {
		t.Fatalf("AWS got %+v, %+v", gotSpec, gotCreds)
	}

	// Cached until a minute before the 15-minute lifetime ends.
	clk.advance(13*time.Minute + 59*time.Second)
	if get(t, client, srv.URL) != auth || retrieves != 1 {
		t.Fatalf("token not reused (%d retrieves)", retrieves)
	}
	clk.advance(time.Second)
	if next := get(t, client, srv.URL); next == auth || retrieves != 2 {
		t.Fatalf("token not refreshed (%d retrieves)", retrieves)
	}
}

func TestApplyGKE(t *testing.T) {
	clk := &clock{t: signedAt}
	fake := &fakeGoogle{clock: clk}
	var gotCreds *Credentials
	s := Sources{
		Google: func(creds *Credentials) (oauth2.TokenSource, error) { gotCreds = creds; return fake, nil },
		Now:    clk.now,
	}
	user := &clientcmdapi.AuthInfo{Exec: stanza("gke-gcloud-auth-plugin", nil)}
	client := clientFor(t, s, user, nil)
	srv := apiServer(t)

	if got := get(t, client, srv.URL); got != "Bearer ya29.token-1" {
		t.Fatalf("Authorization = %q", got)
	}
	if got := get(t, client, srv.URL); got != "Bearer ya29.token-1" || fake.calls != 1 || gotCreds != nil {
		t.Fatalf("second request: %q after %d fetches", got, fake.calls)
	}
	// A 401 drops the token the request used; the next request fetches.
	clk.advance(time.Second)
	if _, err := client.Get(srv.URL + "/unauthorized"); err != nil {
		t.Fatal(err)
	}
	if got := get(t, client, srv.URL); got != "Bearer ya29.token-2" {
		t.Fatalf("after a 401: %q", got)
	}
}

func TestApplyAKS(t *testing.T) {
	clk := &clock{t: signedAt}
	fake := &fakeAzure{clock: clk}
	var gotSpec *ExecSpec
	s := Sources{
		Azure: func(spec *ExecSpec, _ *Credentials) (azcore.TokenCredential, error) { gotSpec = spec; return fake, nil },
		Now:   clk.now,
	}
	user := &clientcmdapi.AuthInfo{Exec: stanza("kubelogin", []string{"get-token", "--login", "msi", "--server-id", "custom-server-app"})}
	client := clientFor(t, s, user, nil)
	if got := get(t, client, apiServer(t).URL); got != "Bearer eyJ.aks" {
		t.Fatalf("Authorization = %q", got)
	}
	if len(fake.scopes) != 1 || fake.scopes[0] != "custom-server-app/.default" || gotSpec.Login != LoginMSI {
		t.Fatalf("scopes %v, spec %+v", fake.scopes, gotSpec)
	}
}

// A failing identity lookup fails the request and is retried by the next.
func TestApplyRetriesFailedLookups(t *testing.T) {
	attempts := 0
	s := Sources{
		Google: func(*Credentials) (oauth2.TokenSource, error) {
			attempts++
			return nil, errors.New("no metadata server")
		},
		Now: time.Now,
	}
	client := clientFor(t, s, &clientcmdapi.AuthInfo{Exec: stanza("gke-gcloud-auth-plugin", nil)}, nil)
	srv := apiServer(t)
	for range 2 {
		if _, err := client.Get(srv.URL); err == nil || !strings.Contains(err.Error(), "no metadata server") {
			t.Fatalf("err = %v", err)
		}
	}
	if attempts != 2 {
		t.Fatalf("attempts = %d", attempts)
	}
}

func TestApplyLeavesOtherUsersAlone(t *testing.T) {
	exec := stanza("/usr/bin/vault-k8s-login", nil)
	user := &clientcmdapi.AuthInfo{Exec: exec}
	wrap, err := Default.Apply(user, nil)
	if wrap != nil || err != nil || user.Exec != exec {
		t.Fatalf("Apply = %v, %v; exec %v", wrap != nil, err, user.Exec)
	}
	if wrap, err := Default.Apply(&clientcmdapi.AuthInfo{Token: "t"}, nil); wrap != nil || err != nil {
		t.Fatal("a token user was changed")
	}
	if wrap, err := Default.Apply(nil, nil); wrap != nil || err != nil {
		t.Fatal("a missing user was changed")
	}
}

func TestCheckCredentials(t *testing.T) {
	eks := &clientcmdapi.AuthInfo{Exec: stanza("aws", []string{"eks", "get-token", "--cluster-name", "x"})}
	if err := CheckCredentials(eks, AWS); err != nil {
		t.Fatal(err)
	}
	if err := CheckCredentials(eks, GCP); err == nil || !strings.Contains(err.Error(), "signs in to aws, not gcp") {
		t.Fatalf("mismatch: %v", err)
	}
	if err := CheckCredentials(&clientcmdapi.AuthInfo{Token: "t"}, Azure); err == nil {
		t.Fatal("credentials accepted for a token user")
	}
	if _, err := Default.Apply(eks, &Credentials{Provider: Azure}); err == nil {
		t.Fatal("Apply took credentials of another cloud")
	}
	if _, err := Default.Apply(nil, &Credentials{Provider: AWS}); err == nil {
		t.Fatal("Apply took credentials without a user")
	}
}

func TestCachedSourceWithoutExpiry(t *testing.T) {
	calls := 0
	src := newCachedSource(tokenFunc(func() (*oauth2.Token, error) {
		calls++
		return &oauth2.Token{AccessToken: "static"}, nil
	}), time.Now)
	for range 3 {
		if tok, err := src.Token(); err != nil || tok.AccessToken != "static" {
			t.Fatal(tok, err)
		}
	}
	if calls != 1 {
		t.Fatalf("calls = %d", calls)
	}
	src.ResetTokenOlderThan(time.Now().Add(time.Second))
	if _, _ = src.Token(); calls != 2 {
		t.Fatalf("calls = %d", calls)
	}
}

type tokenFunc func() (*oauth2.Token, error)

func (f tokenFunc) Token() (*oauth2.Token, error) { return f() }

func TestAzureCredential(t *testing.T) {
	for _, env := range []string{"", "AzurePublicCloud", "AzureChinaCloud", "azureusgovernmentcloud"} {
		if _, err := azureCloud(env); err != nil {
			t.Errorf("azureCloud(%q): %v", env, err)
		}
	}
	if _, err := azureCredential(&ExecSpec{Environment: "AzureStackCloud"}, nil); err == nil {
		t.Error("an unknown environment was accepted")
	}
	cred, err := azureCredential(&ExecSpec{Login: LoginAmbient},
		&Credentials{Provider: Azure, TenantID: tenant, ClientID: client, ClientSecret: "s"})
	if err != nil || cred == nil {
		t.Fatalf("service principal: %v", err)
	}
	if azureScope("6dae42f8-4368-4678-94ff-3960e28e3630") != "6dae42f8-4368-4678-94ff-3960e28e3630/.default" {
		t.Error("scope")
	}
}

func TestGoogleTokenSourceRejectsBadKeys(t *testing.T) {
	if _, err := googleTokenSource(&Credentials{Provider: GCP, ServiceAccountJSON: `{"type":"authorized_user"}`}); err == nil {
		t.Fatal("a non-service-account key was accepted")
	}
}
