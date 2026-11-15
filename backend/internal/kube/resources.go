package kube

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/kubectl/pkg/scheme"
	"sigs.k8s.io/yaml"
)

func (r *Runtime) ListNodes(ctx context.Context) ([]NodeInfo, error) {
	list, err := r.CS.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	out := []NodeInfo{}
	for i := range list.Items {
		out = append(out, MapNode(&list.Items[i]))
	}
	return out, nil
}

func (r *Runtime) ListNamespaces(ctx context.Context) ([]NamespaceInfo, error) {
	list, err := r.CS.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	out := []NamespaceInfo{}
	for i := range list.Items {
		out = append(out, MapNamespace(&list.Items[i]))
	}
	return out, nil
}

func (r *Runtime) ListPods(ctx context.Context, namespace string) ([]PodInfo, error) {
	list, err := r.CS.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	out := []PodInfo{}
	for i := range list.Items {
		out = append(out, MapPod(&list.Items[i]))
	}
	return out, nil
}

func (r *Runtime) ListDeployments(ctx context.Context, namespace string) ([]DeploymentInfo, error) {
	list, err := r.CS.AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	out := []DeploymentInfo{}
	for i := range list.Items {
		out = append(out, MapDeployment(&list.Items[i]))
	}
	return out, nil
}

func (r *Runtime) ListServices(ctx context.Context, namespace string) ([]ServiceInfo, error) {
	list, err := r.CS.CoreV1().Services(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	out := []ServiceInfo{}
	for i := range list.Items {
		out = append(out, MapService(&list.Items[i]))
	}
	return out, nil
}

// ListEvents lists the events in namespace ("" = all) about objects of the
// given kind and name; empty filters match everything. Filtering on the
// kind keeps e.g. Deployment "web" and Service "web" apart.
func (r *Runtime) ListEvents(ctx context.Context, namespace, kind, name string) ([]EventInfo, error) {
	var terms []fields.Selector
	if kind != "" {
		terms = append(terms, fields.OneTermEqualSelector("involvedObject.kind", kind))
	}
	if name != "" {
		terms = append(terms, fields.OneTermEqualSelector("involvedObject.name", name))
	}
	opts := metav1.ListOptions{FieldSelector: fields.AndSelectors(terms...).String()}
	list, err := r.CS.CoreV1().Events(namespace).List(ctx, opts)
	if err != nil {
		return nil, err
	}
	out := []EventInfo{}
	for i := range list.Items {
		out = append(out, MapEvent(&list.Items[i]))
	}
	return out, nil
}

// ObjectYAML fetches a typed object (or a custom resource by kind) and
// renders it as YAML with managedFields stripped.
func (r *Runtime) ObjectYAML(ctx context.Context, kind, namespace, name string) (string, error) {
	var obj runtime.Object
	var err error
	switch kind {
	case "Pod":
		obj, err = r.CS.CoreV1().Pods(namespace).Get(ctx, name, metav1.GetOptions{})
	case "Deployment":
		obj, err = r.CS.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
	case "Service":
		obj, err = r.CS.CoreV1().Services(namespace).Get(ctx, name, metav1.GetOptions{})
	case "Node":
		obj, err = r.CS.CoreV1().Nodes().Get(ctx, name, metav1.GetOptions{})
	case "Namespace":
		obj, err = r.CS.CoreV1().Namespaces().Get(ctx, name, metav1.GetOptions{})
	default:
		// Catalog kinds (configmaps, secrets, ingresses, …) via dynamic get.
		if rt, ok := catalogByKind[kind]; ok {
			object, gerr := r.GetGeneric(ctx, rt.Key, namespace, name) // redacts secrets
			if gerr != nil {
				return "", gerr
			}
			if meta, ok := object["metadata"].(map[string]any); ok {
				delete(meta, "managedFields")
			}
			out, merr := yaml.Marshal(object)
			return string(out), merr
		}
		unstr, cerr := r.GetCustomResourceByKind(ctx, kind, namespace, name)
		if cerr != nil {
			return "", cerr
		}
		unstr.SetManagedFields(nil)
		out, merr := yaml.Marshal(unstr.Object)
		return string(out), merr
	}
	if err != nil {
		return "", err
	}
	// Typed clients decode objects without apiVersion and kind; restore them
	// so the YAML is a complete manifest.
	if gvks, _, err := scheme.Scheme.ObjectKinds(obj); err == nil && len(gvks) > 0 {
		obj.GetObjectKind().SetGroupVersionKind(gvks[0])
	}
	if meta, ok := obj.(metav1.Object); ok {
		meta.SetManagedFields(nil)
	}
	out, err := yaml.Marshal(obj)
	return string(out), err
}

// LogOptions selects the lines PodLogs returns.
type LogOptions struct {
	Container string
	// TailLines is the number of lines from the end; it must be positive.
	TailLines int64
	// Previous reads the previous (crashed) instance of the container.
	Previous bool
	// SinceSeconds limits the lines to that many seconds back; 0 = no limit.
	SinceSeconds int64
}

// maxLogBytes bounds one PodLogs response, whatever the line lengths.
const maxLogBytes = 16 << 20

// PodLogs returns log lines with their timestamps shortened to
// HH:MM:SS.mmm.
func (r *Runtime) PodLogs(ctx context.Context, namespace, pod string, o LogOptions) ([]string, error) {
	limitBytes := int64(maxLogBytes)
	opts := &corev1.PodLogOptions{
		Container:  o.Container,
		TailLines:  &o.TailLines,
		Previous:   o.Previous,
		Timestamps: true,
		LimitBytes: &limitBytes,
	}
	if o.SinceSeconds > 0 {
		opts.SinceSeconds = &o.SinceSeconds
	}
	// Do + Result.Error (unlike DoRaw) decodes the API server's Status, so
	// e.g. "previous terminated container ... not found" reaches the UI.
	res := r.CS.CoreV1().Pods(namespace).GetLogs(pod, opts).Do(ctx)
	raw, err := res.Raw()
	if err != nil {
		return nil, res.Error()
	}
	lines := []string{}
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if space := strings.IndexByte(line, ' '); space > 0 {
			ts := line[:space]
			if t, err := time.Parse(time.RFC3339Nano, ts); err == nil {
				line = t.Format("15:04:05.000") + " " + line[space+1:]
			}
		}
		lines = append(lines, line)
	}
	return lines, nil
}

