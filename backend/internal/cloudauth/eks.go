package cloudauth

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"golang.org/x/oauth2"
)

// The EKS token is aws-iam-authenticator's: a presigned STS
// GetCallerIdentity URL, base64url-encoded behind a version prefix. The
// API server's authenticator replays the request to learn the caller's
// IAM identity; x-k8s-aws-id, a signed header, binds the token to one
// cluster.
const (
	eksTokenPrefix = "k8s-aws-v1."
	eksClusterHdr  = "x-k8s-aws-id"
	// The authenticator accepts a token for 15 minutes after its signing
	// time, whatever X-Amz-Expires says.
	eksTokenLifetime = 15 * time.Minute
	// emptyPayloadHash is the SHA-256 of an empty body (a GET).
	emptyPayloadHash = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
)

// eksSource presigns EKS tokens with the credentials of an AWS provider.
type eksSource struct {
	clusterID string
	region    string // "" signs for the global endpoint (us-east-1)
	creds     aws.CredentialsProvider
	signer    *v4.Signer
	now       func() time.Time
}

func (s *eksSource) Token() (*oauth2.Token, error) {
	ctx, cancel := context.WithTimeout(context.Background(), fetchTimeout)
	defer cancel()
	creds, err := s.creds.Retrieve(ctx)
	if err != nil {
		return nil, fmt.Errorf("AWS credentials: %w", err)
	}
	signedAt := s.now()
	token, err := presignEKSToken(ctx, s.signer, creds, s.clusterID, s.region, signedAt)
	if err != nil {
		return nil, err
	}
	return &oauth2.Token{AccessToken: token, TokenType: "Bearer", Expiry: signedAt.Add(eksTokenLifetime)}, nil
}

// presignEKSToken builds the token for cluster, signed at signedAt.
func presignEKSToken(ctx context.Context, signer *v4.Signer, creds aws.Credentials,
	cluster, region string, signedAt time.Time) (string, error) {
	host, signingRegion := stsHost(region)
	query := url.Values{
		"Action":        {"GetCallerIdentity"},
		"Version":       {"2011-06-15"},
		"X-Amz-Expires": {"60"},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://"+host+"/?"+query.Encode(), nil)
	if err != nil {
		return "", err
	}
	req.Header.Set(eksClusterHdr, cluster)
	presigned, _, err := signer.PresignHTTP(ctx, creds, req, emptyPayloadHash, "sts", signingRegion, signedAt)
	if err != nil {
		return "", fmt.Errorf("presign the EKS token: %w", err)
	}
	return eksTokenPrefix + base64.RawURLEncoding.EncodeToString([]byte(presigned)), nil
}

// stsHost is the STS endpoint and signing region for region: the regional
// endpoint (the AWS CLI's default since v2), or the global one without a
// region.
func stsHost(region string) (host, signingRegion string) {
	if region == "" {
		return "sts.amazonaws.com", "us-east-1"
	}
	host = "sts." + region + ".amazonaws.com"
	if strings.HasPrefix(region, "cn-") {
		host += ".cn"
	}
	return host, region
}

// awsCredentials resolves the AWS identity for spec: per-cluster keys, a
// per-cluster or kubeconfig profile, or the default chain (environment,
// shared files, web identity/IRSA, EKS Pod Identity and ECS container
// credentials, EC2 instance metadata). A role from the stanza is assumed
// on top. It also returns the region: the stanza's, else the profile's.
func awsCredentials(ctx context.Context, spec *ExecSpec, creds *Credentials) (aws.CredentialsProvider, string, error) {
	var opts []func(*config.LoadOptions) error
	if spec.Region != "" {
		opts = append(opts, config.WithRegion(spec.Region))
	}
	profile := spec.Profile
	if creds != nil {
		profile = creds.Profile
		if creds.AccessKeyID != "" {
			opts = append(opts, config.WithCredentialsProvider(
				credentials.NewStaticCredentialsProvider(creds.AccessKeyID, creds.SecretAccessKey, creds.SessionToken)))
		}
	}
	if profile != "" {
		opts = append(opts, config.WithSharedConfigProfile(profile))
	}
	cfg, err := config.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, "", fmt.Errorf("AWS configuration: %w", err)
	}
	provider := cfg.Credentials
	if spec.RoleARN != "" {
		provider = aws.NewCredentialsCache(stscreds.NewAssumeRoleProvider(sts.NewFromConfig(cfg), spec.RoleARN))
	}
	return provider, cfg.Region, nil
}
