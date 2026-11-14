package kube

import (
	"sync"

	"kubebench/internal/store"
)

// Manager caches one Runtime per cluster id, building them lazily.
type Manager struct {
	mu       sync.Mutex
	build    func(store.Cluster) (*Runtime, error)
	runtimes map[int64]*Runtime
}

// NewManager builds real runtimes that connect to the clusters.
func NewManager() *Manager { return NewManagerWith(newRuntime) }

// NewManagerWith builds runtimes with build, e.g. one returning
// NewRuntimeForTest runtimes around fake clients.
func NewManagerWith(build func(store.Cluster) (*Runtime, error)) *Manager {
	return &Manager{build: build, runtimes: map[int64]*Runtime{}}
}

// Runtime returns the runtime of cluster, building it on first use. When
// the stored record changed since (new credentials, rename, read-only or
// environment flag) the runtime is rebuilt, so Runtime.Cluster always
// matches the database. Such edits are rare admin actions; the price is a
// fresh metrics-server sample history.
func (m *Manager) Runtime(cluster *store.Cluster) (*Runtime, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if rt, ok := m.runtimes[cluster.ID]; ok {
		if rt.Cluster == *cluster {
			return rt, nil
		}
		rt.Close()
		delete(m.runtimes, cluster.ID)
	}
	rt, err := m.build(*cluster)
	if err != nil {
		return nil, err
	}
	m.runtimes[cluster.ID] = rt
	return rt, nil
}

// Drop closes and forgets a cached runtime (e.g. when the cluster is deleted).
func (m *Manager) Drop(clusterID int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if rt, ok := m.runtimes[clusterID]; ok {
		rt.Close()
		delete(m.runtimes, clusterID)
	}
}
