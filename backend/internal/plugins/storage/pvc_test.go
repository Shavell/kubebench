package storage

import (
	"context"
	"net/http"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	k8stesting "k8s.io/client-go/testing"

	"kubebench/internal/plugin"
)

func TestRoutesAccess(t *testing.T) {
	e := newEnv(t, envOptions{})
	want := map[string]plugin.Access{
		"GET /overview":                  plugin.AccessRead,
		"GET /pvcs":                      plugin.AccessRead,
		"GET /pvs":                       plugin.AccessRead,
		"GET /csidrivers":                plugin.AccessRead,
		"GET /snapshots":                 plugin.AccessRead,
		"POST /pvcs":                     plugin.AccessWrite | plugin.DryRunnable,
		"POST /pvs":                      plugin.AccessWrite | plugin.DryRunnable,
		"POST /storageclasses":           plugin.AccessWrite | plugin.DryRunnable,
		"PATCH /pvcs/{ns}/{name}/resize": plugin.AccessWrite,
	}
	if len(e.access) != len(want) {
		t.Fatalf("routes = %v", e.access)
	}
	for route, a := range want {
		if e.access[route] != a {
			t.Errorf("%s access = %v, want %v", route, e.access[route], a)
		}
	}
}

func TestCreatePVC(t *testing.T) {
	e := newEnv(t, envOptions{}, class("fast", "hostpath.csi.k8s.io", true, true))
	body := map[string]any{
		"namespace": "apps", "name": "data", "storageClassName": "fast",
		"accessModes": []string{"RWO"}, "size": "5Gi", "volumeMode": "Filesystem",
		"labels": map[string]string{"app": "db"},
	}

	// A dry run reaches the API server as one.
	res := decode[CreateResult](t, expect(t, e.do(http.MethodPost, "/pvcs?dryRun=true", body), http.StatusOK, ""))
	if !res.DryRun || res.Name != "data" || res.Namespace != "apps" || res.Kind != "PersistentVolumeClaim" {
		t.Fatalf("dry-run result = %+v", res)
	}
	create := lastCreate(t, e)
	if opts := create.GetCreateOptions(); len(opts.DryRun) != 1 || opts.DryRun[0] != metav1.DryRunAll || opts.FieldManager != fieldManager {
		t.Fatalf("create options = %+v", opts)
	}
	e.audits = nil // the router does not record dry runs (§3.4)
	if err := e.cs.CoreV1().PersistentVolumeClaims("apps").Delete(context.Background(), "data", metav1.DeleteOptions{}); err != nil {
		t.Fatal(err) // the fake ignores dry runs
	}

	expect(t, e.do(http.MethodPost, "/pvcs", body), http.StatusOK, "")
	if opts := lastCreate(t, e).GetCreateOptions(); len(opts.DryRun) != 0 {
		t.Fatalf("real create sent as a dry run: %+v", opts)
	}
	pvc, err := e.cs.CoreV1().PersistentVolumeClaims("apps").Get(context.Background(), "data", metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if got := requested(pvc); got.String() != "5Gi" || *pvc.Spec.StorageClassName != "fast" ||
		pvc.Spec.AccessModes[0] != corev1.ReadWriteOnce || pvc.Labels["app"] != "db" ||
		*pvc.Spec.VolumeMode != corev1.PersistentVolumeFilesystem {
		t.Fatalf("created claim = %+v", pvc.Spec)
	}
	if len(e.audits) != 1 || !strings.HasPrefix(e.audits[0], `pvc/apps/data {"accessModes":["ReadWriteOnce"],"size":"5Gi","storageClass":"fast"}`) {
		t.Fatalf("audit = %v", e.audits)
	}

	// The cluster default applies when no class is named; "" asks for none.
	delete(body, "storageClassName")
	body["name"] = "defaulted"
	expect(t, e.do(http.MethodPost, "/pvcs", body), http.StatusOK, "")
	if pvc, _ := e.cs.CoreV1().PersistentVolumeClaims("apps").Get(context.Background(), "defaulted", metav1.GetOptions{}); pvc.Spec.StorageClassName != nil {
		t.Fatalf("class = %q, want unset", *pvc.Spec.StorageClassName)
	}
}

func lastCreate(t *testing.T, e *env) k8stesting.CreateActionImpl {
	t.Helper()
	actions := e.cs.Actions()
	for i := len(actions) - 1; i >= 0; i-- {
		if create, ok := actions[i].(k8stesting.CreateActionImpl); ok {
			return create
		}
	}
	t.Fatal("no create action")
	return k8stesting.CreateActionImpl{}
}

func TestCreatePVCValidation(t *testing.T) {
	e := newEnv(t, envOptions{})
	base := func() map[string]any {
		return map[string]any{"namespace": "apps", "name": "data", "accessModes": []string{"ReadWriteOnce"}, "size": "1Gi"}
	}
	cases := map[string]struct {
		change func(map[string]any)
		want   string
	}{
		"upper-case name": {func(b map[string]any) { b["name"] = "Data" }, "name:"},
		"bad namespace":   {func(b map[string]any) { b["namespace"] = "a.b" }, "namespace:"},
		"bad quantity":    {func(b map[string]any) { b["size"] = "10 gigs" }, "size:"},
		"zero size":       {func(b map[string]any) { b["size"] = "0" }, "size: must be greater than zero"},
		"no access mode":  {func(b map[string]any) { b["accessModes"] = []string{} }, "accessModes:"},
		"unknown mode":    {func(b map[string]any) { b["accessModes"] = []string{"ReadWriteSome"} }, "accessModes:"},
		"duplicate mode":  {func(b map[string]any) { b["accessModes"] = []string{"RWO", "ReadWriteOnce"} }, "listed twice"},
		"RWOP mixed":      {func(b map[string]any) { b["accessModes"] = []string{"RWOP", "RWX"} }, "ReadWriteOncePod"},
		"bad volume mode": {func(b map[string]any) { b["volumeMode"] = "Raw" }, "volumeMode:"},
		"bad label":       {func(b map[string]any) { b["labels"] = map[string]string{"a b": "c"} }, "labels:"},
		"unknown field":   {func(b map[string]any) { b["sizee"] = "1Gi" }, "unknown field"},
		"bind and clone": {func(b map[string]any) {
			b["volumeName"] = "pv1"
			b["dataSource"] = map[string]string{"kind": sourcePVC, "name": "x"}
		}, "dataSource:"},
		"bad source kind":   {func(b map[string]any) { b["dataSource"] = map[string]string{"kind": "Pod", "name": "x"} }, "dataSource.kind:"},
		"missing clone src": {func(b map[string]any) { b["dataSource"] = map[string]string{"kind": sourcePVC, "name": "gone"} }, "no PersistentVolumeClaim"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			body := base()
			tc.change(body)
			rec := e.do(http.MethodPost, "/pvcs", body)
			msg := decode[plugin.ErrorBody](t, expect(t, rec, http.StatusBadRequest, plugin.CodeBadRequest)).Error
			if !strings.Contains(msg, tc.want) {
				t.Fatalf("error %q does not contain %q", msg, tc.want)
			}
		})
	}
	expect(t, e.do(http.MethodPost, "/pvcs", "{} {}"), http.StatusBadRequest, plugin.CodeBadRequest)
	if len(e.cs.Actions()) > 1 { // the one Get of the missing clone source
		t.Fatalf("invalid requests reached the cluster: %v", e.cs.Actions())
	}
}

