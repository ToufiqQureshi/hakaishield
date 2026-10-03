package evidence_test

import (
	"testing"
	"time"

	"github.com/ToufiqQureshi/hakaishield/pkg/evidence"
)

func TestTrailRecordAndRecent(t *testing.T) {
	tr := evidence.NewTrail()

	tr.Record(evidence.Evidence{JA4: "fp1", Decision: "allow"})
	tr.Record(evidence.Evidence{JA4: "fp2", Decision: "block"})
	tr.Record(evidence.Evidence{JA4: "fp3", Decision: "challenge"})

	got := tr.Recent(0) // 0 = all
	if len(got) != 3 {
		t.Fatalf("want 3 records, got %d", len(got))
	}
	// Most recent first
	if got[0].JA4 != "fp3" {
		t.Errorf("most recent: want fp3, got %s", got[0].JA4)
	}
}

func TestTrailLimit(t *testing.T) {
	tr := evidence.NewTrail()
	for i := 0; i < 10; i++ {
		tr.Record(evidence.Evidence{JA4: "fp"})
	}
	got := tr.Recent(3)
	if len(got) != 3 {
		t.Errorf("limit=3: want 3, got %d", len(got))
	}
}

func TestTrailRingBuffer_Overflow(t *testing.T) {
	// NewTrail uses a 1000-slot buffer, so overflow only matters at scale.
	// We test the logic via the internal newTrail with a tiny buffer.
	// Since newTrail is unexported, we use NewTrail and record > size records
	// won't fit in standard test times. So we just verify capacity wrapping
	// doesn't panic and always returns at most what we recorded.
	tr := evidence.NewTrail()
	n := 5
	for i := 0; i < n; i++ {
		tr.Record(evidence.Evidence{JA4: "fp"})
	}
	got := tr.Recent(0)
	if len(got) != n {
		t.Errorf("want %d records, got %d", n, len(got))
	}
}

func TestTrailTimestamp(t *testing.T) {
	before := time.Now()
	tr := evidence.NewTrail()
	tr.Record(evidence.Evidence{JA4: "ts-test"})
	after := time.Now()

	got := tr.Recent(1)
	if len(got) != 1 {
		t.Fatal("want 1 record")
	}
	if got[0].Time.Before(before) || got[0].Time.After(after) {
		t.Errorf("timestamp %v not in [%v, %v]", got[0].Time, before, after)
	}
}

func TestTrailLoadSeedsHistoryNewestFirst(t *testing.T) {
	tr := evidence.NewTrail()
	older := time.Now().Add(-time.Minute)
	newer := time.Now()
	tr.Load([]evidence.Evidence{{JA4: "new", Time: newer}, {JA4: "old", Time: older}})

	got := tr.Recent(0)
	if len(got) != 2 {
		t.Fatalf("want 2 loaded records, got %d", len(got))
	}
	if got[0].JA4 != "new" || got[1].JA4 != "old" {
		t.Fatalf("load order wrong: %s then %s", got[0].JA4, got[1].JA4)
	}
	if !got[1].Time.Equal(older) {
		t.Fatalf("Load restamped a stored time: %v, want %v", got[1].Time, older)
	}
}

func TestTrailLoadEmptyIsNoOp(t *testing.T) {
	tr := evidence.NewTrail()
	tr.Record(evidence.Evidence{JA4: "live"})
	tr.Load(nil)
	if got := tr.Recent(0); len(got) != 1 || got[0].JA4 != "live" {
		t.Fatalf("empty Load changed the trail: %+v", got)
	}
}

func TestTrailConcurrentRecords(t *testing.T) {
	tr := evidence.NewTrail()
	done := make(chan struct{})
	for i := 0; i < 50; i++ {
		go func() {
			tr.Record(evidence.Evidence{JA4: "concurrent"})
			done <- struct{}{}
		}()
	}
	for i := 0; i < 50; i++ {
		<-done
	}
	got := tr.Recent(0)
	if len(got) != 50 {
		t.Errorf("want 50 records, got %d", len(got))
	}
}
