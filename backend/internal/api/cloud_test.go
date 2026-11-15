package api

import (
	"net/http"
	"strconv"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/version"
	fakediscovery "k8s.io/client-go/discovery/fake"

	"kubebench/internal/cloud"
	"kubebench/internal/cloudauth"
	"kubebench/internal/plugin"
	"kubebench/internal/store"
)

// eksKubeconfig is what `aws eks update-kubeconfig` writes.
const eksKubeconfig = `apiVersion: v1
kind: Config
clusters:
- name: arn:aws:eks:eu-west-1:123456789012:cluster/prod
  cluster:
    server: https://ABCDEF0123.gr7.eu-west-1.eks.amazonaws.com
    certificate-authority-data: ""
contexts:
- name: arn:aws:eks:eu-west-1:123456789012:cluster/prod
  context:
    cluster: arn:aws:eks:eu-west-1:123456789012:cluster/prod
    user: arn:aws:eks:eu-west-1:123456789012:cluster/prod
current-context: arn:aws:eks:eu-west-1:123456789012:cluster/prod
users:
- name: arn:aws:eks:eu-west-1:123456789012:cluster/prod
  user:
    exec:
      apiVersion: client.authentication.k8s.io/v1beta1
      command: aws
      args: [--region, eu-west-1, eks, get-token, --cluster-name, prod, --output, json]
`

const awsSecret = "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"

func TestClusterPlatform(t *testing.T) {
	e := newTestEnv(t, nil)
	e.cs.Discovery().(*fakediscovery.FakeDiscovery).FakedServerVersion = &version.Info{GitVersion: "v1.35.5+k3s1"}
	if _, err := e.cs.CoreV1().Nodes().Create(t.Context(), &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: "k3d-e2e-server-0"},
		Spec:       corev1.NodeSpec{ProviderID: "k3s://k3d-e2e-server-0"},
	}, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	if _, err := e.store.AddCluster(store.Cluster{Name: "bookmark", Source: store.SourceAddress,
		Address: "https://abc.hcp.westeurope.azmk8s.io:443"}); err != nil {
		t.Fatal(err)
	}

	// The test runtime starts with nothing detected; detect as a fresh
	// runtime does in the background.
	expect(t, e.do(http.MethodGet, "/api/info", "bob", nil), http.StatusOK, "")
	cluster, _ := e.store.GetCluster(e.cluster)
	e.srv.manager.Peek(cluster).DetectCloudForTest()

	info := decode[infoView](t, e.do(http.MethodGet, "/api/info", "bob", nil))
	p := info.Cluster.Platform
	if p == nil || p.Provider != cloud.ProviderK3s || p.Version != "v1.35.5+k3s1" || !p.Detected ||
		p.Support.Basis != "kubernetes" || p.Support.EndDate != "2027-02-28" || p.Support.Status == cloud.StatusUnknown {
		t.Fatalf("info platform = %+v", p)
	}

	views := decode[[]clusterView](t, e.do(http.MethodGet, "/api/clusters", "admin", nil))
	byName := map[string]clusterView{}
	for _, v := range views {
		byName[v.Name] = v
	}
	if p := byName["test"].Platform; p == nil || p.Provider != cloud.ProviderK3s || !p.Detected {
		t.Fatalf("test platform = %+v", p)
	}
	// Never connected: the URL alone is known.
	if p := byName["bookmark"].Platform; p == nil || p.Provider != cloud.ProviderAKS || p.Cloud != cloud.CloudAzure ||
		p.Detected || p.Version != "" || p.Support.Status != cloud.StatusUnknown {
		t.Fatalf("bookmark platform = %+v", p)
	}
}

// ambientOptIn lends the server's own cloud identity to pasted kubeconfigs.
var ambientOptIn = map[string]string{"KUBEBENCH_ALLOW_AMBIENT_CLOUD_IDENTITY": "true"}

