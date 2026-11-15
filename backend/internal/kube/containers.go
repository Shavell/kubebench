package kube

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// defaultContainerAnnotation names the container kubectl exec and kubectl
// logs pick when none is given.
const defaultContainerAnnotation = "kubectl.kubernetes.io/default-container"

// PodContainer resolves the container of a pod that an exec session or a
// log stream addresses: container itself when the pod has it, or the pod's
// default container when container is empty.
func (r *Runtime) PodContainer(ctx context.Context, namespace, pod, container string) (string, error) {
	p, err := r.CS.CoreV1().Pods(namespace).Get(ctx, pod, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	return containerOf(p, container)
}

// containerOf is container when pod has one of that name (regular, init
// and ephemeral containers alike, so debug containers qualify), else a
// Kubernetes-style bad request. An empty container selects the default.
func containerOf(pod *corev1.Pod, container string) (string, error) {
	if container == "" {
		if name := defaultContainer(pod); name != "" {
			return name, nil
		}
		return "", apierrors.NewBadRequest(fmt.Sprintf("pod %q has no containers", pod.Name))
	}
	if !hasContainer(pod, container) {
		return "", apierrors.NewBadRequest(fmt.Sprintf("container %q is not valid for pod %q", container, pod.Name))
	}
	return container, nil
}

// defaultContainer is the container the annotation names when the pod has
// it, else the first container, as in kubectl.
func defaultContainer(pod *corev1.Pod) string {
	if name := pod.Annotations[defaultContainerAnnotation]; name != "" && hasContainer(pod, name) {
		return name
	}
	if len(pod.Spec.Containers) > 0 {
		return pod.Spec.Containers[0].Name
	}
	return ""
}

func hasContainer(pod *corev1.Pod, name string) bool {
	for _, c := range pod.Spec.Containers {
		if c.Name == name {
			return true
		}
	}
	for _, c := range pod.Spec.InitContainers {
		if c.Name == name {
			return true
		}
	}
	for _, c := range pod.Spec.EphemeralContainers {
		if c.Name == name {
			return true
		}
	}
	return false
}
