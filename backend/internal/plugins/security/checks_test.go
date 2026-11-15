package security

import (
	"slices"
	"testing"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func ptr[T any](v T) *T { return &v }

// compliant is a container that passes every workload check.
func compliant(name string) corev1.Container {
	return corev1.Container{
		Name: name, Image: "registry.example/app:1.4.2",
		Resources: corev1.ResourceRequirements{
			Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("100m"), corev1.ResourceMemory: resource.MustParse("64Mi")},
			Limits:   corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("500m"), corev1.ResourceMemory: resource.MustParse("128Mi")},
		},
		LivenessProbe:  &corev1.Probe{},
		ReadinessProbe: &corev1.Probe{},
		SecurityContext: &corev1.SecurityContext{
			AllowPrivilegeEscalation: ptr(false),
			RunAsNonRoot:             ptr(true),
			ReadOnlyRootFilesystem:   ptr(true),
			Capabilities:             &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}},
		},
	}
}

// compliantSpec is a pod spec that passes every workload check.
func compliantSpec() corev1.PodSpec {
	return corev1.PodSpec{
		ServiceAccountName: "app",
		SecurityContext:    &corev1.PodSecurityContext{SeccompProfile: &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeRuntimeDefault}},
		Containers:         []corev1.Container{compliant("app")},
		Volumes:            []corev1.Volume{{Name: "cache", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}}},
	}
}

func testWorkload(kind string, mutate func(*corev1.PodSpec)) *workload {
	spec := compliantSpec()
	if mutate != nil {
		mutate(&spec)
	}
	return &workload{Resource: Resource{APIVersion: "apps/v1", Kind: kind, Namespace: "shop", Name: "web"}, spec: &spec, annotations: map[string]string{}}
}

// app is the spec's first container, for mutations.
func app(s *corev1.PodSpec) *corev1.Container { return &s.Containers[0] }

