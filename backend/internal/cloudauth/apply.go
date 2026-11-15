package cloudauth

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/aws/aws-sdk-go-v2/aws"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"golang.org/x/oauth2"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/client-go/transport"
)

// fetchTimeout bounds one token fetch (credential lookup and exchange).
const fetchTimeout = 30 * time.Second

// Sources builds the identities behind the tokens. Default is the real
// one; tests replace the fields with fakes.
type Sources struct {
	// AWS returns the credentials and the region for an EKS stanza.
	AWS func(ctx context.Context, spec *ExecSpec, creds *Credentials) (aws.CredentialsProvider, string, error)
	// Google returns the access-token source for GKE.
	Google func(creds *Credentials) (oauth2.TokenSource, error)
	// Azure returns the Entra ID credential for AKS.
	Azure func(spec *ExecSpec, creds *Credentials) (azcore.TokenCredential, error)
	Now   func() time.Time
}

// Default uses the AWS SDK, Google's and Azure's identity libraries.
var Default = Sources{AWS: awsCredentials, Google: googleTokenSource, Azure: azureCredential, Now: time.Now}

// ProviderFor is the credential provider of user's exec stanza when it is
// handled in process, else "".
func ProviderFor(user *clientcmdapi.AuthInfo) string {
	if user == nil {
		return ""
	}
	if spec, ok := Recognize(user.Exec); ok {
		return spec.Provider
	}
	return ""
}

// CheckCredentials reports whether per-cluster credentials of provider
// can serve user: its exec stanza must be a recognised one of the same
// cloud.
func CheckCredentials(user *clientcmdapi.AuthInfo, provider string) error {
	switch got := ProviderFor(user); got {
	case provider:
		return nil
	case "":
		return fmt.Errorf("%s credentials need a kubeconfig whose user runs aws eks get-token or aws-iam-authenticator (aws), gke-gcloud-auth-plugin (gcp) or kubelogin get-token (azure)", provider)
	default:
		return fmt.Errorf("the kubeconfig signs in to %s, not %s", got, provider)
	}
}

// Apply prepares user for in-process authentication. When its exec stanza
// is recognised, Apply removes the stanza, so client-go never runs the
// command, and returns the transport wrapper that adds the tokens instead
// (rest.Config.WrapTransport). Otherwise it returns nil and leaves user
// alone. creds, when given, must match the stanza (see CheckCredentials).
//
// Nothing is looked up or fetched here: the identity is resolved on the
// first request, so building a cluster's runtime never waits for a cloud.
func (s Sources) Apply(user *clientcmdapi.AuthInfo, creds *Credentials) (func(http.RoundTripper) http.RoundTripper, error) {
	if user == nil {
		if creds != nil {
			return nil, CheckCredentials(nil, creds.Provider)
		}
		return nil, nil
	}
	spec, ok := Recognize(user.Exec)
	if creds != nil {
		if err := CheckCredentials(user, creds.Provider); err != nil {
			return nil, err
		}
	}
	if !ok {
		return nil, nil
	}
	user.Exec = nil
	return transport.ResettableTokenSourceWrapTransport(newCachedSource(s.source(spec, creds), s.Now)), nil
}

// source is the uncached token source for spec, resolving its identity
// lazily.
func (s Sources) source(spec *ExecSpec, creds *Credentials) oauth2.TokenSource {
	switch spec.Provider {
	case AWS:
		return &lazySource{build: func() (oauth2.TokenSource, error) {
			ctx, cancel := context.WithTimeout(context.Background(), fetchTimeout)
			defer cancel()
			provider, region, err := s.AWS(ctx, spec, creds)
			if err != nil {
				return nil, err
			}
			return &eksSource{clusterID: spec.ClusterID, region: region, creds: provider, signer: v4.NewSigner(), now: s.Now}, nil
		}}
	case GCP:
		return &lazySource{build: func() (oauth2.TokenSource, error) { return s.Google(creds) }}
	default: // Azure
		return &lazySource{build: func() (oauth2.TokenSource, error) {
			cred, err := s.Azure(spec, creds)
			if err != nil {
				return nil, err
			}
			return &azureSource{cred: cred, scope: azureScope(spec.ServerID)}, nil
		}}
	}
}

// lazySource builds its token source on first use and keeps it once
// built; a failed build is retried by the next request.
type lazySource struct {
	build func() (oauth2.TokenSource, error)

	mu  sync.Mutex
	src oauth2.TokenSource
}

func (l *lazySource) Token() (*oauth2.Token, error) {
	l.mu.Lock()
	if l.src == nil {
		src, err := l.build()
		if err != nil {
			l.mu.Unlock()
			return nil, err
		}
		l.src = src
	}
	src := l.src
	l.mu.Unlock()
	return src.Token()
}
