package kube

import (
	"context"
	"regexp"
	"strings"
	"testing"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8stesting "k8s.io/client-go/testing"
)

func cronJob(name string) *batchv1.CronJob {
	return &batchv1.CronJob{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "shop", UID: "cj-uid"},
		Spec: batchv1.CronJobSpec{
			Schedule: "0 2 * * *",
			JobTemplate: batchv1.JobTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      map[string]string{"app": "report"},
					Annotations: map[string]string{"owner": "team-shop"},
				},
				Spec: batchv1.JobSpec{Template: corev1.PodTemplateSpec{Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					Containers:    []corev1.Container{{Name: "report", Image: "report:1"}},
				}}},
			},
		},
	}
}

func TestTriggerCronJob(t *testing.T) {
	rt, cs := opsRuntime(t, cronJob("nightly-report"))
	ctx := context.Background()
	run, err := rt.TriggerCronJob(ctx, "shop", "nightly-report")
	if err != nil {
		t.Fatal(err)
	}
	if !regexp.MustCompile(`^nightly-report-manual-[a-z0-9]{5}$`).MatchString(run.Job) || run.Namespace != "shop" {
		t.Fatalf("run = %+v", run)
	}
	job, err := cs.BatchV1().Jobs("shop").Get(ctx, run.Job, metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if job.Annotations[instantiateAnnotation] != "manual" || job.Annotations["owner"] != "team-shop" || job.Labels["app"] != "report" {
		t.Fatalf("job metadata = %+v", job.ObjectMeta)
	}
	owner := metav1.GetControllerOf(job)
	if owner == nil || owner.Kind != "CronJob" || owner.Name != "nightly-report" || owner.UID != "cj-uid" || owner.APIVersion != "batch/v1" {
		t.Fatalf("owner = %+v", owner)
	}
	if job.Spec.Template.Spec.Containers[0].Image != "report:1" {
		t.Fatalf("the job runs the CronJob's template: %+v", job.Spec)
	}

	if _, err := rt.TriggerCronJob(ctx, "shop", "ghost"); !apierrors.IsNotFound(err) {
		t.Fatalf("unknown CronJob: %v", err)
	}
}

func TestTriggerCronJobRetriesATakenName(t *testing.T) {
	rt, cs := opsRuntime(t, cronJob("nightly-report"))
	collisions := 1
	cs.PrependReactor("create", "jobs", func(k8stesting.Action) (bool, runtime.Object, error) {
		if collisions == 0 {
			return false, nil, nil
		}
		collisions--
		return true, nil, apierrors.NewAlreadyExists(schema.GroupResource{Group: "batch", Resource: "jobs"}, "taken")
	})
	if _, err := rt.TriggerCronJob(context.Background(), "shop", "nightly-report"); err != nil {
		t.Fatalf("a name collision is retried: %v", err)
	}
}

func TestManualJobName(t *testing.T) {
	long := strings.Repeat("a", 45) + "-" + strings.Repeat("b", 6) // 52 characters, the CronJob limit
	for _, name := range []string{"x", long, strings.Repeat("c", 49) + "-zz", strings.Repeat("d", 49) + ".e"} {
		got := manualJobName(name)
		if len(got) > maxJobName {
			t.Errorf("manualJobName(%q) = %q is longer than %d", name, got, maxJobName)
		}
		base, _, ok := strings.Cut(got, "-manual-")
		if !ok || strings.HasSuffix(base, "-") || strings.HasSuffix(base, ".") || !strings.HasPrefix(name, base) {
			t.Errorf("manualJobName(%q) = %q", name, got)
		}
	}
}

func TestSuspendCronJob(t *testing.T) {
	rt, cs := opsRuntime(t, cronJob("nightly-report"))
	ctx := context.Background()
	suspended := func() bool {
		cj, err := cs.BatchV1().CronJobs("shop").Get(ctx, "nightly-report", metav1.GetOptions{})
		if err != nil {
			t.Fatal(err)
		}
		return cj.Spec.Suspend != nil && *cj.Spec.Suspend
	}
	if err := rt.SuspendCronJob(ctx, "shop", "nightly-report", true); err != nil || !suspended() {
		t.Fatalf("suspend: %v", err)
	}
	if err := rt.SuspendCronJob(ctx, "shop", "nightly-report", false); err != nil || suspended() {
		t.Fatalf("resume: %v", err)
	}
}
