package kube

import (
	"context"
	"encoding/json"
	"strconv"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
)

const webUID = types.UID("web-uid")

func webTemplate(image, hash string) corev1.PodTemplateSpec {
	labels := map[string]string{"app": "web"}
	if hash != "" {
		labels[appsv1.DefaultDeploymentUniqueLabelKey] = hash
	}
	return corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{Labels: labels},
		Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "app", Image: image}}},
	}
}

// webReplicaSet is revision rev of Deployment web, owned by ownerUID.
func webReplicaSet(rev int, image, cause string, ownerUID types.UID) *appsv1.ReplicaSet {
	yes := true
	hash := "h" + strconv.Itoa(rev)
	annotations := map[string]string{revisionAnnotation: strconv.Itoa(rev)}
	if cause != "" {
		annotations[changeCauseAnnotation] = cause
	}
	return &appsv1.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: "web-" + hash, Namespace: "shop", Labels: map[string]string{"app": "web"},
			Annotations:       annotations,
			CreationTimestamp: metav1.NewTime(time.Date(2026, 11, rev, 9, 0, 0, 0, time.UTC)),
			OwnerReferences:   []metav1.OwnerReference{{Kind: "Deployment", Name: "web", UID: ownerUID, Controller: &yes}},
		},
		Spec:   appsv1.ReplicaSetSpec{Template: webTemplate(image, hash)},
		Status: appsv1.ReplicaSetStatus{Replicas: int32(rev / 3 * 2), ReadyReplicas: int32(rev / 3 * 2)},
	}
}

func rolloutObjects(paused bool) []runtime.Object {
	return []runtime.Object{
		&appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name: "web", Namespace: "shop", UID: webUID,
				Annotations: map[string]string{revisionAnnotation: "3", "team": "shop"},
			},
			Spec: appsv1.DeploymentSpec{
				Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "web"}},
				Template: webTemplate("web:3", ""),
				Paused:   paused,
			},
		},
		webReplicaSet(1, "web:1", "first release", webUID),
		webReplicaSet(2, "web:2", "", webUID),
		webReplicaSet(3, "web:3", "kubectl set image web=web:3", webUID),
		// Matches the selector but belongs to someone else.
		webReplicaSet(7, "intruder:7", "", "other-uid"),
	}
}

func TestDeploymentHistory(t *testing.T) {
	rt, _ := opsRuntime(t, rolloutObjects(false)...)
	h, err := rt.DeploymentHistory(context.Background(), "shop", "web")
	if err != nil {
		t.Fatal(err)
	}
	if h.CurrentRevision != 3 || h.Paused || len(h.Revisions) != 3 {
		t.Fatalf("history = %+v", h)
	}
	for i, want := range []int64{3, 2, 1} {
		if h.Revisions[i].Revision != want {
			t.Fatalf("revisions are not newest first: %+v", h.Revisions)
		}
	}
	top := h.Revisions[0]
	if !top.Current || top.ReplicaSet != "web-h3" || top.Images[0] != "web:3" ||
		top.ChangeCause != "kubectl set image web=web:3" || top.Created != "2026-11-03T09:00:00Z" ||
		top.Replicas != 2 || top.ReadyReplicas != 2 {
		t.Fatalf("current revision = %+v", top)
	}
	if h.Revisions[1].Current || h.Revisions[2].ChangeCause != "first release" {
		t.Fatalf("older revisions = %+v", h.Revisions[1:])
	}
	if _, err := rt.DeploymentHistory(context.Background(), "shop", "ghost"); err == nil {
		t.Fatal("an unknown Deployment must fail")
	}
}

