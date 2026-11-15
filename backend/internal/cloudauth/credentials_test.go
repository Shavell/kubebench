package cloudauth

import (
	"strings"
	"testing"
)

const testServiceAccount = `{"type":"service_account","project_id":"p","client_email":"kb@p.iam.gserviceaccount.com",
"private_key":"-----BEGIN PRIVATE KEY-----\nMIIB\n-----END PRIVATE KEY-----\n","token_uri":"https://oauth2.googleapis.com/token"}`

const (
	tenant = "11111111-2222-3333-4444-555555555555"
	client = "66666666-7777-8888-9999-aaaaaaaaaaaa"
)

func TestCredentialsValidate(t *testing.T) {
	tests := []struct {
		name  string
		creds Credentials
		err   string // "" = valid
	}{
		{"aws keys", Credentials{Provider: AWS, AccessKeyID: "AKIAEXAMPLE", SecretAccessKey: "s"}, ""},
		{"aws session", Credentials{Provider: AWS, AccessKeyID: " ASIA ", SecretAccessKey: "s", SessionToken: "t"}, ""},
		{"aws profile", Credentials{Provider: AWS, Profile: "prod-admin"}, ""},
		{"aws nothing", Credentials{Provider: AWS}, "access key or a profile"},
		{"aws half a key", Credentials{Provider: AWS, AccessKeyID: "AKIA"}, "both the key id and the secret"},
		{"aws both", Credentials{Provider: AWS, AccessKeyID: "A", SecretAccessKey: "s", Profile: "p"}, "not both"},
		{"aws bad profile", Credentials{Provider: AWS, Profile: "../etc"}, "invalid AWS profile"},
		{"aws with gcp field", Credentials{Provider: AWS, Profile: "p", ServiceAccountJSON: "{}"}, "access key or a profile only"},
		{"gcp key", Credentials{Provider: GCP, ServiceAccountJSON: testServiceAccount}, ""},
		{"gcp not json", Credentials{Provider: GCP, ServiceAccountJSON: "nope"}, "not valid JSON"},
		{"gcp external account", Credentials{Provider: GCP, ServiceAccountJSON: `{"type":"external_account","credential_source":{"file":"/etc/passwd"}}`}, "only service_account"},
		{"gcp incomplete", Credentials{Provider: GCP, ServiceAccountJSON: `{"type":"service_account"}`}, "lacks client_email"},
		{"azure spn", Credentials{Provider: Azure, TenantID: tenant, ClientID: client, ClientSecret: "x"}, ""},
		{"azure no secret", Credentials{Provider: Azure, TenantID: tenant, ClientID: client}, "client secret is required"},
		{"azure bad guid", Credentials{Provider: Azure, TenantID: "contoso", ClientID: client, ClientSecret: "x"}, "GUIDs"},
		{"azure with aws field", Credentials{Provider: Azure, TenantID: tenant, ClientID: client, ClientSecret: "x", Profile: "p"}, "only"},
		{"unknown provider", Credentials{Provider: "ibm"}, "unknown credential provider"},
		{"too long", Credentials{Provider: AWS, Profile: strings.Repeat("a", maxField+1)}, "16 KiB"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := tt.creds
			err := c.Validate()
			if tt.err == "" && err != nil || tt.err != "" && (err == nil || !strings.Contains(err.Error(), tt.err)) {
				t.Fatalf("Validate() = %v, want %q", err, tt.err)
			}
		})
	}
}

func TestCredentialsRoundTrip(t *testing.T) {
	c := Credentials{Provider: AWS, AccessKeyID: "AKIA", SecretAccessKey: "secret"}
	stored := c.Encode()
	back, err := ParseCredentials(stored)
	if err != nil || *back != c {
		t.Fatalf("round trip = %+v, %v", back, err)
	}
	if ProviderOf(stored) != AWS || ProviderOf("") != "" || ProviderOf("{broken") != "" {
		t.Fatal("ProviderOf")
	}
	if none, err := ParseCredentials(""); none != nil || err != nil {
		t.Fatalf("empty = %+v, %v", none, err)
	}
	if _, err := ParseCredentials("{broken"); err == nil {
		t.Fatal("broken credentials parsed")
	}
}

func TestCredentialsOwnIdentity(t *testing.T) {
	for _, tc := range []struct {
		name  string
		creds *Credentials
		want  bool
	}{
		{"none: the server's ambient identity", nil, false},
		{"an AWS profile of the server's files", &Credentials{Provider: AWS, Profile: "prod"}, false},
		{"an AWS access key", &Credentials{Provider: AWS, AccessKeyID: "AKIA", SecretAccessKey: "s"}, true},
		{"a GCP service-account key", &Credentials{Provider: GCP, ServiceAccountJSON: "{}"}, true},
		{"an Azure service principal", &Credentials{Provider: Azure, TenantID: "t", ClientID: "c", ClientSecret: "s"}, true},
	} {
		if got := tc.creds.OwnIdentity(); got != tc.want {
			t.Errorf("%s: OwnIdentity = %v, want %v", tc.name, got, tc.want)
		}
	}
}
