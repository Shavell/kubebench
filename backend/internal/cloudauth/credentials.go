package cloudauth

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"kubebench/internal/reason"
)

// Credentials are per-cluster cloud credentials an admin supplies, stored
// with the cluster record like its kubeconfig. They are write-only: no
// API response and no audit row carries them, only Provider.
type Credentials struct {
	Provider string `json:"provider"` // AWS | GCP | Azure

	// AWS: an access key (with an optional session token), or a profile of
	// the server's shared config files.
	AccessKeyID     string `json:"accessKeyId,omitempty"`
	SecretAccessKey string `json:"secretAccessKey,omitempty"`
	SessionToken    string `json:"sessionToken,omitempty"`
	Profile         string `json:"profile,omitempty"`

	// GCP: a service-account key file (JSON).
	ServiceAccountJSON string `json:"serviceAccountJson,omitempty"`

	// Azure: a service principal with a client secret.
	TenantID     string `json:"tenantId,omitempty"`
	ClientID     string `json:"clientId,omitempty"`
	ClientSecret string `json:"clientSecret,omitempty"`
}

// maxField bounds every credential field; a service-account key is a few
// KiB, everything else far less.
const maxField = 16 << 10

var (
	guidPattern    = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
	profilePattern = regexp.MustCompile(`^[A-Za-z0-9_.+=,@-]{1,128}$`)
)

// Validate trims the fields and checks that they are complete for the
// provider and carry nothing for another one.
func (c *Credentials) Validate() error {
	for _, f := range []*string{&c.Provider, &c.AccessKeyID, &c.SecretAccessKey, &c.SessionToken,
		&c.Profile, &c.ServiceAccountJSON, &c.TenantID, &c.ClientID, &c.ClientSecret} {
		*f = strings.TrimSpace(*f)
		if len(*f) > maxField {
			return reason.New(reasonFieldTooLong, nil)
		}
	}
	aws := c.AccessKeyID + c.SecretAccessKey + c.SessionToken + c.Profile
	gcp := c.ServiceAccountJSON
	azure := c.TenantID + c.ClientID + c.ClientSecret
	switch c.Provider {
	case AWS:
		if gcp+azure != "" {
			return reason.New(reasonAWSOnly, nil)
		}
		return c.validateAWS()
	case GCP:
		if aws+azure != "" {
			return reason.New(reasonGCPOnly, nil)
		}
		return validateServiceAccount(c.ServiceAccountJSON)
	case Azure:
		if aws+gcp != "" {
			return reason.New(reasonAzureOnly, nil)
		}
		if !guidPattern.MatchString(c.TenantID) || !guidPattern.MatchString(c.ClientID) {
			return reason.New(reasonAzureGUIDs, nil)
		}
		if c.ClientSecret == "" {
			return reason.New(reasonAzureSecret, nil)
		}
		return nil
	default:
		return reason.New(reasonUnknownProvider, reason.Params{"provider": c.Provider})
	}
}

func (c *Credentials) validateAWS() error {
	keys := c.AccessKeyID != "" || c.SecretAccessKey != "" || c.SessionToken != ""
	switch {
	case keys && c.Profile != "":
		return reason.New(reasonAWSKeyAndProfile, nil)
	case keys && (c.AccessKeyID == "" || c.SecretAccessKey == ""):
		return reason.New(reasonAWSKeyIncomplete, nil)
	case !keys && c.Profile == "":
		return reason.New(reasonAWSMissing, nil)
	case c.Profile != "" && !profilePattern.MatchString(c.Profile):
		return reason.New(reasonAWSProfile, nil)
	}
	return nil
}

// validateServiceAccount accepts service-account keys only: other Google
// credential files (external accounts, impersonation) may make the server
// read local files, call arbitrary URLs or run commands.
func validateServiceAccount(raw string) error {
	var key struct {
		Type        string `json:"type"`
		ClientEmail string `json:"client_email"`
		PrivateKey  string `json:"private_key"`
	}
	if err := json.Unmarshal([]byte(raw), &key); err != nil {
		return reason.New(reasonGCPJSON, nil)
	}
	if key.Type != "service_account" {
		return reason.New(reasonGCPType, reason.Params{"type": key.Type})
	}
	if key.ClientEmail == "" || !strings.Contains(key.PrivateKey, "PRIVATE KEY") {
		return reason.New(reasonGCPFields, nil)
	}
	return nil
}

// ParseCredentials decodes stored credentials; "" means none (nil).
func ParseCredentials(stored string) (*Credentials, error) {
	if stored == "" {
		return nil, nil
	}
	var c Credentials
	if err := json.Unmarshal([]byte(stored), &c); err != nil {
		return nil, fmt.Errorf("stored cloud credentials are unreadable: %w", err)
	}
	return &c, nil
}

// OwnIdentity reports whether c names an identity of its own rather than
// one of the server's: an AWS profile is read from the server's
// configuration files, and no credentials at all mean the server's
// ambient identity.
func (c *Credentials) OwnIdentity() bool {
	return c != nil && !(c.Provider == AWS && c.Profile != "")
}

// Encode serializes validated credentials for storage.
func (c *Credentials) Encode() string {
	b, _ := json.Marshal(c) // plain strings cannot fail to marshal
	return string(b)
}

// ProviderOf is the provider of stored credentials, "" for none or
// unreadable ones; it is the only part ever shown.
func ProviderOf(stored string) string {
	c, err := ParseCredentials(stored)
	if err != nil || c == nil {
		return ""
	}
	return c.Provider
}
