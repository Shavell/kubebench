package kube

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/remotecommand"
	utilexec "k8s.io/client-go/util/exec"
	"k8s.io/kubectl/pkg/scheme"
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
// given kind and name, newest first; empty filters match everything.
// Filtering on the kind keeps e.g. Deployment "web" and Service "web"
// apart, on the namespace the objects of that name in other namespaces.
// uid selects one incarnation of the object; when it is empty and kind
// and name are given, the current object's UID is looked up, so events of
// a deleted object of the same name are left out. When the object cannot
// be read (gone, not allowed) its events are matched by name alone.
func (r *Runtime) ListEvents(ctx context.Context, namespace, kind, name, uid string) ([]EventInfo, error) {
	var terms []fields.Selector
	if kind != "" {
		terms = append(terms, fields.OneTermEqualSelector("involvedObject.kind", kind))
	}
	if name != "" {
		terms = append(terms, fields.OneTermEqualSelector("involvedObject.name", name))
	}
	if namespace != "" {
		terms = append(terms, fields.OneTermEqualSelector("involvedObject.namespace", namespace))
	}
	if uid == "" && kind != "" && name != "" {
		uid = r.objectUID(ctx, kind, namespace, name)
	}
	if uid != "" {
		terms = append(terms, fields.OneTermEqualSelector("involvedObject.uid", uid))
	}
	opts := metav1.ListOptions{FieldSelector: fields.AndSelectors(terms...).String()}
	list, err := r.CS.CoreV1().Events(namespace).List(ctx, opts)
	if err != nil {
		return nil, err
	}
	sort.SliceStable(list.Items, func(i, j int) bool {
		return eventLastSeen(&list.Items[i]).After(eventLastSeen(&list.Items[j]))
	})
	out := []EventInfo{}
	for i := range list.Items {
		out = append(out, MapEvent(&list.Items[i]))
	}
	return out, nil
}

// objectUID is the UID of the object, or "" when it cannot be read.
func (r *Runtime) objectUID(ctx context.Context, kind, namespace, name string) string {
	obj, err := r.objectByKind(ctx, kind, namespace, name)
	if err != nil {
		return ""
	}
	uid, _, _ := unstructured.NestedString(obj, "metadata", "uid")
	return uid
}

// objectByKind fetches an object by kind alone, as the YAML views of
// v0.4 asked for it: a typed object, a generic catalog kind (Secrets
// redacted) or a custom resource whose CRD has that kind.
func (r *Runtime) objectByKind(ctx context.Context, kind, namespace, name string) (map[string]any, error) {
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
			return r.GetGeneric(ctx, rt.Key, namespace, name) // redacts secrets
		}
		unstr, err := r.GetCustomResourceByKind(ctx, kind, namespace, name)
		if err != nil {
			return nil, err
		}
		return unstr.Object, nil
	}
	if err != nil {
		return nil, err
	}
	// Typed clients decode objects without apiVersion and kind; restore them
	// so the YAML is a complete manifest.
	if gvks, _, err := scheme.Scheme.ObjectKinds(obj); err == nil && len(gvks) > 0 {
		obj.GetObjectKind().SetGroupVersionKind(gvks[0])
	}
	return runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
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
		ctx, name, types.MergePatchType, []byte(patch), metav1.PatchOptions{FieldManager: FieldManager},
	)
	return err
}

// execTimeout bounds a one-shot Exec.
const execTimeout = 20 * time.Second

// Exec runs one command through sh -c and returns combined output lines.
// A command that ran and exited non-zero is an answer, not an error: its
// output (or, when it printed nothing, "command terminated with exit code
// N") is returned. Failing to run it (pod or container missing, RBAC, the
// upgrade refused, the time limit) is returned as the error.
func (r *Runtime) Exec(ctx context.Context, namespace, pod, container, command string) ([]string, error) {
	executor, err := r.execExecutor(&corev1.PodExecOptions{
		Container: container,
		Command:   []string{"sh", "-c", command},
		Stdout:    true,
		Stderr:    true,
	}, namespace, pod)
	if err != nil {
		return nil, err
	}
	execCtx, cancel := context.WithTimeout(ctx, execTimeout)
	defer cancel()
	return runOneShot(execCtx, executor)
}

func runOneShot(ctx context.Context, executor remotecommand.Executor) ([]string, error) {
	var stdout, stderr bytes.Buffer
	err := executor.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdout: &stdout, Stderr: &stderr,
	})
	output := stdout.String() + stderr.String()
	if err != nil {
		var exit utilexec.ExitError
		if !errors.As(err, &exit) {
			return nil, err
		}
		if output == "" {
			output = err.Error()
		}
	}
	return strings.Split(strings.TrimRight(output, "\n"), "\n"), nil
}
