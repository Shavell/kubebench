package kube

import (
	"fmt"
	"strings"

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
	return flatten(kc)
}

// MergeKubeconfigs reads a KUBECONFIG path list for storage in the
// inventory, merged the way kubectl merges it: the first file that sets a
// value wins, and files that do not exist are skipped. Referenced files are
// embedded as in ReadKubeconfig.
func MergeKubeconfigs(paths []string) ([]byte, error) {
	rules := &clientcmd.ClientConfigLoadingRules{Precedence: paths}
	kc, err := rules.Load() // resolves each file's relative paths
	if err != nil {
		return nil, err
	}
	if len(kc.Contexts) == 0 {
		return nil, fmt.Errorf("no kubeconfig context found in %s (missing files are skipped)", strings.Join(paths, ", "))
	}
	return flatten(kc)
}

// flatten embeds the referenced certificate and key files and serializes kc.
func flatten(kc *clientcmdapi.Config) ([]byte, error) {
	if err := clientcmdapi.FlattenConfig(kc); err != nil {
		return nil, err
	}
	return clientcmd.Write(*kc)
}
