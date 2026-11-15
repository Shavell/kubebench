package storage

import (
	"context"
	"net/http"
	"sort"

	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"kubebench/internal/kube"
	"kubebench/internal/plugin"
)

// JSON shapes mirror src/plugins/storage/data/types.ts.

// Overview answers GET /overview.
type Overview struct {
	Classes []ClassSummary `json:"classes"`
	// DefaultClass is the class claims without one get: the newest class
	// marked default, "" when there is none.
	DefaultClass       string       `json:"defaultClass"`
	PVCs               ClaimCounts  `json:"pvcs"`
	PVs                VolumeCounts `json:"pvs"`
	Capacity           Capacity     `json:"capacity"`
	CSIDrivers         int          `json:"csiDrivers"`
	SnapshotsAvailable bool         `json:"snapshotsAvailable"`
}

// ClassSummary is a StorageClass with the claims and volumes using it.
type ClassSummary struct {
	Name                 string            `json:"name"`
	Provisioner          string            `json:"provisioner"`
	ReclaimPolicy        string            `json:"reclaimPolicy"`
	VolumeBindingMode    string            `json:"volumeBindingMode"`
	AllowVolumeExpansion bool              `json:"allowVolumeExpansion"`
	IsDefault            bool              `json:"isDefault"`
	Parameters           map[string]string `json:"parameters"`
	MountOptions         []string          `json:"mountOptions"`
	PVCs                 int               `json:"pvcs"`
	PVs                  int               `json:"pvs"`
	RequestedBytes       int64             `json:"requestedBytes"`
	Age                  string            `json:"age"`
}

// ClaimCounts counts claims by phase.
type ClaimCounts struct {
	Total   int `json:"total"`
	Bound   int `json:"bound"`
	Pending int `json:"pending"`
	Lost    int `json:"lost"`
}

// VolumeCounts counts volumes by phase.
type VolumeCounts struct {
	Total     int `json:"total"`
	Available int `json:"available"`
	Bound     int `json:"bound"`
	Released  int `json:"released"`
	Failed    int `json:"failed"`
	Pending   int `json:"pending"`
}

// Capacity sums storage in bytes.
type Capacity struct {
	// RequestedBytes: what claims request.
	RequestedBytes int64 `json:"requestedBytes"`
	// ProvisionedBytes: the capacity of every PersistentVolume.
	ProvisionedBytes int64 `json:"provisionedBytes"`
	// AvailableBytes: the capacity of Available (unclaimed) volumes.
	AvailableBytes int64 `json:"availableBytes"`
}

// PVCSummary is one claim in GET /pvcs.
type PVCSummary struct {
	Namespace      string   `json:"namespace"`
	Name           string   `json:"name"`
	Phase          string   `json:"phase"`
	StorageClass   string   `json:"storageClass"`
	Requested      string   `json:"requested"`
	RequestedBytes int64    `json:"requestedBytes"`
	Capacity       string   `json:"capacity"`
	CapacityBytes  int64    `json:"capacityBytes"`
	VolumeName     string   `json:"volumeName"`
	AccessModes    []string `json:"accessModes"`
	VolumeMode     string   `json:"volumeMode"`
	// ExpandBlocker says why the claim cannot be expanded
	// (not_bound, no_class, class_missing, not_allowed); "" when it can.
	ExpandBlocker string `json:"expandBlocker"`
	// Resizing: an expansion is in progress (Resizing or
	// FileSystemResizePending condition).
	Resizing   bool              `json:"resizing"`
	DataSource *DataSource       `json:"dataSource,omitempty"`
	Labels     map[string]string `json:"labels"`
	Age        string            `json:"age"`
}

