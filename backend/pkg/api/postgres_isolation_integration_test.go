package api

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/ToufiqQureshi/hakaishield/pkg/config"
	"github.com/ToufiqQureshi/hakaishield/pkg/db"
	"github.com/ToufiqQureshi/hakaishield/pkg/rules"
	"github.com/ToufiqQureshi/hakaishield/pkg/settings"
	"github.com/ToufiqQureshi/hakaishield/pkg/signals"
	"github.com/ToufiqQureshi/hakaishield/pkg/tenant"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestPostgresTenantIsolationIntegration(t *testing.T) {
	rawURL := os.Getenv("HAKAISHIELD_TEST_DATABASE_URL")
	if rawURL == "" {
		t.Skip("set HAKAISHIELD_TEST_DATABASE_URL to run the real Postgres tenant-isolation integration test")
	}
	if !safeTestDatabaseURL(rawURL) && os.Getenv("HAKAISHIELD_ALLOW_REMOTE_TEST_DATABASE") != "1" {
		t.Skip("HAKAISHIELD_TEST_DATABASE_URL must point at localhost/contain 'test', or set HAKAISHIELD_ALLOW_REMOTE_TEST_DATABASE=1 for a deliberate remote dev DB run")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	adminPool, err := pgxpool.New(ctx, rawURL)
	if err != nil {
		t.Fatalf("connect admin pool: %v", err)
	}
	defer adminPool.Close()

	schema := fmt.Sprintf("hs_test_%d", time.Now().UnixNano())
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
	if _, err := pool.Exec(ctx, `
		INSERT INTO tenants (id, host, target, mode, owner_user_id, name, status, created_at)
		VALUES
			('tenant-a', 'a.example.com', 'http://127.0.0.1:1', 'enforce', 'user-a', 'a.example.com', 'active', now() - interval '1 minute'),
			('tenant-b', 'b.example.com', 'http://127.0.0.1:1', 'enforce', 'user-b', 'b.example.com', 'active', now());
		INSERT INTO mitigation_rules (id, owner_user_id, name, conditions_json, action, enabled)
		VALUES ('rule-b', 'user-b', 'block ja4', '[{"field":"ja4","operator":"equals","value":"bad"}]', 'block', true);
		INSERT INTO protection_settings (owner_user_id, block_threshold, challenge_threshold, challenge_type, honeypot_enabled)
		VALUES ('user-b', 95, 60, 'pow', true);
	`); err != nil {
		t.Fatalf("seed integration tables: %v", err)
	}

	oldDB := db.DB
	db.DB = pool
	t.Cleanup(func() { db.DB = oldDB })

	store := tenant.NewStore()
	store.Add("tenant-a", tenant.TenantConfig{Mode: config.ModeEnforce}, []string{"a.example.com"}, nil)
	store.Add("tenant-b", tenant.TenantConfig{Mode: config.ModeEnforce}, []string{"b.example.com"}, nil)
	tenB, _ := store.GetByID("tenant-b")
	tenB.Stats.Record(signals.DecisionBlock)

	ia := newIsolationAuth(t)
	verifier := ia.verifier(t)
	userAToken := ia.sign(t, "user-a")

	statsReq := httptest.NewRequest(http.MethodGet, "/api/v1/dashboard/stats?tenant=tenant-b", nil)
	statsReq.Header.Set("Authorization", "Bearer "+userAToken)
	statsRec := httptest.NewRecorder()
	DashboardStatsHandler(store, verifier).ServeHTTP(statsRec, statsReq)
	if statsRec.Code != http.StatusNotFound {
		t.Fatalf("stats cross-tenant status = %d, want 404", statsRec.Code)
	}

	toggleReq := httptest.NewRequest(http.MethodPut, "/api/v1/rules/rule-b/toggle", bytes.NewBufferString(`{"enabled":false}`))
	toggleReq.Header.Set("Authorization", "Bearer "+userAToken)
	toggleReq.Header.Set("Content-Type", "application/json")
	toggleReq.SetPathValue("id", "rule-b")
	toggleRec := httptest.NewRecorder()
	ToggleRuleHandler(rules.NewStore(pool), verifier).ServeHTTP(toggleRec, toggleReq)
	if toggleRec.Code != http.StatusNotFound {
		t.Fatalf("rule cross-tenant toggle status = %d, want 404", toggleRec.Code)
	}

	settingsReq := httptest.NewRequest(http.MethodPut, "/api/v1/settings/protection", bytes.NewBufferString(`{"blockThreshold":80,"challengeThreshold":40,"challengeType":"pow","honeypotEnabled":false}`))
	settingsReq.Header.Set("Authorization", "Bearer "+userAToken)
	settingsReq.Header.Set("Content-Type", "application/json")
	settingsRec := httptest.NewRecorder()
	ProtectionSettingsHandler(settings.NewStore(pool), verifier).ServeHTTP(settingsRec, settingsReq)
	if settingsRec.Code != http.StatusOK {
		t.Fatalf("settings update status = %d, want 200", settingsRec.Code)
	}

	var userBBlock int
	if err := pool.QueryRow(ctx, `SELECT block_threshold FROM protection_settings WHERE owner_user_id = 'user-b'`).Scan(&userBBlock); err != nil {
		t.Fatalf("read user-b settings: %v", err)
	}
	if userBBlock != 95 {
		t.Fatalf("user-b settings changed to %d, want 95", userBBlock)
	}

	ruleStore := rules.NewStore(pool)
	created, err := ruleStore.Create(ctx, "user-a", "login challenge", []rules.Condition{{
		Field: "Request Path", Operator: "EQUALS", Value: "/login",
	}}, "CHALLENGE")
	if err != nil || created == nil {
		t.Fatalf("create valid owner-scoped rule: rule=%+v err=%v", created, err)
	}
	userARules, err := ruleStore.List(ctx, "user-a")
	if err != nil || len(userARules) != 1 || userARules[0].ID != created.ID {
		t.Fatalf("user-a rules = %+v, err=%v", userARules, err)
	}
	userBRules, err := ruleStore.List(ctx, "user-b")
	if err != nil || len(userBRules) != 1 || userBRules[0].ID != "rule-b" {
		t.Fatalf("user-b rules leaked or changed: %+v, err=%v", userBRules, err)
	}

	if err := settings.NewStore(pool).Upsert(ctx, "user-a", settings.Protection{
		BlockThreshold: 120, ChallengeThreshold: 50, ChallengeType: "pow", HoneypotEnabled: true,
	}); err != nil {
		t.Fatalf("raise user-a block threshold: %v", err)
	}
	_, err = ruleStore.Create(ctx, "user-a", "unsafe deceive", []rules.Condition{{
		Field: "Threat Score", Operator: ">=", Value: "101",
	}}, "DECEIVE")
	if !errors.Is(err, rules.ErrInvalidRule) {
		t.Fatalf("deceive below owner block floor = %v, want ErrInvalidRule", err)
	}
}

func safeTestDatabaseURL(rawURL string) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	target := strings.ToLower(u.Host + u.Path)
	return strings.Contains(target, "localhost") || strings.Contains(target, "127.0.0.1") || strings.Contains(target, "test")
}

const integrationSchemaSQL = `
CREATE TABLE tenants (
	id VARCHAR(255) PRIMARY KEY,
	host VARCHAR(255) UNIQUE NOT NULL,
	target VARCHAR(255) NOT NULL,
	mode VARCHAR(50) NOT NULL,
	evidence_token VARCHAR(255),
	owner_user_id VARCHAR(255),
	name VARCHAR(255),
	status VARCHAR(50) NOT NULL DEFAULT 'active',
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	verification_token VARCHAR(64)
);
CREATE TABLE mitigation_rules (
	id VARCHAR(255) PRIMARY KEY,
	owner_user_id VARCHAR(255) NOT NULL,
	name VARCHAR(255) NOT NULL,
	conditions_json TEXT NOT NULL,
	action VARCHAR(50) NOT NULL,
	enabled BOOLEAN NOT NULL DEFAULT true,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE protection_settings (
	owner_user_id VARCHAR(255) PRIMARY KEY,
	block_threshold INT NOT NULL DEFAULT 90,
	challenge_threshold INT NOT NULL DEFAULT 50,
	challenge_type VARCHAR(50) NOT NULL DEFAULT 'pow',
	honeypot_enabled BOOLEAN NOT NULL DEFAULT true,
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
`
