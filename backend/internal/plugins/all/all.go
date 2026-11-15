// Package all links the built-in backend plugins into the binary; main.go
// imports it for its side effects.
//
// To add a plugin, create backend/internal/plugins/<id>/ with a type that
// implements plugin.Plugin (see internal/plugin) and registers itself:
//
//	package argocd
//
//	func init() { plugin.Register(&Plugin{}) }
//
// then add a blank import to this file:
//
//	import _ "kubebench/internal/plugins/argocd"
//
// Its routes are served under /api/plugins/<id>/.
package all

import (
	_ "kubebench/internal/plugins/argocd"  // Argo CD applications (§7.5)
	_ "kubebench/internal/plugins/helm"    // Helm releases, repositories and charts (§7.4)
	_ "kubebench/internal/plugins/storage" // PV, PVC and StorageClass creation (§7.6)

	_ "kubebench/internal/plugins/security" // Security posture checks and accepted risks (§7.7)
)
