package ldapauth

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/go-ldap/ldap/v3"
)

// conn is the part of *ldap.Conn the provider uses; tests replace it.
type conn interface {
	Bind(username, password string) error
	Search(req *ldap.SearchRequest) (*ldap.SearchResult, error)
	SearchWithPaging(req *ldap.SearchRequest, pagingSize uint32) (*ldap.SearchResult, error)
	IsClosing() bool
	Close() error
}

const (
	// maxIdle is how many service-account connections are kept open
	// between sign-ins.
	maxIdle = 4
	// maxIdleTime retires an idle connection before the directory drops
	// it (Active Directory closes idle connections after 15 minutes).
	maxIdleTime = 5 * time.Minute
	// maxConcurrent bounds the directory operations in flight, so a burst
	// of sign-ins cannot open unbounded connections to the directory.
	maxConcurrent = 8
)

var errBusy = errors.New("the directory is busy: too many sign-ins in progress")

type idleConn struct {
	c     conn
	since time.Time
}

// pool keeps a few connections bound as the service account, and limits
// how many directory operations run at once.
type pool struct {
	open    func() (conn, error) // dials and binds as the service account
	now     func() time.Time
	timeout time.Duration
	slots   chan struct{}

	mu   sync.Mutex
	idle []idleConn
}

func newPool(open func() (conn, error), timeout time.Duration, now func() time.Time) *pool {
	return &pool{open: open, now: now, timeout: timeout, slots: make(chan struct{}, maxConcurrent)}
}

// acquire waits for a free slot; release gives it back.
func (p *pool) acquire(ctx context.Context) error {
	timer := time.NewTimer(p.timeout)
	defer timer.Stop()
	select {
	case p.slots <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return errBusy
	}
}

func (p *pool) release() { <-p.slots }

// get returns an idle connection that is still usable, or a new one.
func (p *pool) get() (conn, bool, error) {
	p.mu.Lock()
	for len(p.idle) > 0 {
		last := p.idle[len(p.idle)-1]
		p.idle = p.idle[:len(p.idle)-1]
		if last.c.IsClosing() || p.now().Sub(last.since) > maxIdleTime {
			_ = last.c.Close()
			continue
		}
		p.mu.Unlock()
		return last.c, true, nil
	}
	p.mu.Unlock()
	c, err := p.open()
	return c, false, err
}

// put returns a healthy connection to the pool, or closes it when the pool
// is full.
func (p *pool) put(c conn) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.idle) >= maxIdle || c.IsClosing() {
		_ = c.Close()
		return
	}
	p.idle = append(p.idle, idleConn{c: c, since: p.now()})
}

// with runs fn on a service-account connection. A pooled connection the
// directory has dropped in the meantime is replaced once. The caller holds
// a slot.
func (p *pool) with(fn func(conn) error) error {
	for {
		c, pooled, err := p.get()
		if err != nil {
			return err
		}
		err = fn(c)
		if err != nil && ldap.IsErrorWithCode(err, ldap.ErrorNetwork) {
			_ = c.Close()
			if pooled {
				continue
			}
			return err
		}
		p.put(c)
		return err
	}
}
