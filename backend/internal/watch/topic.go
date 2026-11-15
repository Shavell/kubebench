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
	"regexp"
	"strings"

	"k8s.io/apimachinery/pkg/runtime/schema"
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

// TopicStatus is the current mode of one subscribed topic.
type TopicStatus struct {
	Topic  string `json:"topic"`
	Mode   Mode   `json:"mode"`
	Reason string `json:"reason,omitempty"`
}

// Resolver maps a topic to the resource it names, at the version the
// cluster prefers. Its error is shown to users as the reason the topic is
// polled, so it should read as one (e.g. "the cluster does not serve …").
type Resolver func(topic string) (schema.GroupVersionResource, error)

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
