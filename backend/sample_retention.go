package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/ToufiqQureshi/hakaishield/pkg/db"
)

const (
	sampleRetentionBatch = 1000
	maxRetentionBatches  = 10
)

// pruneSamples removes at most 10,000 old rows per run, keeping a large
// backlog from monopolizing Postgres while normal traffic is served.
func pruneSamples(ctx context.Context, now time.Time, days int, remove func(context.Context, time.Time) (int64, error)) (int64, error) {
	if days < 1 || days > 365 {
		return 0, fmt.Errorf("sample retention days must be between 1 and 365")
	}
	cutoff := now.AddDate(0, 0, -days)
	var total int64
	for range maxRetentionBatches {
		if err := ctx.Err(); err != nil {
			return total, err
		}
		n, err := remove(ctx, cutoff)
		if err != nil {
			return total, err
		}
		total += n
		if n < sampleRetentionBatch {
			break
		}
	}
	return total, nil
}

// runEvidenceRetention prunes durable evidence the same way, at startup
// and hourly, off the request path.
func runEvidenceRetention(ctx context.Context, days int) {
	prune := func() {
		runCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		n, err := pruneSamples(runCtx, time.Now().UTC(), days, db.DeleteEvidenceBefore)
		if err != nil && ctx.Err() == nil {
			log.Printf("hakaishield: evidence retention failed: %v", err)
		} else if n > 0 {
			log.Printf("hakaishield: deleted %d evidence records older than %d days", n, days)
		}
	}
	prune()
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			prune()
		}
	}
}

// runSampleRetention prunes at startup and hourly. It runs only off the
// request path; a failed run is logged and retried at the next tick.
func runSampleRetention(ctx context.Context, days int) {
	prune := func() {
		runCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		n, err := pruneSamples(runCtx, time.Now().UTC(), days, db.DeleteSamplesBefore)
		if err != nil && ctx.Err() == nil {
			log.Printf("hakaishield: sample retention failed: %v", err)
		} else if n > 0 {
			log.Printf("hakaishield: deleted %d training samples older than %d days", n, days)
		}
	}
	prune()
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			prune()
		}
	}
}