func TestCreatePVCBindsToAvailableVolume(t *testing.T) {
	e := newEnv(t, envOptions{},
		volume("pv-free", "manual", "10Gi", corev1.VolumeAvailable),
		volume("pv-taken", "manual", "10Gi", corev1.VolumeBound),
	)
	body := func(volumeName, class, size string, modes ...string) map[string]any {
		return map[string]any{
			"namespace": "apps", "name": "bound", "storageClassName": class,
			"accessModes": modes, "size": size, "volumeName": volumeName,
		}
	}
	for _, tc := range []struct {
		name string
		body map[string]any
		want string
	}{
		{"missing volume", body("nope", "manual", "1Gi", "RWO"), "no PersistentVolume"},
		{"bound volume", body("pv-taken", "manual", "1Gi", "RWO"), "is Bound, not Available"},
		{"other class", body("pv-free", "fast", "1Gi", "RWO"), "storageClassName: must be the volume's class (manual)"},
		{"too large", body("pv-free", "manual", "20Gi", "RWO"), "holds only 10Gi"},
		{"mode not offered", body("pv-free", "manual", "1Gi", "RWX"), "does not offer ReadWriteMany"},
	} {
		msg := decode[plugin.ErrorBody](t, expect(t, e.do(http.MethodPost, "/pvcs", tc.body), http.StatusBadRequest, plugin.CodeBadRequest)).Error
		if !strings.Contains(msg, tc.want) {
			t.Errorf("%s: error %q does not contain %q", tc.name, msg, tc.want)
		}
	}
	expect(t, e.do(http.MethodPost, "/pvcs", body("pv-free", "manual", "10Gi", "RWO", "ROX")), http.StatusOK, "")
	pvc, err := e.cs.CoreV1().PersistentVolumeClaims("apps").Get(context.Background(), "bound", metav1.GetOptions{})
	if err != nil || pvc.Spec.VolumeName != "pv-free" {
		t.Fatalf("claim = %+v, %v", pvc, err)
	}
}

