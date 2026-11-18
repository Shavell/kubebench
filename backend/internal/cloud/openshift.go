package cloud

import (
	"strconv"
	"time"
)

// Phases of an OpenShift minor version's life cycle (Support.Phase).
const (
	PhaseFull        = "full"        // full support
	PhaseMaintenance = "maintenance" // maintenance support: critical and security fixes
	PhaseEUS         = "eus"         // Extended Update Support (even minors; part of some subscriptions)
	PhaseEnded       = "ended"       // past every phase in the table
)

// openShiftDates are the ends of an OpenShift minor's support phases
// (YYYY-MM-DD); EUS is "" for minors without Extended Update Support.
type openShiftDates struct {
	Full, Maintenance, EUS string
}

// openShiftLifecycle is Red Hat's OpenShift Container Platform life cycle.
// OpenShift versions are not Kubernetes versions (OpenShift 5.0 ships
// Kubernetes 1.36), so the cluster's own version is looked up, read from
// its ClusterVersion.
//
// As of 2026-11-18, from https://access.redhat.com/support/policy/updates/openshift
// (dates: https://access.redhat.com/product-life-cycles/?product=Red%20Hat%20OpenShift%20Container%20Platform,
// also https://endoflife.date/red-hat-openshift). EUS is EUS term 1; term 2
// and 5.x's Extended Life Cycle are paid add-ons and not listed. To update,
// add the new minors and correct changed dates, then adjust the "as of"
// date.
var openShiftLifecycle = map[string]openShiftDates{
	"4.12": {"2023-08-17", "2024-07-17", "2025-01-17"},
	"4.13": {"2024-01-31", "2024-11-17", ""},
	"4.14": {"2024-05-27", "2025-05-01", "2025-10-31"},
	"4.15": {"2024-09-27", "2025-08-27", ""},
	"4.16": {"2025-01-01", "2025-12-27", "2026-06-27"},
	"4.17": {"2025-05-25", "2026-04-01", ""},
	"4.18": {"2025-09-17", "2026-08-25", "2027-02-25"},
	"4.19": {"2026-01-21", "2026-12-17", ""},
	"4.20": {"2026-05-03", "2027-04-21", "2027-10-21"},
	"4.21": {"2026-09-09", "2027-08-03", ""},
	"4.22": {"2026-12-31", "2027-12-31", "2028-06-30"},
	"5.0":  {"2027-11-30", "2028-11-30", ""},
}

// openShiftSupport is VersionSupport for OpenShift: version is the
// OpenShift version ("4.20.3", "5.0.0"). The status follows the end of
// maintenance support, the last phase every subscription has (EUS is not
// assumed, like the clouds' extended support); Phase, FullSupportEnd and
// EUSEnd describe the rest of the life cycle. A minor older than the table
// is end-of-life; a newer one, or an unparseable version, unknown.
func openShiftSupport(version string, now time.Time) Support {
	major, minor, ok := parseMinor(version)
	if !ok {
		return Support{Status: StatusUnknown}
	}
	dates, found := openShiftLifecycle[strconv.Itoa(major)+"."+strconv.Itoa(minor)]
	if !found {
		if olderThanOpenShiftTable(major, minor) {
			return Support{Status: StatusEndOfLife, Basis: ProviderOpenShift, Phase: PhaseEnded}
		}
		return Support{Status: StatusUnknown, Basis: ProviderOpenShift}
	}
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	out := Support{
		Status:         statusAt(today, dates.Maintenance),
		EndDate:        dates.Maintenance,
		Basis:          ProviderOpenShift,
		FullSupportEnd: dates.Full,
		EUSEnd:         dates.EUS,
	}
	switch {
	case !after(today, dates.Full):
		out.Phase = PhaseFull
	case !after(today, dates.Maintenance):
		out.Phase = PhaseMaintenance
	case dates.EUS != "" && !after(today, dates.EUS):
		out.Phase = PhaseEUS
	default:
		out.Phase = PhaseEnded
	}
	return out
}

// olderThanOpenShiftTable reports whether major.minor precedes every
// minor of the table.
func olderThanOpenShiftTable(major, minor int) bool {
	for key := range openShiftLifecycle {
		kMajor, kMinor, _ := parseMinor(key)
		if major > kMajor || (major == kMajor && minor >= kMinor) {
			return false
		}
	}
	return true
}

// after reports whether today is past the end day (YYYY-MM-DD).
func after(today time.Time, end string) bool {
	day, err := time.Parse(time.DateOnly, end)
	return err == nil && today.After(day)
}

// statusAt is the support status on today of a window ending on end.
func statusAt(today time.Time, end string) string {
	day, err := time.Parse(time.DateOnly, end)
	switch {
	case err != nil: // the tables are checked by tests
		return StatusUnknown
	case today.After(day):
		return StatusEndOfLife
	case day.Sub(today) <= EndingSoonWindow:
		return StatusEndingSoon
	default:
		return StatusSupported
	}
}
