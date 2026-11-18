package cloud

import (
	"testing"
	"time"
)

func TestOpenShiftSupport(t *testing.T) {
	tests := []struct {
		name, version, now string
		want               Support
	}{
		{"5.0 in full support", "5.0.0", "2026-11-18",
			Support{Status: StatusSupported, EndDate: "2028-11-30", Basis: ProviderOpenShift, Phase: PhaseFull, FullSupportEnd: "2027-11-30"}},
		{"4.20 in maintenance", "4.20.46", "2026-11-18",
			Support{Status: StatusSupported, EndDate: "2027-04-21", Basis: ProviderOpenShift, Phase: PhaseMaintenance,
				FullSupportEnd: "2026-05-03", EUSEnd: "2027-10-21"}},
		{"4.19 ending soon", "4.19.55", "2026-11-18",
			Support{Status: StatusEndingSoon, EndDate: "2026-12-17", Basis: ProviderOpenShift, Phase: PhaseMaintenance, FullSupportEnd: "2026-01-21"}},
		{"4.18 past maintenance, in EUS", "4.18.60", "2026-11-18",
			Support{Status: StatusEndOfLife, EndDate: "2026-08-25", Basis: ProviderOpenShift, Phase: PhaseEUS,
				FullSupportEnd: "2025-09-17", EUSEnd: "2027-02-25"}},
		{"4.17 ended", "4.17.60", "2026-11-18",
			Support{Status: StatusEndOfLife, EndDate: "2026-04-01", Basis: ProviderOpenShift, Phase: PhaseEnded, FullSupportEnd: "2025-05-25"}},
		{"last day of full support", "5.0.1", "2027-11-30",
			Support{Status: StatusSupported, EndDate: "2028-11-30", Basis: ProviderOpenShift, Phase: PhaseFull, FullSupportEnd: "2027-11-30"}},
		{"older than the table", "4.9.59", "2026-11-18",
			Support{Status: StatusEndOfLife, Basis: ProviderOpenShift, Phase: PhaseEnded}},
		{"3.x is older", "3.11.0", "2026-11-18",
			Support{Status: StatusEndOfLife, Basis: ProviderOpenShift, Phase: PhaseEnded}},
		{"newer than the table", "5.3.0", "2026-11-18",
			Support{Status: StatusUnknown, Basis: ProviderOpenShift}},
		{"unparseable", "latest", "2026-11-18", Support{Status: StatusUnknown}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := openShiftSupport(tt.version, day(tt.now)); got != tt.want {
				t.Fatalf("openShiftSupport(%q, %s) = %+v, want %+v", tt.version, tt.now, got, tt.want)
			}
		})
	}
}

func TestSupportOf(t *testing.T) {
	now := day("2026-11-18")
	// OpenShift is judged by its own version, never by Kubernetes'.
	got := SupportOf(Detection{Provider: ProviderOpenShift, Version: "v1.36.5", ProductVersion: "5.0.0"}, now)
	if got.Basis != ProviderOpenShift || got.Status != StatusSupported || got.EndDate != "2028-11-30" {
		t.Fatalf("OpenShift 5.0 = %+v", got)
	}
	if got := SupportOf(Detection{Provider: ProviderOpenShift, Version: "v1.36.5"}, now); got != (Support{Status: StatusUnknown}) {
		t.Fatalf("OpenShift without a ClusterVersion = %+v, want unknown", got)
	}
	if got := SupportOf(Detection{Provider: ProviderK3s, Version: "v1.35.5+k3s1"}, now); got.Basis != "kubernetes" {
		t.Fatalf("k3s = %+v, want the upstream table", got)
	}
}

// Every date parses, phases end in order, and minors end in release order.
func TestOpenShiftTable(t *testing.T) {
	var previous time.Time
	for _, key := range []string{"4.12", "4.13", "4.14", "4.15", "4.16", "4.17", "4.18", "4.19", "4.20", "4.21", "4.22", "5.0"} {
		d, ok := openShiftLifecycle[key]
		if !ok {
			t.Fatalf("%s missing", key)
		}
		full, err1 := time.Parse(time.DateOnly, d.Full)
		maint, err2 := time.Parse(time.DateOnly, d.Maintenance)
		if err1 != nil || err2 != nil {
			t.Fatalf("%s: %v %v", key, err1, err2)
		}
		if !maint.After(full) {
			t.Errorf("%s: maintenance ends before full support", key)
		}
		if d.EUS != "" {
			eus, err := time.Parse(time.DateOnly, d.EUS)
			if err != nil || !eus.After(maint) {
				t.Errorf("%s: EUS %q does not end after maintenance", key, d.EUS)
			}
		}
		if !maint.After(previous) {
			t.Errorf("%s ends maintenance %s, not after the previous minor", key, d.Maintenance)
		}
		previous = maint
	}
	if len(openShiftLifecycle) != 12 {
		t.Errorf("table has %d minors; list the new ones above", len(openShiftLifecycle))
	}
}

func TestDetectOpenShiftByClusterVersion(t *testing.T) {
	d := Detect(Signals{GitVersion: "v1.36.5", OpenShiftVersion: "5.0.0", Nodes: []Node{{ProviderID: "aws:///eu-west-1a/i-1"}}})
	want := Detection{Provider: ProviderOpenShift, Cloud: CloudAWS, Version: "v1.36.5", ProductVersion: "5.0.0"}
	if d != want {
		t.Fatalf("Detect = %+v, want %+v", d, want)
	}
}
