package tenant_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/ToufiqQureshi/hakaishield/pkg/config"
	"github.com/ToufiqQureshi/hakaishield/pkg/evidence"
	"github.com/ToufiqQureshi/hakaishield/pkg/tenant"
)

type recordingSink struct {
	mu      sync.Mutex
	records []evidence.Evidence
}

func (r *recordingSink) AppendBatch(_ context.Context, _ string, records []evidence.Evidence) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.records = append(r.records, records...)
	return nil
}

func (r *recordingSink) Recent(context.Context, string, int, time.Time) ([]evidence.Evidence, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]evidence.Evidence(nil), r.records...), nil
}

func (r *recordingSink) stored() []evidence.Evidence {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]evidence.Evidence(nil), r.records...)
}

func TestNewTenantTrailPersistsWhenWriterIsSet(t *testing.T) {
	sink := &recordingSink{}
	writer := evidence.NewWriter(sink, evidence.WriterOptions{QueueSize: 16, BatchSize: 2, FlushInterval: 5 * time.Millisecond})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go writer.Run(ctx)

	store := tenant.NewStore()
	store.EvidenceWriter = writer
	if err := store.Add("t1", tenant.TenantConfig{Mode: config.ModeEnforce}, []string{"a.example"}, nil); err != nil {
		t.Fatalf("add tenant: %v", err)
	}

	tn, err := store.GetByID("t1")
	if err != nil {
		t.Fatalf("get tenant: %v", err)
	}
	tn.Trail.Record(evidence.Evidence{JA4: "fp1"})

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && len(sink.stored()) == 0 {
		time.Sleep(2 * time.Millisecond)
	}
	if got := len(sink.stored()); got != 1 {
		t.Fatalf("persisted %d records, want 1", got)
	}
}

func TestHydrateEvidenceSeedsTheTrailFromTheSink(t *testing.T) {
	store := tenant.NewStore()
	if err := store.Add("t1", tenant.TenantConfig{Mode: config.ModeEnforce}, []string{"a.example"}, nil); err != nil {
		t.Fatalf("add tenant: %v", err)
	}
	store.EvidenceSink = &recordingSink{records: []evidence.Evidence{
		{JA4: "new", Time: time.Now()},
		{JA4: "old", Time: time.Now().Add(-time.Minute)},
	}}

	store.HydrateEvidence(context.Background())

	tn, _ := store.GetByID("t1")
	got := tn.Trail.Recent(0)
	if len(got) != 2 || got[0].JA4 != "new" || got[1].JA4 != "old" {
		t.Fatalf("hydrated trail = %+v, want new then old", got)
	}
}

func TestHydrateEvidenceWithoutSinkKeepsTrailEmpty(t *testing.T) {
	store := tenant.NewStore()
	if err := store.Add("t1", tenant.TenantConfig{Mode: config.ModeEnforce}, []string{"a.example"}, nil); err != nil {
		t.Fatalf("add tenant: %v", err)
	}
	tn, _ := store.GetByID("t1")
	tn.Trail.Record(evidence.Evidence{JA4: "live"})

	store.HydrateEvidence(context.Background())

	if got := tn.Trail.Recent(0); len(got) != 1 || got[0].JA4 != "live" {
		t.Fatalf("trail changed without a sink: %+v", got)
	}
}
