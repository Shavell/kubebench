// Package ldapauth signs users in against an LDAP directory or Active
// Directory (docs/design-v0.5.md §3.6): a service account searches the
// user by a filter, a bind as the user checks the password (hashes are
// never compared), and the user's groups come from memberOf or a group
// search (nested in Active Directory).
package ldapauth

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/go-ldap/ldap/v3"

	"kubebench/internal/config"
	"kubebench/internal/identity"
	"kubebench/internal/store"
)

// maxLoginLength bounds what a user may type as their login name.
const maxLoginLength = 256

// Provider is the configured directory.
type Provider struct {
	cfg  config.LDAPConfig
	tls  *tls.Config
	dial func() (conn, error)
	pool *pool
}

// New returns the provider for cfg. Its TLS connections verify the
// server's certificate against the system roots plus cfg.CAFile, unless
// cfg.InsecureSkipVerify.
func New(cfg config.LDAPConfig) (*Provider, error) {
	u, err := url.Parse(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("parse the LDAP URL: %w", err)
	}
	tlsConfig := &tls.Config{
		ServerName:         u.Hostname(),
		MinVersion:         tls.VersionTLS12,
		InsecureSkipVerify: cfg.InsecureSkipVerify, // opt-in, warned about at startup and in Settings
	}
	if cfg.CAFile != "" {
		pool, err := x509.SystemCertPool()
		if err != nil {
			pool = x509.NewCertPool()
		}
		pem, err := os.ReadFile(cfg.CAFile)
		if err != nil {
			return nil, fmt.Errorf("read the LDAP CA bundle: %w", err)
		}
		if !pool.AppendCertsFromPEM(pem) {
			return nil, fmt.Errorf("the LDAP CA bundle %s holds no PEM certificate", cfg.CAFile)
		}
		tlsConfig.RootCAs = pool
	}
	p := &Provider{cfg: cfg, tls: tlsConfig}
	p.dial = p.dialDirectory
	p.pool = newPool(p.openService, cfg.Timeout, time.Now)
	return p, nil
}

func (p *Provider) Source() string { return store.UserSourceLDAP }
func (p *Provider) Name() string   { return p.cfg.Name }

// Config is the provider's configuration (it holds the bind password).
func (p *Provider) Config() config.LDAPConfig { return p.cfg }

// Warnings name the insecure parts of the configuration: "tlsVerifyDisabled"
// and "plaintext" (passwords cross the network unencrypted).
func (p *Provider) Warnings() []string {
	var out []string
	if !p.cfg.TLS() {
		out = append(out, "plaintext")
	} else if p.cfg.InsecureSkipVerify {
		out = append(out, "tlsVerifyDisabled")
	}
	return out
}

// dialDirectory connects to the directory, upgrading with StartTLS when
// configured.
func (p *Provider) dialDirectory() (conn, error) {
	c, err := ldap.DialURL(p.cfg.URL,
		ldap.DialWithDialer(&net.Dialer{Timeout: p.cfg.Timeout}),
		ldap.DialWithTLSConfig(p.tls))
	if err != nil {
		return nil, err
	}
	c.SetTimeout(p.cfg.Timeout)
	if p.cfg.StartTLS {
		if err := c.StartTLS(p.tls); err != nil {
			_ = c.Close()
			return nil, fmt.Errorf("StartTLS: %w", err)
		}
	}
	return c, nil
}

// openService connects as the service account (anonymously without a
// bind DN).
func (p *Provider) openService() (conn, error) {
	c, err := p.dial()
	if err != nil {
		return nil, fmt.Errorf("connect to %s: %w", p.cfg.URL, err)
	}
	if p.cfg.BindDN != "" {
		if err := c.Bind(p.cfg.BindDN, p.cfg.BindPassword); err != nil {
			_ = c.Close()
			return nil, fmt.Errorf("bind as the service account %s: %w", p.cfg.BindDN, err)
		}
	}
	return c, nil
}

// Check reports whether the service account can read the user base DN.
func (p *Provider) Check(ctx context.Context) error {
	if err := p.pool.acquire(ctx); err != nil {
		return err
	}
	defer p.pool.release()
	return p.pool.with(func(c conn) error {
		_, err := c.Search(ldap.NewSearchRequest(p.cfg.UserBaseDN, ldap.ScopeBaseObject, ldap.NeverDerefAliases,
			1, p.timeLimit(), false, "(objectClass=*)", []string{"1.1"}, nil))
		if err != nil {
			return fmt.Errorf("read %s: %w", p.cfg.UserBaseDN, err)
		}
		return nil
	})
}

