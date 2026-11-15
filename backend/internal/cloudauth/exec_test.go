package cloudauth

import (
	"testing"

	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

func stanza(command string, args []string, env ...string) *clientcmdapi.ExecConfig {
	e := &clientcmdapi.ExecConfig{APIVersion: "client.authentication.k8s.io/v1beta1", Command: command, Args: args}
	for i := 0; i+1 < len(env); i += 2 {
		e.Env = append(e.Env, clientcmdapi.ExecEnvVar{Name: env[i], Value: env[i+1]})
	}
	return e
}

func TestRecognize(t *testing.T) {
	tests := []struct {
		name string
		exec *clientcmdapi.ExecConfig
		want *ExecSpec // nil = not recognised
	}{
		{"nil", nil, nil},
		// aws eks update-kubeconfig
		{"aws cli", stanza("aws", []string{"--region", "eu-west-1", "eks", "get-token", "--cluster-name", "prod", "--output", "json"}),
			&ExecSpec{Provider: AWS, ClusterID: "prod", Region: "eu-west-1"}},
		{"aws cli with profile env", stanza("aws", []string{"eks", "get-token", "--cluster-name", "prod"}, "AWS_PROFILE", "ops", "AWS_REGION", "us-west-2"),
			&ExecSpec{Provider: AWS, ClusterID: "prod", Region: "us-west-2", Profile: "ops"}},
		{"aws cli flags win over env", stanza("aws", []string{"eks", "get-token", "--cluster-name=prod", "--region=eu-central-1", "--profile", "p2", "--role-arn", "arn:aws:iam::123456789012:role/eks"}, "AWS_PROFILE", "p1", "AWS_DEFAULT_REGION", "us-east-1"),
			&ExecSpec{Provider: AWS, ClusterID: "prod", Region: "eu-central-1", Profile: "p2", RoleARN: "arn:aws:iam::123456789012:role/eks"}},
		{"aws cli absolute path and boolean global flag", stanza("/usr/local/bin/aws", []string{"--no-cli-pager", "eks", "get-token", "--cluster-id", "outpost-1"}),
			&ExecSpec{Provider: AWS, ClusterID: "outpost-1"}},
		{"aws.exe on windows", stanza(`C:\Program Files\Amazon\AWSCLIV2\aws.exe`, []string{"eks", "get-token", "--cluster-name", "dev"}),
			&ExecSpec{Provider: AWS, ClusterID: "dev"}},
		{"aws cli without a cluster", stanza("aws", []string{"eks", "get-token"}), nil},
		{"aws cli other command", stanza("aws", []string{"sts", "get-caller-identity"}), nil},
		// eksctl / older kubeconfigs
		{"aws-iam-authenticator", stanza("aws-iam-authenticator", []string{"token", "-i", "prod"}, "AWS_PROFILE", "ops"),
			&ExecSpec{Provider: AWS, ClusterID: "prod", Profile: "ops"}},
		{"aws-iam-authenticator with role", stanza("aws-iam-authenticator", []string{"token", "--cluster-id", "prod", "-r", "arn:aws:iam::1:role/r", "--region", "ap-south-1"}),
			&ExecSpec{Provider: AWS, ClusterID: "prod", Region: "ap-south-1", RoleARN: "arn:aws:iam::1:role/r"}},
		{"aws-iam-authenticator verify", stanza("aws-iam-authenticator", []string{"verify", "-i", "prod"}), nil},
		// gcloud container clusters get-credentials
		{"gke plugin", stanza("gke-gcloud-auth-plugin", nil), &ExecSpec{Provider: GCP}},
		{"gke plugin with flags", stanza("/google-cloud-sdk/bin/gke-gcloud-auth-plugin", []string{"--use_application_default_credentials"}), &ExecSpec{Provider: GCP}},
		// az aks get-credentials (+ kubelogin convert-kubeconfig)
		{"kubelogin devicecode", stanza("kubelogin", []string{"get-token", "--login", "devicecode", "--server-id", "6dae42f8-4368-4678-94ff-3960e28e3630", "--client-id", "80faf920-1908-4b52-b5ef-a8e7bedfc67a", "--tenant-id", "t", "--environment", "AzurePublicCloud"}),
			&ExecSpec{Provider: Azure, ServerID: DefaultAKSServerID, Login: LoginAmbient, ClientID: "80faf920-1908-4b52-b5ef-a8e7bedfc67a", TenantID: "t", Environment: "AzurePublicCloud"}},
		{"kubelogin azurecli", stanza("kubelogin", []string{"get-token", "-l", "azurecli", "--server-id", "custom-app"}),
			&ExecSpec{Provider: Azure, ServerID: "custom-app", Login: LoginAmbient}},
		{"kubelogin default mode and server id", stanza("kubelogin", []string{"get-token"}),
			&ExecSpec{Provider: Azure, ServerID: DefaultAKSServerID, Login: LoginAmbient}},
		{"kubelogin spn", stanza("kubelogin", []string{"get-token", "--login", "spn", "-t", "tenant", "--client-id", "app"}),
			&ExecSpec{Provider: Azure, ServerID: DefaultAKSServerID, Login: LoginSPN, ClientID: "app", TenantID: "tenant"}},
		{"kubelogin msi", stanza("kubelogin", []string{"get-token", "--login=msi", "--client-id=mi"}),
			&ExecSpec{Provider: Azure, ServerID: DefaultAKSServerID, Login: LoginMSI, ClientID: "mi"}},
		{"kubelogin workload identity", stanza("kubelogin", []string{"get-token", "--login", "workloadidentity"}),
			&ExecSpec{Provider: Azure, ServerID: DefaultAKSServerID, Login: LoginWorkloadIdentity}},
		{"kubelogin ropc needs a password", stanza("kubelogin", []string{"get-token", "--login", "ropc"}), nil},
		{"kubelogin other command", stanza("kubelogin", []string{"convert-kubeconfig"}), nil},
		// anything else stays an exec plugin
		{"shell", stanza("/bin/sh", []string{"-c", "aws eks get-token --cluster-name x"}), nil},
		{"look-alike name", stanza("aws-wrapper", []string{"eks", "get-token", "--cluster-name", "x"}), nil},
		{"gcloud config-helper", stanza("gcloud", []string{"config", "config-helper"}), nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := Recognize(tt.exec)
			if tt.want == nil {
				if ok {
					t.Fatalf("recognised %+v", got)
				}
				return
			}
			if !ok || *got != *tt.want {
				t.Fatalf("Recognize = %+v, %v; want %+v", got, ok, tt.want)
			}
		})
	}
}

// Variables other than the region and profile, which could point the
// AWS SDK at server-side files or other identities, are not read.
func TestRecognizeIgnoresOtherEnv(t *testing.T) {
	spec, ok := Recognize(stanza("aws", []string{"eks", "get-token", "--cluster-name", "x"},
		"AWS_SHARED_CREDENTIALS_FILE", "/var/lib/kubebench/kubebench.db",
		"AWS_WEB_IDENTITY_TOKEN_FILE", "/etc/shadow",
		"AWS_ACCESS_KEY_ID", "AKIA..."))
	if !ok || *spec != (ExecSpec{Provider: AWS, ClusterID: "x"}) {
		t.Fatalf("spec = %+v", spec)
	}
}
