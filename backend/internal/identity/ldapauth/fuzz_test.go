package ldapauth

import (
	"bytes"
	"strings"
	"testing"

	ber "github.com/go-asn1-ber/asn1-ber"
	"github.com/go-ldap/ldap/v3"
)

// sameShape checks that the compiled filter got is template's compiled
// filter want with each value holding a sentinel replaced by the value
// the sentinel stands for: user input fills assertion values and never
// adds, removes or changes a filter element.
func sameShape(t *testing.T, got, want *ber.Packet, values map[string]string) {
	t.Helper()
	if got.ClassType != want.ClassType || got.TagType != want.TagType || got.Tag != want.Tag ||
		len(got.Children) != len(want.Children) {
		t.Fatalf("filter element changed: %s, want %s", ldapDescribe(got), ldapDescribe(want))
	}
	if len(want.Children) == 0 {
		expected := want.Data.Bytes()
		for sentinel, value := range values {
			expected = bytes.ReplaceAll(expected, []byte(sentinel), []byte(value))
		}
		if !bytes.Equal(got.Data.Bytes(), expected) {
			t.Fatalf("assertion value %q, want %q", got.Data.Bytes(), expected)
		}
	}
	for i := range want.Children {
		sameShape(t, got.Children[i], want.Children[i], values)
	}
}

func ldapDescribe(p *ber.Packet) string {
	f, err := ldap.DecompileFilter(p)
	if err != nil {
		return err.Error()
	}
	return f
}

var userTemplates = []string{"(uid=%s)", "(|(sAMAccountName=%s)(userPrincipalName=%s))", "(&(objectClass=person)(cn=*%s*))"}

// FuzzUserFilter checks that a login name, however crafted, only fills
// the assertion values of the user filter: "*", ")(uid=*", NUL or a
// backslash never change what the filter matches on.
func FuzzUserFilter(f *testing.F) {
	for _, seed := range []string{"alice", "*", ")(uid=*", "a)(|(uid=*", `\2a`, "\x00", "é", "\xff", "%s", "%d", "a\\", "(", ""} {
		f.Add(seed)
	}
	const sentinel = "kbSENTINEL"
	f.Fuzz(func(t *testing.T, login string) {
		// As Authenticate and Lookup search.
		name := (&Provider{}).NormalizeUsername(login)
		if name == "" || len(name) > maxLoginLength {
			return
		}
		for _, template := range userTemplates {
			want, err := ldap.CompileFilter(userFilter(template, sentinel))
			if err != nil {
				t.Fatal(err)
			}
			got, err := ldap.CompileFilter(userFilter(template, name))
			if err != nil {
				t.Fatalf("name %q breaks filter %s: %v", name, template, err)
			}
			sameShape(t, got, want, map[string]string{sentinel: name})
		}
	})
}

// FuzzGroupFilter is FuzzUserFilter for the group filter, filled with a
// user's DN and username from the directory.
func FuzzGroupFilter(f *testing.F) {
	f.Add("cn=alice,ou=people,dc=example,dc=org", "alice")
	f.Add(`cn=a\,b,dc=x`, "%d")
	f.Add("cn=*)(member=*", ")(memberUid=*")
	f.Add("%u", "%d")
	f.Add("", "")
	const dnSentinel, userSentinel = "kbDNSENTINEL", "kbUSERSENTINEL"
	f.Fuzz(func(t *testing.T, dn, username string) {
		for _, template := range []string{"(|(member=%d)(uniqueMember=%d)(memberUid=%u))", "(member:1.2.840.113556.1.4.1941:=%d)"} {
			want, err := ldap.CompileFilter(groupFilter(template, dnSentinel, userSentinel))
			if err != nil {
				t.Fatal(err)
			}
			got, err := ldap.CompileFilter(groupFilter(template, dn, username))
			if err != nil {
				t.Fatalf("DN %q, username %q break filter %s: %v", dn, username, template, err)
			}
			values := map[string]string{dnSentinel: dn, userSentinel: username}
			if !strings.Contains(template, "%u") {
				delete(values, userSentinel)
			}
			sameShape(t, got, want, values)
		}
	})
}