func TestWorkloadChecks(t *testing.T) {
	tests := []struct {
		check    string
		mutate   func(*corev1.PodSpec)
		evidence []string // nil: passes
	}{
		{"privileged", nil, nil},
		{"privileged", func(s *corev1.PodSpec) { app(s).SecurityContext.Privileged = ptr(true) }, []string{"app"}},
		{"privileged", func(s *corev1.PodSpec) {
			s.InitContainers = []corev1.Container{{Name: "init", SecurityContext: &corev1.SecurityContext{Privileged: ptr(true)}}}
		}, []string{"init"}},
		{"privileged", func(s *corev1.PodSpec) {
			app(s).SecurityContext.WindowsOptions = &corev1.WindowsSecurityContextOptions{HostProcess: ptr(true)}
		}, []string{"app (hostProcess)"}},
		{"host-namespaces", nil, nil},
		{"host-namespaces", func(s *corev1.PodSpec) { s.HostNetwork, s.HostPID = true, true }, []string{"hostNetwork", "hostPID"}},
		{"host-namespaces", func(s *corev1.PodSpec) { s.HostIPC = true }, []string{"hostIPC"}},
		{"host-path", nil, nil},
		{"host-path", func(s *corev1.PodSpec) {
			s.Volumes = append(s.Volumes, corev1.Volume{Name: "root", VolumeSource: corev1.VolumeSource{HostPath: &corev1.HostPathVolumeSource{Path: "/"}}})
		}, []string{"root (/)"}},
		{"host-ports", nil, nil},
		{"host-ports", func(s *corev1.PodSpec) {
			app(s).Ports = []corev1.ContainerPort{{ContainerPort: 80}, {ContainerPort: 443, HostPort: 8443}}
		}, []string{"app: 8443"}},
		{"capabilities", func(s *corev1.PodSpec) {
			app(s).SecurityContext.Capabilities.Add = []corev1.Capability{"NET_BIND_SERVICE", "CHOWN"}
		}, nil},
		{"capabilities", func(s *corev1.PodSpec) {
			app(s).SecurityContext.Capabilities.Add = []corev1.Capability{"NET_ADMIN", "cap_sys_admin", "KILL"}
		}, []string{"app: NET_ADMIN", "app: SYS_ADMIN"}},
		{"proc-mount", func(s *corev1.PodSpec) { app(s).SecurityContext.ProcMount = ptr(corev1.DefaultProcMount) }, nil},
		{"proc-mount", func(s *corev1.PodSpec) { app(s).SecurityContext.ProcMount = ptr(corev1.UnmaskedProcMount) }, []string{"app: Unmasked"}},
		{"unsafe-sysctls", func(s *corev1.PodSpec) {
			s.SecurityContext.Sysctls = []corev1.Sysctl{{Name: "net.ipv4.tcp_syncookies", Value: "1"}}
		}, nil},
		{"unsafe-sysctls", func(s *corev1.PodSpec) {
			s.SecurityContext.Sysctls = []corev1.Sysctl{{Name: "kernel.msgmax", Value: "65536"}, {Name: "net.ipv4.ping_group_range", Value: "0 1"}}
		}, []string{"kernel.msgmax"}},
		{"selinux-apparmor", func(s *corev1.PodSpec) {
			s.SecurityContext.SELinuxOptions = &corev1.SELinuxOptions{Type: "container_t", Level: "s0:c1"}
		}, nil},
		{"selinux-apparmor", func(s *corev1.PodSpec) {
			s.SecurityContext.SELinuxOptions = &corev1.SELinuxOptions{Type: "spc_t"}
			app(s).SecurityContext.SELinuxOptions = &corev1.SELinuxOptions{User: "system_u"}
		}, []string{"pod: seLinuxOptions.type spc_t", "app: seLinuxOptions.user/role"}},
		{"selinux-apparmor", func(s *corev1.PodSpec) {
			app(s).SecurityContext.AppArmorProfile = &corev1.AppArmorProfile{Type: corev1.AppArmorProfileTypeUnconfined}
		}, []string{"app: appArmorProfile Unconfined"}},
		{"privilege-escalation", nil, nil},
		{"privilege-escalation", func(s *corev1.PodSpec) { app(s).SecurityContext.AllowPrivilegeEscalation = nil }, []string{"app"}},
		{"privilege-escalation", func(s *corev1.PodSpec) { app(s).SecurityContext = nil }, []string{"app"}},
		{"run-as-non-root", nil, nil},
		{"run-as-non-root", func(s *corev1.PodSpec) {
			app(s).SecurityContext.RunAsNonRoot = nil
			s.SecurityContext.RunAsNonRoot = ptr(true)
		}, nil},
		{"run-as-non-root", func(s *corev1.PodSpec) { app(s).SecurityContext.RunAsNonRoot = nil }, []string{"app"}},
		{"run-as-non-root", func(s *corev1.PodSpec) { app(s).SecurityContext.RunAsUser = ptr(int64(0)) }, []string{"app (runAsUser 0)"}},
		{"seccomp", func(s *corev1.PodSpec) {
			s.SecurityContext.SeccompProfile = nil
			app(s).SecurityContext.SeccompProfile = &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeLocalhost}
		}, nil},
		{"seccomp", func(s *corev1.PodSpec) { s.SecurityContext.SeccompProfile = nil }, []string{"app (not set)"}},
		{"seccomp", func(s *corev1.PodSpec) {
			app(s).SecurityContext.SeccompProfile = &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeUnconfined}
		}, []string{"app (Unconfined)"}},
		{"drop-capabilities", nil, nil},
		{"drop-capabilities", func(s *corev1.PodSpec) { app(s).SecurityContext.Capabilities.Drop = []corev1.Capability{"NET_RAW"} }, []string{"app"}},
		{"volume-types", func(s *corev1.PodSpec) {
			s.Volumes = append(s.Volumes, corev1.Volume{Name: "host", VolumeSource: corev1.VolumeSource{HostPath: &corev1.HostPathVolumeSource{Path: "/data"}}})
		}, nil},
		{"volume-types", func(s *corev1.PodSpec) {
			s.Volumes = append(s.Volumes, corev1.Volume{Name: "share", VolumeSource: corev1.VolumeSource{NFS: &corev1.NFSVolumeSource{Server: "nfs", Path: "/"}}})
		}, []string{"share (nfs)"}},
		{"resource-requests", nil, nil},
		{"resource-requests", func(s *corev1.PodSpec) { delete(app(s).Resources.Requests, corev1.ResourceMemory) }, []string{"app (memory)"}},
		{"resource-limits", func(s *corev1.PodSpec) { app(s).Resources = corev1.ResourceRequirements{} }, []string{"app (cpu, memory)"}},
		{"probes", nil, nil},
		{"probes", func(s *corev1.PodSpec) { app(s).LivenessProbe, app(s).ReadinessProbe = nil, nil }, []string{"app (liveness, readiness)"}},
		{"image-tag", func(s *corev1.PodSpec) { app(s).Image = "nginx@sha256:abcd" }, nil},
		{"image-tag", func(s *corev1.PodSpec) { app(s).Image = "localhost:5000/app" }, []string{"app: localhost:5000/app"}},
		{"image-tag", func(s *corev1.PodSpec) { app(s).Image = "nginx:latest" }, []string{"app: nginx:latest"}},
		{"read-only-root", nil, nil},
		{"read-only-root", func(s *corev1.PodSpec) { app(s).SecurityContext.ReadOnlyRootFilesystem = ptr(false) }, []string{"app"}},
		{"secret-env", nil, nil},
		{"secret-env", func(s *corev1.PodSpec) {
			app(s).Env = []corev1.EnvVar{{Name: "DB_PASSWORD", ValueFrom: &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{Key: "pw"}}}}
			app(s).EnvFrom = []corev1.EnvFromSource{{SecretRef: &corev1.SecretEnvSource{LocalObjectReference: corev1.LocalObjectReference{Name: "creds"}}}}
		}, []string{"app: DB_PASSWORD", "app: envFrom secret creds"}},
		{"hardcoded-secret", func(s *corev1.PodSpec) {
			app(s).Env = []corev1.EnvVar{{Name: "LOG_LEVEL", Value: "debug"}, {Name: "API_KEY", Value: "$(OTHER)"}, {Name: "TOKEN", Value: ""}}
		}, nil},
		{"hardcoded-secret", func(s *corev1.PodSpec) {
			app(s).Env = []corev1.EnvVar{{Name: "DB_PASSWORD", Value: "hunter2"}, {Name: "github_token", Value: "x"}}
		}, []string{"app: DB_PASSWORD", "app: github_token"}},
		{"service-account-token", nil, nil},
		{"service-account-token", func(s *corev1.PodSpec) { s.ServiceAccountName = "" }, []string{"serviceAccountName: default", "automountServiceAccountToken: true"}},
		{"service-account-token", func(s *corev1.PodSpec) {
			s.ServiceAccountName = "default"
			s.AutomountServiceAccountToken = ptr(false)
		}, nil},
	}
	for _, tt := range tests {
		w := testWorkload("Deployment", tt.mutate)
		ev, failed := checksByID[tt.check].workload(&scan{automount: map[string]*bool{}}, w)
		if failed != (tt.evidence != nil) || !slices.Equal(ev, tt.evidence) {
			t.Errorf("%s: evidence %q (failed %v), want %q", tt.check, ev, failed, tt.evidence)
		}
	}
}

