package storage

import (
	"net/http"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"kubebench/internal/plugin"
)

func TestOverview(t *testing.T) {
	older := class("standard", "rancher.io/local-path", false, true)
	older.CreationTimestamp = metav1.NewTime(time.Now().Add(-time.Hour))
	newer := class("fast", "hostpath.csi.k8s.io", true, true)
	newer.CreationTimestamp = metav1.NewTime(time.Now())
	bound := volume("pv-1", "standard", "10Gi", corev1.VolumeBound)
	free := volume("pv-2", "standard", "5Gi", corev1.VolumeAvailable)
	e := newEnv(t, envOptions{withSnapshotCRD: true}, older, newer, bound, free,
		claim("a", "one", "standard", "10Gi", corev1.ClaimBound),
		claim("a", "two", "fast", "2Gi", corev1.ClaimPending),
		claim("b", "three", "-", "1Gi", corev1.ClaimLost),
		&storagev1.CSIDriver{ObjectMeta: metav1.ObjectMeta{Name: "hostpath.csi.k8s.io"}},
	)
	got := decode[Overview](t, expect(t, e.do(http.MethodGet, "/overview", nil), http.StatusOK, ""))
	if got.DefaultClass != "fast" {
		t.Errorf("default class = %q, want the newest default", got.DefaultClass)
	}
	if got.PVCs != (ClaimCounts{Total: 3, Bound: 1, Pending: 1, Lost: 1}) {
		t.Errorf("claims = %+v", got.PVCs)
	}
	if got.PVs != (VolumeCounts{Total: 2, Available: 1, Bound: 1}) {
		t.Errorf("volumes = %+v", got.PVs)
	}
	const gi = int64(1) << 30
	if got.Capacity != (Capacity{RequestedBytes: 13 * gi, ProvisionedBytes: 15 * gi, AvailableBytes: 5 * gi}) {
		t.Errorf("capacity = %+v", got.Capacity)
	}
	if got.CSIDrivers != 1 || !got.SnapshotsAvailable {
		t.Errorf("drivers = %d, snapshots = %v", got.CSIDrivers, got.SnapshotsAvailable)
	}
	byName := map[string]ClassSummary{}
	for _, c := range got.Classes {
		byName[c.Name] = c
	}
	std := byName["standard"]
	if std.PVCs != 1 || std.PVs != 2 || std.RequestedBytes != 10*gi || !std.IsDefault || std.ReclaimPolicy != "Delete" || std.VolumeBindingMode != "Immediate" {
		t.Errorf("standard = %+v", std)
	}
	if fast := byName["fast"]; !fast.AllowVolumeExpansion || fast.PVCs != 1 {
		t.Errorf("fast = %+v", fast)
	}

	e = newEnv(t, envOptions{})
	if got := decode[Overview](t, expect(t, e.do(http.MethodGet, "/overview", nil), http.StatusOK, "")); got.SnapshotsAvailable || got.Classes == nil {
		t.Errorf("empty overview = %+v", got)
	}
}

