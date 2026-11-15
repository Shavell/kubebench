package updates

import "testing"

func TestParseBuildVersion(t *testing.T) {
	for _, tc := range []struct {
		in, want string // want "" = not a release build
	}{
		{"0.5.0", "0.5.0"},
		{"v0.5.0", "0.5.0"},
		{"1.2.3+build.7", "1.2.3+build.7"},
		{"0.6.0-rc.1", "0.6.0-rc.1"},
		{"0.6.0-rc1", "0.6.0-rc1"},
		{"0.6.0-beta.2.x", "0.6.0-beta.2.x"},
		{"dev", ""},
		{"", ""},
		{"0.5", ""},
		{"0.5.0-3-gabc1234", ""},       // git describe past a tag
		{"0.5.0-abc1234", ""},          // no tag yet
		{"0.5.0-abc1234-dirty", ""},    // no tag, modified tree
		{"0.5.0-dirty", ""},            // tagged, modified tree
		{"0.5.1-snapshot-abc1234", ""}, // GoReleaser snapshot
		{"0.6.0-rc.1-dirty", ""},
		{"0.6.0-rc.1-2-gabc1234", ""},
	} {
		got := ParseBuildVersion(tc.in)
		switch {
		case tc.want == "" && got != nil:
			t.Errorf("ParseBuildVersion(%q) = %s, want no release version", tc.in, got)
		case tc.want != "" && (got == nil || got.String() != tc.want):
			t.Errorf("ParseBuildVersion(%q) = %v, want %s", tc.in, got, tc.want)
		}
	}
}

func TestVersionOrdering(t *testing.T) {
	// Each version precedes the next (SemVer §11), "v" prefixes aside.
	ordered := []string{"v0.4.0", "0.5.0-alpha", "0.5.0-alpha.1", "v0.5.0-beta.2", "0.5.0-beta.11", "0.5.0-rc.1", "v0.5.0", "0.5.1", "0.10.0", "1.0.0"}
	for i := 1; i < len(ordered); i++ {
		a, b := parseVersion(ordered[i-1]), parseVersion(ordered[i])
		if a == nil || b == nil {
			t.Fatalf("cannot parse %q or %q", ordered[i-1], ordered[i])
		}
		if !b.GreaterThan(a) {
			t.Errorf("%s must be newer than %s", ordered[i], ordered[i-1])
		}
	}
}

func TestParseMode(t *testing.T) {
	for _, s := range []string{"off", "stable", "prerelease"} {
		if m, ok := ParseMode(s); !ok || string(m) != s {
			t.Errorf("ParseMode(%q) = %q, %v", s, m, ok)
		}
	}
	for _, s := range []string{"", "Stable", "nightly"} {
		if _, ok := ParseMode(s); ok {
			t.Errorf("ParseMode(%q) must fail", s)
		}
	}
}
