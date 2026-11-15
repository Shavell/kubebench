package api

import (
	"context"
	"sync"
	"time"

	"kubebench/internal/kube"
	"kubebench/internal/plugin"
)

// Detect results are cached per cluster and plugin (§4.6).
const (
	detectTTL     = time.Minute
	detectTimeout = 10 * time.Second
)

type detectKey struct {
	clusterID int64
	pluginID  string
}

type detectEntry struct {
	status  plugin.Status
	expires time.Time
}

type detectCache struct {
	mu      sync.Mutex
	now     func() time.Time
	timeout time.Duration // detectTimeout; shorter in tests
	entries map[detectKey]detectEntry
}

func newDetectCache(now func() time.Time) *detectCache {
	return &detectCache{now: now, timeout: detectTimeout, entries: map[detectKey]detectEntry{}}
}

// status returns p's Detect result for rt's cluster, cached for a minute.
// Detect runs detached from the request, so a client hanging up cannot
// cache a false "unavailable", and after the capability probe, so a Detect
// based on rt.Caps() sees its result. The wait for the probe and Detect
// each get the cache's timeout; a result obtained after either ran out is
// returned but not cached, since it may only reflect a slow cluster.
func (d *detectCache) status(p plugin.Plugin, rt *kube.Runtime) plugin.Status {
	key := detectKey{clusterID: rt.Cluster.ID, pluginID: p.Manifest().ID}
	d.mu.Lock()
	entry, ok := d.entries[key]
	d.mu.Unlock()
	if ok && d.now().Before(entry.expires) {
		return entry.status
	}
	probed := waitFor(d.timeout, rt.WaitProbed) == nil
	ctx, cancel := context.WithTimeout(context.Background(), d.timeout)
	defer cancel()
	st := p.Detect(ctx, rt)
	if probed && ctx.Err() == nil {
		d.mu.Lock()
		d.entries[key] = detectEntry{status: st, expires: d.now().Add(detectTTL)}
		d.mu.Unlock()
	}
	return st
}

// waitFor runs wait with a context that ends after timeout.
func waitFor(timeout time.Duration, wait func(context.Context) error) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return wait(ctx)
}
