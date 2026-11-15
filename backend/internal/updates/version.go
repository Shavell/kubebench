package updates

import (
	"regexp"
	"strings"

	"github.com/Masterminds/semver/v3"
)

// Mode selects which releases the check reports (docs/design-v0.5.md §7.11).
type Mode string

const (
	// ModeOff never contacts the releases URL (air-gapped installs).
	ModeOff Mode = "off"
	// ModeStable reports new releases, not prereleases.
	ModeStable Mode = "stable"
	// ModePrerelease reports prereleases too.
	ModePrerelease Mode = "prerelease"
)

// ParseMode returns the mode named by s, or false for an unknown name.
func ParseMode(s string) (Mode, bool) {
	switch m := Mode(s); m {
	case ModeOff, ModeStable, ModePrerelease:
		return m, true
	}
	return "", false
}

// candidateIdent is the first prerelease identifier of a release candidate
// build ("rc", "rc1", "beta", …). Anything else in that place comes from
// git describe ("3-gabc1234"), an untagged commit or a snapshot build.
var candidateIdent = regexp.MustCompile(`^(alpha|beta|rc|pre|preview)[0-9]*$`)

// ParseBuildVersion returns the version of a release build, or nil when
// version names a build that is not one: "dev", a git-describe or untagged
// commit version, a "-dirty" tree or a snapshot. Such builds report
// "unknown" and never claim that an update is available.
func ParseBuildVersion(version string) *semver.Version {
	v := parseVersion(version)
	if v == nil || v.Prerelease() == "" {
		return v
	}
	ids := strings.Split(v.Prerelease(), ".")
	if !candidateIdent.MatchString(ids[0]) {
		return nil
	}
	for _, id := range ids[1:] {
		if strings.Contains(id, "-") {
			return nil
		}
	}
	return v
}

// parseVersion parses a SemVer version with an optional "v" prefix.
func parseVersion(s string) *semver.Version {
	v, err := semver.StrictNewVersion(strings.TrimPrefix(strings.TrimSpace(s), "v"))
	if err != nil {
		return nil
	}
	return v
}
