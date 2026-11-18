package cloudauth

import "kubebench/internal/reason"

// Reasons of Credentials.Validate and CheckCredentials (§3.3).
var (
	reasonFieldTooLong     = reason.Define("cloud.field_too_long", "a credential field is longer than 16 KiB")
	reasonAWSOnly          = reason.Define("cloud.aws_only", "AWS credentials take an access key or a profile only")
	reasonGCPOnly          = reason.Define("cloud.gcp_only", "GCP credentials take a service-account key only")
	reasonAzureOnly        = reason.Define("cloud.azure_only", "Azure credentials take a tenant id, client id and client secret only")
	reasonAzureGUIDs       = reason.Define("cloud.azure_guids", "the Azure tenant id and client id are GUIDs")
	reasonAzureSecret      = reason.Define("cloud.azure_secret_required", "the Azure client secret is required")
	reasonUnknownProvider  = reason.Define("cloud.unknown_provider", "unknown credential provider \"{{provider}}\": use aws, gcp or azure")
	reasonAWSKeyAndProfile = reason.Define("cloud.aws_key_and_profile", "give either an AWS access key or a profile, not both")
	reasonAWSKeyIncomplete = reason.Define("cloud.aws_key_incomplete", "an AWS access key needs both the key id and the secret")
	reasonAWSMissing       = reason.Define("cloud.aws_missing", "give an AWS access key or a profile")
	reasonAWSProfile       = reason.Define("cloud.aws_profile", "invalid AWS profile name")
	reasonGCPJSON          = reason.Define("cloud.gcp_json", "the GCP service-account key is not valid JSON")
	reasonGCPType          = reason.Define("cloud.gcp_type", "the GCP key has type \"{{type}}\"; only service_account keys are accepted")
	reasonGCPFields        = reason.Define("cloud.gcp_fields", "the GCP service-account key lacks client_email or private_key")
	reasonNoCloudExec      = reason.Define("cloud.no_exec", "{{provider}} credentials need a kubeconfig whose user runs aws eks get-token or aws-iam-authenticator (aws), gke-gcloud-auth-plugin (gcp) or kubelogin get-token (azure)")
	reasonOtherCloud       = reason.Define("cloud.other_provider", "the kubeconfig signs in to {{actual}}, not {{provider}}")
)