func TestRollbackDeployment(t *testing.T) {
	rt, cs := opsRuntime(t, rolloutObjects(false)...)
	ctx := context.Background()
	if err := rt.RollbackDeployment(ctx, "shop", "web", 1); err != nil {
		t.Fatal(err)
	}
	d, err := cs.AppsV1().Deployments("shop").Get(ctx, "web", metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if img := d.Spec.Template.Spec.Containers[0].Image; img != "web:1" {
		t.Fatalf("template image = %s, want the revision's", img)
	}
	if _, ok := d.Spec.Template.Labels[appsv1.DefaultDeploymentUniqueLabelKey]; ok || d.Spec.Template.Labels["app"] != "web" {
		t.Fatalf("template labels = %v (pod-template-hash must go)", d.Spec.Template.Labels)
	}
	// kubectl rollout undo semantics: the ReplicaSet's change-cause comes
	// back, the Deployment keeps its revision bookkeeping; unlike kubectl,
	// its own other annotations stay.
	if d.Annotations[changeCauseAnnotation] != "first release" || d.Annotations[revisionAnnotation] != "3" ||
		d.Annotations["team"] != "shop" || len(d.Annotations) != 3 {
		t.Fatalf("annotations = %v", d.Annotations)
	}
}

func TestRollbackAnnotationOps(t *testing.T) {
	rs := func(annotations map[string]string) *appsv1.ReplicaSet {
		return &appsv1.ReplicaSet{ObjectMeta: metav1.ObjectMeta{Annotations: annotations}}
	}
	dep := func(annotations map[string]string) *appsv1.Deployment {
		return &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Annotations: annotations}}
	}
	for _, tc := range []struct {
		name string
		d    *appsv1.Deployment
		rs   *appsv1.ReplicaSet
		want string
	}{
		{"no annotations on either side", dep(nil), rs(map[string]string{revisionAnnotation: "1"}), `null`},
		{"the Deployment has none: the map is added", dep(nil),
			rs(map[string]string{revisionAnnotation: "1", "a/b": "x"}),
			`[{"op":"add","path":"/metadata/annotations","value":{"a/b":"x"}}]`},
		{"changed and new keys are set one by one, unchanged and own ones left alone",
			dep(map[string]string{revisionAnnotation: "4", "team": "shop", "same": "1", "example.com/tier": "new"}),
			rs(map[string]string{revisionAnnotation: "1", "same": "1", "example.com/tier": "old", "x~y": "z"}),
			`[{"op":"add","path":"/metadata/annotations/example.com~1tier","value":"old"},{"op":"add","path":"/metadata/annotations/x~0y","value":"z"}]`},
		{"a change-cause the revision lacks goes",
			dep(map[string]string{changeCauseAnnotation: "v4", "team": "shop"}), rs(map[string]string{revisionAnnotation: "1"}),
			`[{"op":"remove","path":"/metadata/annotations/kubernetes.io~1change-cause"}]`},
		{"the bookkeeping stays the Deployment's",
			dep(map[string]string{revisionAnnotation: "4", "kubectl.kubernetes.io/last-applied-configuration": "{}"}),
			rs(map[string]string{revisionAnnotation: "1", "kubectl.kubernetes.io/last-applied-configuration": "{old}", "deployment.kubernetes.io/desired-replicas": "3"}),
			`null`},
	} {
		got, _ := json.Marshal(rollbackAnnotationOps(tc.d, tc.rs))
		if string(got) != tc.want {
			t.Errorf("%s: ops = %s, want %s", tc.name, got, tc.want)
		}
	}
}

func TestRollbackRefusals(t *testing.T) {
	ctx := context.Background()
	rt, cs := opsRuntime(t, rolloutObjects(false)...)
	expectOpError(t, rt.RollbackDeployment(ctx, "shop", "web", 3), OpConflict)
	expectOpError(t, rt.RollbackDeployment(ctx, "shop", "web", 9), OpNotFound)
	// The other owner's ReplicaSet is not a revision of web.
	expectOpError(t, rt.RollbackDeployment(ctx, "shop", "web", 7), OpNotFound)
	if n := len(writes(cs.Actions())); n != 0 {
		t.Fatalf("refused rollbacks sent %d writes", n)
	}

	paused, cs := opsRuntime(t, rolloutObjects(true)...)
	expectOpError(t, paused.RollbackDeployment(ctx, "shop", "web", 1), OpConflict)
	if n := len(writes(cs.Actions())); n != 0 {
		t.Fatal("a paused Deployment was rolled back")
	}
}

// While the controller moves a Deployment to another ReplicaSet, the
// revision annotations lag behind: the template decides what is current.
func TestCurrentRevisionFollowsTheTemplate(t *testing.T) {
	objects := rolloutObjects(false)
	d := objects[0].(*appsv1.Deployment)
	d.Spec.Template = webTemplate("web:1", "") // just rolled back to revision 1
	rt, _ := opsRuntime(t, objects...)
	ctx := context.Background()

	h, err := rt.DeploymentHistory(ctx, "shop", "web")
	if err != nil {
		t.Fatal(err)
	}
	if h.CurrentRevision != 1 || !h.Revisions[2].Current || h.Revisions[0].Current {
		t.Fatalf("the ReplicaSet with the Deployment's template is current: %+v", h)
	}
	expectOpError(t, rt.RollbackDeployment(ctx, "shop", "web", 1), OpConflict)
	if err := rt.RollbackDeployment(ctx, "shop", "web", 3); err != nil {
		t.Fatalf("revision 3 is no longer current: %v", err)
	}
}

func TestPauseDeployment(t *testing.T) {
	rt, cs := opsRuntime(t, rolloutObjects(false)...)
	ctx := context.Background()
	isPaused := func() bool {
		d, err := cs.AppsV1().Deployments("shop").Get(ctx, "web", metav1.GetOptions{})
		if err != nil {
			t.Fatal(err)
		}
		return d.Spec.Paused && MapDeployment(d).Paused
	}
	if err := rt.PauseDeployment(ctx, "shop", "web", true); err != nil || !isPaused() {
		t.Fatalf("pause: %v", err)
	}
	h, err := rt.DeploymentHistory(ctx, "shop", "web")
	if err != nil || !h.Paused {
		t.Fatalf("history of a paused Deployment: %+v, %v", h, err)
	}
	if err := rt.PauseDeployment(ctx, "shop", "web", false); err != nil || isPaused() {
		t.Fatalf("resume: %v", err)
	}
}