// NormalizeUsername is the name a login is searched by: trimmed, lower
// case, without a DOMAIN\ prefix. A userPrincipalName (user@domain) is
// kept; the default Active Directory filter matches it.
func (p *Provider) NormalizeUsername(login string) string {
	name := strings.TrimSpace(login)
	if i := strings.LastIndexByte(name, '\\'); i >= 0 {
		name = name[i+1:]
	}
	return strings.ToLower(name)
}

// Authenticate checks a login name and password against the directory.
func (p *Provider) Authenticate(ctx context.Context, login, password string) (*identity.Identity, error) {
	name := p.NormalizeUsername(login)
	if name == "" || len(name) > maxLoginLength {
		return nil, identity.Fail(identity.ClassInvalidCredentials, "empty or overlong login name")
	}
	// A simple bind with a DN and no password is an unauthenticated bind
	// that many directories accept: it proves nothing.
	if password == "" {
		return nil, identity.Fail(identity.ClassInvalidCredentials, "empty password for %q refused", name)
	}
	if err := p.pool.acquire(ctx); err != nil {
		return nil, identity.Fail(identity.ClassProviderError, "%v", err)
	}
	defer p.pool.release()

	u, err := p.findUser(name)
	if err != nil {
		return nil, err
	}
	if u.disabled {
		// Sessions of a disabled account end either way (Subject); whether
		// it is disabled is only told to someone who knows the password.
		class := identity.ClassInvalidCredentials
		if err := p.verifyPassword(u.dn, password); err == nil || identity.ClassOf(err) == identity.ClassAccountDisabled {
			class = identity.ClassAccountDisabled
		}
		return nil, &identity.Failure{Class: class, Subject: u.id.Subject,
			Err: fmt.Errorf("directory account %s is disabled", u.dn)}
	}
	if err := p.verifyPassword(u.dn, password); err != nil {
		if f, ok := errors.AsType[*identity.Failure](err); ok && f.Class == identity.ClassAccountDisabled {
			f.Subject = u.id.Subject
		}
		return nil, err
	}
	if u.id.Groups, err = p.groupsOf(u); err != nil {
		return nil, err
	}
	return u.id, nil
}

// Lookup finds a user and their groups without their password, for the
// admins' sign-in preview.
func (p *Provider) Lookup(ctx context.Context, login string) (*identity.Identity, error) {
	name := p.NormalizeUsername(login)
	if name == "" || len(name) > maxLoginLength {
		return nil, identity.Fail(identity.ClassInvalidCredentials, "empty or overlong login name")
	}
	if err := p.pool.acquire(ctx); err != nil {
		return nil, identity.Fail(identity.ClassProviderError, "%v", err)
	}
	defer p.pool.release()
	u, err := p.findUser(name)
	if err != nil {
		return nil, err
	}
	if u.disabled {
		return nil, &identity.Failure{Class: identity.ClassAccountDisabled, Subject: u.id.Subject,
			Err: fmt.Errorf("directory account %s is disabled", u.dn)}
	}
	if u.id.Groups, err = p.groupsOf(u); err != nil {
		return nil, err
	}
	return u.id, nil
}

// verifyPassword binds as the user on a connection of its own (never a
// pooled one, which must stay bound as the service account).
func (p *Provider) verifyPassword(dn, password string) error {
	c, err := p.dial()
	if err != nil {
		return identity.Fail(identity.ClassProviderError, "connect to %s: %v", p.cfg.URL, err)
	}
	defer func() { _ = c.Close() }()
	err = c.Bind(dn, password)
	switch {
	case err == nil:
		return nil
	case ldap.IsErrorWithCode(err, ldap.LDAPResultInvalidCredentials):
		if adAccountDisabled(err) {
			return identity.Fail(identity.ClassAccountDisabled, "bind as %s: %v", dn, err)
		}
		return identity.Fail(identity.ClassInvalidCredentials, "bind as %s: %v", dn, err)
	default:
		return identity.Fail(identity.ClassProviderError, "bind as %s: %v", dn, err)
	}
}

// timeLimit is the server-side time limit of searches, in seconds.
func (p *Provider) timeLimit() int {
	return max(1, int(p.cfg.Timeout/time.Second))
}
