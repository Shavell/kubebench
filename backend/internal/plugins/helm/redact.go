package helm

import (
	"regexp"
	"strings"

	"sigs.k8s.io/yaml"

	"kubebench/internal/kube"
)

// documentSeparator starts a new document: "---" at column 0, where Helm
// splits a manifest (releaseutil.SplitManifests), the widest split of its
// readers. A YAML parser also starts a document with content on the
// marker's line ("--- {kind: Secret, …}"); splitting only at bare "---"
// lines would leave such a document inside the previous one, where parsing
// that one's text never sees it. Inside a document "---" at column 0
// cannot be content: block scalars of a Kubernetes object are always
// indented.
var documentSeparator = regexp.MustCompile(`(?m)^---[ \t]*`)

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
		writeDocument(&b, manifest[start:sep[0]])
		b.WriteString(manifest[sep[0]:sep[1]])
		start = sep[1]
	}
	writeDocument(&b, manifest[start:])
	return b.String()
}

// writeDocument writes doc redacted. A replaced document that started on
// its separator's line starts on a line of its own.
func writeDocument(b *strings.Builder, doc string) {
	out := redactDocument(doc)
	if out != doc && b.Len() > 0 && !strings.HasSuffix(b.String(), "\n") {
		b.WriteByte('\n')
	}
	b.WriteString(out)
}

// mentionsCredentialKind is a cheap test whether text can hold an object
// of a kind that is redacted. It must not miss one: a kind can also be
// spelled with escapes in a double-quoted scalar ("Secre\x74") or as a
// tagged value (!!binary U2VjcmV0), so text with a backslash or a tag
// indicator is parsed too.
func mentionsCredentialKind(text string) bool {
	if strings.ContainsAny(text, `\!`) {
		return true
	}
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
	// Readers disagree on a document's text: Helm's SplitManifests trims
	// it, kubectl's YAML reader does not, and a stray leading indent makes
	// the YAML parser stop after the first key without an error. Redact
	// when any reading finds a credential kind; withhold when none parses.
	var obj map[string]any
	parsed := false
	for _, text := range []string{doc, strings.TrimSpace(doc)} {
		var candidate map[string]any
		if yaml.Unmarshal([]byte(text), &candidate) != nil {
			continue
		}
		parsed = true
		if redactObject(candidate) {
			obj = candidate
			break
		}
	}
	if !parsed {
		return leadingComments(doc) + withheldDocument
	}
	if obj == nil {
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
