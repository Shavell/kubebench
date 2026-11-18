package ldapauth

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/go-ldap/ldap/v3"

	"kubebench/internal/config"
	"kubebench/internal/identity"
)

// fakeDirectory answers searches by their exact filter and binds by DN and
// password, like a directory whose content the test spells out.
type fakeDirectory struct {
	mu        sync.Mutex
	results   map[string][]*ldap.Entry // filter -> entries
	passwords map[string]string        // DN -> password
	disabled  map[string]bool          // DN -> bind refused like Active Directory (data 533)
	dials     int
	filters   []string
	// dropNext makes the next search fail like a connection the server
	// closed.
	dropNext bool
}

func newFakeDirectory() *fakeDirectory {
	return &fakeDirectory{
		results:   map[string][]*ldap.Entry{},
		passwords: map[string]string{"cn=svc,dc=example,dc=org": "svc-pw"},
		disabled:  map[string]bool{},
	}
}

func (d *fakeDirectory) dial() (conn, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.dials++
	return &fakeConn{d: d}, nil
}

type fakeConn struct {
	d      *fakeDirectory
	closed bool
}

func (c *fakeConn) Bind(dn, password string) error {
	c.d.mu.Lock()
	defer c.d.mu.Unlock()
	want, ok := c.d.passwords[dn]
	switch {
	case ok && want == password && c.d.disabled[dn]:
		return ldap.NewError(ldap.LDAPResultInvalidCredentials,
			errors.New("80090308: LdapErr: DSID-0C090569, comment: AcceptSecurityContext error, data 533, v4563"))
	case !ok || want != password:
		return ldap.NewError(ldap.LDAPResultInvalidCredentials, errors.New("invalid credentials"))
	}
	return nil
}

func (c *fakeConn) Search(req *ldap.SearchRequest) (*ldap.SearchResult, error) {
	c.d.mu.Lock()
	defer c.d.mu.Unlock()
	if c.d.dropNext {
		c.d.dropNext = false
		c.closed = true
		return nil, ldap.NewError(ldap.ErrorNetwork, errors.New("connection closed"))
	}
	c.d.filters = append(c.d.filters, req.Filter)
	if req.Filter == "(objectClass=*)" {
		return &ldap.SearchResult{Entries: []*ldap.Entry{{DN: req.BaseDN}}}, nil
	}
	return &ldap.SearchResult{Entries: c.d.results[req.Filter]}, nil
}

func (c *fakeConn) SearchWithPaging(req *ldap.SearchRequest, _ uint32) (*ldap.SearchResult, error) {
	return c.Search(req)
}

func (c *fakeConn) IsClosing() bool { return c.closed }
func (c *fakeConn) Close() error    { c.closed = true; return nil }

func entry(dn string, attrs map[string][]string) *ldap.Entry {
	return ldap.NewEntry(dn, attrs)
}

func openLDAPConfig() config.LDAPConfig {
	return config.LDAPConfig{
		URL: "ldaps://ldap.example.org", Type: config.LDAPTypeOpenLDAP, Name: "Directory",
		BindDN: "cn=svc,dc=example,dc=org", BindPassword: "svc-pw",
		UserBaseDN: "ou=people,dc=example,dc=org", UserFilter: "(uid=%s)",
		UsernameAttribute: "uid", IDAttribute: "entryUUID", DisplayNameAttribute: "cn", EmailAttribute: "mail",
		GroupsFrom: config.LDAPGroupsFromSearch, GroupBaseDN: "ou=groups,dc=example,dc=org",
		GroupFilter: "(|(member=%d)(uniqueMember=%d)(memberUid=%u))", GroupNameAttribute: "cn",
		Timeout: 5 * time.Second,
	}
}

func adConfig() config.LDAPConfig {
	return config.LDAPConfig{
		URL: "ldap://dc.corp.example", StartTLS: true, Type: config.LDAPTypeActiveDirectory, Name: "Active Directory",
		BindDN: "cn=svc,dc=example,dc=org", BindPassword: "svc-pw",
		UserBaseDN: "DC=corp,DC=example", UserFilter: "(|(sAMAccountName=%s)(userPrincipalName=%s))",
		UsernameAttribute: "sAMAccountName", IDAttribute: "objectGUID", DisplayNameAttribute: "displayName",
		EmailAttribute: "mail", GroupsFrom: config.LDAPGroupsFromSearch, GroupBaseDN: "DC=corp,DC=example",
		GroupFilter: "(member:1.2.840.113556.1.4.1941:=%d)", GroupNameAttribute: "cn", Timeout: 5 * time.Second,
	}
}