// PVSummary is one volume in GET /pvs.
type PVSummary struct {
	Name          string   `json:"name"`
	Phase         string   `json:"phase"`
	Capacity      string   `json:"capacity"`
	CapacityBytes int64    `json:"capacityBytes"`
	AccessModes   []string `json:"accessModes"`
	ReclaimPolicy string   `json:"reclaimPolicy"`
	StorageClass  string   `json:"storageClass"`
	VolumeMode    string   `json:"volumeMode"`
	// Source is hostPath, nfs, local, csi or other; SourceDetail the path,
	// server:path or driver.
	Source       string     `json:"source"`
	SourceDetail string     `json:"sourceDetail"`
	Claim        *ObjectRef `json:"claim,omitempty"`
	// Nodes are the hostnames a required node affinity pins the volume to.
	Nodes  []string `json:"nodes"`
	Reason string   `json:"reason,omitempty"`
	Age    string   `json:"age"`
}

// ObjectRef names a namespaced object.
type ObjectRef struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
}

// overview serves GET /overview.
func overview(w http.ResponseWriter, r *http.Request, c *plugin.Context) {
	ctx, rt := r.Context(), c.Runtime
	classes, err := rt.CS.StorageV1().StorageClasses().List(ctx, metav1.ListOptions{})
	if err != nil {
		plugin.WriteK8sError(w, err)
		return
	}
	pvcs, err := rt.CS.CoreV1().PersistentVolumeClaims("").List(ctx, metav1.ListOptions{})
	if err != nil {
		plugin.WriteK8sError(w, err)
		return
	}
	pvs, err := rt.CS.CoreV1().PersistentVolumes().List(ctx, metav1.ListOptions{})
	if err != nil {
		plugin.WriteK8sError(w, err)
		return
	}
	drivers, err := rt.CS.StorageV1().CSIDrivers().List(ctx, metav1.ListOptions{})
	if err != nil {
		plugin.WriteK8sError(w, err)
		return
	}
	snapshots, err := snapshotsServed(rt)
	if err != nil {
		plugin.WriteK8sError(w, err)
		return
	}
	out := summarize(classes.Items, pvcs.Items, pvs.Items)
	out.CSIDrivers = len(drivers.Items)
	out.SnapshotsAvailable = snapshots
	plugin.WriteJSON(w, out)
}

// summarize builds the overview from the listed objects.
func summarize(classes []storagev1.StorageClass, pvcs []corev1.PersistentVolumeClaim, pvs []corev1.PersistentVolume) Overview {
	out := Overview{Classes: make([]ClassSummary, 0, len(classes))}
	byName := map[string]int{}
	var newestDefault *storagev1.StorageClass
	for i := range classes {
		sc := &classes[i]
		byName[sc.Name] = len(out.Classes)
		out.Classes = append(out.Classes, summarizeClass(sc))
		if isDefaultClass(sc) && (newestDefault == nil || newer(sc, newestDefault)) {
			newestDefault = sc
		}
	}
	if newestDefault != nil {
		out.DefaultClass = newestDefault.Name
	}
	for i := range pvcs {
		pvc := &pvcs[i]
		size := requested(pvc)
		out.PVCs.Total++
		out.Capacity.RequestedBytes += size.Value()
		switch pvc.Status.Phase {
		case corev1.ClaimBound:
			out.PVCs.Bound++
		case corev1.ClaimLost:
			out.PVCs.Lost++
		default:
			out.PVCs.Pending++
		}
		if pvc.Spec.StorageClassName != nil {
			if j, ok := byName[*pvc.Spec.StorageClassName]; ok {
				out.Classes[j].PVCs++
				out.Classes[j].RequestedBytes += size.Value()
			}
		}
	}
	for i := range pvs {
		pv := &pvs[i]
		capacity := pv.Spec.Capacity[corev1.ResourceStorage]
		out.PVs.Total++
		out.Capacity.ProvisionedBytes += capacity.Value()
		switch pv.Status.Phase {
		case corev1.VolumeAvailable:
			out.PVs.Available++
			out.Capacity.AvailableBytes += capacity.Value()
		case corev1.VolumeBound:
			out.PVs.Bound++
		case corev1.VolumeReleased:
			out.PVs.Released++
		case corev1.VolumeFailed:
			out.PVs.Failed++
		default:
			out.PVs.Pending++
		}
		if j, ok := byName[pv.Spec.StorageClassName]; ok {
			out.Classes[j].PVs++
		}
	}
	return out
}

