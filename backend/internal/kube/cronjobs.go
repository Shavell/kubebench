package kube

import (
	"context"
	"fmt"
	"strings"

	batchv1 "k8s.io/api/batch/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/rand"
)

const (
	// instantiateAnnotation marks a Job created by hand from a CronJob, as
	// kubectl create job --from=cronjob/<name> does.
	instantiateAnnotation = "cronjob.kubernetes.io/instantiate"
	// maxJobName keeps a Job's name usable as the value of the job-name
	// label its pods get.
	maxJobName = 63
	// nameAttempts bounds the retries when a random Job name is taken.
	nameAttempts = 3
)

// CronJobRun names the Job TriggerCronJob created.
type CronJobRun struct {
	Namespace string `json:"namespace"`
	Job       string `json:"job"`
}

// TriggerCronJob runs a CronJob now: it creates a Job from the CronJob's
// jobTemplate, named "<cronjob>-manual-xxxxx" and owned by the CronJob,
// with the annotation cronjob.kubernetes.io/instantiate=manual (kubectl
// create job --from=cronjob/<name>). It works while the CronJob is
// suspended, too.
func (r *Runtime) TriggerCronJob(ctx context.Context, namespace, name string) (*CronJobRun, error) {
	cronJob, err := r.CS.BatchV1().CronJobs(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	jobs := r.CS.BatchV1().Jobs(namespace)
	for attempt := 1; ; attempt++ {
		created, err := jobs.Create(ctx, jobFromCronJob(cronJob, manualJobName(name)), metav1.CreateOptions{})
		if err == nil {
			return &CronJobRun{Namespace: namespace, Job: created.Name}, nil
		}
		if !apierrors.IsAlreadyExists(err) || attempt == nameAttempts {
			return nil, err
		}
	}
}

// jobFromCronJob is the Job kubectl create job --from=cronjob builds: the
// template's spec, labels and annotations plus the manual marker, owned by
// the CronJob (as its controller, without blockOwnerDeletion, which would
// need permission on cronjobs/finalizers).
func jobFromCronJob(cronJob *batchv1.CronJob, name string) *batchv1.Job {
	annotations := map[string]string{instantiateAnnotation: "manual"}
	for k, v := range cronJob.Spec.JobTemplate.Annotations {
		annotations[k] = v
	}
	controller := true
	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   cronJob.Namespace,
			Labels:      cronJob.Spec.JobTemplate.Labels,
			Annotations: annotations,
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion: batchv1.SchemeGroupVersion.String(),
				Kind:       "CronJob",
				Name:       cronJob.Name,
				UID:        cronJob.UID,
				Controller: &controller,
			}},
		},
		Spec: *cronJob.Spec.JobTemplate.Spec.DeepCopy(),
	}
}

// manualJobName is "<cronjob>-manual-xxxxx", shortening the CronJob's name
// so the whole stays within 63 characters and ends a DNS label cleanly.
func manualJobName(cronJob string) string {
	suffix := "-manual-" + rand.String(5)
	base := cronJob
	if keep := maxJobName - len(suffix); len(base) > keep {
		base = strings.TrimRight(base[:keep], "-.")
	}
	return base + suffix
}

// SuspendCronJob suspends a CronJob's schedule, or resumes it. Jobs that
// already run are not affected.
func (r *Runtime) SuspendCronJob(ctx context.Context, namespace, name string, suspend bool) error {
	patch := fmt.Sprintf(`{"spec":{"suspend":%t}}`, suspend)
	_, err := r.CS.BatchV1().CronJobs(namespace).Patch(ctx, name, types.MergePatchType, []byte(patch), metav1.PatchOptions{})
	return err
}