func TestAddClusterWithCloudExec(t *testing.T) {
	e := newTestEnv(t, ambientOptIn) // KUBEBENCH_ALLOW_KUBECONFIG_EXEC unset
	// The EKS plugin is handled in process: no exec opt-in needed. Any
	// other command still is refused.
	expect(t, e.do(http.MethodPost, "/api/clusters", "admin", map[string]string{"name": "eks", "content": eksKubeconfig}),
		http.StatusOK, "")
	expect(t, e.do(http.MethodPost, "/api/clusters", "admin", map[string]string{"name": "sh", "content": kubeconfigWithExec()}),
		http.StatusForbidden, plugin.CodeForbidden)
	disguised := strings.Replace(eksKubeconfig, "command: aws", "command: /tmp/aws-helper", 1)
	expect(t, e.do(http.MethodPost, "/api/clusters", "admin", map[string]string{"name": "other", "content": disguised}),
		http.StatusForbidden, plugin.CodeForbidden)

	views := decode[[]clusterView](t, e.do(http.MethodGet, "/api/clusters", "admin", nil))
	if v := views[len(views)-1]; v.Name != "eks" || v.CloudAuth != "aws" || v.CloudCredentials != "" ||
		v.Platform.Provider != cloud.ProviderEKS {
		t.Fatalf("eks view = %+v", v)
	}
}

// Without the opt-in, a pasted EKS, GKE or AKS kubeconfig must bring its
// own identity: the server's token would otherwise go to whatever API
// server the kubeconfig names.
func TestAmbientCloudIdentityNeedsOptIn(t *testing.T) {
	e := newTestEnv(t, nil)
	attacker := strings.Replace(eksKubeconfig, "https://ABCDEF0123.gr7.eu-west-1.eks.amazonaws.com", "https://attacker.example", 1)
	keys := map[string]string{"provider": "aws", "accessKeyId": "AKIAEXAMPLE", "secretAccessKey": awsSecret}
	profile := map[string]string{"provider": "aws", "profile": "prod"}
	for _, creds := range []any{nil, profile} {
		rec := e.do(http.MethodPost, "/api/clusters", "admin", map[string]any{"name": "eks", "content": attacker, "credentials": creds})
		expect(t, rec, http.StatusForbidden, plugin.CodeForbidden)
		if !strings.Contains(rec.Body.String(), "KUBEBENCH_ALLOW_AMBIENT_CLOUD_IDENTITY") {
			t.Fatalf("the refusal does not name the setting: %s", rec.Body.String())
		}
	}
	rec := e.do(http.MethodPost, "/api/clusters", "admin", map[string]any{"name": "eks", "content": attacker, "credentials": keys})
	expect(t, rec, http.StatusOK, "")
	id := int64(decode[map[string]any](t, rec)["id"].(float64))

	// Edits may not fall back to the server's identity either.
	path := "/api/clusters/" + strconv.FormatInt(id, 10)
	for _, creds := range []any{nil, profile} {
		expect(t, e.do(http.MethodPatch, path, "admin", map[string]any{"credentials": creds}), http.StatusForbidden, plugin.CodeForbidden)
	}
	if c, _ := e.store.GetCluster(id); cloudauth.ProviderOf(c.Credentials) != "aws" || !strings.Contains(c.Credentials, "AKIAEXAMPLE") {
		t.Fatalf("a refused edit changed the credentials: %s", c.Credentials)
	}
	expect(t, e.do(http.MethodPatch, path, "admin", map[string]any{"readOnly": true}), http.StatusOK, "")

	// Kubeconfigs without a cloud stanza are not affected.
	expect(t, e.do(http.MethodPost, "/api/clusters", "admin", map[string]string{"name": "plain", "content": testKubeconfig}), http.StatusOK, "")
}

