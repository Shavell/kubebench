package api

import (
	"context"
	"net/http"
	"strings"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"kubebench/internal/kube"
	"kubebench/internal/kube/kubetest"
	"kubebench/internal/plugin"
)

// addRollout adds Deployment "api" at revision 2 with the ReplicaSets of
// revisions 1 and 2.
func (e *testEnv) addRollout() {
	e.t.Helper()
	yes := true
	labels := map[string]string{"app": "api"}
	rs := func(rev, image string) *appsv1.ReplicaSet {
		return &appsv1.ReplicaSet{
			ObjectMeta: metav1.ObjectMeta{
				Name: "api-" + rev, Namespace: "default", Labels: labels,
				Annotations:     map[string]string{"deployment.kubernetes.io/revision": rev},
				OwnerReferences: []metav1.OwnerReference{{Kind: "Deployment", Name: "api", UID: "api-uid", Controller: &yes}},
			},
			Spec: appsv1.ReplicaSetSpec{Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: labels},
				Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "api", Image: image}}},
			}},
		}
	}
	e.add(
		&appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name: "api", Namespace: "default", UID: "api-uid",
				Annotations: map[string]string{"deployment.kubernetes.io/revision": "2"},
			},
			Spec: appsv1.DeploymentSpec{
				Selector: &metav1.LabelSelector{MatchLabels: labels},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{Labels: labels},
					Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "api", Image: "api:2"}}},
				},
			},
		},
		rs("1", "api:1"), rs("2", "api:2"),
	)
}

func (e *testEnv) deployment(name string) *appsv1.Deployment {
	e.t.Helper()
	d, err := e.cs.AppsV1().Deployments("default").Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		e.t.Fatal(err)
	}
	return d
}

func TestRolloutThroughTheAPI(t *testing.T) {
	e := newTestEnv(t, nil)
	e.addRollout()

	history := decode[kube.RolloutHistory](t, e.do(http.MethodGet, "/api/deployments/history?ns=default&name=api", "bob", nil))
	if history.CurrentRevision != 2 || len(history.Revisions) != 2 || !history.Revisions[0].Current {
		t.Fatalf("history = %+v", history)
	}
	expect(t, e.do(http.MethodGet, "/api/deployments/history?ns=default", "bob", nil), http.StatusBadRequest, plugin.CodeBadRequest)

	expect(t, e.do(http.MethodPost, "/api/deployments/rollback?ns=default&name=api&revision=2", "alice", nil), http.StatusConflict, plugin.CodeConflict)
	if row := e.lastAudit(); row.Status != http.StatusConflict || !strings.HasPrefix(row.Detail, "conflict: ") {
		t.Fatalf("a refused rollback is audited with the reason: %+v", row)
	}
	expect(t, e.do(http.MethodPost, "/api/deployments/rollback?ns=default&name=api&revision=5", "alice", nil), http.StatusNotFound, plugin.CodeNotFound)
	for _, rev := range []string{"", "0", "two"} {
		expect(t, e.do(http.MethodPost, "/api/deployments/rollback?ns=default&name=api&revision="+rev, "alice", nil), http.StatusBadRequest, plugin.CodeBadRequest)
	}

	// Paused Deployments are not rolled back.
	expect(t, e.do(http.MethodPost, "/api/deployments/pause?ns=default&name=api", "alice", nil), http.StatusOK, "")
	expectAudit(t, e, "POST /api/deployments/pause", "deployment/default/api", http.StatusOK, "")
	if !e.deployment("api").Spec.Paused {
		t.Fatal("pause is the default")
	}
	deployments := decode[[]kube.DeploymentInfo](t, e.do(http.MethodGet, "/api/deployments?ns=default", "bob", nil))
	for _, d := range deployments {
		if d.Name == "api" && !d.Paused {
			t.Fatalf("the list reports the pause: %+v", d)
		}
	}
	expect(t, e.do(http.MethodPost, "/api/deployments/rollback?ns=default&name=api&revision=1", "alice", nil), http.StatusConflict, plugin.CodeConflict)
	expect(t, e.do(http.MethodPost, "/api/deployments/pause?ns=default&name=api&paused=false", "alice", nil), http.StatusOK, "")

	expect(t, e.do(http.MethodPost, "/api/deployments/rollback?ns=default&name=api&revision=1", "alice", nil), http.StatusOK, "")
	expectAudit(t, e, "POST /api/deployments/rollback", "deployment/default/api", http.StatusOK, `{"revision":"1"}`)
	if img := e.deployment("api").Spec.Template.Spec.Containers[0].Image; img != "api:1" {
		t.Fatalf("image after rollback = %s", img)
	}
}

