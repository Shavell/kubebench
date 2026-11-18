package openshift

import (
	"context"
	"maps"
	"slices"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
)

// SCC is the part of a security.openshift.io/v1 SecurityContextConstraints
// that KubeBench judges. Fields keep the API's JSON names.
type SCC struct {
	metav1.ObjectMeta `json:"metadata"`

	AllowPrivilegedContainer bool              `json:"allowPrivilegedContainer"`
	AllowHostDirVolumePlugin bool              `json:"allowHostDirVolumePlugin"`
	AllowHostNetwork         bool              `json:"allowHostNetwork"`
	AllowHostPorts           bool              `json:"allowHostPorts"`
	AllowHostPID             bool              `json:"allowHostPID"`
	AllowHostIPC             bool              `json:"allowHostIPC"`
	AllowedCapabilities      []string          `json:"allowedCapabilities"`
	DefaultAddCapabilities   []string          `json:"defaultAddCapabilities"`
	Volumes                  []string          `json:"volumes"`
	Users                    []string          `json:"users"`
	Groups                   []string          `json:"groups"`
	RunAsUser                RunAsUserStrategy `json:"runAsUser"`
}

// RunAsUserStrategy is an SCC's runAsUser: the strategy and, when set,
// the UID or UID range it forces (unset: the namespace's).
type RunAsUserStrategy struct {
	Type        string `json:"type"`
	UID         *int64 `json:"uid,omitempty"`
	UIDRangeMin *int64 `json:"uidRangeMin,omitempty"`
}

// allowsRoot reports whether containers may run as UID 0: any UID, a
// forced UID 0, or a forced range that starts at 0.
func (r RunAsUserStrategy) allowsRoot() bool {
	switch r.Type {
	case "RunAsAny":
		return true
	case "MustRunAs":
		return r.UID != nil && *r.UID == 0
	case "MustRunAsRange":
		return r.UIDRangeMin != nil && *r.UIDRangeMin == 0
	}
	return false
}

// breakoutCapabilities are the capabilities that give a container its
// node: kernel modules, mounts and namespaces, raw devices, other
// processes, the host network stack, reading any file, eBPF. A non-root
// process gets them through file capabilities, so they count at any UID.
var breakoutCapabilities = []string{
	"ALL", "BPF", "DAC_READ_SEARCH", "NET_ADMIN", "SYS_ADMIN", "SYS_BOOT", "SYS_MODULE", "SYS_PTRACE", "SYS_RAWIO",
}

// addsBreakoutCapability reports whether the SCC lets containers add,
// or adds itself, any capability or one of breakoutCapabilities.
func (s *SCC) addsBreakoutCapability() bool {
	for _, c := range slices.Concat(s.AllowedCapabilities, s.DefaultAddCapabilities) {
		name := strings.TrimPrefix(strings.ToUpper(c), "CAP_")
		if name == "*" || slices.Contains(breakoutCapabilities, name) {
			return true
		}
	}
	return false
}

// Levels of what an SCC lets a pod do, from the most to the least
// dangerous (see Level).
const (
	// LevelPrivileged: privileged containers, any capability or one that
	// breaks out of the container, or host namespaces, ports or
	// directories: the pod can take over its node.
	LevelPrivileged = "privileged"
	// LevelAnyUID: no host access, but containers may run as root (UID 0):
	// any UID, or a forced UID or UID range that is or starts at 0.
	LevelAnyUID = "anyuid"
	// LevelRestricted: neither; the namespace's UID range or non-root.
	LevelRestricted = "restricted"
)

// Level classifies the SCC by its fields, not its name, so custom SCCs
// are judged like the defaults.
func (s *SCC) Level() string {
	switch {
	case s.AllowPrivilegedContainer, s.AllowHostDirVolumePlugin, s.AllowHostNetwork,
		s.AllowHostPorts, s.AllowHostPID, s.AllowHostIPC,
		s.addsBreakoutCapability(),
		slices.Contains(s.Volumes, "*"), slices.Contains(s.Volumes, "hostPath"):
		return LevelPrivileged
	case s.RunAsUser.allowsRoot():
		return LevelAnyUID
	default:
		return LevelRestricted
	}
}

// Risky reports whether the SCC is privileged or lets containers run as
// root: the SCCs whose use should be granted sparingly.
func (s *SCC) Risky() bool { return s.Level() != LevelRestricted }

// Default reports whether the SCC is one OpenShift ships: the cluster
// version operator creates those from the release payload, which marks
// them with include.release.openshift.io/* annotations (or, for newer
// ones, as owned by the ClusterVersion); platform operators label theirs
// as part of an openshift-* component (node-exporter: openshift-monitoring).
func (s *SCC) Default() bool {
	if strings.HasPrefix(s.Labels["app.kubernetes.io/part-of"], "openshift-") {
		return true
	}
	for key := range s.Annotations {
		if strings.HasPrefix(key, "include.release.openshift.io/") {
			return true
		}
	}
	return slices.ContainsFunc(s.OwnerReferences, func(o metav1.OwnerReference) bool {
		return o.Kind == "ClusterVersion" && strings.HasPrefix(o.APIVersion, "config.openshift.io/")
	})
}

// defaultRisky are the levels of the SCCs OpenShift ships that allow
// privileged or root pods. They stand in for the classification when the
// SCCs themselves cannot be read (see RiskyLevels).
var defaultRisky = map[string]string{
	"privileged": LevelPrivileged, "hostaccess": LevelPrivileged,
	"hostmount-anyuid": LevelPrivileged, "hostmount-anyuid-v2": LevelPrivileged,
	"hostnetwork": LevelPrivileged, "hostnetwork-v2": LevelPrivileged,
	"node-exporter": LevelPrivileged, "anyuid": LevelAnyUID,
}

// RiskyLevels maps the risky SCCs among sccs to their level, or
// OpenShift's default risky ones when sccs is nil (they could not be
// listed).
func RiskyLevels(sccs []SCC) map[string]string {
	if sccs == nil {
		return maps.Clone(defaultRisky)
	}
	out := map[string]string{}
	for i := range sccs {
		if sccs[i].Risky() {
			out[sccs[i].Name] = sccs[i].Level()
		}
	}
	return out
}

// ListSCCs lists the cluster's SCCs. served is false (and err nil) on a
// cluster that does not serve security.openshift.io; any other failure is
// returned.
func ListSCCs(ctx context.Context, dyn dynamic.Interface) (sccs []SCC, served bool, err error) {
	list, err := dyn.Resource(SCCGVR).List(ctx, metav1.ListOptions{})
	if err != nil {
		if NotServed(err) {
			return nil, false, nil
		}
		return nil, true, err
	}
	sccs = make([]SCC, 0, len(list.Items))
	for i := range list.Items {
		var s SCC
		if err := fromUnstructured(list.Items[i].Object, &s); err != nil {
			return nil, true, err
		}
		sccs = append(sccs, s)
	}
	return sccs, true, nil
}

func fromUnstructured(obj map[string]any, into any) error {
	return runtime.DefaultUnstructuredConverter.FromUnstructured(obj, into)
}
