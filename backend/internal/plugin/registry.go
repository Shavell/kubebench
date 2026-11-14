package plugin

import (
	"fmt"
	"sort"
	"sync"
)

var registry = struct {
	sync.RWMutex
	byID map[string]Plugin
}{byID: map[string]Plugin{}}

// Register adds a built-in plugin. Call it from the plugin package's
// init(); an invalid or duplicate id is a programming error and panics.
func Register(p Plugin) {
	id := p.Manifest().ID
	if !ValidID(id) {
		panic(fmt.Sprintf("plugin: invalid id %q", id))
	}
	registry.Lock()
	defer registry.Unlock()
	if _, dup := registry.byID[id]; dup {
		panic(fmt.Sprintf("plugin: %q registered twice", id))
	}
	registry.byID[id] = p
}

// All returns the registered plugins ordered by id.
func All() []Plugin {
	registry.RLock()
	defer registry.RUnlock()
	out := make([]Plugin, 0, len(registry.byID))
	for _, p := range registry.byID {
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Manifest().ID < out[j].Manifest().ID })
	return out
}

// Get returns the registered plugin with the given id.
func Get(id string) (Plugin, bool) {
	registry.RLock()
	defer registry.RUnlock()
	p, ok := registry.byID[id]
	return p, ok
}