func newTestProvider(t *testing.T, cfg config.LDAPConfig) (*Provider, *fakeDirectory) {
	t.Helper()
	p, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	d := newFakeDirectory()
	p.dial = d.dial
	return p, d
}

const aliceDN = "uid=alice,ou=people,dc=example,dc=org"

func seedAlice(d *fakeDirectory) {
	d.passwords[aliceDN] = "alice-pw"
	d.results["(uid=alice)"] = []*ldap.Entry{entry(aliceDN, map[string][]string{
		"uid": {"Alice"}, "entryUUID": {"0b7e5c1e-1111-4a8e-9a63-2d6f5b8a0001"}, "cn": {"Alice Liddell"},
		"mail": {"alice@example.org"},
	})}
	d.results["(|(member="+aliceDN+")(uniqueMember="+aliceDN+")(memberUid=Alice))"] = []*ldap.Entry{
		entry("cn=kb-ops,ou=groups,dc=example,dc=org", map[string][]string{"cn": {"kb-ops"}}),
		entry("cn=KB-Ops,ou=legacy,dc=example,dc=org", map[string][]string{"cn": {"KB-Ops"}}),
		entry("cn=developers,ou=groups,dc=example,dc=org", map[string][]string{"cn": {"developers"}}),
	}
}

func classOf(t *testing.T, err error) string {
	t.Helper()
	if err == nil {
		t.Fatal("expected a refusal")
	}
	return identity.ClassOf(err)
}

func TestAuthenticateOpenLDAP(t *testing.T) {
	p, d := newTestProvider(t, openLDAPConfig())
	seedAlice(d)
	ctx := context.Background()

	id, err := p.Authenticate(ctx, "  ALICE ", "alice-pw")
	if err != nil {
		t.Fatal(err)
	}
	want := identity.Identity{Source: "ldap", Subject: "0b7e5c1e-1111-4a8e-9a63-2d6f5b8a0001", Username: "alice",
		DisplayName: "Alice Liddell", Email: "alice@example.org"}
	got := *id
	got.Groups = nil
	if !reflect.DeepEqual(got, want) || !slices.Equal(id.Groups, []string{"developers", "kb-ops"}) {
		t.Fatalf("identity = %+v", id)
	}

	// The service connection is pooled; each password check dials its own.
	if _, err := p.Authenticate(ctx, "alice", "alice-pw"); err != nil {
		t.Fatal(err)
	}
	if d.dials != 3 {
		t.Fatalf("dials = %d, want 1 pooled service connection + 2 user binds", d.dials)
	}

	if c := classOf(t, second(p.Authenticate(ctx, "alice", "wrong"))); c != identity.ClassInvalidCredentials {
		t.Fatalf("wrong password: %s", c)
	}
	if c := classOf(t, second(p.Authenticate(ctx, "nobody", "x"))); c != identity.ClassInvalidCredentials {
		t.Fatalf("unknown user: %s", c)
	}
}

func second[T any](_ T, err error) error { return err }

func TestEmptyPasswordNeverReachesTheDirectory(t *testing.T) {
	p, d := newTestProvider(t, openLDAPConfig())
	seedAlice(d)
	// An unauthenticated bind (a DN and no password) succeeds on many
	// directories; it must not count as a sign-in.
	d.passwords[aliceDN] = ""
	for _, login := range []string{"alice", "", "   "} {
		if c := classOf(t, second(p.Authenticate(context.Background(), login, ""))); c != identity.ClassInvalidCredentials {
			t.Fatalf("%q: %s", login, c)
		}
	}
	if d.dials != 0 {
		t.Fatalf("dials = %d, want none", d.dials)
	}
}

func TestFilterInjectionIsEscaped(t *testing.T) {
	p, d := newTestProvider(t, openLDAPConfig())
	seedAlice(d)
	d.results["(uid=*)"] = d.results["(uid=alice)"]
	if c := classOf(t, second(p.Authenticate(context.Background(), "*", "alice-pw"))); c != identity.ClassInvalidCredentials {
		t.Fatalf("wildcard login: %s", c)
	}
	if c := classOf(t, second(p.Authenticate(context.Background(), "x)(uid=*", "alice-pw"))); c != identity.ClassInvalidCredentials {
		t.Fatalf("injected login: %s", c)
	}
	if !slices.Contains(d.filters, `(uid=\2a)`) || !slices.Contains(d.filters, `(uid=x\29\28uid=\2a)`) {
		t.Fatalf("filters = %q", d.filters)
	}
}

