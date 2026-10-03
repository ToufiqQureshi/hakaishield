package db

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/ToufiqQureshi/hakaishield/pkg/evidence"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestEvidenceStoreRoundTrip(t *testing.T) {
	raw := os.Getenv("HAKAISHIELD_TEST_DATABASE_URL")
	if raw == "" {
		t.Skip("set HAKAISHIELD_TEST_DATABASE_URL for the real Postgres durable-evidence test")
	}
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(u.Host, "localhost") && !strings.Contains(u.Host, "127.0.0.1") && !strings.Contains(strings.ToLower(u.Host+u.Path), "test") && os.Getenv("HAKAISHIELD_ALLOW_REMOTE_TEST_DATABASE") != "1" {
		t.Skip("test database URL safety gate")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	admin, err := pgxpool.New(ctx, raw)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	schema := fmt.Sprintf("hs_evidence_test_%d", time.Now().UnixNano())
	if _, err := admin.Exec(ctx, `CREATE SCHEMA `+schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanup, stop := context.WithTimeout(context.Background(), 5*time.Second)
		defer stop()
		_, _ = admin.Exec(cleanup, `DROP SCHEMA IF EXISTS `+schema+` CASCADE`)
	})

	cfg, err := pgxpool.ParseConfig(raw)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ConnConfig.RuntimeParams == nil {
		cfg.ConnConfig.RuntimeParams = map[string]string{}
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err := InitSchema(ctx, pool); err != nil {
		t.Fatal(err)
	}
	old := DB
	DB = pool
	t.Cleanup(func() { DB = old })

	store := EvidenceStore{}
	now := time.Now().UTC()
	if err := store.AppendBatch(ctx, "t1", []evidence.Evidence{
		{Time: now.Add(-time.Minute), JA4: "old", Decision: "allow", Enforced: true, Signals: []string{"none"}},
		{Time: now, JA4: "new", Decision: "block", Score: 100, Signals: []string{"ja4_blocklist"}},
	}); err != nil {
		t.Fatalf("append: %v", err)
	}
	// A different tenant must never show up in t1's history.
	if err := store.AppendBatch(ctx, "t2", []evidence.Evidence{{Time: now, JA4: "other"}}); err != nil {
		t.Fatalf("append t2: %v", err)
	}

	got, err := store.Recent(ctx, "t1", 10, now.Add(-time.Hour))
	if err != nil {
		t.Fatalf("recent: %v", err)
	}
	if len(got) != 2 || got[0].JA4 != "new" || got[1].JA4 != "old" {
		t.Fatalf("recent = %+v, want new then old for t1 only", got)
	}
	if len(got[0].Signals) != 1 || got[0].Signals[0] != "ja4_blocklist" || got[0].Score != 100 {
		t.Fatalf("record did not round-trip: %+v", got[0])
	}

	// The window and limit must both apply.
	if older, err := store.Recent(ctx, "t1", 10, now.Add(-30*time.Second)); err != nil || len(older) != 1 {
		t.Fatalf("windowed recent = %+v err=%v, want just the new record", older, err)
	}
	if limited, err := store.Recent(ctx, "t1", 1, now.Add(-time.Hour)); err != nil || len(limited) != 1 || limited[0].JA4 != "new" {
		t.Fatalf("limited recent = %+v err=%v", limited, err)
	}
}
