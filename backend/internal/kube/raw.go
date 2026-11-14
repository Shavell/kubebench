package kube

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"k8s.io/client-go/rest"
)

// ErrResponseTooLarge reports a RawGet response above the caller's limit.
var ErrResponseTooLarge = errors.New("response too large")

// RawResponse is the API server's answer to RawGet.
type RawResponse struct {
	Status      int
	ContentType string
	Body        []byte
}

// RawGet sends GET <path>?<query> to the API server with the runtime's
// credentials and returns the response as is, whatever its status. path is
// relative to the server URL (e.g. "api/v1/namespaces"); callers decide
// which paths are allowed. At most limit body bytes are read.
func (r *Runtime) RawGet(ctx context.Context, path string, query url.Values, limit int64) (*RawResponse, error) {
	client, err := r.rawHTTPClient()
	if err != nil {
		return nil, err
	}
	base, _, err := rest.DefaultServerUrlFor(r.Config)
	if err != nil {
		return nil, err
	}
	target := *base
	target.Path = strings.TrimSuffix(base.Path, "/") + "/" + strings.TrimPrefix(path, "/")
	target.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json, */*")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > limit {
		return nil, fmt.Errorf("%w: more than %d bytes", ErrResponseTooLarge, limit)
	}
	return &RawResponse{Status: resp.StatusCode, ContentType: resp.Header.Get("Content-Type"), Body: body}, nil
}

// rawHTTPClient is an authenticated client for Config, built on first use.
func (r *Runtime) rawHTTPClient() (*http.Client, error) {
	r.rawOnce.Do(func() { r.rawClient, r.rawErr = rest.HTTPClientFor(r.Config) })
	return r.rawClient, r.rawErr
}
