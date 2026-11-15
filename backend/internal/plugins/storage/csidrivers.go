package storage

import (
	"net/http"
	"sort"

	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"kubebench/internal/kube"
	"kubebench/internal/plugin"
)

// CSIDriverSummary is one CSIDriver in GET /csidrivers, with the API
// defaults filled in for unset fields.
type CSIDriverSummary struct {
	Name           string `json:"name"`
	AttachRequired bool   `json:"attachRequired"`
	PodInfoOnMount bool   `json:"podInfoOnMount"`
	// StorageCapacity: the driver publishes CSIStorageCapacity objects.
	StorageCapacity bool     `json:"storageCapacity"`
	FSGroupPolicy   string   `json:"fsGroupPolicy"`
	LifecycleModes  []string `json:"lifecycleModes"`
	Age             string   `json:"age"`
}

// listCSIDrivers serves GET /csidrivers.
func listCSIDrivers(w http.ResponseWriter, r *http.Request, c *plugin.Context) {
	list, err := c.Runtime.CS.StorageV1().CSIDrivers().List(r.Context(), metav1.ListOptions{})
	if err != nil {
		plugin.WriteK8sError(w, err)
		return
	}
	out := make([]CSIDriverSummary, 0, len(list.Items))
	for i := range list.Items {
		out = append(out, summarizeDriver(&list.Items[i]))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	plugin.WriteJSON(w, out)
}

func summarizeDriver(d *storagev1.CSIDriver) CSIDriverSummary {
	s := CSIDriverSummary{
		Name:            d.Name,
		AttachRequired:  d.Spec.AttachRequired == nil || *d.Spec.AttachRequired,
		PodInfoOnMount:  d.Spec.PodInfoOnMount != nil && *d.Spec.PodInfoOnMount,
		StorageCapacity: d.Spec.StorageCapacity != nil && *d.Spec.StorageCapacity,
		FSGroupPolicy:   string(storagev1.ReadWriteOnceWithFSTypeFSGroupPolicy),
		LifecycleModes:  []string{string(storagev1.VolumeLifecyclePersistent)},
		Age:             kube.FormatAge(d.CreationTimestamp.Time),
	}
	if d.Spec.FSGroupPolicy != nil {
		s.FSGroupPolicy = string(*d.Spec.FSGroupPolicy)
	}
	if len(d.Spec.VolumeLifecycleModes) > 0 {
		s.LifecycleModes = s.LifecycleModes[:0]
		for _, m := range d.Spec.VolumeLifecycleModes {
			s.LifecycleModes = append(s.LifecycleModes, string(m))
		}
	}
	return s
}
