// Package cloud recognises where a cluster runs (a managed Kubernetes
// service, a distribution, the cloud underneath) and whether its
// Kubernetes version is still supported there (docs/design-v0.5.md §7.12).
package cloud

import (
	"net/url"
	"strings"
)

// Providers: a managed service or a distribution, decided by the
// strongest signal found (see Detect).
const (
	ProviderEKS        = "eks"
	ProviderAKS        = "aks"
	ProviderGKE        = "gke"
	ProviderDOKS       = "doks"
	ProviderLKE        = "lke"
	ProviderOVH        = "ovh"
	ProviderOpenShift  = "openshift"
	ProviderRKE2       = "rke2"
	ProviderK3s        = "k3s"
	ProviderKubernetes = "kubernetes" // nothing more specific recognised
)

// Clouds: the infrastructure the nodes run on, from their providerID.
const (
	CloudAWS          = "aws"
	CloudAzure        = "azure"
	CloudGCP          = "gcp"
	CloudDigitalOcean = "digitalocean"
	CloudLinode       = "linode"
	CloudOVH          = "ovh"
)

// Node is what detection reads from a Node object.
type Node struct {
	ProviderID string
	Labels     map[string]string
}

// Signals are the facts detection works from: the API server URL, the
// server's gitVersion and a sample of the nodes (any of them may be empty).
type Signals struct {
	Server     string
	GitVersion string
	Nodes      []Node
}

// Detection is the result: Provider is always set, Cloud is "" when the
// infrastructure is unknown, Version is the server's gitVersion.
type Detection struct {
	Provider string `json:"provider"`
	Cloud    string `json:"cloud"`
	Version  string `json:"version"`
}

// managed maps the managed services to the cloud they run on.
var managed = map[string]string{
	ProviderEKS: CloudAWS, ProviderAKS: CloudAzure, ProviderGKE: CloudGCP,
	ProviderDOKS: CloudDigitalOcean, ProviderLKE: CloudLinode, ProviderOVH: CloudOVH,
}

// hostSuffixes are API server host names only a managed service hands out.
var hostSuffixes = []struct{ suffix, provider string }{
	{".eks.amazonaws.com", ProviderEKS},
	{".eks.amazonaws.com.cn", ProviderEKS},
	{".azmk8s.io", ProviderAKS},
	{".gke.goog", ProviderGKE}, // GKE DNS-based control plane endpoints
	{"container.googleapis.com", ProviderGKE},
	{".k8s.ondigitalocean.com", ProviderDOKS},
	{".linodelke.net", ProviderLKE},
	{".ovh.net", ProviderOVH},
	{".ovh.cloud", ProviderOVH},
}

// nodeLabels are labels the managed services (and their tooling) put on
// their nodes; the value does not matter.
var nodeLabels = []struct{ label, provider string }{
	{"eks.amazonaws.com/nodegroup", ProviderEKS},
	{"eks.amazonaws.com/compute-type", ProviderEKS}, // Fargate and Auto Mode
	{"alpha.eksctl.io/cluster-name", ProviderEKS},
	{"kubernetes.azure.com/cluster", ProviderAKS},
	{"kubernetes.azure.com/agentpool", ProviderAKS},
	{"cloud.google.com/gke-nodepool", ProviderGKE},
	{"cloud.google.com/gke-os-distribution", ProviderGKE},
	{"doks.digitalocean.com/node-pool", ProviderDOKS},
	{"lke.linode.com/pool-id", ProviderLKE},
}

// providerIDClouds maps a node's spec.providerID scheme to its cloud.
var providerIDClouds = map[string]string{
	"aws": CloudAWS, "azure": CloudAzure, "gce": CloudGCP,
	"digitalocean": CloudDigitalOcean, "linode": CloudLinode,
}

// Detect decides the provider from the signals, strongest first:
//
//  1. OpenShift markers (OpenShift has its own lifecycle even on a cloud,
//     e.g. ROSA or ARO);
//  2. a managed service's API host name, gitVersion suffix (-eks-, -gke.)
//     or node label;
//  3. a providerID that only a managed service uses (digitalocean://,
//     linode://);
//  4. k3s and RKE2 (gitVersion suffix, providerID scheme, node label);
//  5. otherwise "kubernetes".
//
// A providerID of aws://, azure:// or gce:// alone names the cloud but not
// the service: self-managed clusters (kops, kubeadm) use them too, and
// they do not follow the managed services' support windows.
func Detect(s Signals) Detection {
	d := Detection{Provider: ProviderKubernetes, Version: s.GitVersion}
	for _, n := range s.Nodes {
		if d.Cloud = cloudOf(n.ProviderID); d.Cloud != "" {
			break
		}
	}
	d.Provider = providerOf(s)
	if cloud, ok := managed[d.Provider]; ok {
		d.Cloud = cloud
	}
	return d
}

func providerOf(s Signals) string {
	for _, n := range s.Nodes {
		if _, ok := n.Labels["node.openshift.io/os_id"]; ok {
			return ProviderOpenShift
		}
	}
	if p := providerFromHost(s.Server); p != "" {
		return p
	}
	version := strings.ToLower(s.GitVersion)
	switch {
	case strings.Contains(version, "-eks-"):
		return ProviderEKS
	case strings.Contains(version, "-gke."):
		return ProviderGKE
	}
	for _, n := range s.Nodes {
		for _, l := range nodeLabels {
			if _, ok := n.Labels[l.label]; ok {
				return l.provider
			}
		}
	}
	for _, n := range s.Nodes {
		switch cloudOf(n.ProviderID) {
		case CloudDigitalOcean:
			return ProviderDOKS
		case CloudLinode:
			return ProviderLKE
		}
	}
	switch {
	case strings.Contains(version, "+k3s"):
		return ProviderK3s
	case strings.Contains(version, "+rke2"):
		return ProviderRKE2
	}
	for _, n := range s.Nodes {
		switch scheme(n.ProviderID) {
		case "k3s":
			return ProviderK3s
		case "rke2":
			return ProviderRKE2
		}
		if n.Labels["node.kubernetes.io/instance-type"] == "k3s" {
			return ProviderK3s
		}
	}
	return ProviderKubernetes
}

// ProviderFromServer is the provider an API server URL alone reveals, or
// ProviderKubernetes; it serves clusters whose runtime is not built yet.
func ProviderFromServer(server string) Detection {
	d := Detection{Provider: ProviderKubernetes}
	if p := providerFromHost(server); p != "" {
		d.Provider, d.Cloud = p, managed[p]
	}
	return d
}

func providerFromHost(server string) string {
	if server == "" {
		return ""
	}
	if !strings.Contains(server, "://") {
		server = "https://" + server
	}
	u, err := url.Parse(server)
	if err != nil {
		return ""
	}
	host := strings.ToLower(u.Hostname())
	for _, h := range hostSuffixes {
		if strings.HasSuffix(host, h.suffix) {
			return h.provider
		}
	}
	return ""
}

func cloudOf(providerID string) string { return providerIDClouds[scheme(providerID)] }

func scheme(providerID string) string {
	s, _, ok := strings.Cut(providerID, "://")
	if !ok {
		return ""
	}
	return strings.ToLower(s)
}
