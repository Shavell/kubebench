package helm

import (
	"regexp"
	"strings"

	"sigs.k8s.io/yaml"

	"kubebench/internal/kube"
)

// documentSeparator is a YAML document separator line. Inside a document
// "---" at column 0 cannot be content: block scalars of a Kubernetes
// object are always indented.
var documentSeparator = regexp.MustCompile(`(?m)^---[ \t]*$`)

// withheldDocument replaces a document that may hold credentials but cannot
// be parsed, so they cannot be redacted.
const withheldDocument = "# This document is not shown: it could not be parsed to hide Secret values and credentials.\n"

// redactManifest hides the values of every Secret in a rendered Helm
// manifest the way every other Secret read path does (§3.1): data and
// stringData values become byte-count markers, keys stay visible. The
// credentials of the other kinds that hold some (an OpenShift Route's TLS
// key, webhook secrets; kube.RedactManifestObject) are hidden the same
// way. Other documents, and the "# Source:" comments Helm writes, are kept
// verbatim; a redacted document is re-serialized, so its keys come out
// sorted.
func redactManifest(manifest string) string {
	if !mentionsCredentialKind(manifest) {
		return manifest
	}
	var b strings.Builder
	start := 0
	for _, sep := range documentSeparator.FindAllStringIndex(manifest, -1) {
		b.WriteString(redactDocument(manifest[start:sep[0]]))
		b.WriteString(manifest[sep[0]:sep[1]])
		start = sep[1]
	}
	b.WriteString(redactDocument(manifest[start:]))
	return b.String()
}

// mentionsCredentialKind is a cheap test whether text can hold an object
// of a kind that is redacted.
func mentionsCredentialKind(text string) bool {
	for _, kind := range kube.CredentialKindNames() {
		if strings.Contains(text, kind) {
			return true
		}
	}
	return false
}

// redactDocument redacts one YAML document if it is of a kind that holds
// credentials or a list holding such objects.
func redactDocument(doc string) string {
	if !mentionsCredentialKind(doc) {
		return doc
	}
	var obj map[string]any
	if err := yaml.Unmarshal([]byte(doc), &obj); err != nil {
		return leadingComments(doc) + withheldDocument
	}
	if !redactObject(obj) {
		return doc
	}
	out, err := yaml.Marshal(obj)
	if err != nil {
		return leadingComments(doc) + withheldDocument
	}
	return leadingComments(doc) + string(out)
}

// redactObject redacts obj if it is of a kind that holds credentials, or
// those among its items if it is a list, and reports whether anything was.
func redactObject(obj map[string]any) bool {
	if kube.RedactManifestObject(obj) {
		return true
	}
	items, ok := obj["items"].([]any)
	if !ok {
		return false
	}
	found := false
	for _, item := range items {
		if m, ok := item.(map[string]any); ok && redactObject(m) {
			found = true
		}
	}
	return found
}

// leadingComments is the blank and comment lines a document starts with
// (Helm's "# Source: chart/templates/x.yaml"), which re-serializing would
// drop.
func leadingComments(doc string) string {
	end := 0
	for end < len(doc) {
		next := strings.IndexByte(doc[end:], '\n')
		if next < 0 {
			next = len(doc) - end
		} else {
			next++
		}
		line := strings.TrimSpace(doc[end : end+next])
		if line != "" && !strings.HasPrefix(line, "#") {
			break
		}
		end += next
	}
	return doc[:end]
}
