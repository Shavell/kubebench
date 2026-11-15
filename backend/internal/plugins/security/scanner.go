package security

import (
	"context"
	"errors"
	"sync"
	"time"

	"kubebench/internal/kube"
)

// scanTimeout bounds one scan, however many requests wait for it.
const scanTimeout = 2 * time.Minute

// errBusy refuses a scan while one with other options runs on the cluster.
var errBusy = errors.New("a scan with other options is already running on this cluster; try again when it has finished")

// scanner runs at most one scan per cluster at a time and keeps each
// cluster's last result. A request for the scan that is running waits for
// it and shares its result, so repeated clicks or several users cost one
// pass over the cluster.
type scanner struct {
	now     func() time.Time
	mu      sync.Mutex
	results map[int64]*result
	running map[int64]*flight
}

// flight is a running scan.
type flight struct {
	opts ScanOptions
	done chan struct{}
	res  *result
	err  error
}

func newScanner(now func() time.Time) *scanner {
	return &scanner{now: now, results: map[int64]*result{}, running: map[int64]*flight{}}
}

// last is the cluster's last scan result, or nil.
func (s *scanner) last(clusterID int64) *result {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.results[clusterID]
}

// run scans rt's cluster with opts, or joins the scan with the same options
// already running there.
func (s *scanner) run(ctx context.Context, rt *kube.Runtime, opts ScanOptions) (*result, error) {
	id := rt.Cluster.ID
	s.mu.Lock()
	if f := s.running[id]; f != nil {
		s.mu.Unlock()
		if f.opts != opts {
			return nil, errBusy
		}
		select {
		case <-f.done:
			return f.res, f.err
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	f := &flight{opts: opts, done: make(chan struct{})}
	s.running[id] = f
	s.mu.Unlock()

	// Others may be waiting for this scan: it outlives the request that
	// started it, up to scanTimeout.
	scanCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), scanTimeout)
	f.res, f.err = s.scan(scanCtx, rt, opts)
	cancel()

	s.mu.Lock()
	delete(s.running, id)
	if f.err == nil {
		s.results[id] = f.res
	}
	s.mu.Unlock()
	close(f.done)
	return f.res, f.err
}

func (s *scanner) scan(ctx context.Context, rt *kube.Runtime, opts ScanOptions) (*result, error) {
	inv, err := collect(ctx, rt.CS)
	if err != nil {
		return nil, err
	}
	// Nothing could be read: the cluster or the credential is the problem,
	// not one permission.
	if len(inv.errs) == len(allKinds) {
		return nil, inv.errs[kindNamespaces]
	}
	res := newScan(inv, opts).run(catalog)
	res.generatedAt = s.now().UTC()
	res.cluster = rt.Cluster.Name
	return res, nil
}
