package storage

import (
	"context"
	"net"
	"net/http"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation"

	"kubebench/internal/kube"
	"kubebench/internal/plugin"
)

// hostnameLabel is the node label a node-affinity term matches on.
const hostnameLabel = "kubernetes.io/hostname"

// PVRequest is the body of POST /pvs. Exactly one source is set.
type PVRequest struct {
	Name        string   `json:"name"`
	Capacity    string   `json:"capacity"`
	AccessModes []string `json:"accessModes"`
	// ReclaimPolicy is Retain (the default) or Delete.
	ReclaimPolicy    string            `json:"reclaimPolicy,omitempty"`
	StorageClassName string            `json:"storageClassName,omitempty"`
	VolumeMode       string            `json:"volumeMode,omitempty"`
	MountOptions     []string          `json:"mountOptions,omitempty"`
	Labels           map[string]string `json:"labels,omitempty"`
	HostPath         *HostPathSource   `json:"hostPath,omitempty"`
	NFS              *NFSSource        `json:"nfs,omitempty"`
	Local            *LocalSource      `json:"local,omitempty"`
	CSI              *CSISource        `json:"csi,omitempty"`
	// Nodes pins the volume to these nodes (required node affinity on
	// kubernetes.io/hostname). Local volumes need it; NFS volumes, which
	// every node reaches, cannot have it.
	Nodes []string `json:"nodes,omitempty"`
}

type HostPathSource struct {
	Path string `json:"path"`
	// Type is a hostPath type such as DirectoryOrCreate; "" checks nothing.
	Type string `json:"type,omitempty"`
}

type NFSSource struct {
	Server   string `json:"server"`
	Path     string `json:"path"`
	ReadOnly bool   `json:"readOnly,omitempty"`
}

type LocalSource struct {
	Path   string `json:"path"`
	FSType string `json:"fsType,omitempty"`
}

type CSISource struct {
	Driver           string            `json:"driver"`
	VolumeHandle     string            `json:"volumeHandle"`
	FSType           string            `json:"fsType,omitempty"`
	ReadOnly         bool              `json:"readOnly,omitempty"`
	VolumeAttributes map[string]string `json:"volumeAttributes,omitempty"`
}

var hostPathTypes = map[string]bool{
	"": true, "DirectoryOrCreate": true, "Directory": true, "FileOrCreate": true,
	"File": true, "Socket": true, "CharDevice": true, "BlockDevice": true,
}

// pvSpec is a validated PVRequest.
type pvSpec struct {
	req      PVRequest
	capacity resource.Quantity
	modes    []corev1.PersistentVolumeAccessMode
	mode     *corev1.PersistentVolumeMode
	reclaim  corev1.PersistentVolumeReclaimPolicy
	source   string // the kind of source, for the audit row
}

func (req PVRequest) validate() (pvSpec, error) {
	s := pvSpec{req: req}
	var err error
	if err = objectName("name", req.Name); err != nil {
		return s, err
	}
	if s.capacity, err = storageSize("capacity", req.Capacity); err != nil {
		return s, err
	}
	if s.modes, err = accessModes("accessModes", req.AccessModes); err != nil {
		return s, err
	}
	switch p := corev1.PersistentVolumeReclaimPolicy(req.ReclaimPolicy); p {
	case "":
		s.reclaim = corev1.PersistentVolumeReclaimRetain
	case corev1.PersistentVolumeReclaimRetain, corev1.PersistentVolumeReclaimDelete:
		s.reclaim = p
	default:
		return s, invalidf("reclaimPolicy", "%q is not Retain or Delete", req.ReclaimPolicy)
	}
	if err = storageClassRef("storageClassName", req.StorageClassName); err != nil {
		return s, err
	}
	if s.mode, err = volumeMode("volumeMode", req.VolumeMode); err != nil {
		return s, err
	}
	if err = mountOptions("mountOptions", req.MountOptions); err != nil {
		return s, err
	}
	if err = labels("labels", req.Labels); err != nil {
		return s, err
	}
	if s.source, err = req.validateSource(modeOf(s.mode)); err != nil {
		return s, err
	}
	return s, req.validateNodes(s.source)
}

