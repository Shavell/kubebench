package kube

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
)

// OptionalResources are the catalog keys of resource types only some
// clusters serve: OpenShift's (§7.15). The capability probe reports which
// of them the cluster serves (Capabilities.OptionalResources), so the UI
// lists only those.
var OptionalResources = []string{
	"routes", "projects", "securitycontextconstraints", "deploymentconfigs",
	"buildconfigs", "builds", "imagestreams", "imagestreamtags",
}

// servedOptional returns the optional catalog keys whose group and version
// the cluster serves, from one discovery request (the group list, not
// every group's resources). A discovery failure reports none.
func servedOptional(d discovery.DiscoveryInterface) []string {
	served := []string{}
	groups, err := d.ServerGroups()
	if err != nil {
		return served
	}
	versions := map[schema.GroupVersion]bool{}
	for _, g := range groups.Groups {
		for _, v := range g.Versions {
			versions[schema.GroupVersion{Group: g.Name, Version: v.Version}] = true
		}
	}
	for _, key := range OptionalResources {
		if versions[Catalog[key].GVR.GroupVersion()] {
			served = append(served, key)
		}
	}
	return served
}