func TestListPVCsAndPVs(t *testing.T) {
	resizing := claim("a", "growing", "fast", "2Gi", corev1.ClaimBound)
	resizing.Status.Conditions = []corev1.PersistentVolumeClaimCondition{{Type: corev1.PersistentVolumeClaimResizing, Status: corev1.ConditionTrue}}
	local := volume("pv-local", "", "1Gi", corev1.VolumeAvailable)
	local.Spec.HostPath = nil
	local.Spec.Local = &corev1.LocalVolumeSource{Path: "/mnt/ssd"}
	local.Spec.NodeAffinity = &corev1.VolumeNodeAffinity{Required: &corev1.NodeSelector{NodeSelectorTerms: []corev1.NodeSelectorTerm{{
		MatchExpressions: []corev1.NodeSelectorRequirement{{Key: hostnameLabel, Operator: corev1.NodeSelectorOpIn, Values: []string{"n1"}}},
	}}}}
	bound := volume("pv-bound", "fast", "2Gi", corev1.VolumeBound)
	bound.Spec.ClaimRef = &corev1.ObjectReference{Namespace: "a", Name: "growing"}
	e := newEnv(t, envOptions{}, class("fast", "x.io/y", true, false), resizing,
		claim("b", "other", "fast", "1Gi", corev1.ClaimPending), local, bound)

	pvcs := decode[[]PVCSummary](t, expect(t, e.do(http.MethodGet, "/pvcs?ns=a", nil), http.StatusOK, ""))
	if len(pvcs) != 1 || pvcs[0].Name != "growing" || !pvcs[0].Resizing || pvcs[0].ExpandBlocker != "" ||
		pvcs[0].Requested != "2Gi" || pvcs[0].CapacityBytes != 2<<30 || pvcs[0].VolumeMode != "Filesystem" {
		t.Fatalf("pvcs = %+v", pvcs)
	}
	all := decode[[]PVCSummary](t, expect(t, e.do(http.MethodGet, "/pvcs", nil), http.StatusOK, ""))
	if len(all) != 2 || all[1].ExpandBlocker != blockerNotBound || all[1].Capacity != "" {
		t.Fatalf("all pvcs = %+v", all)
	}
	expect(t, e.do(http.MethodGet, "/pvcs?ns=Bad_NS", nil), http.StatusBadRequest, plugin.CodeBadRequest)

	pvs := decode[[]PVSummary](t, expect(t, e.do(http.MethodGet, "/pvs", nil), http.StatusOK, ""))
	if len(pvs) != 2 || pvs[0].Name != "pv-bound" || pvs[0].Claim == nil || pvs[0].Claim.Name != "growing" || pvs[0].Source != "hostPath" {
		t.Fatalf("pvs = %+v", pvs)
	}
	if l := pvs[1]; l.Source != "local" || l.SourceDetail != "/mnt/ssd" || len(l.Nodes) != 1 || l.Nodes[0] != "n1" || l.Phase != "Available" {
		t.Fatalf("local pv = %+v", l)
	}
}

func TestCSIDrivers(t *testing.T) {
	no := false
	policy := storagev1.FileFSGroupPolicy
	e := newEnv(t, envOptions{},
		&storagev1.CSIDriver{ObjectMeta: metav1.ObjectMeta{Name: "b.csi"}, Spec: storagev1.CSIDriverSpec{
			AttachRequired: &no, FSGroupPolicy: &policy,
			VolumeLifecycleModes: []storagev1.VolumeLifecycleMode{storagev1.VolumeLifecyclePersistent, storagev1.VolumeLifecycleEphemeral},
		}},
		&storagev1.CSIDriver{ObjectMeta: metav1.ObjectMeta{Name: "a.csi"}},
	)
	got := decode[[]CSIDriverSummary](t, expect(t, e.do(http.MethodGet, "/csidrivers", nil), http.StatusOK, ""))
	if len(got) != 2 || got[0].Name != "a.csi" || !got[0].AttachRequired || got[0].FSGroupPolicy != "ReadWriteOnceWithFSType" || len(got[0].LifecycleModes) != 1 {
		t.Fatalf("defaults = %+v", got)
	}
	if b := got[1]; b.AttachRequired || b.FSGroupPolicy != "File" || len(b.LifecycleModes) != 2 {
		t.Fatalf("b.csi = %+v", b)
	}
}

func TestSnapshots(t *testing.T) {
	e := newEnv(t, envOptions{})
	expect(t, e.do(http.MethodGet, "/snapshots", nil), http.StatusServiceUnavailable, plugin.CodeCapabilityUnavailable)

	numeric := snapshot("b", "bytes", "db", "", false)
	numeric.Object["status"] = map[string]any{"restoreSize": int64(1 << 30), "error": map[string]any{"message": "boom"}}
	e = newEnv(t, envOptions{withSnapshotCRD: true, snapshots: []*unstructured.Unstructured{
		snapshot("a", "snap-1", "data", "4Gi", true), numeric,
	}})
	got := decode[[]Snapshot](t, expect(t, e.do(http.MethodGet, "/snapshots", nil), http.StatusOK, ""))
	if len(got) != 2 {
		t.Fatalf("snapshots = %+v", got)
	}
	if s := got[0]; s.Name != "snap-1" || s.SourcePVC != "data" || !s.ReadyToUse || s.RestoreSize != "4Gi" || s.SnapshotClass != "csi-hostpath-snapclass" {
		t.Fatalf("snap-1 = %+v", s)
	}
	if s := got[1]; s.RestoreSize != "1Gi" || s.RestoreSizeBytes != 1<<30 || s.Error != "boom" {
		t.Fatalf("numeric = %+v", s)
	}
	if only := decode[[]Snapshot](t, expect(t, e.do(http.MethodGet, "/snapshots?ns=a", nil), http.StatusOK, "")); len(only) != 1 {
		t.Fatalf("ns filter = %+v", only)
	}
}
