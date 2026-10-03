package api

import (
	"context"
	"errors"
	"fmt"
	"os"
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
}
