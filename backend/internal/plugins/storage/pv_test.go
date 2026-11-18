package storage

import (
	"context"
	"net/http"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"kubebench/internal/plugin"
)

func TestCreatePVSources(t *testing.T) {
	e := newEnv(t, envOptions{}, node("node-a"), node("node-b"))
	base := func(name string) map[string]any {
		return map[string]any{"name": name, "capacity": "20Gi", "accessModes": []string{"RWO"}, "storageClassName": "manual"}
	}

	hostPath := base("hp")
	hostPath["hostPath"] = map[string]string{"path": "/srv/data", "type": "DirectoryOrCreate"}
	local := base("loc")
	local["local"] = map[string]string{"path": "/mnt/disks/ssd1", "fsType": "ext4"}
	local["nodes"] = []string{"node-b"}
	local["volumeMode"] = "Block"
	nfs := base("share")
	nfs["accessModes"] = []string{"RWX"}
	nfs["nfs"] = map[string]any{"server": "10.0.0.5", "path": "/exports/share", "readOnly": true}
	nfs["reclaimPolicy"] = "Delete"
	csi := base("disk")
	csi["csi"] = map[string]any{"driver": "ebs.csi.aws.com", "volumeHandle": "vol-0abc", "fsType": "xfs",
		"volumeAttributes": map[string]string{"type": "gp3"}}

	for _, body := range []map[string]any{hostPath, local, nfs, csi} {
		res := decode[CreateResult](t, expect(t, e.do(http.MethodPost, "/pvs", body), http.StatusOK, ""))
		if res.Kind != "PersistentVolume" || res.Name != body["name"] {
			t.Fatalf("result = %+v", res)
		}
	}
	get := func(name string) *corev1.PersistentVolume {
		pv, err := e.cs.CoreV1().PersistentVolumes().Get(context.Background(), name, metav1.GetOptions{})
		if err != nil {
			t.Fatal(err)
		}
		return pv
	}
	if pv := get("hp"); pv.Spec.HostPath.Path != "/srv/data" || *pv.Spec.HostPath.Type != corev1.HostPathDirectoryOrCreate ||
		pv.Spec.PersistentVolumeReclaimPolicy != corev1.PersistentVolumeReclaimRetain || pv.Spec.NodeAffinity != nil {
		t.Fatalf("hostPath volume = %+v", pv.Spec)
	}
	pv := get("loc")
	if pv.Spec.Local.Path != "/mnt/disks/ssd1" || *pv.Spec.Local.FSType != "ext4" || *pv.Spec.VolumeMode != corev1.PersistentVolumeBlock {
		t.Fatalf("local volume = %+v", pv.Spec)
	}
	if got := affinityNodes(pv); len(got) != 1 || got[0] != "node-b" {
		t.Fatalf("local affinity = %v", got)
	}
	if pv := get("share"); pv.Spec.NFS.Server != "10.0.0.5" || !pv.Spec.NFS.ReadOnly || pv.Spec.PersistentVolumeReclaimPolicy != corev1.PersistentVolumeReclaimDelete {
		t.Fatalf("nfs volume = %+v", pv.Spec)
	}
	if pv := get("disk"); pv.Spec.CSI.Driver != "ebs.csi.aws.com" || pv.Spec.CSI.VolumeHandle != "vol-0abc" || pv.Spec.CSI.VolumeAttributes["type"] != "gp3" {
		t.Fatalf("csi volume = %+v", pv.Spec)
	}
	if len(e.audits) != 4 || !strings.HasPrefix(e.audits[1], `pv/loc {"capacity":"20Gi","nodes":["node-b"],"reclaimPolicy":"Retain","source":"local","storageClass":"manual"}`) {
		t.Fatalf("audits = %v", e.audits)
	}
}

func TestCreatePVValidation(t *testing.T) {
	e := newEnv(t, envOptions{}, node("node-a"))
	base := func() map[string]any {
		return map[string]any{"name": "pv", "capacity": "1Gi", "accessModes": []string{"RWO"},
			"hostPath": map[string]string{"path": "/srv"}}
	}
	cases := map[string]struct {
		change func(map[string]any)
		want   string
	}{
		"bad name":          {func(b map[string]any) { b["name"] = "-pv" }, "name:"},
		"bad capacity":      {func(b map[string]any) { b["capacity"] = "1 TB" }, "capacity:"},
		"bad reclaim":       {func(b map[string]any) { b["reclaimPolicy"] = "Recycle" }, "reclaimPolicy:"},
		"no source":         {func(b map[string]any) { delete(b, "hostPath") }, "exactly one"},
		"two sources":       {func(b map[string]any) { b["nfs"] = map[string]string{"server": "a", "path": "/b"} }, "exactly one"},
		"relative path":     {func(b map[string]any) { b["hostPath"] = map[string]string{"path": "srv"} }, "hostPath.path:"},
		"unclean path":      {func(b map[string]any) { b["hostPath"] = map[string]string{"path": "/srv/../etc"} }, "hostPath.path:"},
		"bad hostPath type": {func(b map[string]any) { b["hostPath"] = map[string]string{"path": "/srv", "type": "Dir"} }, "hostPath.type:"},
		"block hostPath":    {func(b map[string]any) { b["volumeMode"] = "Block" }, "raw block"},
		"local no nodes": {func(b map[string]any) {
			delete(b, "hostPath")
			b["local"] = map[string]string{"path": "/mnt"}
		}, "nodes:"},
		"unknown node": {func(b map[string]any) { b["nodes"] = []string{"node-z"} }, `no node "node-z"`},
		"nfs nodes": {func(b map[string]any) {
			delete(b, "hostPath")
			b["nfs"] = map[string]string{"server": "nfs.local", "path": "/x"}
			b["nodes"] = []string{"node-a"}
		}, "takes no node affinity"},
		"bad nfs server": {func(b map[string]any) {
			delete(b, "hostPath")
			b["nfs"] = map[string]string{"server": "not a host", "path": "/x"}
		}, "nfs.server:"},
		"csi no handle": {func(b map[string]any) {
			delete(b, "hostPath")
			b["csi"] = map[string]string{"driver": "pd.csi.storage.gke.io"}
		}, "csi.volumeHandle is required"},
		"csi bad driver": {func(b map[string]any) {
			delete(b, "hostPath")
			b["csi"] = map[string]string{"driver": "Bad Driver", "volumeHandle": "x"}
		}, "csi.driver:"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			body := base()
			tc.change(body)
			msg := decode[plugin.ErrorBody](t, expect(t, e.do(http.MethodPost, "/pvs", body), http.StatusBadRequest, plugin.CodeBadRequest)).Error
			if !strings.Contains(msg, tc.want) {
				t.Fatalf("error %q does not contain %q", msg, tc.want)
			}
		})
	}
	if len(e.audits) != 0 {
		t.Fatalf("refused requests audited: %v", e.audits)
	}
}

