package cloudauth

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"net/url"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
)

var signedAt = time.Date(2026, 11, 15, 12, 0, 0, 0, time.UTC)

// decodeEKSToken returns the presigned URL inside a token.
func decodeEKSToken(t *testing.T, token string) *url.URL {
	t.Helper()
	encoded, ok := strings.CutPrefix(token, "k8s-aws-v1.")
	if !ok {
		t.Fatalf("token %q lacks the k8s-aws-v1. prefix", token)
	}
	if strings.ContainsAny(encoded, "=+/") {
		t.Fatalf("token is not unpadded base64url: %q", encoded)
	}
	raw, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatal(err)
	}
	u, err := url.Parse(string(raw))
	if err != nil {
		t.Fatal(err)
	}
	return u
}

func TestPresignEKSToken(t *testing.T) {
	creds := aws.Credentials{AccessKeyID: "AKIDEXAMPLE", SecretAccessKey: "wJalrXUtnFEMI/K7MDENG+bPxRfiCYEXAMPLEKEY", SessionToken: "session/token"}
	token, err := presignEKSToken(context.Background(), v4.NewSigner(), creds, "prod", "eu-west-1", signedAt)
	if err != nil {
		t.Fatal(err)
	}
	u := decodeEKSToken(t, token)
	q := u.Query()
	if u.Scheme != "https" || u.Host != "sts.eu-west-1.amazonaws.com" || u.Path != "/" {
		t.Fatalf("url = %s", u)
	}
	want := map[string]string{
		"Action":               "GetCallerIdentity",
		"Version":              "2011-06-15",
		"X-Amz-Expires":        "60",
		"X-Amz-Algorithm":      "AWS4-HMAC-SHA256",
		"X-Amz-Credential":     "AKIDEXAMPLE/20261115/eu-west-1/sts/aws4_request",
		"X-Amz-Date":           "20261115T120000Z",
		"X-Amz-SignedHeaders":  "host;x-k8s-aws-id",
		"X-Amz-Security-Token": "session/token",
	}
	for k, v := range want {
		if q.Get(k) != v {
			t.Errorf("%s = %q, want %q", k, q.Get(k), v)
		}
	}
	// The signature is what STS checks when the API server replays the
	// URL with x-k8s-aws-id: recompute it from the SigV4 specification.
	if got, want := q.Get("X-Amz-Signature"), sigV4(u, "prod", creds.SecretAccessKey, "eu-west-1"); got != want {
		t.Fatalf("signature = %s, want %s", got, want)
	}

	again, _ := presignEKSToken(context.Background(), v4.NewSigner(), creds, "prod", "eu-west-1", signedAt)
	if again != token {
		t.Fatal("the token is not deterministic for a fixed signer and clock")
	}
	other, _ := presignEKSToken(context.Background(), v4.NewSigner(), creds, "staging", "eu-west-1", signedAt)
	if other == token {
		t.Fatal("the cluster id is not signed")
	}
}

func TestSTSHost(t *testing.T) {
	for region, want := range map[string][2]string{
		"":           {"sts.amazonaws.com", "us-east-1"},
		"us-east-2":  {"sts.us-east-2.amazonaws.com", "us-east-2"},
		"cn-north-1": {"sts.cn-north-1.amazonaws.com.cn", "cn-north-1"},
	} {
		if host, signing := stsHost(region); host != want[0] || signing != want[1] {
			t.Errorf("stsHost(%q) = %s, %s", region, host, signing)
		}
	}
}

// sigV4 computes a presigned URL's signature independently of the SDK.
func sigV4(u *url.URL, cluster, secret, region string) string {
	q := u.Query()
	q.Del("X-Amz-Signature")
	keys := make([]string, 0, len(q))
	for k := range q {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var pairs []string
	for _, k := range keys {
		pairs = append(pairs, awsEscape(k)+"="+awsEscape(q.Get(k)))
	}
	canonical := strings.Join([]string{
		"GET", "/", strings.Join(pairs, "&"),
		"host:" + u.Host + "\nx-k8s-aws-id:" + cluster + "\n",
		"host;x-k8s-aws-id",
		emptyPayloadHash,
	}, "\n")
	date := q.Get("X-Amz-Date")
	scope := date[:8] + "/" + region + "/sts/aws4_request"
	sum := sha256.Sum256([]byte(canonical))
	toSign := "AWS4-HMAC-SHA256\n" + date + "\n" + scope + "\n" + hex.EncodeToString(sum[:])
	key := []byte("AWS4" + secret)
	for _, part := range []string{date[:8], region, "sts", "aws4_request"} {
		key = hmacSHA256(key, part)
	}
	return hex.EncodeToString(hmacSHA256(key, toSign))
}

func hmacSHA256(key []byte, data string) []byte {
	h := hmac.New(sha256.New, key)
	h.Write([]byte(data))
	return h.Sum(nil)
}

// awsEscape is SigV4's URI encoding: everything but A-Z a-z 0-9 -_.~.
func awsEscape(s string) string {
	return strings.ReplaceAll(url.QueryEscape(s), "+", "%20")
}
