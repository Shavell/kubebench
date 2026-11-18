package helm

import (
	"bufio"
	"strconv"
	"strings"
	"testing"

	"helm.sh/helm/v3/pkg/releaseutil"
	utilyaml "k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/yaml"

	"kubebench/internal/kube"
)

// consumerDocuments splits a manifest the ways its readers do: Helm's own
// split (install, upgrade, the manifest view of other tools) and the
// Kubernetes YAML reader (kubectl apply -f).
func consumerDocuments(manifest string) []string {
	var docs []string
	for _, doc := range releaseutil.SplitManifests(manifest) {
		docs = append(docs, doc)
	}
	reader := utilyaml.NewYAMLReader(bufio.NewReader(strings.NewReader(manifest)))
	for {
		doc, err := reader.Read()
		if err != nil {
			break
		}
		docs = append(docs, string(doc))
	}
	return docs
}

// leaks reports the first Secret value or credential in obj (or its list
// items) that is not a redaction marker.
func leaks(obj map[string]any) (string, bool) {
	apiVersion, _ := obj["apiVersion"].(string)
	kind, _ := obj["kind"].(string)
	marker := func(v any) bool { s, ok := v.(string); return ok && kube.IsRedacted(s) }
	if apiVersion == "v1" && kind == "Secret" {
		for _, field := range []string{"data", "stringData"} {
			values, _ := obj[field].(map[string]any)
			for key, v := range values {
				if !marker(v) {
					return field + "." + key, true
				}
			}
		}
	}
	if strings.HasPrefix(apiVersion, "route.openshift.io/") && kind == "Route" {
		spec, _ := obj["spec"].(map[string]any)
		tls, _ := spec["tls"].(map[string]any)
		if v, ok := tls["key"]; ok && v != nil && !marker(v) {
			return "spec.tls.key", true
		}
	}
	if apiVersion == "v1" && kind == "Secret" || kind == "Route" || kind == "BuildConfig" || kind == "Build" {
		meta, _ := obj["metadata"].(map[string]any)
		annotations, _ := meta["annotations"].(map[string]any)
		if v, ok := annotations["kubectl.kubernetes.io/last-applied-configuration"]; ok && !marker(v) {
			return "last-applied-configuration", true
		}
	}
	items, _ := obj["items"].([]any)
	for i, item := range items {
		if m, ok := item.(map[string]any); ok {
			if field, found := leaks(m); found {
				return "items[" + strconv.Itoa(i) + "]." + field, true
			}
		}
	}
	return "", false
}

// FuzzRedactManifest checks the Helm manifest redaction: however a reader
// splits and parses the redacted manifest, no Secret data or stringData
// value, Route TLS key or last-applied annotation of those kinds is left.
func FuzzRedactManifest(f *testing.F) {
	for _, seed := range []string{
		"---\n# Source: a/templates/s.yaml\napiVersion: v1\nkind: Secret\nmetadata: {name: s}\ndata: {token: aHVudGVyMg==}\nstringData: {p: hunter2}\n",
		"apiVersion: v1\nkind: ConfigMap\nmetadata: {name: c}\ndata: {note: mentions a Secret}\n---\napiVersion: v1\nkind: Secret\nmetadata: {name: s}\nstringData: {p: hunter2}\n",
		"apiVersion: v1\nkind: ConfigMap\nmetadata: {name: c}\n--- {apiVersion: v1, kind: Secret, metadata: {name: s}, stringData: {p: hunter2}}\n",
		"apiVersion: v1\nkind: \"Secre\\x74\"\nmetadata: {name: s}\nstringData: {p: hunter2}\n",
		"apiVersion: v1\nkind: List\nitems:\n- apiVersion: v1\n  kind: Secret\n  metadata: {name: s}\n  data: {k: aHVudGVyMg==}\n",
		"apiVersion: route.openshift.io/v1\nkind: Route\nmetadata: {name: r}\nspec: {tls: {key: PRIVATE}}\n",
		"apiVersion: v1\nkind: Secret\nmetadata:\n  name: s\n  annotations:\n    kubectl.kubernetes.io/last-applied-configuration: '{\"stringData\":{\"p\":\"x\"}}'\n",
		"apiVersion: v1\nkind: Secret\nstringData: {p: [unclosed\n", "kind: Secret\n...\napiVersion: v1\n", "---\n---\n", "",
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, manifest string) {
		out := redactManifest(manifest)
		for _, doc := range consumerDocuments(out) {
			var obj map[string]any
			if yaml.Unmarshal([]byte(doc), &obj) != nil {
				continue
			}
			if field, found := leaks(obj); found {
				t.Fatalf("%s survived redaction:\n--- manifest\n%s\n--- redacted\n%s", field, manifest, out)
			}
		}
	})
}
