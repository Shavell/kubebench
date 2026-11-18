package access

import (
	"math"
	"net/http"
	"strconv"
	"time"

	authv1 "k8s.io/api/authorization/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"kubebench/internal/plugin"
	"kubebench/internal/reason"
)

// reviewRequest is the body of POST /review: a subject and the request to
// ask about, e.g. {"subject":{"kind":"ServiceAccount","namespace":"argocd",
// "name":"argocd-server"},"verb":"list","resource":"secrets","namespace":"argocd"}.
type reviewRequest struct {
	Subject Subject `json:"subject"`
	Request
}

// reviewResponse is the API server's answer next to KubeBench's own
// evaluation of the same request (Grants: the paths that allow it).
type reviewResponse struct {
	Allowed         bool     `json:"allowed"`
	Denied          bool     `json:"denied,omitempty"`
	Reason          string   `json:"reason,omitempty"`
	EvaluationError string   `json:"evaluationError,omitempty"`
	User            string   `json:"user,omitempty"`
	Groups          []string `json:"groups"`
	Grants          []Grant  `json:"grants"`
}

// review serves POST /review. It creates a SubjectAccessReview for the
// subject's identity (user name and groups as the API server would see
// them), so the cluster's whole authorizer chain answers, not only RBAC.
// It never impersonates: the review is an ordinary create with
// KubeBench's credential, which therefore needs create on
// subjectaccessreviews at the cluster level. Reviews are rate-limited per
// user because each one is a request to the API server's authorizers.
func (p *Plugin) review(w http.ResponseWriter, r *http.Request, c *plugin.Context) {
	var body reviewRequest
	if err := decodeBody(w, r, &body); err != nil {
		writeErr(w, err)
		return
	}
	if err := validateSubject(body.Subject); err != nil {
		writeErr(w, err)
		return
	}
	if err := validateRequest(body.Request); err != nil {
		writeErr(w, err)
		return
	}
	var userID int64
	if c.User != nil {
		userID = c.User.ID
	}
	if wait := p.limiter.take(userID); wait > 0 {
		w.Header().Set("Retry-After", strconv.Itoa(int(math.Ceil(wait.Seconds()))))
		plugin.WriteError(w, http.StatusTooManyRequests, plugin.CodeRateLimited,
			reason.New(reasonReviewRate, reason.Params{"max": reviewsPerWindow, "wait": wait.Round(time.Second).String()}))
		return
	}

	id := identityOf(body.Subject)
	sar := &authv1.SubjectAccessReview{Spec: authv1.SubjectAccessReviewSpec{User: id.User, Groups: id.Groups}}
	if q := body.Request; q.IsResource() {
		sar.Spec.ResourceAttributes = &authv1.ResourceAttributes{
			Verb: q.Verb, Group: q.Group, Resource: q.Resource, Subresource: q.Subresource,
			Namespace: q.Namespace, Name: q.Name,
		}
	} else {
		sar.Spec.NonResourceAttributes = &authv1.NonResourceAttributes{Verb: q.Verb, Path: q.Path}
	}
	res, err := c.Runtime.CS.AuthorizationV1().SubjectAccessReviews().Create(r.Context(), sar, metav1.CreateOptions{})
	if err != nil {
		if apierrors.IsForbidden(err) {
			plugin.WriteError(w, http.StatusForbidden, plugin.CodeKubernetes,
				reason.Wrap(err, reasonCannotReview, reason.Params{"detail": err.Error()}))
			return
		}
		plugin.WriteK8sError(w, err)
		return
	}

	snap, err := loadSnapshot(r.Context(), c.Runtime.CS)
	if err != nil {
		plugin.WriteK8sError(w, err)
		return
	}
	grants := snap.allows(id, body.Request)
	if grants == nil {
		grants = []Grant{}
	}
	plugin.WriteJSON(w, reviewResponse{
		Allowed: res.Status.Allowed, Denied: res.Status.Denied,
		Reason: res.Status.Reason, EvaluationError: res.Status.EvaluationError,
		User: id.User, Groups: id.Groups, Grants: grants,
	})
}