func summarizeClass(sc *storagev1.StorageClass) ClassSummary {
	reclaim, binding := string(corev1.PersistentVolumeReclaimDelete), string(storagev1.VolumeBindingImmediate)
	if sc.ReclaimPolicy != nil {
		reclaim = string(*sc.ReclaimPolicy)
	}
	if sc.VolumeBindingMode != nil {
		binding = string(*sc.VolumeBindingMode)
	}
	return ClassSummary{
		Name: sc.Name, Provisioner: sc.Provisioner, ReclaimPolicy: reclaim, VolumeBindingMode: binding,
		AllowVolumeExpansion: sc.AllowVolumeExpansion != nil && *sc.AllowVolumeExpansion,
		IsDefault:            isDefaultClass(sc),
		Parameters:           nonNilMap(sc.Parameters), MountOptions: nonNil(sc.MountOptions),
		Age: kube.FormatAge(sc.CreationTimestamp.Time),
	}
}

// newer orders objects as the API server picks among default classes:
// the newest creation time wins, then the name.
func newer(a, b *storagev1.StorageClass) bool {
	if !a.CreationTimestamp.Equal(&b.CreationTimestamp) {
		return b.CreationTimestamp.Before(&a.CreationTimestamp)
	}
	return a.Name < b.Name
}

// listPVCs serves GET /pvcs?ns=.
func listPVCs(w http.ResponseWriter, r *http.Request, c *plugin.Context) {
	ns := r.URL.Query().Get("ns")
	if ns != "" {
		if err := namespaceName("ns", ns); err != nil {
			writeErr(w, err)
			return
		}
	}
	list, err := c.Runtime.CS.CoreV1().PersistentVolumeClaims(ns).List(r.Context(), metav1.ListOptions{})
	if err != nil {
		plugin.WriteK8sError(w, err)
		return
	}
	classes, err := classExpansion(r.Context(), c.Runtime)
	if err != nil {
		plugin.WriteK8sError(w, err)
		return
	}
	out := make([]PVCSummary, 0, len(list.Items))
	for i := range list.Items {
		out = append(out, summarizePVC(&list.Items[i], classes))
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Namespace != out[j].Namespace {
			return out[i].Namespace < out[j].Namespace
		}
		return out[i].Name < out[j].Name
	})
	plugin.WriteJSON(w, out)
}

// classExpansion maps every StorageClass to whether it allows expansion.
func classExpansion(ctx context.Context, rt *kube.Runtime) (map[string]bool, error) {
	list, err := rt.CS.StorageV1().StorageClasses().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	out := make(map[string]bool, len(list.Items))
	for _, sc := range list.Items {
		out[sc.Name] = sc.AllowVolumeExpansion != nil && *sc.AllowVolumeExpansion
	}
	return out, nil
}

func summarizePVC(pvc *corev1.PersistentVolumeClaim, classes map[string]bool) PVCSummary {
	req := requested(pvc)
	capacity := pvc.Status.Capacity[corev1.ResourceStorage]
	s := PVCSummary{
		Namespace: pvc.Namespace, Name: pvc.Name, Phase: phaseOr(string(pvc.Status.Phase), "Pending"),
		Requested: quantityText(req), RequestedBytes: req.Value(),
		Capacity: quantityText(capacity), CapacityBytes: capacity.Value(),
		VolumeName: pvc.Spec.VolumeName, AccessModes: modeNames(pvc.Spec.AccessModes),
		VolumeMode:    string(modeOf(pvc.Spec.VolumeMode)),
		ExpandBlocker: expandBlocker(pvc, classes),
		Labels:        nonNilMap(pvc.Labels), Age: kube.FormatAge(pvc.CreationTimestamp.Time),
	}
	if pvc.Spec.StorageClassName != nil {
		s.StorageClass = *pvc.Spec.StorageClassName
	}
	if ds := pvc.Spec.DataSource; ds != nil {
		s.DataSource = &DataSource{Kind: ds.Kind, Name: ds.Name}
	}
	for _, cond := range pvc.Status.Conditions {
		if cond.Status == corev1.ConditionTrue &&
			(cond.Type == corev1.PersistentVolumeClaimResizing || cond.Type == corev1.PersistentVolumeClaimFileSystemResizePending) {
			s.Resizing = true
		}
	}
	return s
}

