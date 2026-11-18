// Package watch is the live-update hub of one cluster (docs/design-v0.5.md
// §7.10): clients subscribe to topics, a topic names a resource type
// ("pods", "deployments", "applications.argoproj.io"), and the hub tells
// every subscriber in which namespace an object of a subscribed type
// changed. Each resource type is watched by one shared dynamic informer,
// started on its first subscription and stopped once it has had no
// subscriber for a while. A type the cluster credential may not list and
// watch is reported as polled instead, with the reason.
package watch

import (
	"errors"
	"regexp"
	"strings"

	"k8s.io/apimachinery/pkg/runtime/schema"

	"kubebench/internal/reason"
)

// Mode is how a topic's changes reach its subscribers.
type Mode string

const (
	// ModeStarting: the informer is listing the resource; changes are
	// reported once it has synced.
	ModeStarting Mode = "starting"
	// ModeLive: changes are reported as they happen.
	ModeLive Mode = "live"
	// ModePolling: changes are not reported (see the status reason);
	// clients refresh the topic on their own schedule.
	ModePolling Mode = "polling"
)

// Change says that an object of Topic changed in Namespace ("" for a
// cluster-scoped object, or for "anything of the topic" after an overflow).
type Change struct {
	Topic     string `json:"topic"`
	Namespace string `json:"namespace"`
}

// TopicStatus is the current mode of one subscribed topic. Reason is the
// English explanation of a polled topic; ReasonCode and ReasonParams let
// the UI translate it (§3.3).
type TopicStatus struct {
	Topic        string        `json:"topic"`
	Mode         Mode          `json:"mode"`
	Reason       string        `json:"reason,omitempty"`
	ReasonCode   string        `json:"reasonCode,omitempty"`
	ReasonParams reason.Params `json:"reasonParams,omitempty"`
}

// topicStatus is the status of topic in mode, explained by why (nil for
// none).
func topicStatus(topic string, mode Mode, why *reason.Error) TopicStatus {
	st := TopicStatus{Topic: topic, Mode: mode}
	if why != nil {
		st.Reason, st.ReasonCode, st.ReasonParams = why.Error(), why.Code.String(), why.Params
	}
	return st
}

// Reasons of polled topics and refused subscriptions.
var (
	reasonNotServed       = reason.Define("watch.not_served", "the cluster does not serve {{topic}}")
	reasonDenied          = reason.Define("watch.denied", "the cluster credential may not list and watch {{resource}}")
	reasonCannotWatch     = reason.Define("watch.cannot_watch", "{{resource}} cannot be watched: {{detail}}")
	reasonCannotList      = reason.Define("watch.cannot_list", "cannot list {{resource}}: {{detail}}")
	reasonOwnerTypes      = reason.Define("watch.too_many_own_types", "you watch too many resource types on this cluster at once")
	reasonClusterTypes    = reason.Define("watch.too_many_types", "too many resource types are watched on this cluster at once")
	reasonClosed          = reason.Define("watch.closed", "the cluster connection was closed")
	reasonTooManyWatchers = reason.Define("watch.too_many_subscribers", "too many live-update subscribers on this cluster")
)

// Resolver maps a topic to the resource it names, at the version the
// cluster prefers. Its error is shown to users as the reason the topic is
// polled, so it should read as one. A *NotServedError is final; the hub
// resolves the topic again after any other error (e.g. discovery failed).
type Resolver func(topic string) (schema.GroupVersionResource, error)

// NotServedError is the Resolver error for a topic whose resource the
// cluster does not serve.
type NotServedError struct{ Topic string }

func (e *NotServedError) Error() string { return reasonNotServed.Text(e.params()) }

// Reason implements reason.Reasoned.
func (e *NotServedError) Reason() (reason.Code, reason.Params) { return reasonNotServed, e.params() }

func (e *NotServedError) params() reason.Params { return reason.Params{"topic": e.Topic} }

func isNotServed(err error) bool {
	var ns *NotServedError
	return errors.As(err, &ns)
}

// MaxTopicLength bounds a topic: a resource name and a DNS subdomain group.
const MaxTopicLength = 317

var topicPattern = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$`)

// ValidTopic reports whether topic has the form "<resource>" or
// "<resource>.<group>", in lower case, like kubectl's resource arguments.
func ValidTopic(topic string) bool {
	return len(topic) <= MaxTopicLength && topicPattern.MatchString(topic)
}

// SplitTopic returns the resource and the group a valid topic names; the
// group is "" for a bare resource name (which matches it in any group).
func SplitTopic(topic string) (resource, group string) {
	resource, group, _ = strings.Cut(topic, ".")
	return resource, group
}

// gvrName is how a resource type appears in logs and statistics.
func gvrName(gvr schema.GroupVersionResource) string {
	if gvr.Group == "" {
		return gvr.Version + "/" + gvr.Resource
	}
	return gvr.Group + "/" + gvr.Version + "/" + gvr.Resource
}