// validateSource checks the one source and returns its kind.
func (req PVRequest) validateSource(mode corev1.PersistentVolumeMode) (string, error) {
	var kinds []string
	if req.HostPath != nil {
		kinds = append(kinds, "hostPath")
	}
	if req.NFS != nil {
		kinds = append(kinds, "nfs")
	}
	if req.Local != nil {
		kinds = append(kinds, "local")
	}
	if req.CSI != nil {
		kinds = append(kinds, "csi")
	}
	if len(kinds) != 1 {
		return "", invalidf("source", "set exactly one of hostPath, nfs, local or csi")
	}
	kind := kinds[0]
	block := mode == corev1.PersistentVolumeBlock
	switch kind {
	case "hostPath":
		if err := absolutePath("hostPath.path", req.HostPath.Path); err != nil {
			return kind, err
		}
		if !hostPathTypes[req.HostPath.Type] {
			return kind, invalidf("hostPath.type", "%q is not a hostPath type", req.HostPath.Type)
		}
		if block {
			return kind, invalidf("volumeMode", "a hostPath volume cannot be a raw block device")
		}
	case "nfs":
		if err := nfsServer(req.NFS.Server); err != nil {
			return kind, err
		}
		if err := absolutePath("nfs.path", req.NFS.Path); err != nil {
			return kind, err
		}
		if block {
			return kind, invalidf("volumeMode", "an NFS volume cannot be a raw block device")
		}
	case "local":
		if err := absolutePath("local.path", req.Local.Path); err != nil {
			return kind, err
		}
		if err := fsType("local.fsType", req.Local.FSType); err != nil {
			return kind, err
		}
	case "csi":
		if err := qualifiedDriver("csi.driver", req.CSI.Driver); err != nil {
			return kind, err
		}
		if strings.TrimSpace(req.CSI.VolumeHandle) == "" {
			return kind, invalidf("csi.volumeHandle", "is required")
		}
		if err := fsType("csi.fsType", req.CSI.FSType); err != nil {
			return kind, err
		}
		for key := range req.CSI.VolumeAttributes {
			if key == "" {
				return kind, invalidf("csi.volumeAttributes", "keys cannot be empty")
			}
		}
	}
	return kind, nil
}

// validateNodes checks the node-affinity list against the source kind.
func (req PVRequest) validateNodes(source string) error {
	switch {
	case source == "local" && len(req.Nodes) == 0:
		return invalidf("nodes", "a local volume lives on one node: pick the node(s) it is on")
	case source == "nfs" && len(req.Nodes) > 0:
		return invalidf("nodes", "an NFS volume is reachable from every node and takes no node affinity")
	case len(req.Nodes) > maxNodes:
		return invalidf("nodes", "at most %d nodes", maxNodes)
	}
	seen := map[string]bool{}
	for _, n := range req.Nodes {
		if errs := validation.IsDNS1123Subdomain(n); len(errs) > 0 {
			return invalidf("nodes", "%q is not a node name", n)
		}
		if seen[n] {
			return invalidf("nodes", "%s is listed twice", n)
		}
		seen[n] = true
	}
	return nil
}

func nfsServer(server string) error {
	if server == "" {
		return invalidf("nfs.server", "is required")
	}
	if net.ParseIP(server) != nil {
		return nil
	}
	if errs := validation.IsDNS1123Subdomain(strings.ToLower(server)); len(errs) > 0 {
		return invalidf("nfs.server", "%q is not a host name or IP address", server)
	}
	return nil
}

// fsType checks a file system name such as ext4; "" leaves the driver's default.
func fsType(field, fs string) error {
	if fs == "" {
		return nil
	}
	if errs := validation.IsDNS1123Label(fs); len(errs) > 0 {
		return invalidf(field, "%q is not a file system type such as ext4 or xfs", fs)
	}
	return nil
}

