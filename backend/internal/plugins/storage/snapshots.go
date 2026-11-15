package storage

import (
	"context"
	"errors"
	"net/http"
	"sort"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"kubebench/internal/kube"
	"kubebench/internal/plugin"
)

// snapshotGroup is the API group of the external-snapshotter CRDs.
const snapshotGroup = "snapshot.storage.k8s.io"

// errNoSnapshots: the cluster does not serve VolumeSnapshots.
var errNoSnapshots = errors.New("the cluster does not serve " + snapshotGroup +
	" VolumeSnapshots: install the external-snapshotter CRDs and snapshot controller")

// Snapshot is one VolumeSnapshot in GET /snapshots.
type Snapshot struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	// SourcePVC is the claim the snapshot was taken of; SourceContent a
	// pre-provisioned VolumeSnapshotContent instead.
	SourcePVC        string `json:"sourcePVC"`
	SourceContent    string `json:"sourceContent"`
	SnapshotClass    string `json:"snapshotClass"`
	ReadyToUse       bool   `json:"readyToUse"`
	RestoreSize      string `json:"restoreSize"`
	RestoreSizeBytes int64  `json:"restoreSizeBytes"`
	Error            string `json:"error"`
	Age              string `json:"age"`
}

// snapshotResource resolves the VolumeSnapshot resource, or errNoSnapshots.
func snapshotResource(rt *kube.Runtime) (schema.GroupVersionResource, error) {
	gvr, _, err := rt.ResolveResource(snapshotGroup, "volumesnapshots")
	if meta.IsNoMatchError(err) {
		return gvr, errNoSnapshots
	}
	return gvr, err
}

// listSnapshots serves GET /snapshots?ns=: 503 capability_unavailable
// when the snapshot CRDs are not installed.
func listSnapshots(w http.ResponseWriter, r *http.Request, c *plugin.Context) {
	ns := r.URL.Query().Get("ns")
	if ns != "" {
		if err := namespaceName("ns", ns); err != nil {
			writeErr(w, err)
			return
		}
	}
	gvr, err := snapshotResource(c.Runtime)
	if errors.Is(err, errNoSnapshots) {
		plugin.WriteError(w, http.StatusServiceUnavailable, plugin.CodeCapabilityUnavailable, err)
		return
	}
	if err != nil {
		plugin.WriteK8sError(w, err)
		return
	}
	list, err := c.Runtime.Dyn.Resource(gvr).Namespace(ns).List(r.Context(), metav1.ListOptions{})
	if err != nil {
		plugin.WriteK8sError(w, err)
		return
	}
	out := make([]Snapshot, 0, len(list.Items))
	for i := range list.Items {
		out = append(out, parseSnapshot(&list.Items[i]))
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Namespace != out[j].Namespace {
			return out[i].Namespace < out[j].Namespace
		}
		return out[i].Name < out[j].Name
	})
	plugin.WriteJSON(w, out)
}

// getSnapshot reads one snapshot a claim is to be restored from; a missing
// CRD or snapshot is the request's fault.
func getSnapshot(ctx context.Context, rt *kube.Runtime, ns, name string) (Snapshot, error) {
	gvr, err := snapshotResource(rt)
	if errors.Is(err, errNoSnapshots) {
		return Snapshot{}, invalidf("dataSource.kind", "%v", err)
	}
	if err != nil {
		return Snapshot{}, err
	}
	obj, err := rt.Dyn.Resource(gvr).Namespace(ns).Get(ctx, name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return Snapshot{}, invalidf("dataSource.name", "no VolumeSnapshot %q in namespace %q", name, ns)
	}
	if err != nil {
		return Snapshot{}, err
	}
	return parseSnapshot(obj), nil
}

func parseSnapshot(obj *unstructured.Unstructured) Snapshot {
	str := func(fields ...string) string {
		v, _, _ := unstructured.NestedString(obj.Object, fields...)
		return v
	}
	ready, _, _ := unstructured.NestedBool(obj.Object, "status", "readyToUse")
	s := Snapshot{
		Namespace: obj.GetNamespace(), Name: obj.GetName(),
		SourcePVC:     str("spec", "source", "persistentVolumeClaimName"),
		SourceContent: str("spec", "source", "volumeSnapshotContentName"),
		SnapshotClass: str("spec", "volumeSnapshotClassName"),
		ReadyToUse:    ready,
		Error:         str("status", "error", "message"),
		Age:           kube.FormatAge(obj.GetCreationTimestamp().Time),
	}
	// restoreSize is a quantity; JSON may carry it as a number of bytes.
	if v, found, _ := unstructured.NestedFieldNoCopy(obj.Object, "status", "restoreSize"); found {
		var text string
		switch n := v.(type) {
		case string:
			text = n
		case int64:
			text = resource.NewQuantity(n, resource.BinarySI).String()
		case float64:
			text = resource.NewQuantity(int64(n), resource.BinarySI).String()
		}
		if q, err := resource.ParseQuantity(text); err == nil {
			s.RestoreSize, s.RestoreSizeBytes = q.String(), q.Value()
		}
	}
	return s
}