func TestRestartAndScaleThroughTheAPI(t *testing.T) {
	e := newTestEnv(t, nil)
	e.add(&appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: "db", Namespace: "default"}})
	dyn := e.dynamicFake()
	statefulSets := schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "statefulsets"}
	if err := dyn.Tracker().Add(&unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "apps/v1", "kind": "StatefulSet",
		"metadata": map[string]any{"name": "db", "namespace": "default"},
		"spec":     map[string]any{"replicas": int64(1)},
	}}); err != nil {
		t.Fatal(err)
	}

	expect(t, e.do(http.MethodPost, "/api/workloads/restart?kind=StatefulSet&ns=default&name=db", "alice", nil), http.StatusOK, "")
	expectAudit(t, e, "POST /api/workloads/restart", "statefulset/default/db", http.StatusOK, "")
	s, err := e.cs.AppsV1().StatefulSets("default").Get(context.Background(), "db", metav1.GetOptions{})
	if err != nil || s.Spec.Template.Annotations["kubectl.kubernetes.io/restartedAt"] == "" {
		t.Fatalf("restart: %+v, %v", s, err)
	}
	// The old Deployment route restarts the same way.
	expect(t, e.do(http.MethodPost, "/api/deployments/restart?ns=default&name=web", "alice", nil), http.StatusOK, "")
	if e.deployment("web").Spec.Template.Annotations["kubectl.kubernetes.io/restartedAt"] == "" {
		t.Fatal("legacy restart")
	}

	expect(t, e.do(http.MethodPost, "/api/workloads/scale?kind=StatefulSet&ns=default&name=db&replicas=3", "alice", nil), http.StatusOK, "")
	expectAudit(t, e, "POST /api/workloads/scale", "statefulset/default/db", http.StatusOK, `{"replicas":"3"}`)
	obj, err := dyn.Resource(statefulSets).Namespace("default").Get(context.Background(), "db", metav1.GetOptions{})
	if n, _, _ := unstructured.NestedInt64(obj.Object, "spec", "replicas"); err != nil || n != 3 {
		t.Fatalf("replicas = %d, %v", n, err)
	}

	for _, target := range []string{
		"/api/workloads/restart?kind=ReplicaSet&ns=default&name=db",
		"/api/workloads/restart?ns=default&name=db",
		"/api/workloads/scale?kind=DaemonSet&ns=default&name=db&replicas=1",
		"/api/workloads/scale?kind=StatefulSet&ns=default&name=db&replicas=-1",
		"/api/workloads/scale?kind=StatefulSet&ns=default&name=db&replicas=many",
		"/api/workloads/scale?kind=StatefulSet&ns=default&name=db&replicas=99999999999",
		"/api/workloads/scale?kind=StatefulSet&name=db&replicas=1",
	} {
		expect(t, e.do(http.MethodPost, target, "alice", nil), http.StatusBadRequest, plugin.CodeBadRequest)
	}
	// With an apiVersion the kind is resolved through discovery.
	e.cs.Fake.Resources = kubetest.Resources(kubetest.Kinds)
	expect(t, e.do(http.MethodPost, "/api/workloads/scale?apiVersion=example.com/v1&kind=Gadget&ns=default&name=g&replicas=1", "alice", nil),
		http.StatusNotFound, plugin.CodeNotFound)
}

func TestCronJobsThroughTheAPI(t *testing.T) {
	e := newTestEnv(t, nil)
	e.add(&batchv1.CronJob{
		ObjectMeta: metav1.ObjectMeta{Name: "report", Namespace: "default", UID: "report-uid"},
		Spec: batchv1.CronJobSpec{Schedule: "@daily", JobTemplate: batchv1.JobTemplateSpec{Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "r", Image: "r:1"}}}},
		}}},
	})

	run := decode[kube.CronJobRun](t, e.do(http.MethodPost, "/api/cronjobs/trigger?ns=default&name=report", "alice", nil))
	if !strings.HasPrefix(run.Job, "report-manual-") {
		t.Fatalf("run = %+v", run)
	}
	expectAudit(t, e, "POST /api/cronjobs/trigger", "cronjob/default/report", http.StatusOK, `{"job":"`+run.Job+`"}`)
	if _, err := e.cs.BatchV1().Jobs("default").Get(context.Background(), run.Job, metav1.GetOptions{}); err != nil {
		t.Fatal(err)
	}
	expect(t, e.do(http.MethodPost, "/api/cronjobs/trigger?ns=default&name=ghost", "alice", nil), http.StatusNotFound, plugin.CodeKubernetes)

	expect(t, e.do(http.MethodPost, "/api/cronjobs/suspend?ns=default&name=report", "alice", nil), http.StatusOK, "")
	cj, err := e.cs.BatchV1().CronJobs("default").Get(context.Background(), "report", metav1.GetOptions{})
	if err != nil || cj.Spec.Suspend == nil || !*cj.Spec.Suspend {
		t.Fatalf("suspend: %+v, %v", cj, err)
	}
	expectAudit(t, e, "POST /api/cronjobs/suspend", "cronjob/default/report", http.StatusOK, "")
	expect(t, e.do(http.MethodPost, "/api/cronjobs/suspend?ns=default&name=report&suspend=perhaps", "alice", nil), http.StatusBadRequest, plugin.CodeBadRequest)
}
