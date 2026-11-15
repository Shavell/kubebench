package kube

import (
	"errors"
	"sync"

	"kubebench/internal/store"
)

// ErrClusterRemoved is Manager.Runtime's error for a cluster that was
// removed (Manager.Remove) while a request still held its record.
var ErrClusterRemoved = errors.New("the cluster was removed")

// Manager caches one Runtime per cluster id, building them lazily.
//
// Closing a runtime waits for its background work (the watch hub's
// goroutines, which may sit in a call to an unreachable API server), so a
// runtime leaves the cache at once but is closed in the background: one
// cluster's slow shutdown must hold up neither the request that replaced
// or removed it nor the requests for every other cluster. Requests still
// holding it keep working (Close is idempotent).
type Manager struct {
	mu       sync.Mutex
	build    func(store.Cluster) (*Runtime, error)
	runtimes map[int64]*Runtime
	// removed holds the ids of deleted clusters. Cluster ids are never
	// reused, so a request that read a record before its deletion cannot
	// build (and leave behind) a runtime that nothing would ever close.
	removed map[int64]struct{}
}

// NewManager builds real runtimes that connect to the clusters.
func NewManager() *Manager { return NewManagerWith(newRuntime) }

// NewManagerWith builds runtimes with build, e.g. one returning
// NewRuntimeForTest runtimes around fake clients.
func NewManagerWith(build func(store.Cluster) (*Runtime, error)) *Manager {
	return &Manager{build: build, runtimes: map[int64]*Runtime{}, removed: map[int64]struct{}{}}
}

// Runtime returns the runtime of cluster, building it on first use. When
// the stored record changed since (new credentials, rename, read-only or
// environment flag) the runtime is rebuilt, so Runtime.Cluster always
// matches the database. Such edits are rare admin actions; the price is a
// fresh metrics-server sample history. A removed cluster gets
// ErrClusterRemoved.
func (m *Manager) Runtime(cluster *store.Cluster) (*Runtime, error) {
	m.mu.Lock()
	if _, gone := m.removed[cluster.ID]; gone {
		m.mu.Unlock()
		return nil, ErrClusterRemoved
	}
	stale, ok := m.runtimes[cluster.ID]
	if ok && stale.Cluster == *cluster {
		m.mu.Unlock()
		return stale, nil
	}
	delete(m.runtimes, cluster.ID)
	rt, err := m.build(*cluster)
	if err == nil {
		m.runtimes[cluster.ID] = rt
	}
	m.mu.Unlock()
	if stale != nil {
		go stale.Close()
	}
	return rt, err
}

// Drop forgets the cached runtime of a cluster whose record
// changed (e.g. new credentials) and closes it, so that its background work
// stops using the old record; the next request builds a fresh one.
func (m *Manager) Drop(clusterID int64) {
	m.mu.Lock()
	rt := m.runtimes[clusterID]
	delete(m.runtimes, clusterID)
	m.mu.Unlock()
	if rt != nil {
		go rt.Close()
	}
}

// Remove forgets and closes the runtime of a deleted cluster for good:
// Runtime refuses to build another one for its id.
func (m *Manager) Remove(clusterID int64) {
	m.mu.Lock()
	m.removed[clusterID] = struct{}{}
	m.mu.Unlock()
	m.Drop(clusterID)
}

// Peek returns the cached runtime of cluster when one is built for its
// current record, without building one (e.g. to show what is known about
// clusters nobody uses right now).
func (m *Manager) Peek(cluster *store.Cluster) *Runtime {
	m.mu.Lock()
	defer m.mu.Unlock()
	if rt, ok := m.runtimes[cluster.ID]; ok && rt.Cluster == *cluster {
		return rt
	}
	return nil
}
