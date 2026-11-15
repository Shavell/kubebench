package security

import (
	"regexp"
	"slices"
	"strings"

	corev1 "k8s.io/api/core/v1"
)

// Workload hygiene, secrets and ServiceAccount checks on pod templates.

// missing lists which of the named resources a resource list leaves out.
func missing(list corev1.ResourceList, names ...corev1.ResourceName) []string {
	var out []string
	for _, n := range names {
		if _, ok := list[n]; !ok {
			out = append(out, string(n))
		}
	}
	return out
}

func checkRequests(_ *scan, w *workload) ([]string, bool) {
	var ev []string
	for _, c := range w.spec.Containers {
		if m := missing(c.Resources.Requests, corev1.ResourceCPU, corev1.ResourceMemory); len(m) > 0 {
			ev = append(ev, c.Name+" ("+strings.Join(m, ", ")+")")
		}
	}
	return found(ev)
}

func checkLimits(_ *scan, w *workload) ([]string, bool) {
	var ev []string
	for _, c := range w.spec.Containers {
		if m := missing(c.Resources.Limits, corev1.ResourceCPU, corev1.ResourceMemory); len(m) > 0 {
			ev = append(ev, c.Name+" ("+strings.Join(m, ", ")+")")
		}
	}
	return found(ev)
}

func checkProbes(_ *scan, w *workload) ([]string, bool) {
	var ev []string
	for _, c := range w.spec.Containers {
		var m []string
		if c.LivenessProbe == nil {
			m = append(m, "liveness")
		}
		if c.ReadinessProbe == nil {
			m = append(m, "readiness")
		}
		if len(m) > 0 {
			ev = append(ev, c.Name+" ("+strings.Join(m, ", ")+")")
		}
	}
	return found(ev)
}

// mutableTag reports whether an image reference names no digest and either
// no tag or "latest", e.g. "nginx", "nginx:latest" or "host:5000/app".
func mutableTag(image string) bool {
	if strings.Contains(image, "@") {
		return false
	}
	last := image[strings.LastIndex(image, "/")+1:]
	_, tag, hasTag := strings.Cut(last, ":")
	return !hasTag || tag == "latest"
}

func checkImageTag(_ *scan, w *workload) ([]string, bool) {
	var ev []string
	for _, c := range w.containers() {
		if mutableTag(c.Image) {
			ev = append(ev, c.Name+": "+c.Image)
		}
	}
	return found(ev)
}

func checkReadOnlyRoot(_ *scan, w *workload) ([]string, bool) {
	var ev []string
	for _, c := range slices.Concat(w.spec.InitContainers, w.spec.Containers) {
		if c.SecurityContext == nil || !isTrue(c.SecurityContext.ReadOnlyRootFilesystem) {
			ev = append(ev, c.Name)
		}
	}
	return found(ev)
}

func checkSecretEnv(_ *scan, w *workload) ([]string, bool) {
	var ev []string
	for _, c := range w.containers() {
		for _, e := range c.Env {
			if e.ValueFrom != nil && e.ValueFrom.SecretKeyRef != nil {
				ev = append(ev, c.Name+": "+e.Name)
			}
		}
		for _, from := range c.EnvFrom {
			if from.SecretRef != nil {
				ev = append(ev, c.Name+": envFrom secret "+from.SecretRef.Name)
			}
		}
	}
	return found(ev)
}

// credentialName matches environment variable names that usually carry
// credentials, e.g. DB_PASSWORD, API_KEY, GITHUB_TOKEN.
var credentialName = regexp.MustCompile(`(?i)(passw(or)?d|passwd|secret|token|api_?key|private_?key|access_?key|credential)`)

// checkHardcodedSecret reports literal values of credential-like variables.
// Values are never part of the evidence; references to other variables
// ("$(DB_PASSWORD)") are not literals.
func checkHardcodedSecret(_ *scan, w *workload) ([]string, bool) {
	var ev []string
	for _, c := range w.containers() {
		for _, e := range c.Env {
			if e.ValueFrom == nil && e.Value != "" && !strings.HasPrefix(e.Value, "$(") && credentialName.MatchString(e.Name) {
				ev = append(ev, c.Name+": "+e.Name)
			}
		}
	}
	return found(ev)
}

// checkServiceAccountToken reports pods that run as their namespace's
// default ServiceAccount with its token mounted: the pod's own setting
// wins over the ServiceAccount's, and both default to mounting.
func checkServiceAccountToken(s *scan, w *workload) ([]string, bool) {
	account := w.spec.ServiceAccountName
	if account == "" {
		account = w.spec.DeprecatedServiceAccount
	}
	if account != "" && account != "default" {
		return nil, false
	}
	mount := w.spec.AutomountServiceAccountToken
	if mount == nil {
		mount = s.automount[w.Namespace+"/default"]
	}
	if mount != nil && !*mount {
		return nil, false
	}
	return []string{"serviceAccountName: default", "automountServiceAccountToken: true"}, true
}

func checkDefaultNamespace(_ *scan, w *workload) ([]string, bool) {
	if w.Namespace != "default" {
		return nil, false
	}
	return []string{"namespace: default"}, true
}

// checkDefaultServiceAccount reports a default ServiceAccount that mounts
// its token or that a binding grants permissions to.
func checkDefaultServiceAccount(s *scan, sa *corev1.ServiceAccount) ([]string, bool) {
	var ev []string
	if sa.AutomountServiceAccountToken == nil || *sa.AutomountServiceAccountToken {
		ev = append(ev, "automountServiceAccountToken: true")
	}
	for _, b := range s.allBindings {
		for _, sub := range b.subjects {
			ns := sub.Namespace
			if ns == "" {
				ns = b.Namespace
			}
			if sub.Kind == "ServiceAccount" && sub.Name == sa.Name && ns == sa.Namespace {
				ev = append(ev, "bound by "+b.Kind+" "+joinName(b.Namespace, b.Name))
				break
			}
		}
	}
	return found(ev)
}

// joinName is "ns/name", or name for cluster-scoped objects.
func joinName(ns, name string) string {
	if ns == "" {
		return name
	}
	return ns + "/" + name
}

const psaEnforceLabel = "pod-security.kubernetes.io/enforce"

func checkPSALabels(_ *scan, ns *corev1.Namespace) ([]string, bool) {
	switch level, ok := ns.Labels[psaEnforceLabel]; {
	case !ok:
		return []string{psaEnforceLabel + ": unset"}, true
	case level == "privileged":
		return []string{psaEnforceLabel + ": privileged"}, true
	}
	return nil, false
}

func checkNetworkPolicy(s *scan, ns *corev1.Namespace) ([]string, bool) {
	return nil, s.policies[ns.Name] == 0
}