// Every workload check passes the compliant spec, so a finding in the
// tests above comes from the mutation alone.
func TestCompliantSpecPassesEveryWorkloadCheck(t *testing.T) {
	w := testWorkload("Deployment", nil)
	for _, c := range catalog {
		if c.workload == nil {
			continue
		}
		if ev, failed := c.workload(&scan{automount: map[string]*bool{}}, w); failed {
			t.Errorf("%s fails the compliant spec: %q", c.ID, ev)
		}
	}
}

func TestServiceAccountTokenFollowsTheServiceAccount(t *testing.T) {
	w := testWorkload("Deployment", func(s *corev1.PodSpec) { s.ServiceAccountName = "" })
	s := &scan{automount: map[string]*bool{"shop/default": ptr(false)}}
	if _, failed := checkServiceAccountToken(s, w); failed {
		t.Fatal("the ServiceAccount turns automount off, but the check failed")
	}
	w.spec.AutomountServiceAccountToken = ptr(true)
	if _, failed := checkServiceAccountToken(s, w); !failed {
		t.Fatal("the pod turns automount on, but the check passed")
	}
}

func TestProbesOnlyApplyToLongRunningWorkloads(t *testing.T) {
	applies := checksByID["probes"].applies
	if applies(testWorkload("CronJob", nil)) || applies(testWorkload("Job", nil)) || !applies(testWorkload("Pod", nil)) {
		t.Fatal("probes should apply to serving workloads only")
	}
}