// listPVs serves GET /pvs.
func listPVs(w http.ResponseWriter, r *http.Request, c *plugin.Context) {
	list, err := c.Runtime.CS.CoreV1().PersistentVolumes().List(r.Context(), metav1.ListOptions{})
	if err != nil {
		plugin.WriteK8sError(w, err)
		return
	}
	out := make([]PVSummary, 0, len(list.Items))
	for i := range list.Items {
		out = append(out, summarizePV(&list.Items[i]))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	plugin.WriteJSON(w, out)
}

func summarizePV(pv *corev1.PersistentVolume) PVSummary {
	capacity := pv.Spec.Capacity[corev1.ResourceStorage]
	s := PVSummary{
		Name: pv.Name, Phase: phaseOr(string(pv.Status.Phase), "Pending"),
		Capacity: quantityText(capacity), CapacityBytes: capacity.Value(),
		AccessModes:   modeNames(pv.Spec.AccessModes),
		ReclaimPolicy: string(pv.Spec.PersistentVolumeReclaimPolicy),
		StorageClass:  pv.Spec.StorageClassName, VolumeMode: string(modeOf(pv.Spec.VolumeMode)),
		Nodes: affinityNodes(pv), Reason: pv.Status.Message, Age: kube.FormatAge(pv.CreationTimestamp.Time),
	}
	s.Source, s.SourceDetail = volumeSource(&pv.Spec.PersistentVolumeSource)
	if ref := pv.Spec.ClaimRef; ref != nil {
		s.Claim = &ObjectRef{Namespace: ref.Namespace, Name: ref.Name}
	}
	return s
}

func volumeSource(src *corev1.PersistentVolumeSource) (kind, detail string) {
	switch {
	case src.HostPath != nil:
		return "hostPath", src.HostPath.Path
	case src.NFS != nil:
		return "nfs", src.NFS.Server + ":" + src.NFS.Path
	case src.Local != nil:
		return "local", src.Local.Path
	case src.CSI != nil:
		return "csi", src.CSI.Driver
	}
	return "other", ""
}

// affinityNodes lists the hostnames of In requirements in the volume's
// required node affinity.
func affinityNodes(pv *corev1.PersistentVolume) []string {
	nodes := []string{}
	if pv.Spec.NodeAffinity == nil || pv.Spec.NodeAffinity.Required == nil {
		return nodes
	}
	for _, term := range pv.Spec.NodeAffinity.Required.NodeSelectorTerms {
		for _, expr := range term.MatchExpressions {
			if expr.Key == hostnameLabel && expr.Operator == corev1.NodeSelectorOpIn {
				nodes = append(nodes, expr.Values...)
			}
		}
	}
	return nodes
}

func modeNames(modes []corev1.PersistentVolumeAccessMode) []string {
	out := make([]string, len(modes))
	for i, m := range modes {
		out[i] = string(m)
	}
	return out
}

// snapshotsServed reports whether the cluster serves VolumeSnapshots.
func snapshotsServed(rt *kube.Runtime) (bool, error) {
	_, _, err := rt.ResolveResource(snapshotGroup, "volumesnapshots")
	if meta.IsNoMatchError(err) {
		return false, nil
	}
	return err == nil, err
}

func nonNil[T any](s []T) []T {
	if s == nil {
		return []T{}
	}
	return s
}

func nonNilMap(m map[string]string) map[string]string {
	if m == nil {
		return map[string]string{}
	}
	return m
}

// quantityText is a quantity as Kubernetes writes it; "" when unset.
func quantityText(q resource.Quantity) string {
	if q.IsZero() {
		return ""
	}
	return q.String()
}
