package cloud

import (
	"strconv"
	"testing"
	"time"
)

func day(s string) time.Time {
	d, err := time.Parse(time.DateOnly, s)
	if err != nil {
		panic(err)
	}
	return d.Add(15 * time.Hour) // any time of the day counts as that day
}

func TestVersionSupport(t *testing.T) {
	tests := []struct {
		name, provider, version, now string
		want                         Support
	}{
		{"upstream supported", ProviderKubernetes, "v1.35.5", "2026-11-15",
			Support{StatusSupported, "2027-02-28", "kubernetes"}},
		{"k3s follows upstream", ProviderK3s, "v1.35.5+k3s1", "2026-12-01",
			Support{StatusEndingSoon, "2027-02-28", "kubernetes"}},
		{"exactly 90 days left", ProviderKubernetes, "v1.35.0", "2026-11-30",
			Support{StatusEndingSoon, "2027-02-28", "kubernetes"}},
		{"91 days left", ProviderKubernetes, "v1.35.0", "2026-11-29",
			Support{StatusSupported, "2027-02-28", "kubernetes"}},
		{"last day still supported", ProviderEKS, "v1.34.1-eks-abc", "2026-12-02",
			Support{StatusEndingSoon, "2026-12-02", ProviderEKS}},
		{"day after the end", ProviderEKS, "v1.34.1-eks-abc", "2026-12-03",
			Support{StatusEndOfLife, "2026-12-02", ProviderEKS}},
		{"aks table", ProviderAKS, "1.33.2", "2026-11-15",
			Support{StatusEndOfLife, "2026-07-31", ProviderAKS}},
		{"gke table", ProviderGKE, "v1.36.1-gke.100", "2026-11-15",
			Support{StatusSupported, "2027-08-09", ProviderGKE}},
		{"doks follows upstream", ProviderDOKS, "v1.34.1", "2026-11-15",
			Support{StatusEndOfLife, "2026-10-27", "kubernetes"}},
		{"older than the table", ProviderEKS, "v1.24.17-eks-abc", "2026-11-15",
			Support{StatusEndOfLife, "", ProviderEKS}},
		{"newer than the table", ProviderKubernetes, "v1.42.0", "2026-11-15",
			Support{StatusUnknown, "", "kubernetes"}},
		{"openshift has no table", ProviderOpenShift, "v1.33.5", "2026-11-15",
			Support{Status: StatusUnknown}},
		{"unparseable version", ProviderKubernetes, "unknown", "2026-11-15",
			Support{Status: StatusUnknown}},
		{"empty version", ProviderEKS, "", "2026-11-15",
			Support{Status: StatusUnknown}},
		{"other major", ProviderKubernetes, "v2.0.0", "2026-11-15",
			Support{Status: StatusUnknown}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := VersionSupport(tt.provider, tt.version, day(tt.now)); got != tt.want {
				t.Fatalf("VersionSupport(%q, %q, %s) = %+v, want %+v", tt.provider, tt.version, tt.now, got, tt.want)
			}
		})
	}
}

// The table must stay parseable and ordered: each lifecycle's minors end
// in the order they were released.
func TestSupportTable(t *testing.T) {
	for basis, table := range supportEnds {
		oldest, newest := bounds(table)
		if newest-oldest+1 != len(table) {
			t.Errorf("%s: minors 1.%d-1.%d have gaps (%d entries)", basis, oldest, newest, len(table))
		}
		var previous time.Time
		for minor := oldest; minor <= newest; minor++ {
			key := "1." + strconv.Itoa(minor)
			end, err := time.Parse(time.DateOnly, table[key])
			if err != nil {
				t.Errorf("%s %s: %v", basis, key, err)
				continue
			}
			if !end.After(previous) {
				t.Errorf("%s %s ends %s, not after the previous minor", basis, key, table[key])
			}
			previous = end
		}
	}
	for _, provider := range []string{ProviderEKS, ProviderAKS, ProviderGKE, ProviderKubernetes, ProviderK3s} {
		if _, ok := supportEnds[basisFor(provider)]; !ok {
			t.Errorf("no table for %s", provider)
		}
	}
}
