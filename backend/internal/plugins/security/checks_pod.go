package security

import (
	"fmt"
	"reflect"
	"slices"
	"strings"

	corev1 "k8s.io/api/core/v1"
)

// Pod Security Standards checks (https://kubernetes.io/docs/concepts/security/pod-security-standards/).
// Each reports the containers or fields at fault as its evidence.

// found is an evaluator's answer: the evidence, failing when there is some.
func found(evidence []string) ([]string, bool) { return evidence, len(evidence) > 0 }

func isTrue(b *bool) bool { return b != nil && *b }

// baselineCapabilities may be added under the baseline level.
var baselineCapabilities = []string{
	"AUDIT_WRITE", "CHOWN", "DAC_OVERRIDE", "FOWNER", "FSETID", "KILL", "MKNOD",
	"NET_BIND_SERVICE", "SETFCAP", "SETGID", "SETPCAP", "SETUID", "SYS_CHROOT",
}

// safeSysctls are the namespaced sysctls the baseline level allows.
var safeSysctls = []string{
	"kernel.shm_rmid_forced", "net.ipv4.ip_local_port_range", "net.ipv4.ip_unprivileged_port_start",
	"net.ipv4.tcp_syncookies", "net.ipv4.ping_group_range", "net.ipv4.ip_local_reserved_ports",
	"net.ipv4.tcp_keepalive_time", "net.ipv4.tcp_fin_timeout", "net.ipv4.tcp_keepalive_intvl",
	"net.ipv4.tcp_keepalive_probes",
}

// allowedSELinuxTypes may be set under the baseline level ("" is unset).
var allowedSELinuxTypes = []string{"", "container_t", "container_init_t", "container_kvm_t", "container_engine_t"}

// restrictedVolumes are the volume sources the restricted level allows, by
// their JSON field names. hostPath is reported by its own baseline check.
var restrictedVolumes = []string{
	"configMap", "csi", "downwardAPI", "emptyDir", "ephemeral", "persistentVolumeClaim",
	"projected", "secret", "hostPath",
}

const appArmorAnnotationPrefix = "container.apparmor.security.beta.kubernetes.io/"

// capName normalises a capability ("cap_sys_admin" → "SYS_ADMIN").
func capName(c corev1.Capability) string {
	return strings.TrimPrefix(strings.ToUpper(string(c)), "CAP_")
}

func checkPrivileged(_ *scan, w *workload) ([]string, bool) {
	var ev []string
	for _, c := range w.containers() {
		sc := c.SecurityContext
		switch {
		case sc == nil:
		case isTrue(sc.Privileged):
			ev = append(ev, c.Name)
		case sc.WindowsOptions != nil && isTrue(sc.WindowsOptions.HostProcess):
			ev = append(ev, c.Name+" (hostProcess)")
		}
	}
	if sc := w.spec.SecurityContext; sc != nil && sc.WindowsOptions != nil && isTrue(sc.WindowsOptions.HostProcess) {
		ev = append(ev, "pod (hostProcess)")
	}
	return found(ev)
}

func checkHostNamespaces(_ *scan, w *workload) ([]string, bool) {
	var ev []string
	if w.spec.HostNetwork {
		ev = append(ev, "hostNetwork")
	}
	if w.spec.HostPID {
		ev = append(ev, "hostPID")
	}
	if w.spec.HostIPC {
		ev = append(ev, "hostIPC")
	}
	return found(ev)
}

func checkHostPath(_ *scan, w *workload) ([]string, bool) {
	var ev []string
	for _, v := range w.spec.Volumes {
		if v.HostPath != nil {
			ev = append(ev, fmt.Sprintf("%s (%s)", v.Name, v.HostPath.Path))
		}
	}
	return found(ev)
}

func checkHostPorts(_ *scan, w *workload) ([]string, bool) {
	var ev []string
	for _, c := range w.containers() {
		for _, p := range c.Ports {
			if p.HostPort != 0 {
				ev = append(ev, fmt.Sprintf("%s: %d", c.Name, p.HostPort))
			}
		}
	}
	return found(ev)
}

func checkCapabilities(_ *scan, w *workload) ([]string, bool) {
	var ev []string
	for _, c := range w.containers() {
		if c.SecurityContext == nil || c.SecurityContext.Capabilities == nil {
			continue
		}
		for _, add := range c.SecurityContext.Capabilities.Add {
			if name := capName(add); !slices.Contains(baselineCapabilities, name) {
				ev = append(ev, c.Name+": "+name)
			}
		}
	}
	return found(ev)
}

func checkProcMount(_ *scan, w *workload) ([]string, bool) {
	var ev []string
	for _, c := range w.containers() {
		if sc := c.SecurityContext; sc != nil && sc.ProcMount != nil && *sc.ProcMount != corev1.DefaultProcMount {
			ev = append(ev, fmt.Sprintf("%s: %s", c.Name, *sc.ProcMount))
		}
	}
	return found(ev)
}

func checkSysctls(_ *scan, w *workload) ([]string, bool) {
	var ev []string
	if sc := w.spec.SecurityContext; sc != nil {
		for _, s := range sc.Sysctls {
			if !slices.Contains(safeSysctls, s.Name) {
				ev = append(ev, s.Name)
			}
		}
	}
	return found(ev)
}

