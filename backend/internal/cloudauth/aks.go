package cloudauth

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/cloud"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"golang.org/x/oauth2"
)

// azureSource turns an Entra ID credential into tokens for the AKS server
// application.
type azureSource struct {
	cred  azcore.TokenCredential
	scope string
}

func (s *azureSource) Token() (*oauth2.Token, error) {
	ctx, cancel := context.WithTimeout(context.Background(), fetchTimeout)
	defer cancel()
	tok, err := s.cred.GetToken(ctx, policy.TokenRequestOptions{Scopes: []string{s.scope}})
	if err != nil {
		return nil, fmt.Errorf("Azure token: %w", err)
	}
	return &oauth2.Token{AccessToken: tok.Token, TokenType: "Bearer", Expiry: tok.ExpiresOn}, nil
}

// azureScope is the token scope of an AKS server application.
func azureScope(serverID string) string { return strings.TrimSuffix(serverID, "/") + "/.default" }

// azureCloud maps kubelogin's --environment to the Entra ID authority.
func azureCloud(environment string) (cloud.Configuration, error) {
	switch strings.ToLower(environment) {
	case "", "azurepubliccloud":
		return cloud.AzurePublic, nil
	case "azurechinacloud":
		return cloud.AzureChina, nil
	case "azureusgovernmentcloud":
		return cloud.AzureGovernment, nil
	}
	return cloud.Configuration{}, fmt.Errorf("unsupported Azure environment %q", environment)
}

// azureCredential is the identity for AKS: per-cluster service-principal
// credentials, else the stanza's login mode: spn reads the server's
// AZURE_* environment, msi its managed identity (the stanza's client id
// picks a user-assigned one), workloadidentity Azure Workload Identity's
// projected token; the interactive and CLI modes fall back to the ambient
// chain of the three. The Azure CLI is never run.
func azureCredential(spec *ExecSpec, creds *Credentials) (azcore.TokenCredential, error) {
	authority, err := azureCloud(spec.Environment)
	if err != nil {
		return nil, err
	}
	client := azcore.ClientOptions{Cloud: authority}
	if creds != nil {
		return azidentity.NewClientSecretCredential(creds.TenantID, creds.ClientID, creds.ClientSecret,
			&azidentity.ClientSecretCredentialOptions{ClientOptions: client})
	}
	switch spec.Login {
	case LoginSPN:
		return azidentity.NewEnvironmentCredential(&azidentity.EnvironmentCredentialOptions{ClientOptions: client})
	case LoginMSI:
		return managedIdentity(client, spec.ClientID)
	case LoginWorkloadIdentity:
		return workloadIdentity(client, spec.ClientID, spec.TenantID)
	}
	// The ambient chain. The client and tenant ids of an interactive
	// stanza name the public client it would sign in with (the Azure
	// CLI's), not an identity of this server, so they are not passed on.
	var chain []azcore.TokenCredential
	var errs []error
	for _, build := range []func() (azcore.TokenCredential, error){
		func() (azcore.TokenCredential, error) {
			return azidentity.NewEnvironmentCredential(&azidentity.EnvironmentCredentialOptions{ClientOptions: client})
		},
		func() (azcore.TokenCredential, error) { return workloadIdentity(client, "", "") },
		func() (azcore.TokenCredential, error) { return managedIdentity(client, "") },
	} {
		cred, err := build()
		if err != nil { // not configured on this server
			errs = append(errs, err)
			continue
		}
		chain = append(chain, cred)
	}
	if len(chain) == 0 {
		return nil, fmt.Errorf("no Azure identity is configured on the server: %w", errors.Join(errs...))
	}
	return azidentity.NewChainedTokenCredential(chain, nil)
}

// workloadIdentity reads Azure Workload Identity's environment
// (AZURE_FEDERATED_TOKEN_FILE and, unless given, AZURE_CLIENT_ID and
// AZURE_TENANT_ID), which its webhook injects into the pod.
func workloadIdentity(client azcore.ClientOptions, clientID, tenantID string) (azcore.TokenCredential, error) {
	return azidentity.NewWorkloadIdentityCredential(&azidentity.WorkloadIdentityCredentialOptions{
		ClientOptions: client, ClientID: clientID, TenantID: tenantID,
	})
}

// managedIdentity is the system-assigned identity, or the user-assigned
// one with clientID.
func managedIdentity(client azcore.ClientOptions, clientID string) (azcore.TokenCredential, error) {
	opts := &azidentity.ManagedIdentityCredentialOptions{ClientOptions: client}
	if clientID != "" {
		opts.ID = azidentity.ClientID(clientID)
	}
	return azidentity.NewManagedIdentityCredential(opts)
}
