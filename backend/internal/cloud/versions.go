package cloud

import (
	"regexp"
	"strconv"
	"time"
)

// Support statuses of a cluster's Kubernetes minor version.
const (
	StatusSupported  = "supported"
	StatusEndingSoon = "ending-soon" // support ends within EndingSoonWindow
	StatusEndOfLife  = "end-of-life"
	StatusUnknown    = "unknown"
)

// EndingSoonWindow is how long before its end date a version is reported
// as ending soon.
const EndingSoonWindow = 90 * 24 * time.Hour

// Support is a version's support status. EndDate (YYYY-MM-DD) is the end
// of support found in the table, "" when none applies; Basis names the
// table: "eks", "aks", "gke" or "kubernetes" (upstream), "" for none.
type Support struct {
	Status  string `json:"status"`
	EndDate string `json:"endDate"`
	Basis   string `json:"basis"`
}

// supportEnds lists, per lifecycle, the date standard support of each
// minor version ends (no extended or LTS support: that costs extra or must
// be opted into, and KubeBench cannot tell).
//
// As of 2026-11-15. To update, add the new minors and correct changed
// dates from the providers' calendars, then adjust the "as of" date:
//
//   - kubernetes: end of patch support, https://kubernetes.io/releases/
//     (also https://endoflife.date/kubernetes)
//   - eks: end of standard support,
//     https://docs.aws.amazon.com/eks/latest/userguide/kubernetes-versions.html
//     (also https://endoflife.date/amazon-eks)
//   - aks: end of life (GA, no LTS),
//     https://learn.microsoft.com/azure/aks/supported-kubernetes-versions#aks-kubernetes-release-calendar
//     (also https://endoflife.date/azure-kubernetes-service)
//   - gke: end of standard support,
//     https://cloud.google.com/kubernetes-engine/docs/release-schedule
//     (also https://endoflife.date/google-kubernetes-engine)
var supportEnds = map[string]map[string]string{
	"kubernetes": {
		"1.29": "2025-02-28", "1.30": "2025-07-15", "1.31": "2025-11-11", "1.32": "2026-02-28",
		"1.33": "2026-06-28", "1.34": "2026-10-27", "1.35": "2027-02-28", "1.36": "2027-06-28",
		"1.37": "2027-10-28",
	},
	ProviderEKS: {
		"1.29": "2025-03-23", "1.30": "2025-07-23", "1.31": "2025-11-26", "1.32": "2026-03-23",
		"1.33": "2026-07-29", "1.34": "2026-12-02", "1.35": "2027-03-27", "1.36": "2027-08-02",
		"1.37": "2027-12-01",
	},
	ProviderAKS: {
		"1.29": "2025-03-31", "1.30": "2025-08-22", "1.31": "2025-11-01", "1.32": "2026-03-31",
		"1.33": "2026-07-31", "1.34": "2026-11-30", "1.35": "2027-03-31", "1.36": "2027-06-30",
		"1.37": "2027-10-31",
	},
	ProviderGKE: {
		"1.29": "2025-04-12", "1.30": "2025-09-30", "1.31": "2026-01-16", "1.32": "2026-04-11",
		"1.33": "2026-08-03", "1.34": "2026-10-01", "1.35": "2027-04-11", "1.36": "2027-08-09",
		"1.37": "2027-12-22",
	},
}

// basisFor is the lifecycle a provider follows. Distributions and the
// smaller managed services track upstream; OpenShift has its own
// lifecycle, which the table does not carry.
func basisFor(provider string) string {
	switch provider {
	case ProviderEKS, ProviderAKS, ProviderGKE:
		return provider
	case ProviderOpenShift:
		return ""
	default:
		return "kubernetes"
	}
}

var minorPattern = regexp.MustCompile(`^v?(\d+)\.(\d+)`)

// VersionSupport reports whether the provider still supports gitVersion's
// minor version on the day of now. A minor older than every minor of the
// table is end-of-life (the table drops versions long past their end); a
// newer one is unknown, as are unparseable versions and providers without
// a table.
func VersionSupport(provider, gitVersion string, now time.Time) Support {
	basis := basisFor(provider)
	major, minor, ok := parseMinor(gitVersion)
	if basis == "" || !ok {
		return Support{Status: StatusUnknown}
	}
	table := supportEnds[basis]
	key := strconv.Itoa(major) + "." + strconv.Itoa(minor)
	end, found := table[key]
	if !found {
		oldest, newest := bounds(table)
		switch {
		case major == 1 && minor < oldest:
			return Support{Status: StatusEndOfLife, Basis: basis}
		case major == 1 && minor > newest:
			return Support{Status: StatusUnknown, Basis: basis}
		default:
			return Support{Status: StatusUnknown}
		}
	}
	endDay, err := time.Parse(time.DateOnly, end)
	if err != nil { // the table is checked by a test
		return Support{Status: StatusUnknown}
	}
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	out := Support{EndDate: end, Basis: basis}
	switch {
	case today.After(endDay):
		out.Status = StatusEndOfLife
	case endDay.Sub(today) <= EndingSoonWindow:
		out.Status = StatusEndingSoon
	default:
		out.Status = StatusSupported
	}
	return out
}

func parseMinor(gitVersion string) (major, minor int, ok bool) {
	m := minorPattern.FindStringSubmatch(gitVersion)
	if m == nil {
		return 0, 0, false
	}
	major, err1 := strconv.Atoi(m[1])
	minor, err2 := strconv.Atoi(m[2])
	return major, minor, err1 == nil && err2 == nil
}

// bounds returns the oldest and newest 1.x minor of a table.
func bounds(table map[string]string) (oldest, newest int) {
	oldest = -1
	for key := range table {
		_, minor, _ := parseMinor(key)
		if oldest < 0 || minor < oldest {
			oldest = minor
		}
		if minor > newest {
			newest = minor
		}
	}
	return oldest, newest
}