func checkSELinuxAppArmor(_ *scan, w *workload) ([]string, bool) {
	var ev []string
	seLinux := func(who string, o *corev1.SELinuxOptions) {
		if o == nil {
			return
		}
		if !slices.Contains(allowedSELinuxTypes, o.Type) {
			ev = append(ev, who+": seLinuxOptions.type "+o.Type)
		}
		if o.User != "" || o.Role != "" {
			ev = append(ev, who+": seLinuxOptions.user/role")
		}
	}
	appArmor := func(who string, p *corev1.AppArmorProfile) {
		if p != nil && p.Type == corev1.AppArmorProfileTypeUnconfined {
			ev = append(ev, who+": appArmorProfile Unconfined")
		}
	}
	if sc := w.spec.SecurityContext; sc != nil {
		seLinux("pod", sc.SELinuxOptions)
		appArmor("pod", sc.AppArmorProfile)
	}
	for _, c := range w.containers() {
		if sc := c.SecurityContext; sc != nil {
			seLinux(c.Name, sc.SELinuxOptions)
			appArmor(c.Name, sc.AppArmorProfile)
		}
		if w.annotations[appArmorAnnotationPrefix+c.Name] == "unconfined" {
			ev = append(ev, c.Name+": AppArmor annotation unconfined")
		}
	}
	return found(ev)
}

func checkPrivilegeEscalation(_ *scan, w *workload) ([]string, bool) {
	var ev []string
	for _, c := range w.containers() {
		if sc := c.SecurityContext; sc == nil || sc.AllowPrivilegeEscalation == nil || *sc.AllowPrivilegeEscalation {
			ev = append(ev, c.Name)
		}
	}
	return found(ev)
}

func checkRunAsNonRoot(_ *scan, w *workload) ([]string, bool) {
	var ev []string
	pod := w.spec.SecurityContext
	for _, c := range w.containers() {
		var nonRoot *bool
		var user *int64
		if pod != nil {
			nonRoot, user = pod.RunAsNonRoot, pod.RunAsUser
		}
		if sc := c.SecurityContext; sc != nil {
			if sc.RunAsNonRoot != nil {
				nonRoot = sc.RunAsNonRoot
			}
			if sc.RunAsUser != nil {
				user = sc.RunAsUser
			}
		}
		switch {
		case user != nil && *user == 0:
			ev = append(ev, c.Name+" (runAsUser 0)")
		case !isTrue(nonRoot):
			ev = append(ev, c.Name)
		}
	}
	return found(ev)
}

// checkSeccompUnconfined reports an explicit Unconfined seccomp profile,
// which the baseline level forbids at pod and container level alike.
func checkSeccompUnconfined(_ *scan, w *workload) ([]string, bool) {
	var ev []string
	unconfined := func(p *corev1.SeccompProfile) bool {
		return p != nil && p.Type == corev1.SeccompProfileTypeUnconfined
	}
	if sc := w.spec.SecurityContext; sc != nil && unconfined(sc.SeccompProfile) {
		ev = append(ev, "pod")
	}
	for _, c := range w.containers() {
		if sc := c.SecurityContext; sc != nil && unconfined(sc.SeccompProfile) {
			ev = append(ev, c.Name)
		}
	}
	return found(ev)
}

// checkSeccomp reports containers without an effective profile (neither
// their own nor the pod's). An explicit Unconfined profile is the baseline
// check seccomp-unconfined's finding, so it is not reported twice.
func checkSeccomp(_ *scan, w *workload) ([]string, bool) {
	var ev []string
	podSet := w.spec.SecurityContext != nil && w.spec.SecurityContext.SeccompProfile != nil
	for _, c := range w.containers() {
		if !podSet && (c.SecurityContext == nil || c.SecurityContext.SeccompProfile == nil) {
			ev = append(ev, c.Name+" (not set)")
		}
	}
	return found(ev)
}

// checkDropCapabilities reports containers that do not drop ALL, and each
// capability other than NET_BIND_SERVICE they add back.
func checkDropCapabilities(_ *scan, w *workload) ([]string, bool) {
	var ev []string
	for _, c := range w.containers() {
		caps := (*corev1.Capabilities)(nil)
		if c.SecurityContext != nil {
			caps = c.SecurityContext.Capabilities
		}
		if caps == nil || !slices.ContainsFunc(caps.Drop, func(d corev1.Capability) bool { return capName(d) == "ALL" }) {
			ev = append(ev, c.Name)
		}
		if caps == nil {
			continue
		}
		for _, add := range caps.Add {
			if name := capName(add); name != "NET_BIND_SERVICE" {
				ev = append(ev, c.Name+": adds "+name)
			}
		}
	}
	return found(ev)
}

func checkVolumeTypes(_ *scan, w *workload) ([]string, bool) {
	var ev []string
	for _, v := range w.spec.Volumes {
		if t := volumeType(v.VolumeSource); t != "" && !slices.Contains(restrictedVolumes, t) {
			ev = append(ev, fmt.Sprintf("%s (%s)", v.Name, t))
		}
	}
	return found(ev)
}

// volumeType is the JSON name of the source a volume sets, e.g. "nfs".
// VolumeSource is a union of pointers, so the set field is found by
// reflection rather than by a list that would miss new volume types.
func volumeType(src corev1.VolumeSource) string {
	v := reflect.ValueOf(src)
	for i := range v.NumField() {
		if f := v.Field(i); f.Kind() == reflect.Pointer && !f.IsNil() {
			name, _, _ := strings.Cut(v.Type().Field(i).Tag.Get("json"), ",")
			return name
		}
	}
	return ""
}
