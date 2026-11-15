package cloud

import "testing"

func TestDetect(t *testing.T) {
	label := func(key string) []Node { return []Node{{Labels: map[string]string{key: "x"}}} }
	providerID := func(id string) []Node { return []Node{{ProviderID: id}} }
	tests := []struct {
		name          string
		signals       Signals
		provider, cld string
	}{
		{"empty", Signals{}, ProviderKubernetes, ""},
		{"eks url", Signals{Server: "https://ABCDEF0123.gr7.eu-west-1.eks.amazonaws.com"}, ProviderEKS, CloudAWS},
		{"eks china url", Signals{Server: "https://abc.yl4.cn-north-1.eks.amazonaws.com.cn"}, ProviderEKS, CloudAWS},
		{"aks url", Signals{Server: "https://prod-dns-1a2b3c4d.hcp.westeurope.azmk8s.io:443"}, ProviderAKS, CloudAzure},
		{"aks private url", Signals{Server: "https://prod-1a2b.privatelink.westeurope.azmk8s.io:443"}, ProviderAKS, CloudAzure},
		{"gke dns endpoint", Signals{Server: "https://gke-0123abcd-my-project.europe-west1.gke.goog"}, ProviderGKE, CloudGCP},
		{"gke api", Signals{Server: "https://container.googleapis.com/v1/projects/p/locations/l/clusters/c"}, ProviderGKE, CloudGCP},
		{"doks url", Signals{Server: "https://0a1b2c3d.k8s.ondigitalocean.com"}, ProviderDOKS, CloudDigitalOcean},
		{"lke url", Signals{Server: "https://0a1b2c3d-4e5f.eu-central-2.linodelke.net:443"}, ProviderLKE, CloudLinode},
		{"ovh url", Signals{Server: "https://xyz.c1.gra7.k8s.ovh.net"}, ProviderOVH, CloudOVH},
		{"host without scheme", Signals{Server: "abc.gr7.us-east-1.eks.amazonaws.com:443"}, ProviderEKS, CloudAWS},
		{"look-alike path is not a host", Signals{Server: "https://10.0.0.1:6443/eks.amazonaws.com"}, ProviderKubernetes, ""},
		{"unparseable url", Signals{Server: "https://[::1"}, ProviderKubernetes, ""},
		{"gke ip endpoint, version", Signals{Server: "https://34.77.1.2", GitVersion: "v1.33.5-gke.1080000"}, ProviderGKE, CloudGCP},
		{"eks version", Signals{GitVersion: "v1.32.9-eks-113cf36"}, ProviderEKS, CloudAWS},
		{"eks nodegroup label", Signals{Nodes: label("eks.amazonaws.com/nodegroup")}, ProviderEKS, CloudAWS},
		{"eks fargate label", Signals{Nodes: label("eks.amazonaws.com/compute-type")}, ProviderEKS, CloudAWS},
		{"aks label", Signals{Nodes: label("kubernetes.azure.com/cluster")}, ProviderAKS, CloudAzure},
		{"gke label", Signals{Nodes: label("cloud.google.com/gke-nodepool")}, ProviderGKE, CloudGCP},
		{"doks label", Signals{Nodes: label("doks.digitalocean.com/node-pool")}, ProviderDOKS, CloudDigitalOcean},
		{"lke label", Signals{Nodes: label("lke.linode.com/pool-id")}, ProviderLKE, CloudLinode},
		{"digitalocean providerID", Signals{Nodes: providerID("digitalocean://123456")}, ProviderDOKS, CloudDigitalOcean},
		{"linode providerID", Signals{Nodes: providerID("linode://123456")}, ProviderLKE, CloudLinode},
		// Self-managed clusters on a cloud keep the upstream lifecycle.
		{"aws providerID alone", Signals{Nodes: providerID("aws:///eu-west-1a/i-0abc")}, ProviderKubernetes, CloudAWS},
		{"azure providerID alone", Signals{Nodes: providerID("azure:///subscriptions/s/resourceGroups/r/providers/Microsoft.Compute/virtualMachines/vm")}, ProviderKubernetes, CloudAzure},
		{"gce providerID alone", Signals{Nodes: providerID("gce://project/europe-west1-b/node-1")}, ProviderKubernetes, CloudGCP},
		{"k3s version", Signals{Server: "https://127.0.0.1:6550", GitVersion: "v1.35.5+k3s1"}, ProviderK3s, ""},
		{"k3s providerID", Signals{Nodes: providerID("k3s://k3d-dev-server-0")}, ProviderK3s, ""},
		{"k3s instance type", Signals{Nodes: []Node{{Labels: map[string]string{"node.kubernetes.io/instance-type": "k3s"}}}}, ProviderK3s, ""},
		{"rke2 version", Signals{GitVersion: "v1.34.2+rke2r1"}, ProviderRKE2, ""},
		{"rke2 providerID", Signals{Nodes: providerID("rke2://node-1")}, ProviderRKE2, ""},
		{"k3s on aws", Signals{GitVersion: "v1.35.5+k3s1", Nodes: providerID("aws:///eu-west-1a/i-0abc")}, ProviderK3s, CloudAWS},
		{"openshift on aws (rosa)", Signals{
			Server: "https://api.rosa.abcd.p1.openshiftapps.com:6443",
			Nodes:  []Node{{ProviderID: "aws:///us-east-1a/i-0abc", Labels: map[string]string{"node.openshift.io/os_id": "rhcos"}}},
		}, ProviderOpenShift, CloudAWS},
		{"unlabelled node first", Signals{Nodes: []Node{{}, {ProviderID: "gce://p/z/n", Labels: map[string]string{"cloud.google.com/gke-nodepool": "pool"}}}}, ProviderGKE, CloudGCP},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Detect(tt.signals)
			if got.Provider != tt.provider || got.Cloud != tt.cld || got.Version != tt.signals.GitVersion {
				t.Fatalf("Detect(%+v) = %+v, want provider %q cloud %q", tt.signals, got, tt.provider, tt.cld)
			}
		})
	}
}

func TestProviderFromServer(t *testing.T) {
	if d := ProviderFromServer("https://x.hcp.eastus.azmk8s.io:443"); d.Provider != ProviderAKS || d.Cloud != CloudAzure {
		t.Fatalf("aks = %+v", d)
	}
	if d := ProviderFromServer("kubernetes.default.svc"); d.Provider != ProviderKubernetes || d.Cloud != "" {
		t.Fatalf("in-cluster = %+v", d)
	}
	if d := ProviderFromServer(""); d.Provider != ProviderKubernetes {
		t.Fatalf("empty = %+v", d)
	}
}
