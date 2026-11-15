// Package all links the built-in backend plugins into the binary; main.go
// imports it for its side effects. No plugin ships yet.
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
	_ "kubebench/internal/plugins/storage"
)
