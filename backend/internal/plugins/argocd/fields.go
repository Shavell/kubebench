package argocd

// Defensive readers for unstructured Argo CD objects. Any field may be
// missing or, in an object written by hand, have an unexpected type; the
// readers then return the zero value instead of failing the whole list.

// field returns the value at path, or nil.
func field(m map[string]any, path ...string) any {
	var v any = m
	for _, key := range path {
		obj, ok := v.(map[string]any)
		if !ok {
			return nil
		}
		v = obj[key]
	}
	return v
}

func str(m map[string]any, path ...string) string {
	s, _ := field(m, path...).(string)
	return s
}

func boolean(m map[string]any, path ...string) bool {
	b, _ := field(m, path...).(bool)
	return b
}

func object(m map[string]any, path ...string) map[string]any {
	obj, _ := field(m, path...).(map[string]any)
	return obj
}

// objects returns the maps of the list at path, skipping other items.
func objects(m map[string]any, path ...string) []map[string]any {
	items, _ := field(m, path...).([]any)
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		if obj, ok := item.(map[string]any); ok {
			out = append(out, obj)
		}
	}
	return out
}

// stringList returns the strings of the list at path, skipping other items.
func stringList(m map[string]any, path ...string) []string {
	items, _ := field(m, path...).([]any)
	out := make([]string, 0, len(items))
	for _, item := range items {
		if s, ok := item.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

// integer reads a JSON number, which decodes as int64 from the API server
// and as float64 from encoding/json.
func integer(m map[string]any, path ...string) (int64, bool) {
	switch n := field(m, path...).(type) {
	case int64:
		return n, true
	case int:
		return int64(n), true
	case float64:
		return int64(n), n == float64(int64(n))
	}
	return 0, false
}
