package kube

import (
	"fmt"
	"strconv"
	"strings"

	"k8s.io/apimachinery/pkg/runtime/schema"
)

// credentialKind is a kind other than Secret that holds credentials in
// its own fields: every read path redacts them the way it redacts Secret
// values, and no write may store a redaction marker in them.
type credentialKind struct {
	group, kind, resource string
	// fields are the paths of the credentials; "*" stands for every item
	// of a list.
	fields [][]string
}

// credentialKinds are OpenShift's: a Route's TLS private key (edge and
// re-encrypt termination), the inline webhook secrets of a BuildConfig's
// triggers, and the webhook secrets a Build repeats in the cause that
// triggered it (obfuscated by current OpenShift releases, redacted anyway).
var credentialKinds = []credentialKind{
	{group: "route.openshift.io", kind: "Route", resource: "routes", fields: [][]string{{"spec", "tls", "key"}}},
	{group: "build.openshift.io", kind: "BuildConfig", resource: "buildconfigs",
		fields: listItemFields([]string{"spec", "triggers"}, "secret", "github", "generic", "gitlab", "bitbucket")},
	{group: "build.openshift.io", kind: "Build", resource: "builds",
		fields: listItemFields([]string{"spec", "triggeredBy"}, "secret", "githubWebHook", "genericWebHook", "gitlabWebHook", "bitbucketWebHook")},
}

// listItemFields is the field leaf of every item's members of the list
// at list, one path per member.
func listItemFields(list []string, leaf string, members ...string) [][]string {
	out := make([][]string, 0, len(members))
	for _, member := range members {
		path := append(append(append([]string{}, list...), "*"), member, leaf)
		out = append(out, path)
	}
	return out
}

// credentialKindOf is the credential kind of resource gr, if it is one.
func credentialKindOf(gr schema.GroupResource) (credentialKind, bool) {
	for _, ck := range credentialKinds {
		if ck.group == gr.Group && ck.resource == gr.Resource {
			return ck, true
		}
	}
	return credentialKind{}, false
}

// credentialKindByKind is the credential kind of a manifest's apiVersion
// and kind, if it is one.
func credentialKindByKind(apiVersion, kind string) (credentialKind, bool) {
	group := ""
	if i := strings.IndexByte(apiVersion, '/'); i >= 0 {
		group = apiVersion[:i]
	}
	for _, ck := range credentialKinds {
		if ck.group == group && ck.kind == kind {
			return ck, true
		}
	}
	return credentialKind{}, false
}

// HoldsCredentials reports whether objects of resource gr carry values
// that read paths redact: Secrets and the credential kinds.
func HoldsCredentials(gr schema.GroupResource) bool {
	if gr == (schema.GroupResource{Resource: "secrets"}) {
		return true
	}
	_, ok := credentialKindOf(gr)
	return ok
}

// redactObject redacts the values an object of resource gr must not show:
// a Secret's data, a credential kind's credential fields, and in both the
// last-applied annotation that repeats them.
func redactObject(gr schema.GroupResource, obj map[string]any) {
	if gr == (schema.GroupResource{Resource: "secrets"}) {
		redactSecret(obj)
		return
	}
	if ck, ok := credentialKindOf(gr); ok {
		ck.redact(obj)
	}
}

// RedactManifestObject redacts obj, an object of a manifest read outside
// the runtime's clients (a Helm release's), by its apiVersion and kind
// like every read path, and reports whether it is a kind that holds
// credentials.
func RedactManifestObject(obj map[string]any) bool {
	apiVersion, _ := obj["apiVersion"].(string)
	kind, _ := obj["kind"].(string)
	if apiVersion == "v1" && kind == "Secret" {
		redactSecret(obj)
		return true
	}
	ck, ok := credentialKindByKind(apiVersion, kind)
	if ok {
		ck.redact(obj)
	}
	return ok
}

// CredentialKindNames are the kinds RedactManifestObject redacts, for a
// cheap check whether a manifest can hold any.
func CredentialKindNames() []string {
	names := []string{"Secret"}
	for _, ck := range credentialKinds {
		names = append(names, ck.kind)
	}
	return names
}

// redact replaces the credential values in obj with byte-count markers
// and the last-applied annotation with redactedValue.
func (ck credentialKind) redact(obj map[string]any) {
	for _, field := range ck.fields {
		eachField(obj, field, "", func(_ string, parent map[string]any, key string) {
			if s, ok := parent[key].(string); ok {
				parent[key] = fmt.Sprintf("<%d bytes, redacted>", len(s))
			} else if parent[key] != nil {
				parent[key] = redactedValue
			}
		})
	}
	redactLastApplied(obj)
}

// placeholder is the first credential field of obj (e.g.
// "spec.triggers[0].github.secret") that holds a redaction marker.
func (ck credentialKind) placeholder(obj map[string]any) (string, bool) {
	found := ""
	for _, field := range ck.fields {
		eachField(obj, field, "", func(name string, parent map[string]any, key string) {
			if s, ok := parent[key].(string); ok && found == "" && IsRedacted(s) {
				found = name
			}
		})
		if found != "" {
			return found, true
		}
	}
	return "", false
}

// eachField calls fn for every value present at path below node, with the
// field's dotted name, the map holding it and its key.
func eachField(node any, path []string, name string, fn func(name string, parent map[string]any, key string)) {
	if len(path) == 0 {
		return
	}
	if path[0] == "*" {
		items, _ := node.([]any)
		for i, item := range items {
			eachField(item, path[1:], name+"["+strconv.Itoa(i)+"]", fn)
		}
		return
	}
	m, ok := node.(map[string]any)
	if !ok {
		return
	}
	value, present := m[path[0]]
	if !present {
		return
	}
	child := path[0]
	if name != "" {
		child = name + "." + path[0]
	}
	if len(path) == 1 {
		fn(child, m, path[0])
		return
	}
	eachField(value, path[1:], child, fn)
}

// redactLastApplied replaces kubectl's last-applied annotation, which
// repeats every value of the manifest last applied.
func redactLastApplied(obj map[string]any) {
	meta, _ := obj["metadata"].(map[string]any)
	annotations, _ := meta["annotations"].(map[string]any)
	if _, found := annotations[lastAppliedAnnotation]; found {
		annotations[lastAppliedAnnotation] = redactedValue
	}
}
