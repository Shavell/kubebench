package kube

import (
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

// ReadKubeconfig reads a kubeconfig file for storage in the inventory. The
// certificate and key files it references (relative to the file itself)
// are embedded as *-data fields, so the stored copy works without them.
func ReadKubeconfig(path string) ([]byte, error) {
	kc, err := clientcmd.LoadFromFile(path)
	if err != nil {
		return nil, err
	}
	if err := clientcmd.ResolveLocalPaths(kc); err != nil {
		return nil, err
	}
	if err := clientcmdapi.FlattenConfig(kc); err != nil {
		return nil, err
	}
	return clientcmd.Write(*kc)
}
