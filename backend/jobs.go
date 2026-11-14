package main

import (
	"context"
	"log"
	"time"

	"kubebench/internal/config"
	"kubebench/internal/store"
)

// runJobs prunes expired sessions hourly and, unless the retention is 0
// (keep forever), audit rows older than the retention daily (§2, §3.5).
func runJobs(ctx context.Context, cfg *config.Config, st *store.Store) {
	go every(ctx, time.Hour, func() {
		if n, err := st.PruneSessions(time.Now()); err != nil {
			log.Printf("[jobs] prune sessions: %v", err)
		} else if n > 0 {
			log.Printf("[jobs] pruned %d expired sessions", n)
		}
	})
	if cfg.AuditRetentionDays == 0 {
		return
	}
	go every(ctx, 24*time.Hour, func() {
		cutoff := time.Now().AddDate(0, 0, -cfg.AuditRetentionDays)
		if n, err := st.PruneAudit(cutoff); err != nil {
			log.Printf("[jobs] prune audit log: %v", err)
		} else if n > 0 {
			log.Printf("[jobs] pruned %d audit rows older than %d days", n, cfg.AuditRetentionDays)
		}
	})
}

// every runs job now and then once per interval until ctx ends.
func every(ctx context.Context, interval time.Duration, job func()) {
	job()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			job()
		}
	}
}