func TestAmbiguousLoginIsRefused(t *testing.T) {
	p, d := newTestProvider(t, openLDAPConfig())
	seedAlice(d)
	d.results["(uid=alice)"] = append(d.results["(uid=alice)"], entry("uid=alice,ou=other,dc=example,dc=org", nil))
	if c := classOf(t, second(p.Authenticate(context.Background(), "alice", "alice-pw"))); c != identity.ClassInvalidCredentials {
		t.Fatalf("ambiguous: %s", c)
	}
}

func TestServiceAccountFailureIsAProviderError(t *testing.T) {
	cfg := openLDAPConfig()
	cfg.BindPassword = "stale"
	p, d := newTestProvider(t, cfg)
	seedAlice(d)
	if c := classOf(t, second(p.Authenticate(context.Background(), "alice", "alice-pw"))); c != identity.ClassProviderError {
		t.Fatalf("service bind failure: %s", c)
	}
	if err := p.Check(context.Background()); err == nil {
		t.Fatal("Check passed with a wrong service password")
	}
}

func TestDroppedPooledConnectionIsReplaced(t *testing.T) {
	p, d := newTestProvider(t, openLDAPConfig())
	seedAlice(d)
	if err := p.Check(context.Background()); err != nil {
		t.Fatal(err)
	}
	d.dropNext = true
	if _, err := p.Authenticate(context.Background(), "alice", "alice-pw"); err != nil {
		t.Fatalf("after a dropped connection: %v", err)
	}
}

const (
	johnDN = "CN=John Doe,OU=Staff,DC=corp,DC=example"
	// objectGUID 2a7e5d3c-1b4f-4e8a-9c3d-112233445566 in its wire form.
	johnGUID = "\x3c\x5d\x7e\x2a\x4f\x1b\x8a\x4e\x9c\x3d\x11\x22\x33\x44\x55\x66"
)

func seedJohn(d *fakeDirectory, uac string) {
	d.passwords[johnDN] = "john-pw"
	e := entry(johnDN, map[string][]string{
		"sAMAccountName": {"JDoe"}, "objectGUID": {johnGUID}, "displayName": {"John Doe"},
		"userAccountControl": {uac},
	})
	for _, login := range []string{"jdoe", "jdoe@corp.example"} {
		d.results["(|(sAMAccountName="+login+")(userPrincipalName="+login+"))"] = []*ldap.Entry{e}
	}
	d.results["(member:1.2.840.113556.1.4.1941:="+johnDN+")"] = []*ldap.Entry{
		entry("CN=kb-ops,OU=Groups,DC=corp,DC=example", map[string][]string{"cn": {"kb-ops"}}),
		entry("CN=platform,OU=Groups,DC=corp,DC=example", map[string][]string{"cn": {"platform"}}),
	}
}

func TestAuthenticateActiveDirectory(t *testing.T) {
	p, d := newTestProvider(t, adConfig())
	seedJohn(d, "512")
	for _, login := range []string{"jdoe", "JDoe", `CORP\jdoe`, "JDoe@corp.example"} {
		id, err := p.Authenticate(context.Background(), login, "john-pw")
		if err != nil {
			t.Fatalf("%s: %v", login, err)
		}
		if id.Username != "jdoe" || id.Subject != "2a7e5d3c-1b4f-4e8a-9c3d-112233445566" ||
			!slices.Equal(id.Groups, []string{"kb-ops", "platform"}) {
			t.Fatalf("%s: identity = %+v", login, id)
		}
	}
	if got := p.NormalizeUsername(` CORP\JDoe `); got != "jdoe" {
		t.Fatalf("normalized = %q", got)
	}
}

func TestDisabledActiveDirectoryAccount(t *testing.T) {
	p, d := newTestProvider(t, adConfig())
	seedJohn(d, "514") // NORMAL_ACCOUNT | ACCOUNTDISABLE
	d.disabled[johnDN] = true
	const subject = "2a7e5d3c-1b4f-4e8a-9c3d-112233445566"

	_, err := p.Authenticate(context.Background(), "jdoe", "john-pw")
	var f *identity.Failure
	if !errors.As(err, &f) || f.Class != identity.ClassAccountDisabled || f.Subject != subject {
		t.Fatalf("disabled, right password: %v", err)
	}
	// Without the password nobody learns that the account is disabled,
	// but its sessions still end.
	_, err = p.Authenticate(context.Background(), "jdoe", "guess")
	if !errors.As(err, &f) || f.Class != identity.ClassInvalidCredentials || f.Subject != subject {
		t.Fatalf("disabled, wrong password: %v", err)
	}
	_, err = p.Lookup(context.Background(), "jdoe")
	if identity.ClassOf(err) != identity.ClassAccountDisabled {
		t.Fatalf("lookup of a disabled account: %v", err)
	}
}

