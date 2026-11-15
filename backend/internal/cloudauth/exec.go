// Package cloudauth authenticates to EKS, GKE and AKS in process, in place
// of the exec credential plugins the clouds put into their kubeconfigs
// (aws eks get-token, aws-iam-authenticator, gke-gcloud-auth-plugin,
// kubelogin), so KubeBench never runs a command for them
// (docs/design-v0.5.md §7.12).
package cloudauth

import (
	"path/filepath"
	"strings"

	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

// Credential providers: the cloud whose identity signs in.
const (
	AWS   = "aws"
	GCP   = "gcp"
	Azure = "azure"
)

// DefaultAKSServerID is the AKS AAD server application, whose token every
// AKS cluster with Entra ID integration accepts.
const DefaultAKSServerID = "6dae42f8-4368-4678-94ff-3960e28e3630"

// Azure login modes (kubelogin --login) handled in process.
const (
	LoginSPN              = "spn"
	LoginMSI              = "msi"
	LoginWorkloadIdentity = "workloadidentity"
	// LoginAmbient stands for the interactive and CLI modes (devicecode,
	// interactive, azurecli, azd): a server cannot prompt or run the CLI,
	// so it signs in with its ambient identity (environment, workload
	// identity, managed identity).
	LoginAmbient = "ambient"
)

// ExecSpec is a recognised exec stanza: what the plugin would have done.
type ExecSpec struct {
	Provider string // AWS | GCP | Azure

	// AWS: the cluster name or id sent as x-k8s-aws-id, the STS region,
	// a shared-config profile and a role to assume.
	ClusterID, Region, Profile, RoleARN string

	// Azure: the server application id, the login mode and the client
	// and tenant ids from the arguments, and the cloud environment.
	ServerID, Login, ClientID, TenantID, Environment string
}

// Recognize returns what a kubeconfig exec stanza asks for, when it runs
// one of the cloud plugins in a form handled in process. Other commands,
// and unsupported modes (kubelogin's ropc, which needs a user password),
// are not recognised: they stay exec plugins and are subject to
// KUBEBENCH_ALLOW_KUBECONFIG_EXEC. Only the arguments and the few
// environment variables named below are read; anything else in the stanza
// (other variables, file paths) is ignored.
func Recognize(exec *clientcmdapi.ExecConfig) (*ExecSpec, bool) {
	if exec == nil {
		return nil, false
	}
	env := map[string]string{}
	for _, e := range exec.Env {
		env[e.Name] = e.Value
	}
	switch command(exec.Command) {
	case "aws":
		return recognizeAWSCLI(exec.Args, env)
	case "aws-iam-authenticator":
		return recognizeIAMAuthenticator(exec.Args, env)
	case "gke-gcloud-auth-plugin":
		return &ExecSpec{Provider: GCP}, true
	case "kubelogin":
		return recognizeKubelogin(exec.Args)
	}
	return nil, false
}

// command is the plugin's base name, without a directory or ".exe".
func command(path string) string {
	base := filepath.Base(strings.ReplaceAll(path, `\`, "/"))
	return strings.TrimSuffix(strings.ToLower(base), ".exe")
}

// recognizeAWSCLI handles `aws [global options] eks get-token --cluster-name
// N [--region R] [--role-arn A] [--profile P]` (`--cluster-id` for local
// clusters on Outposts).
func recognizeAWSCLI(args []string, env map[string]string) (*ExecSpec, bool) {
	flags, positional := parseFlags(args, nil, awsBooleanFlags)
	if !containsSequence(positional, "eks", "get-token") {
		return nil, false
	}
	spec := awsSpec(env)
	spec.ClusterID = first(flags["cluster-id"], flags["cluster-name"])
	spec.Region = first(flags["region"], spec.Region)
	spec.Profile = first(flags["profile"], spec.Profile)
	spec.RoleARN = flags["role-arn"]
	return spec, spec.ClusterID != ""
}

// awsBooleanFlags are the AWS CLI's global options without a value, which
// may come before the "eks get-token" command.
var awsBooleanFlags = map[string]bool{
	"debug": true, "no-verify-ssl": true, "no-paginate": true, "no-sign-request": true,
	"no-cli-pager": true, "cli-auto-prompt": true, "no-cli-auto-prompt": true,
}

// recognizeIAMAuthenticator handles `aws-iam-authenticator token -i N
// [-r ROLE] [--region R]`.
func recognizeIAMAuthenticator(args []string, env map[string]string) (*ExecSpec, bool) {
	flags, positional := parseFlags(args, map[string]string{"i": "cluster-id", "r": "role"}, nil)
	if len(positional) == 0 || positional[0] != "token" {
		return nil, false
	}
	spec := awsSpec(env)
	spec.ClusterID = flags["cluster-id"]
	spec.Region = first(flags["region"], spec.Region)
	spec.RoleARN = flags["role"]
	return spec, spec.ClusterID != ""
}

// awsSpec reads the region and profile the AWS tools take from the
// stanza's environment.
func awsSpec(env map[string]string) *ExecSpec {
	return &ExecSpec{
		Provider: AWS,
		Region:   first(env["AWS_REGION"], env["AWS_DEFAULT_REGION"]),
		Profile:  env["AWS_PROFILE"],
	}
}

// recognizeKubelogin handles `kubelogin get-token [--login MODE]
// [--server-id ID] [--client-id ID] [--tenant-id ID] [--environment ENV]`.
func recognizeKubelogin(args []string) (*ExecSpec, bool) {
	flags, positional := parseFlags(args, map[string]string{"l": "login", "t": "tenant-id", "e": "environment"}, nil)
	if len(positional) == 0 || positional[0] != "get-token" {
		return nil, false
	}
	spec := &ExecSpec{
		Provider:    Azure,
		ServerID:    first(flags["server-id"], DefaultAKSServerID),
		ClientID:    flags["client-id"],
		TenantID:    flags["tenant-id"],
		Environment: flags["environment"],
	}
	switch login := strings.ToLower(flags["login"]); login {
	case LoginSPN, LoginMSI, LoginWorkloadIdentity:
		spec.Login = login
	case "", "devicecode", "interactive", "azurecli", "azd":
		spec.Login = LoginAmbient
	default: // ropc and modes this version does not know
		return nil, false
	}
	return spec, true
}

// parseFlags splits args into "--name value" / "--name=value" flags (short
// names mapped through aliases) and positional words. Flags named in
// booleans, and flags followed by another flag or by nothing, take no
// value and get "true".
func parseFlags(args []string, aliases map[string]string, booleans map[string]bool) (map[string]string, []string) {
	flags := map[string]string{}
	var positional []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if !strings.HasPrefix(arg, "-") || arg == "-" {
			positional = append(positional, arg)
			continue
		}
		name := strings.TrimLeft(arg, "-")
		value, hasValue := "", false
		if n, v, ok := strings.Cut(name, "="); ok {
			name, value, hasValue = n, v, true
		}
		if long, ok := aliases[name]; ok {
			name = long
		}
		if !hasValue {
			if !booleans[name] && i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				value = args[i+1]
				i++
			} else {
				value = "true"
			}
		}
		flags[name] = value
	}
	return flags, positional
}

// containsSequence reports whether words contains want in order, adjacent.
func containsSequence(words []string, want ...string) bool {
	for i := 0; i+len(want) <= len(words); i++ {
		match := true
		for j, w := range want {
			if words[i+j] != w {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

func first(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
