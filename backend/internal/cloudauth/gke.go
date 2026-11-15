package cloudauth

import (
	"context"
	"fmt"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

// gcpScope is what gke-gcloud-auth-plugin asks for: GKE accepts any
// Google OAuth access token of the cloud-platform scope.
const gcpScope = "https://www.googleapis.com/auth/cloud-platform"

// googleTokenSource is the identity for GKE: a per-cluster service-account
// key, else Application Default Credentials (GOOGLE_APPLICATION_CREDENTIALS,
// gcloud's application-default file, the metadata server: GKE Workload
// Identity or the VM's service account). The source outlives any request,
// so it is bound to the background context.
func googleTokenSource(creds *Credentials) (oauth2.TokenSource, error) {
	ctx := context.Background()
	if creds != nil {
		c, err := google.CredentialsFromJSONWithType(ctx, []byte(creds.ServiceAccountJSON), google.ServiceAccount, gcpScope)
		if err != nil {
			return nil, fmt.Errorf("GCP service-account key: %w", err)
		}
		return c.TokenSource, nil
	}
	c, err := google.FindDefaultCredentials(ctx, gcpScope)
	if err != nil {
		return nil, fmt.Errorf("GCP default credentials: %w", err)
	}
	return c.TokenSource, nil
}
