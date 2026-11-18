package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLDAPDefaultsPerFlavour(t *testing.T) {
	c, err := FromEnv(env(map[string]string{
		"KUBEBENCH_LDAP_URL":           "ldaps://ldap.example.org",
		"KUBEBENCH_LDAP_BIND_DN":       "cn=kubebench,ou=services,dc=example,dc=org",
		"KUBEBENCH_LDAP_BIND_PASSWORD": "bind-pw",
		"KUBEBENCH_LDAP_USER_BASE_DN":  "ou=people,dc=example,dc=org",
	}), false)
	if err != nil {
		t.Fatal(err)
	}
	l := c.Auth.LDAP
	if !c.Auth.External() || l == nil || l.Type != LDAPTypeOpenLDAP || l.Name != "Directory" || l.UserFilter != "(uid=%s)" ||
		l.IDAttribute != "entryUUID" || l.GroupsFrom != LDAPGroupsFromSearch || l.GroupBaseDN != l.UserBaseDN ||
		l.GroupNameAttribute != "cn" || l.Timeout != 10*time.Second || !l.TLS() || l.BindPassword != "bind-pw" {
		t.Fatalf("openldap defaults: %+v", l)
	}

	c, err = FromEnv(env(map[string]string{
		"KUBEBENCH_LDAP_URL":           "ldap://dc1.corp.example:389",
		"KUBEBENCH_LDAP_TYPE":          "AD",
		"KUBEBENCH_LDAP_START_TLS":     "true",
		"KUBEBENCH_LDAP_USER_BASE_DN":  "DC=corp,DC=example",
		"KUBEBENCH_LDAP_GROUP_BASE_DN": "OU=Groups,DC=corp,DC=example",
	}), false)
	if err != nil {
		t.Fatal(err)
	}
	l = c.Auth.LDAP
	if l.Type != LDAPTypeActiveDirectory || l.Name != "Active Directory" ||
		l.UserFilter != "(|(sAMAccountName=%s)(userPrincipalName=%s))" || l.UsernameAttribute != "sAMAccountName" ||
		l.IDAttribute != "objectGUID" || l.GroupFilter != "(member:1.2.840.113556.1.4.1941:=%d)" ||
		l.GroupBaseDN != "OU=Groups,DC=corp,DC=example" || !l.TLS() || l.BindDN != "" {
		t.Fatalf("ad defaults: %+v", l)
	}
}

func TestLDAPBindPasswordFile(t *testing.T) {
	file := filepath.Join(t.TempDir(), "bind")
	if err := os.WriteFile(file, []byte("from-file\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	c, err := FromEnv(env(map[string]string{
		"KUBEBENCH_LDAP_URL":                "ldaps://ldap.example.org",
		"KUBEBENCH_LDAP_BIND_DN":            "cn=svc,dc=example,dc=org",
		"KUBEBENCH_LDAP_BIND_PASSWORD_FILE": file,
		"KUBEBENCH_LDAP_USER_BASE_DN":       "dc=example,dc=org",
	}), false)
	if err != nil {
		t.Fatal(err)
	}
	if c.Auth.LDAP.BindPassword != "from-file" {
		t.Fatalf("bind password = %q", c.Auth.LDAP.BindPassword)
	}
}

func TestLDAPConfigErrors(t *testing.T) {
	_, err := FromEnv(env(map[string]string{
		"KUBEBENCH_LDAP_URL":          "ldaps://ldap.example.org/dc=x",
		"KUBEBENCH_LDAP_TYPE":         "novell",
		"KUBEBENCH_LDAP_START_TLS":    "true",
		"KUBEBENCH_LDAP_BIND_DN":      "not a dn",
		"KUBEBENCH_LDAP_USER_FILTER":  "(uid=alice)",
		"KUBEBENCH_LDAP_GROUP_FILTER": "(member=%d",
		"KUBEBENCH_LDAP_GROUPS_FROM":  "nested",
		"KUBEBENCH_LDAP_CA_FILE":      "/nonexistent/ca.pem",
		"KUBEBENCH_LDAP_TIMEOUT":      "10ms",
	}), false)
	if err == nil {
		t.Fatal("invalid LDAP configuration accepted")
	}
	for _, key := range []string{
		"KUBEBENCH_LDAP_URL:", "KUBEBENCH_LDAP_TYPE:", "KUBEBENCH_LDAP_BIND_DN:", "KUBEBENCH_LDAP_BIND_PASSWORD:",
		"KUBEBENCH_LDAP_USER_BASE_DN:", "KUBEBENCH_LDAP_USER_FILTER:", "KUBEBENCH_LDAP_GROUPS_FROM:",
		"KUBEBENCH_LDAP_CA_FILE:", "KUBEBENCH_LDAP_TIMEOUT:",
	} {
		if !strings.Contains(err.Error(), key) {
			t.Errorf("missing %s in %v", key, err)
		}
	}

	_, err = FromEnv(env(map[string]string{
		"KUBEBENCH_LDAP_URL":          "ldaps://ldap.example.org",
		"KUBEBENCH_LDAP_START_TLS":    "true",
		"KUBEBENCH_LDAP_USER_BASE_DN": "dc=example,dc=org",
		"KUBEBENCH_LDAP_GROUP_FILTER": "(member=%d",
	}), false)
	for _, key := range []string{"KUBEBENCH_LDAP_START_TLS:", "KUBEBENCH_LDAP_GROUP_FILTER:"} {
		if err == nil || !strings.Contains(err.Error(), key) {
			t.Errorf("missing %s in %v", key, err)
		}
	}
}
