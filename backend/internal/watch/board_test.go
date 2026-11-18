package watch

import (
	"reflect"
	"testing"
)

func TestIsInternalTopic(t *testing.T) {
	for topic, want := range map[string]bool{
		"kubebench:notes":    true,
		"kubebench:a":        true,
		"kubebench:my-data2": true,
		"kubebench:":         false,
		"kubebench:Notes":    false,
		"kubebench:-notes":   false,
		"notes":              false,
		"pods.kubebench":     false,
		"other:notes":        false,
	} {
		if got := IsInternalTopic(topic); got != want {
			t.Errorf("IsInternalTopic(%q) = %v", topic, got)
		}
	}
	if ValidTopic("kubebench:notes") {
		t.Error("an internal topic must never be a valid resource topic")
	}
}

func pending(s *BoardSubscription) []Change {
	select {
	case <-s.Notify():
		return s.TakeChanges()
	default:
		return nil
	}
}

func TestBoardAudiences(t *testing.T) {
	b := NewBoard()
	const notes = "kubebench:notes"
	ann := b.Subscribe(1, 10, []string{notes})           // user 1 on cluster 10
	ben := b.Subscribe(2, 10, []string{notes})           // user 2 on cluster 10
	ann2 := b.Subscribe(1, 20, []string{notes})          // user 1 again, on cluster 20
	other := b.Subscribe(3, 10, []string{"kubebench:x"}) // another topic
	want := []Change{{Topic: notes}}

	b.Publish(notes, Audience{ClusterID: 10})
	for name, s := range map[string]*BoardSubscription{"ann": ann, "ben": ben} {
		if got := pending(s); !reflect.DeepEqual(got, want) {
			t.Errorf("%s: %v", name, got)
		}
	}
	for name, s := range map[string]*BoardSubscription{"ann2": ann2, "other": other} {
		if got := pending(s); got != nil {
			t.Errorf("%s got a cluster-10 change: %v", name, got)
		}
	}

	// A user audience reaches that user on every cluster, nobody else.
	b.Publish(notes, Audience{UserID: 1})
	b.Publish(notes, Audience{UserID: 1}) // collected into one change
	for name, s := range map[string]*BoardSubscription{"ann": ann, "ann2": ann2} {
		if got := pending(s); !reflect.DeepEqual(got, want) {
			t.Errorf("%s: %v", name, got)
		}
	}
	if got := pending(ben); got != nil {
		t.Errorf("ben got user 1's change: %v", got)
	}

	b.Publish(notes, Audience{})
	if got := pending(ann); got != nil {
		t.Errorf("a zero audience reached ann: %v", got)
	}

	ann.Close()
	ann.Close()
	b.Publish(notes, Audience{ClusterID: 10})
	if got := pending(ann); got != nil {
		t.Errorf("a closed subscription got %v", got)
	}
	if n := b.Subscribers(); n != 3 {
		t.Errorf("subscribers = %d", n)
	}
}
