package evidence

import (
	"context"
	"sync"
	"testing"
	"time"
)

type recorded struct {
	tenant  string
	records []Evidence
}

type fakeSink struct {
	mu      sync.Mutex
	batches []recorded
	fail    bool
}

func (f *fakeSink) AppendBatch(_ context.Context, tenantID string, records []Evidence) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.fail {
		return context.DeadlineExceeded
	}
	f.batches = append(f.batches, recorded{tenant: tenantID, records: append([]Evidence(nil), records...)})
	return nil
}

func (f *fakeSink) Recent(context.Context, string, int, time.Time) ([]Evidence, error) {
	return nil, nil
}

func (f *fakeSink) stored() []Evidence {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []Evidence
	for _, b := range f.batches {
		out = append(out, b.records...)
	}
	return out
}

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

func TestWriterBatchesRecordsToTheSink(t *testing.T) {
	sink := &fakeSink{}
	w := NewWriter(sink, WriterOptions{QueueSize: 64, BatchSize: 3, FlushInterval: 10 * time.Millisecond})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go w.Run(ctx)

	for i := 0; i < 5; i++ {
		w.enqueue("tenant-a", Evidence{JA4: "fp"})
	}
	waitFor(t, "5 records written", func() bool { _, written, _, _ := w.Stats(); return written == 5 })

	if got := len(sink.stored()); got != 5 {
		t.Fatalf("sink holds %d records, want 5", got)
	}
}

func TestWriterDropsInsteadOfBlockingWhenFull(t *testing.T) {
	w := NewWriter(&fakeSink{}, WriterOptions{QueueSize: 1, BatchSize: 1000, FlushInterval: time.Hour})
	// Run is deliberately not started: the queue fills and stays full.
	for i := 0; i < 3; i++ {
		w.enqueue("tenant-a", Evidence{JA4: "fp"})
	}
	queued, _, dropped, _ := w.Stats()
	if queued != 1 {
		t.Fatalf("queued = %d, want 1 held in the queue", queued)
	}
	if dropped != 2 {
		t.Fatalf("dropped = %d, want 2 dropped rather than blocking", dropped)
	}
}

func TestWriterFlushesWhatIsBufferedOnShutdown(t *testing.T) {
	sink := &fakeSink{}
	w := NewWriter(sink, WriterOptions{QueueSize: 64, BatchSize: 100, FlushInterval: time.Hour})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { w.Run(ctx); close(done) }()

	w.enqueue("tenant-a", Evidence{JA4: "fp1"})
	w.enqueue("tenant-a", Evidence{JA4: "fp2"})
	cancel()
	<-done

	if got := len(sink.stored()); got != 2 {
		t.Fatalf("shutdown flushed %d records, want 2", got)
	}
}

func TestWriterCountsFailedBatches(t *testing.T) {
	sink := &fakeSink{fail: true}
	w := NewWriter(sink, WriterOptions{QueueSize: 8, BatchSize: 1, FlushInterval: time.Hour})
	ctx, cancel := context.WithCancel(context.Background())
	go w.Run(ctx)
	w.enqueue("tenant-a", Evidence{JA4: "fp"})

	waitFor(t, "failure counted", func() bool { _, _, _, failed := w.Stats(); return failed == 1 })
	cancel()
}

func TestNilWriterStatsAreSafe(t *testing.T) {
	var w *Writer
	if queued, written, dropped, failed := w.Stats(); queued != 0 || written != 0 || dropped != 0 || failed != 0 {
		t.Fatalf("nil writer stats = %d,%d,%d,%d", queued, written, dropped, failed)
	}
	w.enqueue("tenant-a", Evidence{}) // must not panic
}

func TestTrailPersistsRecordsWhenEnabled(t *testing.T) {
	sink := &fakeSink{}
	w := NewWriter(sink, WriterOptions{QueueSize: 16, BatchSize: 2, FlushInterval: 5 * time.Millisecond})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go w.Run(ctx)

	tr := NewTrail()
	tr.EnablePersistence("tenant-a", w)
	tr.Record(Evidence{JA4: "fp1"})
	tr.Record(Evidence{JA4: "fp2"})

	waitFor(t, "trail records persisted", func() bool { return len(sink.stored()) == 2 })
	stored := sink.stored()
	seen := map[string]bool{}
	for _, e := range stored {
		if e.Time.IsZero() {
			t.Fatal("persisted record has no timestamp")
		}
		seen[e.JA4] = true
	}
	if !seen["fp1"] || !seen["fp2"] {
		t.Fatalf("persisted records missing: %+v", seen)
	}
}

func TestTrailWithoutPersistenceDoesNotQueue(t *testing.T) {
	// A trail with no writer must behave exactly as before; this guards
	// against persistence leaking onto the default path.
	tr := NewTrail()
	tr.Record(Evidence{JA4: "fp1"})
	if got := len(tr.Recent(0)); got != 1 {
		t.Fatalf("recent = %d, want 1", got)
	}
}