func TestLookupNeedsNoPassword(t *testing.T) {
	p, d := newTestProvider(t, adConfig())
	seedJohn(d, "512")
	id, err := p.Lookup(context.Background(), `corp\JDOE`)
	if err != nil {
		t.Fatal(err)
	}
	if id.Username != "jdoe" || len(id.Groups) != 2 {
		t.Fatalf("lookup = %+v", id)
	}
	if d.dials != 1 {
		t.Fatalf("dials = %d, want only the service connection", d.dials)
	}
}

func TestGroupsFromMemberOf(t *testing.T) {
	cfg := openLDAPConfig()
	cfg.GroupsFrom = config.LDAPGroupsFromMemberOf
	p, d := newTestProvider(t, cfg)
	seedAlice(d)
	d.results["(uid=alice)"][0].Attributes = append(d.results["(uid=alice)"][0].Attributes,
		&ldap.EntryAttribute{Name: "memberOf", Values: []string{
			"cn=kb-admins,ou=groups,dc=example,dc=org", `cn=ops\2C eu,ou=groups,dc=example,dc=org`,
		}})
	id, err := p.Authenticate(context.Background(), "alice", "alice-pw")
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(id.Groups, []string{"kb-admins", "ops, eu"}) {
		t.Fatalf("groups = %q", id.Groups)
	}

	cfg.GroupNameAttribute = config.LDAPGroupNameDN
	p, d = newTestProvider(t, cfg)
	seedAlice(d)
	d.results["(uid=alice)"][0].Attributes = append(d.results["(uid=alice)"][0].Attributes,
		&ldap.EntryAttribute{Name: "memberOf", Values: []string{"cn=kb-admins,ou=groups,dc=example,dc=org"}})
	id, err = p.Authenticate(context.Background(), "alice", "alice-pw")
	if err != nil || !slices.Equal(id.Groups, []string{"cn=kb-admins,ou=groups,dc=example,dc=org"}) {
		t.Fatalf("groups by DN = %v, %v", id, err)
	}
}

func TestMissingIDAttributeIsAProviderError(t *testing.T) {
	cfg := openLDAPConfig()
	cfg.IDAttribute = "employeeNumber"
	p, d := newTestProvider(t, cfg)
	seedAlice(d)
	if c := classOf(t, second(p.Authenticate(context.Background(), "alice", "alice-pw"))); c != identity.ClassProviderError {
		t.Fatalf("missing ID attribute: %s", c)
	}
}

func TestWarnings(t *testing.T) {
	cases := []struct {
		url      string
		startTLS bool
		insecure bool
		want     []string
	}{
		{"ldaps://ldap.example.org", false, false, nil},
		{"ldap://ldap.example.org", true, false, nil},
		{"ldap://ldap.example.org", false, false, []string{"plaintext"}},
		{"ldaps://ldap.example.org", false, true, []string{"tlsVerifyDisabled"}},
	}
	for _, tc := range cases {
		cfg := openLDAPConfig()
		cfg.URL, cfg.StartTLS, cfg.InsecureSkipVerify = tc.url, tc.startTLS, tc.insecure
		p, _ := newTestProvider(t, cfg)
		if got := p.Warnings(); !slices.Equal(got, tc.want) {
			t.Errorf("%s startTLS=%v insecure=%v: %v", tc.url, tc.startTLS, tc.insecure, got)
		}
	}
}

func TestPoolLimitsConcurrentSignIns(t *testing.T) {
	pl := newPool(func() (conn, error) { return &fakeConn{d: newFakeDirectory()}, nil }, 50*time.Millisecond, time.Now)
	for range maxConcurrent {
		if err := pl.acquire(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	if err := pl.acquire(context.Background()); !errors.Is(err, errBusy) {
		t.Fatalf("acquire beyond the limit: %v", err)
	}
	pl.release()
	if err := pl.acquire(context.Background()); err != nil {
		t.Fatalf("acquire after a release: %v", err)
	}
}

func TestPoolRetiresIdleConnections(t *testing.T) {
	now := time.Unix(0, 0)
	opened := 0
	pl := newPool(func() (conn, error) { opened++; return &fakeConn{d: newFakeDirectory()}, nil }, time.Second,
		func() time.Time { return now })
	use := func() {
		if err := pl.with(func(conn) error { return nil }); err != nil {
			t.Fatal(err)
		}
	}
	use()
	use()
	now = now.Add(maxIdleTime + time.Second)
	use()
	if opened != 2 {
		t.Fatalf("opened = %d, want 2 (reused once, then retired)", opened)
	}
}