func TestClusterCredentialsAreWriteOnly(t *testing.T) {
	e := newTestEnv(t, ambientOptIn)
	creds := map[string]string{"provider": "aws", "accessKeyId": "AKIAEXAMPLE", "secretAccessKey": awsSecret}
	rec := e.do(http.MethodPost, "/api/clusters", "admin", map[string]any{"name": "eks", "content": eksKubeconfig, "credentials": creds})
	expect(t, rec, http.StatusOK, "")
	id := int64(decode[map[string]any](t, rec)["id"].(float64))

	stored, _ := e.store.GetCluster(id)
	if !strings.Contains(stored.Credentials, awsSecret) {
		t.Fatal("the credentials were not stored")
	}
	for _, user := range []string{"admin", "alice"} {
		body := e.do(http.MethodGet, "/api/clusters", user, nil).Body.String()
		if strings.Contains(body, awsSecret) || strings.Contains(body, "AKIAEXAMPLE") {
			t.Fatalf("GET /api/clusters as %s leaks the credentials: %s", user, body)
		}
	}
	views := decode[[]clusterView](t, e.do(http.MethodGet, "/api/clusters", "admin", nil))
	if v := views[len(views)-1]; v.CloudCredentials != "aws" || v.CloudAuth != "aws" {
		t.Fatalf("admin view = %+v", v)
	}
	// Operators do not see how a cluster signs in.
	if err := e.store.SetAssignments(e.users["alice"], []store.Assignment{{ClusterID: id, Role: store.ClusterRoleOperator}}); err != nil {
		t.Fatal(err)
	}
	for _, v := range decode[[]clusterView](t, e.do(http.MethodGet, "/api/clusters", "alice", nil)) {
		if v.CloudAuth != "" || v.CloudCredentials != "" {
			t.Fatalf("alice sees %+v", v)
		}
	}

	path := "/api/clusters/" + strconv.FormatInt(id, 10)
	// Wrong cloud, half a key, unknown fields: refused, audited without values.
	for _, bad := range []any{
		map[string]string{"provider": "gcp", "serviceAccountJson": `{"type":"service_account","client_email":"a@b","private_key":"-----BEGIN PRIVATE KEY-----"}`},
		map[string]string{"provider": "aws", "accessKeyId": "AKIAEXAMPLE"},
		map[string]string{"provider": "aws", "profile": "p", "secret": awsSecret},
	} {
		expect(t, e.do(http.MethodPatch, path, "admin", map[string]any{"credentials": bad}), http.StatusBadRequest, plugin.CodeBadRequest)
	}
	expect(t, e.do(http.MethodPatch, path, "alice", map[string]any{"credentials": nil}), http.StatusForbidden, plugin.CodeAdminRequired)

	expect(t, e.do(http.MethodPatch, path, "admin", map[string]any{"credentials": map[string]string{"provider": "aws", "profile": "prod"}}),
		http.StatusOK, "")
	if c, _ := e.store.GetCluster(id); c.Credentials != `{"provider":"aws","profile":"prod"}` {
		t.Fatalf("after replace = %s", c.Credentials)
	}
	expect(t, e.do(http.MethodPatch, path, "admin", map[string]any{"credentials": nil}), http.StatusOK, "")
	if c, _ := e.store.GetCluster(id); c.Credentials != "" {
		t.Fatalf("after removal = %s", c.Credentials)
	}

	rows := e.audit()
	for _, row := range rows {
		if strings.Contains(row.Detail, awsSecret) || strings.Contains(row.Detail, "AKIAEXAMPLE") || strings.Contains(row.Target, awsSecret) {
			t.Fatalf("audit row carries a credential: %+v", row)
		}
	}
	if rows[0].Detail != `{"credentials":null}` || !strings.Contains(rows[1].Detail, `"credentials":"aws"`) {
		t.Fatalf("latest audit rows = %+v / %+v", rows[0], rows[1])
	}
	if add := rows[len(rows)-1]; add.Action != "POST /api/clusters" || add.Detail != `{"credentials":"aws","source":"kubeconfig"}` {
		t.Fatalf("add audit row = %+v", add)
	}

	// Credentials need a recognised stanza of their cloud.
	expect(t, e.do(http.MethodPost, "/api/clusters", "admin", map[string]any{"name": "plain", "content": testKubeconfig, "credentials": creds}),
		http.StatusBadRequest, plugin.CodeBadRequest)
	expect(t, e.do(http.MethodPost, "/api/clusters", "admin", map[string]any{"name": "addr", "address": "https://x:6443", "credentials": creds}),
		http.StatusBadRequest, plugin.CodeBadRequest)
}
