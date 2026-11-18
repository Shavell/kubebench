package security

import (
	"errors"
	"slices"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	k8stesting "k8s.io/client-go/testing"

	"kubebench/internal/openshift"
)

// scc is an SCC object as the API serves it; shipped ones carry the
// release annotation.
func scc(name string, shipped bool, fields map[string]any) *unstructured.Unstructured {
	meta := map[string]any{"name": name}
	if shipped {
		meta["annotations"] = map[string]any{"include.release.openshift.io/self-managed-high-availability": "true"}
	}
	obj := map[string]any{"apiVersion": "security.openshift.io/v1", "kind": "SecurityContextConstraints", "metadata": meta}
	for k, v := range fields {
		obj[k] = v
	}
	return &unstructured.Unstructured{Object: obj}
}

func admittedPod(ns, name, owner, ownerKind, sccName string) *corev1.Pod {
	m := meta(ns, name)
	if owner != "" {
		m = owned(m, ownerKind, owner)
	}
	m.Annotations = map[string]string{openshift.SCCAnnotation: sccName}
	return &corev1.Pod{ObjectMeta: m, Spec: compliantSpec()}
}

func roleBinding(ns, name, role string, subjects ...rbacv1.Subject) *rbacv1.RoleBinding {
	return &rbacv1.RoleBinding{ObjectMeta: meta(ns, name), RoleRef: rbacv1.RoleRef{Kind: "ClusterRole", Name: role}, Subjects: subjects}
}

func useSCC(names ...string) rbacv1.PolicyRule {
	return rbacv1.PolicyRule{APIGroups: []string{"security.openshift.io"}, Resources: []string{"securitycontextconstraints"},
		Verbs: []string{"use"}, ResourceNames: names}
}

// crc is a small OpenShift cluster: platform namespaces (by name and by
// label), the default SCCs plus a custom privileged one, pods admitted with
// them, and SCC grants by RBAC and by the SCCs' own users field.
func crc() ([]*unstructured.Unstructured, []runtime.Object) {
	sccs := []*unstructured.Unstructured{
		scc("privileged", true, map[string]any{"allowPrivilegedContainer": true, "allowHostNetwork": true, "runAsUser": map[string]any{"type": "RunAsAny"},
			"users":  []any{"system:admin", "system:serviceaccount:openshift-infra:build-controller", "system:serviceaccount:shop:ci"},
			"groups": []any{"system:cluster-admins", "devs"}}),
		scc("anyuid", true, map[string]any{"runAsUser": map[string]any{"type": "RunAsAny"}, "groups": []any{"system:cluster-admins"}}),
		scc("restricted-v2", true, map[string]any{"runAsUser": map[string]any{"type": "MustRunAsRange"}, "groups": []any{"system:authenticated"}}),
		scc("hostpath-provisioner", false, map[string]any{"allowPrivilegedContainer": true, "allowHostDirVolumePlugin": true,
			"runAsUser": map[string]any{"type": "RunAsAny"}, "users": []any{"system:serviceaccount:hostpath-provisioner:csi"}}),
		scc("team-nonroot", false, map[string]any{"runAsUser": map[string]any{"type": "MustRunAsNonRoot"}}),
	}
	objects := []runtime.Object{
		namespace("shop", map[string]string{psaEnforceLabel: "baseline"}),
		namespace("openshift", nil),
		namespace("openshift-monitoring", nil),
		namespace("kube-system", nil),
		namespace("platform-addon", map[string]string{"openshift.io/cluster-monitoring": "true"}),
		namespace("infra-0", map[string]string{"openshift.io/run-level": ""}),
		namespace("monitoring-off", map[string]string{"openshift.io/cluster-monitoring": "false"}),
		deployment("shop", "web", compliantSpec()),
		deployment("shop", "api", compliantSpec()),
		replicaSet("shop", "web-5d8", "web"),
		replicaSet("shop", "api-7c9", "api"),
		admittedPod("shop", "web-5d8-x1", "web-5d8", "ReplicaSet", "privileged"),
		admittedPod("shop", "api-7c9-x1", "api-7c9", "ReplicaSet", "restricted-v2"),
		admittedPod("shop", "debug", "", "", "anyuid"),
		admittedPod("shop", "api-1-build", "api-1", "Build", "privileged"),
		admittedPod("openshift-monitoring", "node-exporter-x", "", "", "privileged"),
		clusterRole("system:openshift:scc:privileged", nil, useSCC("privileged")),
		clusterRole("use-restricted", nil, useSCC("restricted-v2")),
		clusterRole("use-any-scc", nil, useSCC()),
		roleBinding("shop", "system:openshift:scc:privileged", "system:openshift:scc:privileged",
			rbacv1.Subject{Kind: "ServiceAccount", Name: "builder"}),
		roleBinding("shop", "restricted", "use-restricted", rbacv1.Subject{Kind: "Group", Name: "devs"}),
		roleBinding("openshift-monitoring", "system:openshift:scc:privileged", "system:openshift:scc:privileged",
			rbacv1.Subject{Kind: "ServiceAccount", Name: "node-exporter"}),
		clusterRoleBinding("ops-any-scc", "use-any-scc", nil, rbacv1.Subject{Kind: "User", Name: "ops"},
			rbacv1.Subject{Kind: "ServiceAccount", Namespace: "openshift-infra", Name: "ctrl"}),
	}
	return sccs, objects
}

