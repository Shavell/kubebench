package kube

import (
	"context"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"kubebench/internal/store"
)

func TestMapEventSeries(t *testing.T) {
	now := time.Now()
	for name, tc := range map[string]struct {
		ev    corev1.Event
		count int32
		age   time.Duration
	}{
		"legacy": {corev1.Event{Count: 4, LastTimestamp: metav1.NewTime(now.Add(-2 * time.Minute))}, 4, 2 * time.Minute},
		"series": {corev1.Event{
			EventTime: metav1.NewMicroTime(now.Add(-time.Hour)),
			Series:    &corev1.EventSeries{Count: 17, LastObservedTime: metav1.NewMicroTime(now.Add(-3 * time.Minute))},
		}, 17, 3 * time.Minute},
		"eventTime only": {corev1.Event{EventTime: metav1.NewMicroTime(now.Add(-5 * time.Minute))}, 1, 5 * time.Minute},
	} {
		got := MapEvent(&tc.ev)
		if got.Count != tc.count || got.Age != FormatAge(now.Add(-tc.age)) {
			t.Errorf("%s: count %d age %s, want %d %s", name, got.Count, got.Age, tc.count, FormatAge(now.Add(-tc.age)))
		}
	}
}

func TestListEventsNewestFirst(t *testing.T) {
	now := time.Now()
	ev := func(name string, ago time.Duration, series bool) *corev1.Event {
		e := &corev1.Event{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"}, Reason: name,
			LastTimestamp: metav1.NewTime(now.Add(-ago))}
		if series {
			e.LastTimestamp = metav1.NewTime(now.Add(-2 * time.Hour))
			e.Series = &corev1.EventSeries{Count: 3, LastObservedTime: metav1.NewMicroTime(now.Add(-ago))}
		}
		return e
	}
	cs := fake.NewClientset(ev("old", time.Hour, false), ev("new", time.Minute, false), ev("mid", 10*time.Minute, true))
	rt := NewRuntimeForTest(store.Cluster{Name: "t"}, cs, nil)
	out, err := rt.ListEvents(context.Background(), "default", "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, e := range out {
		got = append(got, e.Reason)
	}
	if len(got) != 3 || got[0] != "new" || got[1] != "mid" || got[2] != "old" {
		t.Fatalf("order = %v", got)
	}
}