func TestCreateStorageClass(t *testing.T) {
	e := newEnv(t, envOptions{}, class("old-default", "rancher.io/local-path", false, true))
	body := map[string]any{
		"name": "gp3", "provisioner": "ebs.csi.aws.com", "parameters": map[string]string{"type": "gp3", "encrypted": "true"},
		"reclaimPolicy": "Retain", "volumeBindingMode": "WaitForFirstConsumer", "allowVolumeExpansion": true, "isDefault": true,
	}
	res := decode[CreateResult](t, expect(t, e.do(http.MethodPost, "/storageclasses", body), http.StatusOK, ""))
	if len(res.Warnings) != 1 || !strings.Contains(res.Warnings[0], "old-default") {
		t.Fatalf("warnings = %v", res.Warnings)
	}
	sc, err := e.cs.StorageV1().StorageClasses().Get(context.Background(), "gp3", metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if sc.Provisioner != "ebs.csi.aws.com" || sc.Parameters["type"] != "gp3" || *sc.ReclaimPolicy != corev1.PersistentVolumeReclaimRetain ||
		string(*sc.VolumeBindingMode) != "WaitForFirstConsumer" || !*sc.AllowVolumeExpansion || !isDefaultClass(sc) {
		t.Fatalf("class = %+v", sc)
	}

	// Defaults: Delete, Immediate, no expansion.
	expect(t, e.do(http.MethodPost, "/storageclasses", map[string]any{"name": "plain", "provisioner": "kubernetes.io/no-provisioner"}), http.StatusOK, "")
	plain, _ := e.cs.StorageV1().StorageClasses().Get(context.Background(), "plain", metav1.GetOptions{})
	if *plain.ReclaimPolicy != corev1.PersistentVolumeReclaimDelete || string(*plain.VolumeBindingMode) != "Immediate" || plain.AllowVolumeExpansion != nil || isDefaultClass(plain) {
		t.Fatalf("defaults = %+v", plain)
	}

	for field, b := range map[string]map[string]any{
		"name:":              {"name": "GP3", "provisioner": "x.io/y"},
		"provisioner:":       {"name": "a", "provisioner": "not valid!"},
		"reclaimPolicy:":     {"name": "a", "provisioner": "x.io/y", "reclaimPolicy": "Keep"},
		"volumeBindingMode:": {"name": "a", "provisioner": "x.io/y", "volumeBindingMode": "Later"},
		"parameters:":        {"name": "a", "provisioner": "x.io/y", "parameters": map[string]string{"": "v"}},
		"mountOptions:":      {"name": "a", "provisioner": "x.io/y", "mountOptions": []string{"a b"}},
	} {
		msg := decode[plugin.ErrorBody](t, expect(t, e.do(http.MethodPost, "/storageclasses", b), http.StatusBadRequest, plugin.CodeBadRequest)).Error
		if !strings.HasPrefix(msg, field) {
			t.Errorf("error %q, want field %s", msg, field)
		}
	}
}

// The node affinity matches kubernetes.io/hostname, which need not be the
// node's name: the volume is pinned to the label of the node picked.
func TestCreatePVPinsHostnameLabel(t *testing.T) {
	renamed := node("ip-10-0-1-5.ec2.internal")
	renamed.Labels = map[string]string{hostnameLabel: "ip-10-0-1-5"}
	e := newEnv(t, envOptions{}, renamed, node("plain"))
	body := map[string]any{
		"name": "loc", "capacity": "1Gi", "accessModes": []string{"RWO"}, "storageClassName": "manual",
		"local": map[string]string{"path": "/mnt/ssd"}, "nodes": []string{"ip-10-0-1-5.ec2.internal", "plain"},
	}
	expect(t, e.do(http.MethodPost, "/pvs", body), http.StatusOK, "")
	pv, err := e.cs.CoreV1().PersistentVolumes().Get(context.Background(), "loc", metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if got := affinityNodes(pv); len(got) != 2 || got[0] != "ip-10-0-1-5" || got[1] != "plain" {
		t.Fatalf("affinity = %v", got)
	}
}