// replicaSet ties admitted pods to their Deployment.
func replicaSet(ns, name, deployment string) *appsv1.ReplicaSet {
	return &appsv1.ReplicaSet{ObjectMeta: owned(meta(ns, name), "Deployment", deployment),
		Spec: appsv1.ReplicaSetSpec{Template: corev1.PodTemplateSpec{Spec: compliantSpec()}}}
}

func TestOpenShiftScan(t *testing.T) {
	sccs, objects := crc()
	e := newOpenShiftEnv(t, sccs, objects...)
	rep := e.scan(false)
	got := findingsOf(rep)

	want := map[string][]string{
		"scc-privileged-subjects:SecurityContextConstraints::privileged":          {"User system:serviceaccount:shop:ci", "Group devs"},
		"scc-privileged-binding:RoleBinding:shop:system:openshift:scc:privileged": {"ServiceAccount shop/builder → privileged"},
		"scc-privileged-binding:ClusterRoleBinding::ops-any-scc":                  {"User ops → every SCC"},
		"scc-privileged-workload:Deployment:shop:web":                             {"scc: privileged"},
		"scc-privileged-workload:Pod:shop:debug":                                  {"scc: anyuid"},
		"scc-custom-privileged:SecurityContextConstraints::hostpath-provisioner":  {"allowPrivilegedContainer: true", "allowHostDirVolumePlugin: true"},
		// An add-on's ServiceAccount outside the platform's namespaces.
		"scc-privileged-subjects:SecurityContextConstraints::hostpath-provisioner": {"User system:serviceaccount:hostpath-provisioner:csi"},
	}
	for key, evidence := range want {
		f, ok := got[key]
		if !ok {
			t.Errorf("missing finding %s", key)
			continue
		}
		if !slices.Equal(f.Evidence, evidence) {
			t.Errorf("%s: evidence %q, want %q", key, f.Evidence, evidence)
		}
		if f.Category != CategoryOpenShift {
			t.Errorf("%s: category %s", key, f.Category)
		}
	}
	for key, f := range got {
		if f.Category == CategoryOpenShift {
			if _, ok := want[key]; !ok {
				t.Errorf("unexpected OpenShift finding %s: %q", key, f.Evidence)
			}
		}
	}
	wantSystem := []string{"infra-0", "kube-system", "openshift", "openshift-monitoring", "platform-addon"}
	if !slices.Equal(rep.SystemNamespaces, wantSystem) {
		t.Errorf("systemNamespaces = %v, want %v", rep.SystemNamespaces, wantSystem)
	}
	if rep.Inventory[kindSCCs] != len(sccs) {
		t.Errorf("inventory = %v", rep.Inventory)
	}
	last := rep.Score.ByCategory[len(rep.Score.ByCategory)-1]
	if last.Category != CategoryOpenShift || last.Checks != 4 || last.Evaluated == 0 {
		t.Errorf("OpenShift category = %+v", last)
	}
}