// checkCluster checks that the pinned nodes exist.
func (s pvSpec) checkCluster(ctx context.Context, rt *kube.Runtime) error {
	for _, n := range s.req.Nodes {
		_, err := rt.CS.CoreV1().Nodes().Get(ctx, n, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			return invalidf("nodes", "no node %q", n)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func (s pvSpec) object() *corev1.PersistentVolume {
	req := s.req
	pv := &corev1.PersistentVolume{
		TypeMeta:   metav1.TypeMeta{APIVersion: "v1", Kind: "PersistentVolume"},
		ObjectMeta: metav1.ObjectMeta{Name: req.Name, Labels: req.Labels},
		Spec: corev1.PersistentVolumeSpec{
			Capacity:                      corev1.ResourceList{corev1.ResourceStorage: s.capacity},
			AccessModes:                   s.modes,
			PersistentVolumeReclaimPolicy: s.reclaim,
			StorageClassName:              req.StorageClassName,
			VolumeMode:                    s.mode,
			MountOptions:                  req.MountOptions,
		},
	}
	src := &pv.Spec.PersistentVolumeSource
	switch {
	case req.HostPath != nil:
		src.HostPath = &corev1.HostPathVolumeSource{Path: req.HostPath.Path}
		if req.HostPath.Type != "" {
			t := corev1.HostPathType(req.HostPath.Type)
			src.HostPath.Type = &t
		}
	case req.NFS != nil:
		src.NFS = &corev1.NFSVolumeSource{Server: req.NFS.Server, Path: req.NFS.Path, ReadOnly: req.NFS.ReadOnly}
	case req.Local != nil:
		src.Local = &corev1.LocalVolumeSource{Path: req.Local.Path}
		if req.Local.FSType != "" {
			fs := req.Local.FSType
			src.Local.FSType = &fs
		}
	case req.CSI != nil:
		src.CSI = &corev1.CSIPersistentVolumeSource{
			Driver: req.CSI.Driver, VolumeHandle: req.CSI.VolumeHandle, FSType: req.CSI.FSType,
			ReadOnly: req.CSI.ReadOnly, VolumeAttributes: req.CSI.VolumeAttributes,
		}
	}
	if len(req.Nodes) > 0 {
		pv.Spec.NodeAffinity = &corev1.VolumeNodeAffinity{Required: &corev1.NodeSelector{
			NodeSelectorTerms: []corev1.NodeSelectorTerm{{MatchExpressions: []corev1.NodeSelectorRequirement{{
				Key: hostnameLabel, Operator: corev1.NodeSelectorOpIn, Values: req.Nodes,
			}}}},
		}}
	}
	return pv
}

// createPV serves POST /pvs?dryRun=.
func createPV(w http.ResponseWriter, r *http.Request, c *plugin.Context) {
	var req PVRequest
	if err := decodeBody(w, r, &req); err != nil {
		writeErr(w, err)
		return
	}
	spec, err := req.validate()
	if err != nil {
		writeErr(w, err)
		return
	}
	if err := spec.checkCluster(r.Context(), c.Runtime); err != nil {
		writeErr(w, err)
		return
	}
	detail := map[string]any{"capacity": req.Capacity, "source": spec.source, "reclaimPolicy": spec.reclaim}
	if req.StorageClassName != "" {
		detail["storageClass"] = req.StorageClassName
	}
	if len(req.Nodes) > 0 {
		detail["nodes"] = req.Nodes
	}
	auditCreate(r, c, "pv", req.Name, detail)

	created, err := c.Runtime.CS.CoreV1().PersistentVolumes().Create(r.Context(), spec.object(), createOptions(r))
	if err != nil {
		writeErr(w, err)
		return
	}
	plugin.WriteJSON(w, CreateResult{APIVersion: "v1", Kind: "PersistentVolume", Name: created.Name, DryRun: plugin.IsDryRun(r)})
}
