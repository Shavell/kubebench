package helm

import (
	"context"
	"errors"
	"io/fs"
	"net"
	"net/http"
	"net/url"
	"strings"

	"helm.sh/helm/v3/pkg/storage/driver"
	apierrors "k8s.io/apimachinery/pkg/api/errors"

	"kubebench/internal/plugin"
	"kubebench/internal/reason"
)

// requestError is an error with its HTTP status and §3.3 code, explained
// by a reason (reasons.go).
type requestError struct {
	status int
	code   string
	reason reason.Code
	params reason.Params
}

func (e *requestError) Error() string { return e.reason.Text(e.params) }

// Reason implements reason.Reasoned.
func (e *requestError) Reason() (reason.Code, reason.Params) { return e.reason, e.params }

func badRequest(rc reason.Code, params reason.Params) error {
	return &requestError{http.StatusBadRequest, plugin.CodeBadRequest, rc, params}
}

func notFound(rc reason.Code, params reason.Params) error {
	return &requestError{http.StatusNotFound, plugin.CodeNotFound, rc, params}
}

func conflict(rc reason.Code, params reason.Params) error {
	return &requestError{http.StatusConflict, plugin.CodeConflict, rc, params}
}

// writeError reports err with the status that fits it:
//
//   - KubeBench's own request errors as themselves;
//   - a missing release or revision as 404 not_found;
//   - Kubernetes API errors, and failures to reach the cluster, with code
//     kubernetes (plugin.WriteK8sError);
//   - a release name in use as 409 conflict;
//   - failing files of the Helm data directory as 500 internal;
//   - anything else Helm refuses (a chart that does not render with the
//     given values, a rollback to a missing revision, …) as 400.
//
// Chart downloads report their failures as request errors, so a network
// error that gets here came from the cluster.
func writeError(w http.ResponseWriter, err error) {
	var re *requestError
	var status apierrors.APIStatus
	var urlErr *url.Error
	var netErr net.Error
	var pathErr *fs.PathError
	switch {
	case errors.As(err, &re):
		plugin.WriteError(w, re.status, re.code, re)
	case errors.Is(err, driver.ErrReleaseNotFound):
		plugin.WriteError(w, http.StatusNotFound, plugin.CodeNotFound, err)
	case errors.As(err, &status), errors.As(err, &urlErr), errors.As(err, &netErr),
		errors.Is(err, context.DeadlineExceeded):
		plugin.WriteK8sError(w, err)
	case strings.Contains(err.Error(), "cannot re-use a name that is still in use"):
		// Helm returns this as a plain error, not a sentinel.
		plugin.WriteError(w, http.StatusConflict, plugin.CodeConflict, err)
	case errors.As(err, &pathErr):
		plugin.WriteError(w, http.StatusInternalServerError, plugin.CodeInternal, err)
	default:
		plugin.WriteError(w, http.StatusBadRequest, plugin.CodeBadRequest, err)
	}
}