// With system objects included, the platform's own grants are reported too.
func TestOpenShiftScanIncludeSystem(t *testing.T) {
	sccs, objects := crc()
	got := findingsOf(newOpenShiftEnv(t, sccs, objects...).scan(true))
	for _, key := range []string{
		"scc-privileged-workload:Pod:openshift-monitoring:node-exporter-x",
		"scc-privileged-binding:RoleBinding:openshift-monitoring:system:openshift:scc:privileged",
	} {
		if _, ok := got[key]; !ok {
			t.Errorf("missing %s", key)
		}
	}
}

// A credential that may not list SCCs skips the SCC checks with the
// reason and falls back to OpenShift's default risky SCC names.
func TestOpenShiftSCCsForbidden(t *testing.T) {
	_, objects := crc()
	dyn := sccClient()
	dyn.PrependReactor("list", kindSCCs, func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, apierrors.NewForbidden(openshift.SCCGVR.GroupResource(), "", errors.New("RBAC"))
	})
	e := newEnvWith(t, dyn, objects...)
	rep := e.scan(false)
	skipped := map[string]bool{}
	for _, s := range rep.Skipped {
		skipped[s.CheckID] = true
	}
	if !skipped["scc-privileged-subjects"] || !skipped["scc-custom-privileged"] || skipped["scc-privileged-workload"] {
		t.Errorf("skipped = %+v", rep.Skipped)
	}
	got := findingsOf(rep)
	if _, ok := got["scc-privileged-workload:Deployment:shop:web"]; !ok {
		t.Error("admission with the privileged SCC not reported from the default names")
	}
}

// A custom SCC that forces UID 0 is judged like anyuid (with SYS_ADMIN,
// privileged): its grant and the pods admitted with it are reported.
func TestOpenShiftScanRootSCC(t *testing.T) {
	sccs := []*unstructured.Unstructured{
		scc("tenant-root", false, map[string]any{"runAsUser": map[string]any{"type": "MustRunAs", "uid": int64(0)},
			"allowedCapabilities": []any{"SYS_ADMIN"}}),
		scc("tenant-range0", false, map[string]any{"runAsUser": map[string]any{"type": "MustRunAsRange", "uidRangeMin": int64(0), "uidRangeMax": int64(10)},
			"users": []any{"system:serviceaccount:shop:batch"}}),
	}
	e := newOpenShiftEnv(t, sccs,
		namespace("shop", map[string]string{psaEnforceLabel: "baseline"}),
		admittedPod("shop", "agent", "", "", "tenant-root"),
		clusterRole("use-tenant-root", nil, useSCC("tenant-root")),
		roleBinding("shop", "tenant-root", "use-tenant-root", rbacv1.Subject{Kind: "ServiceAccount", Name: "agent"}),
	)
	got := findingsOf(e.scan(false))
	for key, evidence := range map[string][]string{
		"scc-privileged-binding:RoleBinding:shop:tenant-root":               {"ServiceAccount shop/agent → tenant-root"},
		"scc-privileged-workload:Pod:shop:agent":                            {"scc: tenant-root"},
		"scc-privileged-subjects:SecurityContextConstraints::tenant-range0": {"User system:serviceaccount:shop:batch"},
	} {
		if f, ok := got[key]; !ok || !slices.Equal(f.Evidence, evidence) {
			t.Errorf("%s = %+v, want evidence %q", key, f, evidence)
		}
	}
}
