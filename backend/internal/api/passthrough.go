package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"mime"
	"net/http"
	"net/url"
	"strings"
	"time"

	"kubebench/internal/kube"
	"kubebench/internal/plugin"
	"kubebench/internal/reason"
)

const (
	maxPassthroughBytes = 16 << 20
	passthroughTimeout  = 60 * time.Second
)

// blockedSubresources would turn a GET into something other than a read:
// streams into containers or HTTP requests to workloads (proxy).
var blockedSubresources = []string{"attach", "exec", "portforward", "proxy"}

// handlePassthrough proxies a read to the cluster API for external plugins
// (§4.5): GET /api/k8s/api/v1/namespaces returns the API server's JSON for
// /api/v1/namespaces, with its status code. Errors and redirects become
// API errors.
func (s *Server) handlePassthrough(w http.ResponseWriter, r *http.Request, c *call) {
	path, query := r.PathValue("path"), r.URL.Query()
	if err := checkPassthrough(path, query); err != nil {
		writeErr(w, err)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), passthroughTimeout)
	defer cancel()
	res, err := c.rt.RawGet(ctx, path, query, maxPassthroughBytes)
	if errors.Is(err, kube.ErrResponseTooLarge) {
		plugin.WriteError(w, http.StatusBadGateway, plugin.CodeKubernetes, reason.New(reasonPassthroughSize, nil))
		return
	}
	if err != nil {
		plugin.WriteK8sError(w, err)
		return
	}
	if res.Status >= http.StatusBadRequest {
		writeUpstreamError(w, res)
		return
	}
	if res.Status >= http.StatusMultipleChoices {
		// A redirect: RawGet does not follow it, and passing it on would
		// send the browser to wherever it points.
		plugin.WriteError(w, http.StatusBadGateway, plugin.CodeKubernetes, reason.New(reasonPassthroughRedirect,
			reason.Params{"status": fmt.Sprintf("%d %s", res.Status, http.StatusText(res.Status))}))
		return
	}
	h := w.Header()
	h.Set("Content-Type", passthroughContentType(res.ContentType))
	h.Set("X-Content-Type-Options", "nosniff")
	// The body comes from the cluster (an aggregated API, say): even opened
	// as a page it must not run as a document of KubeBench's origin.
	h.Set("Content-Security-Policy", "sandbox; default-src 'none'")
	w.WriteHeader(res.Status)
	_, _ = w.Write(res.Body)
}

// passthroughContentType is the Content-Type a passthrough response is
// served with. The upstream type is never echoed: JSON stays JSON, plain
// text (pod logs) stays text, and anything else, HTML or SVG included, is
// sent as an opaque download.
func passthroughContentType(upstream string) string {
	mediaType, _, err := mime.ParseMediaType(upstream)
	switch {
	case err != nil:
		return "application/octet-stream"
	case mediaType == "application/json" || strings.HasSuffix(mediaType, "+json"):
		return "application/json"
	case mediaType == "text/plain":
		return "text/plain; charset=utf-8"
	}
	return "application/octet-stream"
}

// checkPassthrough allows reads below /api, /apis and /version, except
// Secrets, watches, followed logs and the subresources that do not read.
func checkPassthrough(path string, q url.Values) error {
	segs := strings.Split(path, "/")
	for _, seg := range segs {
		if seg == "" || seg == "." || seg == ".." {
			return badRequest(reasonPassthroughPath, reason.Params{"path": path})
		}
	}
	var rest []string // the path after the group version
	switch {
	case path == "version":
	case segs[0] == "api" && len(segs) > 2:
		rest = segs[2:]
	case segs[0] == "apis" && len(segs) > 3:
		rest = segs[3:]
	case segs[0] == "api" || segs[0] == "apis": // discovery documents
	default:
		return badRequest(reasonPassthroughPrefix, nil)
	}
	if queryTrue(q, "watch") || (len(rest) > 0 && rest[0] == "watch") {
		return badRequest(reasonPassthroughWatch, nil)
	}
	if queryTrue(q, "follow") {
		return badRequest(reasonPassthroughFollow, nil)
	}
	resource, subresource := splitResourcePath(rest)
	if strings.EqualFold(resource, "secrets") {
		return fail(http.StatusForbidden, plugin.CodeForbidden, reasonPassthroughSecrets, nil)
	}
	for _, blocked := range blockedSubresources {
		if strings.EqualFold(subresource, blocked) {
			return fail(http.StatusForbidden, plugin.CodeForbidden, reasonPassthroughSubres, reason.Params{"subresource": subresource})
		}
	}
	return nil
}

// splitResourcePath finds the resource and subresource in the path after
// the group version, the way the API server does:
// [namespaces/{ns}/]{resource}[/{name}[/{subresource}]]. "status" and
// "finalize" right after a namespace name are subresources of the
// Namespace itself.
func splitResourcePath(rest []string) (resource, subresource string) {
	if len(rest) >= 3 && rest[0] == "namespaces" && rest[2] != "status" && rest[2] != "finalize" {
		rest = rest[2:]
	}
	if len(rest) > 0 {
		resource = rest[0]
	}
	if len(rest) > 2 {
		subresource = rest[2]
	}
	return resource, subresource
}

// queryTrue reports whether the API server reads a boolean query flag as
// true: it takes the first value and treats anything but "0" and "false"
// (even an empty value) as true.
func queryTrue(q url.Values, key string) bool {
	values := q[key]
	if len(values) == 0 {
		return false
	}
	return values[0] != "0" && !strings.EqualFold(values[0], "false")
}

// writeUpstreamError turns the API server's error (a metav1.Status) into
// the API's error shape, keeping the status code. A 401 means KubeBench's
// stored credential was rejected and becomes 502, as in WriteK8sError.
func writeUpstreamError(w http.ResponseWriter, res *kube.RawResponse) {
	status := res.Status
	if status == http.StatusUnauthorized {
		status = http.StatusBadGateway
	}
	var body struct {
		Message string `json:"message"`
	}
	msg := http.StatusText(res.Status)
	if json.Unmarshal(res.Body, &body) == nil && body.Message != "" {
		msg = body.Message
	}
	plugin.WriteError(w, status, plugin.CodeKubernetes, errors.New(msg))
}
