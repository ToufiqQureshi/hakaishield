package api

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/ToufiqQureshi/hakaishield/pkg/db"
	"github.com/jackc/pgx/v5/pgxpool"
)

// TestDomainOnboardingIntegration exercises the real SQL behind self-serve
// domain onboarding: host uniqueness, owner scoping, and the one-way
// pending -> verified transition that clears the token.
func TestDomainOnboardingIntegration(t *testing.T) {
	rawURL := os.Getenv("HAKAISHIELD_TEST_DATABASE_URL")
	if rawURL == "" {
		t.Skip("set HAKAISHIELD_TEST_DATABASE_URL to run the real Postgres domain-onboarding integration test")
	}
	if !safeTestDatabaseURL(rawURL) && os.Getenv("HAKAISHIELD_ALLOW_REMOTE_TEST_DATABASE") != "1" {
		t.Skip("HAKAISHIELD_TEST_DATABASE_URL must point at localhost or contain 'test'")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	adminPool, err := pgxpool.New(ctx, rawURL)
	if err != nil {
		t.Fatalf("connect admin pool: %v", err)
	}
	defer adminPool.Close()

	schema := fmt.Sprintf("hs_domains_%d", time.Now().UnixNano())
	if _, err := adminPool.Exec(ctx, `CREATE SCHEMA `+schema); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		_, _ = adminPool.Exec(cleanupCtx, `DROP SCHEMA IF EXISTS `+schema+` CASCADE`)
	})

	cfg, err := pgxpool.ParseConfig(rawURL)
	if err != nil {
		t.Fatalf("parse test db url: %v", err)
	}
	if cfg.ConnConfig.RuntimeParams == nil {
		cfg.ConnConfig.RuntimeParams = map[string]string{}
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatalf("connect schema pool: %v", err)
	}
	defer pool.Close()

	if _, err := pool.Exec(ctx, integrationSchemaSQL); err != nil {
		t.Fatalf("create integration tables: %v", err)
	}
	if err := db.InitSchema(ctx, pool); err != nil {
		t.Fatalf("apply application schema: %v", err)
	}

	oldDB := db.DB
	db.DB = pool
	t.Cleanup(func() { db.DB = oldDB })

	created, err := db.CreateDomain(ctx, "user-a", "shop.example.com", "https://origin.example")
	if err != nil {
		t.Fatalf("create domain: %v", err)
	}
	if created.Status != db.StatusPendingVerification || created.VerificationToken == "" {
		t.Fatalf("created domain = %+v, want a pending row with a token", created)
	}

	if _, err := db.GetOwnedDomain(ctx, "user-b", created.ID); !errors.Is(err, db.ErrDomainNotFound) {
		t.Fatalf("another account read the domain: err=%v, want ErrDomainNotFound", err)
	}
	if err := db.MarkDomainVerified(ctx, "user-b", created.ID); !errors.Is(err, db.ErrDomainNotFound) {
		t.Fatalf("another account verified the domain: err=%v, want ErrDomainNotFound", err)
	}
	if err := db.MarkDomainVerified(ctx, "user-a", created.ID); err != nil {
		t.Fatalf("owner verify: %v", err)
	}

	got, err := db.GetOwnedDomain(ctx, "user-a", created.ID)
	if err != nil || got.Status != db.StatusVerified || got.VerificationToken != "" {
		t.Fatalf("verified domain = %+v err=%v, want verified with no token", got, err)
	}
	if err := db.MarkDomainVerified(ctx, "user-a", created.ID); !errors.Is(err, db.ErrDomainNotFound) {
		t.Fatalf("second verify matched a verified row: err=%v", err)
	}
	if err := db.MarkDomainActive(ctx, "user-b", created.ID); !errors.Is(err, db.ErrDomainNotFound) {
		t.Fatalf("another account activated the domain: err=%v, want ErrDomainNotFound", err)
	}
	if err := db.MarkDomainActive(ctx, "user-a", created.ID); err != nil {
		t.Fatalf("owner activation: %v", err)
	}
	_, _, mode, _, status, owner, err := db.GetTenant(ctx, created.Host)
	if err != nil || status != db.StatusActive || mode != "shadow" || owner != "user-a" {
		t.Fatalf("activated tenant status=%q mode=%q owner=%q err=%v, want active shadow owned by user-a", status, mode, owner, err)
	}
	if err := db.MarkDomainActive(ctx, "user-a", created.ID); !errors.Is(err, db.ErrDomainNotFound) {
		t.Fatalf("second activation matched active row: err=%v", err)
	}

	listed, err := db.ListDomains(ctx, "user-a")
	if err != nil || len(listed) != 1 || listed[0].VerificationToken != "" {
		t.Fatalf("list after verify = %+v err=%v, want one token-free domain", listed, err)
	}

	if _, err := db.CreateDomain(ctx, "user-b", "shop.example.com", "https://origin.example"); !errors.Is(err, db.ErrHostTaken) {
		t.Fatalf("duplicate host err=%v, want ErrHostTaken", err)
	}

	// The pending cap must stop one account reserving hostnames without
	// bound. maxPendingDomainsPerOwner is 5, and user-a has none pending
	// (the first is verified), so the sixth create from a fresh account
	// is the one that trips it.
	for i := 0; i < 5; i++ {
		if _, err := db.CreateDomain(ctx, "user-c", fmt.Sprintf("c%d.example.com", i), "https://origin.example"); err != nil {
			t.Fatalf("pending create %d: %v", i, err)
		}
	}
	if _, err := db.CreateDomain(ctx, "user-c", "c-extra.example.com", "https://origin.example"); !errors.Is(err, db.ErrTooManyPendingDomains) {
		t.Fatalf("pending cap err=%v, want ErrTooManyPendingDomains", err)
	}

	// Unproved reservations expire, stop disclosing their old TXT tokens,
	// and can be claimed again. Verified domains are never reclaimed.
	if _, err := pool.Exec(ctx, `UPDATE tenants SET created_at = now() - interval '8 days' WHERE owner_user_id = $1`, "user-c"); err != nil {
		t.Fatalf("age pending domains: %v", err)
	}
	var oldID string
	if err := pool.QueryRow(ctx, `SELECT id FROM tenants WHERE host = $1`, "c0.example.com").Scan(&oldID); err != nil {
		t.Fatalf("find expired reservation: %v", err)
	}
	for _, statement := range []string{
		`INSERT INTO tenant_policy_revisions (tenant_id, version, owner_user_id, actor_user_id, document_json)
		 VALUES ($1, 1, 'user-c', 'user-c', '{"mode":"shadow","rules":[]}')`,
		`INSERT INTO evidence_records (tenant_id, at, record) VALUES ($1, now(), '{"decision":"allow"}')`,
		`INSERT INTO training_samples (tenant_id, fired, feature_version, automated, source)
		 VALUES ($1, 0, 'v1', false, 'test')`,
	} {
		if _, err := pool.Exec(ctx, statement, oldID); err != nil {
			t.Fatalf("seed expired domain data: %v", err)
		}
	}
	if expired, err := db.GetOwnedDomain(ctx, "user-c", oldID); err != nil || expired.Status != db.StatusExpired || expired.VerificationToken != "" {
		t.Fatalf("expired domain = %+v err=%v", expired, err)
	}
	if err := db.MarkDomainVerified(ctx, "user-c", oldID); !errors.Is(err, db.ErrDomainNotFound) {
		t.Fatalf("expired reservation verified: %v", err)
	}
	if _, err := db.CreateDomain(ctx, "user-d", "c0.example.com", "https://another-origin.example"); err != nil {
		t.Fatalf("reclaim expired host: %v", err)
	}
	for _, table := range []string{"tenant_policy_revisions", "evidence_records", "training_samples"} {
		var count int
		if err := pool.QueryRow(ctx, `SELECT count(*) FROM `+table+` WHERE tenant_id = $1`, oldID).Scan(&count); err != nil || count != 0 {
			t.Fatalf("expired domain retained %s rows: count=%d err=%v", table, count, err)
		}
	}
	if _, err := db.CreateDomain(ctx, "user-c", "c-extra.example.com", "https://origin.example"); err != nil {
		t.Fatalf("expired reservations still counted against quota: %v", err)
	}

	// The quota must also hold under simultaneous requests from one account.
	var wg sync.WaitGroup
	results := make(chan error, 10)
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, err := db.CreateDomain(ctx, "user-e", fmt.Sprintf("e%d.example.com", i), "https://origin.example")
			results <- err
		}(i)
	}
	wg.Wait()
	close(results)
	successes := 0
	for err := range results {
		if err == nil {
			successes++
		} else if !errors.Is(err, db.ErrTooManyPendingDomains) {
			t.Fatalf("concurrent reservation error: %v", err)
		}
	}
	if successes != 5 {
		t.Fatalf("concurrent reservations = %d, want five", successes)
	}
}