func TestDefaultNamespaceCheck(t *testing.T) {
	w := testWorkload("Deployment", nil)
	if _, failed := checkDefaultNamespace(nil, w); failed {
		t.Fatal("shop is not the default namespace")
	}
	w.Namespace = "default"
	if _, failed := checkDefaultNamespace(nil, w); !failed {
		t.Fatal("a workload in default passed")
	}
}

func TestNamespaceChecks(t *testing.T) {
	s := &scan{policies: map[string]int{"locked": 1}}
	ns := func(name string, labels map[string]string) *corev1.Namespace {
		return &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: name, Labels: labels}}
	}
	if _, failed := checkNetworkPolicy(s, ns("locked", nil)); failed {
		t.Error("a namespace with a NetworkPolicy failed")
	}
	if _, failed := checkNetworkPolicy(s, ns("open", nil)); !failed {
		t.Error("a namespace without a NetworkPolicy passed")
	}
	tests := []struct {
		labels   map[string]string
		evidence []string
	}{
		{map[string]string{psaEnforceLabel: "baseline"}, nil},
		{map[string]string{psaEnforceLabel: "restricted"}, nil},
		{map[string]string{psaEnforceLabel: "privileged"}, []string{psaEnforceLabel + ": privileged"}},
		{map[string]string{"pod-security.kubernetes.io/warn": "restricted"}, []string{psaEnforceLabel + ": unset"}},
	}
	for _, tt := range tests {
		if ev, _ := checkPSALabels(s, ns("x", tt.labels)); !slices.Equal(ev, tt.evidence) {
			t.Errorf("psa-labels %v: evidence %q, want %q", tt.labels, ev, tt.evidence)
		}
	}
}

func TestDefaultServiceAccountCheck(t *testing.T) {
	sa := &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Namespace: "shop", Name: "default"}, AutomountServiceAccountToken: ptr(false)}
	s := &scan{}
	if _, failed := checkDefaultServiceAccount(s, sa); failed {
		t.Fatal("an inert default ServiceAccount failed")
	}
	s.allBindings = []*bindingInfo{{
		Resource: Resource{Kind: "RoleBinding", Namespace: "shop", Name: "edit-default"},
		subjects: []rbacv1.Subject{{Kind: "ServiceAccount", Name: "default"}},
	}}
	sa.AutomountServiceAccountToken = nil
	ev, _ := checkDefaultServiceAccount(s, sa)
	if want := []string{"automountServiceAccountToken: true", "bound by RoleBinding shop/edit-default"}; !slices.Equal(ev, want) {
		t.Fatalf("evidence %q, want %q", ev, want)
	}
}

