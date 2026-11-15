package helm

import (
	"net/http"
	"strings"
	"testing"

	"helm.sh/helm/v3/pkg/chart"
)

func TestRedactManifest(t *testing.T) {
	manifest := `---
# Source: app/templates/configmap.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: app   # kept verbatim, comment included
data:
  note: "mentions a Secret"
---
# Source: app/templates/secret.yaml
apiVersion: v1
kind: Secret
metadata:
  name: app
  annotations:
    kubectl.kubernetes.io/last-applied-configuration: '{"stringData":{"password":"hunter2"}}'
type: Opaque
data:
  token: aHVudGVyMg==
stringData:
  password: hunter2
---
apiVersion: v1
kind: List
items:
- apiVersion: v1
  kind: Secret
  metadata:
    name: listed
  stringData:
    key: hunter2
---
apiVersion: example.com/v1
kind: Secret
metadata:
  name: not-core
spec:
  value: visible
`
	got := redactManifest(manifest)
	if strings.Contains(got, "hunter2") || strings.Contains(got, "aHVudGVyMg==") {
		t.Fatalf("a Secret value survived:\n%s", got)
	}
	for _, want := range []string{
		"# Source: app/templates/configmap.yaml\napiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: app   # kept verbatim",
		"# Source: app/templates/secret.yaml\n",
		"token: <12 bytes, redacted>",
		"password: <7 bytes, redacted>",
		"key: <7 bytes, redacted>",
		"kubectl.kubernetes.io/last-applied-configuration: <redacted>",
		"value: visible",
	} {
		contains(t, "redacted manifest", got, want)
	}
	if n := strings.Count(got, "\n---\n") + 1; n != 4 {
		t.Fatalf("%d documents after redaction, want 4:\n%s", n, got)
	}

	if plain := "apiVersion: v1\nkind: ConfigMap\n"; redactManifest(plain) != plain {
		t.Fatal("a manifest without Secrets changed")
	}
	broken := "# Source: x/secret.yaml\nkind: Secret\ndata: [unterminated\n"
	if got := redactManifest(broken); strings.Contains(got, "unterminated") || !strings.HasPrefix(got, "# Source: x/secret.yaml\n") {
		t.Fatalf("an unparsable document that may be a Secret was shown: %q", got)
	}
}

// secretChart renders a Secret from .Values.password (a release hook
// does too), as charts commonly do for database passwords.
func secretChart() *chart.Chart {
	ch := testChart("vault", "0.1.0")
	ch.Values["password"] = "default-pw"
	ch.Templates = append(ch.Templates,
		&chart.File{Name: "templates/secret.yaml", Data: []byte(
			"apiVersion: v1\nkind: Secret\nmetadata:\n  name: {{ .Release.Name }}\nstringData:\n  password: {{ .Values.password | quote }}\n")},
		&chart.File{Name: "templates/hook-secret.yaml", Data: []byte(
			"apiVersion: v1\nkind: Secret\nmetadata:\n  name: {{ .Release.Name }}-hook\n  annotations:\n    helm.sh/hook: pre-upgrade\ndata:\n  password: {{ .Values.password | b64enc }}\n")},
	)
	return ch
}

// Manifests reach read-level users: every Secret in them is redacted, in
// the release manifest (any revision) and in install and upgrade answers.
func TestManifestsRedactSecrets(t *testing.T) {
	h := newHarness(t)
	id := h.upload(secretChart())
	const pw = "s3cr3t-release-value"
	install := InstallRequest{Name: "db", Namespace: "shop", Chart: ChartRef{UploadID: id}, Values: "password: " + pw + "\n"}
	noSecret := func(what, text string) {
		t.Helper()
		if strings.Contains(text, pw) || strings.Contains(text, "czNjcjN0") { // base64 prefix of pw
			t.Fatalf("%s shows the password:\n%s", what, text)
		}
		contains(t, what, text, "password: <")
	}

	noSecret("dry-run install", ok[InstallResult](t, h.do(http.MethodPost, "/install?dryRun=true", install)).Manifest)
	noSecret("install", ok[InstallResult](t, h.do(http.MethodPost, "/install", install)).Manifest)
	manifest := ok[map[string]any](t, h.do(http.MethodGet, "/releases/shop/db/manifest?revision=1", nil))["manifest"].(string)
	noSecret("release manifest", manifest)
	contains(t, "release manifest", manifest, `greeting: "hello"`) // other documents are untouched

	up := ok[UpgradeResult](t, h.do(http.MethodPost, "/releases/shop/db/upgrade?dryRun=true", UpgradeRequest{Values: "password: other\n"}))
	noSecret("current manifest", up.CurrentManifest)
	if strings.Contains(up.Manifest, "other") {
		t.Fatalf("the upgraded manifest shows the new password:\n%s", up.Manifest)
	}
	if h.audit.detail != `{"chart":"(current)","dryRun":true,"reuseValues":false}` {
		t.Fatalf("dry-run upgrade audit = %+v", h.audit)
	}
}
