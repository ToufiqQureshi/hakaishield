package evidence

import (
	"context"
	"log"
	"sync/atomic"
	"time"
)

// Sink is a durable store for evidence records. It is declared here, next
// to the hot path that produces them, so pkg/evidence never has to import
// the database package.
type Sink interface {
	// AppendBatch persists a tenant's records. Records arrive in the order
	// they were recorded.
	AppendBatch(ctx context.Context, tenantID string, records []Evidence) error
	// Recent returns a tenant's newest records down to since, newest first.
	Recent(ctx context.Context, tenantID string, limit int, since time.Time) ([]Evidence, error)
}

// WriterOptions tune the write-behind buffer. Zero values take the
// defaults below.
type WriterOptions struct {
	QueueSize     int
	BatchSize     int
	FlushInterval time.Duration
}

const (
	defaultQueueSize     = 4096
	defaultBatchSize     = 200
	defaultFlushInterval = time.Second
)

// Writer makes evidence durable without putting a database call in the
// request path. Record() hands a copy to a bounded in-memory queue; a
// background goroutine drains it into the Sink in batches. When the queue
// is full the record is dropped and counted rather than blocking a
// visitor — losing a line of history is better than a slow customer site.
type Writer struct {
	sink    Sink
	ch      chan queued
	opts    WriterOptions
	queued  atomic.Int64
	dropped atomic.Int64
	written atomic.Int64
	failed  atomic.Int64
}

type queued struct {
	tenantID string
	evidence Evidence
}

func NewWriter(sink Sink, opts WriterOptions) *Writer {
	if opts.QueueSize <= 0 {
		opts.QueueSize = defaultQueueSize
	}
	if opts.BatchSize <= 0 {
		opts.BatchSize = defaultBatchSize
	}
	if opts.FlushInterval <= 0 {
		opts.FlushInterval = defaultFlushInterval
	}
	return &Writer{sink: sink, ch: make(chan queued, opts.QueueSize), opts: opts}
}

// enqueue never blocks. It is called from the proxy's request path.
func (w *Writer) enqueue(tenantID string, e Evidence) {
	if w == nil || w.sink == nil {
		return
	}
	select {
	case w.ch <- queued{tenantID: tenantID, evidence: e}:
		w.queued.Add(1)
	default:
		w.dropped.Add(1)
	}
}

// Run drains the queue until ctx is cancelled, then writes what is left.
// It is meant to run in its own goroutine for the life of the process.
func (w *Writer) Run(ctx context.Context) {
	if w == nil || w.sink == nil {
		return
	}
	ticker := time.NewTicker(w.opts.FlushInterval)
	defer ticker.Stop()

	pending := make([]queued, 0, w.opts.BatchSize)
	for {
		select {
		case <-ctx.Done():
			pending = w.drain(pending)
			w.write(ctx, pending)
			return
		case q := <-w.ch:
			pending = append(pending, q)
			if len(pending) >= w.opts.BatchSize {
				w.write(ctx, pending)
				pending = pending[:0]
			}
		case <-ticker.C:
			if len(pending) > 0 {
				w.write(ctx, pending)
				pending = pending[:0]
			}
		}
	}
}

// drain moves everything currently buffered into pending, up to the point
// the channel runs dry, so shutdown does not lose committed records.
func (w *Writer) drain(pending []queued) []queued {
	for {
		select {
		case q := <-w.ch:
			pending = append(pending, q)
		default:
			return pending
		}
	}
}

// write groups a batch by tenant and appends each group. A failed batch is
// counted and logged, never retried here: retrying in the writer would
// stall the queue behind a database that is already unhappy.
func (w *Writer) write(ctx context.Context, batch []queued) {
	if len(batch) == 0 {
		return
	}
	byTenant := make(map[string][]Evidence)
	for _, q := range batch {
		byTenant[q.tenantID] = append(byTenant[q.tenantID], q.evidence)
	}
	for tenantID, records := range byTenant {
		if err := w.sink.AppendBatch(ctx, tenantID, records); err != nil {
			w.failed.Add(int64(len(records)))
			log.Printf("hakaishield: durable evidence write for tenant %q failed, %d records dropped: %v", tenantID, len(records), err)
			continue
		}
		w.written.Add(int64(len(records)))
	}
}

// Stats reports queue health for the observability endpoint.
func (w *Writer) Stats() (queued, written, dropped, failed int64) {
	if w == nil {
		return 0, 0, 0, 0
	}
	return w.queued.Load(), w.written.Load(), w.dropped.Load(), w.failed.Load()
}