func TestRoleChecks(t *testing.T) {
	rule := func(groups, resources, verbs []string) rbacv1.PolicyRule {
		return rbacv1.PolicyRule{APIGroups: groups, Resources: resources, Verbs: verbs}
	}
	core := []string{""}
	tests := []struct {
		check    string
		rules    []rbacv1.PolicyRule
		evidence []string
	}{
		{"rbac-wildcards", []rbacv1.PolicyRule{rule(core, []string{"pods"}, []string{"get", "list"})}, nil},
		{"rbac-wildcards", []rbacv1.PolicyRule{rule([]string{"apps"}, []string{"*"}, []string{"get"})}, []string{"get apps/*"}},
		{"rbac-wildcards", []rbacv1.PolicyRule{{NonResourceURLs: []string{"*"}, Verbs: []string{"get"}}}, []string{"get *"}},
		{"secrets-access", []rbacv1.PolicyRule{rule(core, []string{"configmaps"}, []string{"get"})}, nil},
		{"secrets-access", []rbacv1.PolicyRule{rule(core, []string{"secrets"}, []string{"create"})}, nil},
		{"secrets-access", []rbacv1.PolicyRule{rule(core, []string{"secrets"}, []string{"list"})}, []string{"list secrets"}},
		{"secrets-access", []rbacv1.PolicyRule{rule([]string{"*"}, []string{"*"}, []string{"*"})}, []string{"* */*"}},
		{"secrets-access", []rbacv1.PolicyRule{{APIGroups: core, Resources: []string{"secrets"}, Verbs: []string{"get"}, ResourceNames: []string{"app-tls"}}}, nil},
		{"pod-create", []rbacv1.PolicyRule{rule(core, []string{"pods"}, []string{"get", "list"})}, nil},
		{"pod-create", []rbacv1.PolicyRule{{APIGroups: core, Resources: []string{"pods/exec"}, Verbs: []string{"create"}, ResourceNames: []string{"debug"}}}, nil},
		{"pod-create", []rbacv1.PolicyRule{rule(core, []string{"pods"}, []string{"create"})}, []string{"create pods"}},
		{"pod-create", []rbacv1.PolicyRule{rule(core, []string{"pods/exec"}, []string{"create"})}, []string{"create pods/exec"}},
		{"rbac-escalation", []rbacv1.PolicyRule{rule([]string{"rbac.authorization.k8s.io"}, []string{"roles"}, []string{"get", "create"})}, nil},
		{"rbac-escalation", []rbacv1.PolicyRule{rule([]string{"rbac.authorization.k8s.io"}, []string{"clusterroles"}, []string{"bind", "escalate"})},
			[]string{"bind, escalate: bind,escalate rbac.authorization.k8s.io/clusterroles"}},
		{"rbac-escalation", []rbacv1.PolicyRule{rule(core, []string{"users", "groups"}, []string{"impersonate"})},
			[]string{"impersonate: impersonate users,groups"}},
	}
	for _, tt := range tests {
		ev, failed := checksByID[tt.check].role(nil, &roleInfo{rules: tt.rules})
		if failed != (tt.evidence != nil) || !slices.Equal(ev, tt.evidence) {
			t.Errorf("%s %v: evidence %q, want %q", tt.check, tt.rules, ev, tt.evidence)
		}
	}
}

func TestClusterAdminBindingCheck(t *testing.T) {
	b := &bindingInfo{
		Resource: Resource{Kind: "RoleBinding", Namespace: "shop", Name: "admins"},
		roleRef:  rbacv1.RoleRef{Kind: "ClusterRole", Name: "admin"},
		subjects: []rbacv1.Subject{{Kind: "User", Name: "alice"}, {Kind: "ServiceAccount", Name: "ci"}},
	}
	if _, failed := checkClusterAdminBinding(nil, b); failed {
		t.Fatal("a binding to admin failed")
	}
	b.roleRef.Name = "cluster-admin"
	ev, _ := checkClusterAdminBinding(nil, b)
	if want := []string{"User alice", "ServiceAccount shop/ci"}; !slices.Equal(ev, want) {
		t.Fatalf("evidence %q, want %q", ev, want)
	}
}

func TestCatalogIsWellFormed(t *testing.T) {
	for _, c := range catalog {
		evaluators := 0
		for _, set := range []bool{c.workload != nil, c.namespace != nil, c.account != nil, c.role != nil, c.binding != nil} {
			if set {
				evaluators++
			}
		}
		target := map[string]bool{
			targetWorkload: c.workload != nil, targetNamespace: c.namespace != nil,
			targetServiceAccount: c.account != nil, targetRole: c.role != nil, targetBinding: c.binding != nil,
		}[c.Target]
		if evaluators != 1 || !target {
			t.Errorf("%s: needs exactly the evaluator of target %q", c.ID, c.Target)
		}
		if c.Title == "" || c.Remediation == "" || c.message == "" || len(c.Frameworks) == 0 {
			t.Errorf("%s: incomplete metadata", c.ID)
		}
		if !slices.Contains(severities, c.Severity) || !slices.Contains(categories, c.Category) {
			t.Errorf("%s: unknown severity or category", c.ID)
		}
	}
	if len(checksByID) != len(catalog) {
		t.Error("check ids are not unique")
	}
}
