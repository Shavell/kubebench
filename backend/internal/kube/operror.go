package kube

import "kubebench/internal/reason"

// OpErrorKind says why KubeBench refused an operation.
type OpErrorKind int

const (
	// OpInvalid: the request itself is wrong, e.g. an unknown taint effect.
	OpInvalid OpErrorKind = iota + 1
	// OpNotFound: something the operation needs does not exist, e.g. the
	// rollout revision to go back to.
	OpNotFound
	// OpConflict: the object's state does not allow the operation, e.g.
	// rolling back a paused Deployment.
	OpConflict
	// OpProtected: KubeBench declines unless the caller forces it, e.g.
	// deleting a system namespace.
	OpProtected
)

// OpError is an operation KubeBench refused itself, before or instead of
// sending the request to the cluster, explained by a reason code. Errors
// from the cluster stay client-go errors; the API reports the two
// differently (design §3.3).
type OpError struct {
	Kind   OpErrorKind
	Code   reason.Code
	Params reason.Params
}

func (e *OpError) Error() string { return e.Code.Text(e.Params) }

// Reason implements reason.Reasoned.
func (e *OpError) Reason() (reason.Code, reason.Params) { return e.Code, e.Params }

func opError(kind OpErrorKind, code reason.Code, params reason.Params) error {
	return &OpError{Kind: kind, Code: code, Params: params}
}
