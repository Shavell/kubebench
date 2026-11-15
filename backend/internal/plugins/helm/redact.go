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

// withheldDocument replaces a document that may be a Secret but cannot be
// parsed, so its values cannot be redacted.
const withheldDocument = "# This document is not shown: it could not be parsed to hide Secret values.\n"

// redactManifest hides the values of every Secret in a rendered Helm
// manifest the way every other Secret read path does (§3.1): data and
// stringData values become byte-count markers, keys stay visible. Other
// documents, and the "# Source:" comments Helm writes, are kept verbatim;
// a redacted Secret is re-serialized, so its keys come out sorted.
func redactManifest(manifest string) string {
	if !strings.Contains(manifest, "Secret") {
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

// redactDocument redacts one YAML document if it is a Secret or a list
// holding Secrets.
func redactDocument(doc string) string {
	if !strings.Contains(doc, "Secret") {
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

// redactObject redacts obj if it is a core Secret, or the Secrets among
// its items if it is a list, and reports whether anything was a Secret.
func redactObject(obj map[string]any) bool {
	if obj["apiVersion"] == "v1" && obj["kind"] == "Secret" {
		kube.RedactSecret(obj)
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