func TestCreatePVCClones(t *testing.T) {
	e := newEnv(t, envOptions{
		withSnapshotCRD: true,
		snapshots: []*unstructured.Unstructured{
			snapshot("apps", "snap", "src", "4Gi", true),
			snapshot("apps", "pending", "src", "4Gi", false),
		},
	}, claim("apps", "src", "fast", "4Gi", corev1.ClaimBound))
	body := func(name, kind, source, size string) map[string]any {
		return map[string]any{
			"namespace": "apps", "name": name, "accessModes": []string{"RWO"}, "size": size,
			"storageClassName": "fast", "dataSource": map[string]string{"kind": kind, "name": source},
		}
	}

	msg := decode[plugin.ErrorBody](t, expect(t, e.do(http.MethodPost, "/pvcs", body("c1", sourcePVC, "src", "1Gi")), http.StatusBadRequest, plugin.CodeBadRequest)).Error
	if !strings.Contains(msg, "at least as large as its source (4Gi)") {
		t.Fatalf("small clone: %q", msg)
	}
	expect(t, e.do(http.MethodPost, "/pvcs", body("c1", sourcePVC, "src", "4Gi")), http.StatusOK, "")
	c1, _ := e.cs.CoreV1().PersistentVolumeClaims("apps").Get(context.Background(), "c1", metav1.GetOptions{})
	if ds := c1.Spec.DataSource; ds == nil || ds.Kind != sourcePVC || ds.Name != "src" || ds.APIGroup != nil {
		t.Fatalf("clone data source = %+v", ds)
	}

	msg = decode[plugin.ErrorBody](t, expect(t, e.do(http.MethodPost, "/pvcs", body("r1", sourceSnapshot, "snap", "2Gi")), http.StatusBadRequest, plugin.CodeBadRequest)).Error
	if !strings.Contains(msg, "at least as large as the snapshot (4Gi)") {
		t.Fatalf("small restore: %q", msg)
	}
	expect(t, e.do(http.MethodPost, "/pvcs", body("r1", sourceSnapshot, "missing", "4Gi")), http.StatusBadRequest, plugin.CodeBadRequest)
	expect(t, e.do(http.MethodPost, "/pvcs", body("r1", sourceSnapshot, "snap", "5Gi")), http.StatusOK, "")
	r1, _ := e.cs.CoreV1().PersistentVolumeClaims("apps").Get(context.Background(), "r1", metav1.GetOptions{})
	if ds := r1.Spec.DataSource; ds == nil || ds.Kind != sourceSnapshot || ds.APIGroup == nil || *ds.APIGroup != snapshotGroup {
		t.Fatalf("restore data source = %+v", ds)
	}

	res := decode[CreateResult](t, expect(t, e.do(http.MethodPost, "/pvcs", body("r2", sourceSnapshot, "pending", "4Gi")), http.StatusOK, ""))
	if len(res.Warnings) != 1 || !strings.Contains(res.Warnings[0], "not ready") ||
		len(res.WarningReasons) != 1 || res.WarningReasons[0].Code != "storage.snapshot_not_ready" {
		t.Fatalf("warnings = %v %v", res.Warnings, res.WarningReasons)
	}
}

func TestRestoreNeedsSnapshotCRD(t *testing.T) {
	e := newEnv(t, envOptions{})
	body := map[string]any{
		"namespace": "apps", "name": "r", "accessModes": []string{"RWO"}, "size": "1Gi",
		"dataSource": map[string]string{"kind": sourceSnapshot, "name": "snap"},
	}
	msg := decode[plugin.ErrorBody](t, expect(t, e.do(http.MethodPost, "/pvcs", body), http.StatusBadRequest, plugin.CodeBadRequest)).Error
	if !strings.Contains(msg, "does not serve snapshot.storage.k8s.io") {
		t.Fatalf("error = %q", msg)
	}
}
