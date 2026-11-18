package updates

import (
	"net/url"
	"strings"
	"testing"
)

// FuzzParseBuildVersion checks that a version accepted as a release build
// prints as a version that parses back to the same one, and that builds
// which are not releases (git describe, -dirty, snapshots) are refused.
func FuzzParseBuildVersion(f *testing.F) {
	for _, seed := range []string{
		"0.5.0", "v0.5.0", " v1.2.3 ", "1.2.3-rc.1", "1.2.3-rc1", "1.2.3-beta.2", "1.2.3-3-gabc1234",
		"1.2.3-rc.1-dirty", "1.2.3-SNAPSHOT", "1.2.3+build.5", "01.2.3", "1.2", "dev", "", "v", "1.2.3-rc.01",
		"99999999999999999999.0.0", "1.2.3-alpha.1.x-y",
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, s string) {
		v := ParseBuildVersion(s)
		if v == nil {
			return
		}
		again := parseVersion(v.String())
		if again == nil || !again.Equal(v) || again.Prerelease() != v.Prerelease() {
			t.Fatalf("ParseBuildVersion(%q) = %s, which does not parse back", s, v)
		}
		if pre := v.Prerelease(); pre != "" {
			ids := strings.Split(pre, ".")
			if !candidateIdent.MatchString(ids[0]) || strings.Contains(pre, "-") {
				t.Fatalf("ParseBuildVersion(%q) accepted the non-release build %s", s, v)
			}
		}
	})
}

// FuzzDecodeReleases checks the parsing of the release list the update
// check fetches: it never panics, drops drafts and non-SemVer tags, keeps
// only http(s) links (the UI renders them), flags prereleases and sorts
// the newest version first.
func FuzzDecodeReleases(f *testing.F) {
	for _, seed := range []string{
		`[{"tag_name":"v0.5.0","html_url":"https://github.com/o/r/releases/tag/v0.5.0","body":"notes"},{"tag_name":"v0.6.0-rc.1","prerelease":false},{"tag_name":"v9.0.0","draft":true}]`,
		`[{"tag_name":"nightly"},{"tag_name":"1.0.0","html_url":"javascript:alert(1)"},{"tag_name":"1.0.0-beta","html_url":"//evil"}]`,
		`[{"tag_name":"1.0.0-alpha.1"},{"tag_name":"1.0.0-alpha.beta"},{"tag_name":"1.0.0-alpha"},{"tag_name":"1.0.0-rc.1"},{"tag_name":"1.0.0"}]`,
		`[{"tag_name":"1.0.0","published_at":"2026-01-02T03:04:05Z"},{"tag_name":"1.0.0+meta"}]`,
		`{"message":"Not Found"}`, `[]`, `null`, `[null]`, `[{"tag_name":5}]`, ``,
	} {
		f.Add([]byte(seed))
	}
	f.Fuzz(func(t *testing.T, body []byte) {
		releases, err := decodeReleases(strings.NewReader(string(body)))
		if err != nil {
			return
		}
		for i, r := range releases {
			if r.version == nil {
				t.Fatalf("release %d of %q has no version", i, body)
			}
			if r.version.Prerelease() != "" && !r.prerelease {
				t.Fatalf("release %s of %q is not flagged as a prerelease", r.version, body)
			}
			if r.url != "" {
				u, err := url.Parse(r.url)
				if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
					t.Fatalf("release %s of %q links to %q", r.version, body, r.url)
				}
			}
			if i > 0 && releases[i-1].version.LessThan(r.version) {
				t.Fatalf("releases of %q are not newest first: %s before %s", body, releases[i-1].version, r.version)
			}
		}
	})
}