func (r *Runtime) ScaleDeployment(ctx context.Context, namespace, name string, replicas int) error {
	patch := fmt.Sprintf(`{"spec":{"replicas":%d}}`, replicas)
	_, err := r.CS.AppsV1().Deployments(namespace).Patch(
		ctx, name, types.MergePatchType, []byte(patch), metav1.PatchOptions{},
	)
	return err
}

// Exec runs one command through sh -c and returns combined output lines.
func (r *Runtime) Exec(ctx context.Context, namespace, pod, container, command string) ([]string, error) {
	req := r.CS.CoreV1().RESTClient().Post().
		Resource("pods").Namespace(namespace).Name(pod).SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Container: container,
			Command:   []string{"sh", "-c", command},
			Stdout:    true,
			Stderr:    true,
		}, scheme.ParameterCodec)

	executor, err := remotecommand.NewSPDYExecutor(r.Config, "POST", req.URL())
	if err != nil {
		return nil, err
	}
	var stdout, stderr bytes.Buffer
	execCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	err = executor.StreamWithContext(execCtx, remotecommand.StreamOptions{
		Stdout: &stdout, Stderr: &stderr,
	})
	output := stdout.String()
	if stderr.Len() > 0 {
		output += stderr.String()
	}
	if err != nil && output == "" {
		output = err.Error()
	}
	lines := strings.Split(strings.TrimRight(output, "\n"), "\n")
	return lines, nil
}
