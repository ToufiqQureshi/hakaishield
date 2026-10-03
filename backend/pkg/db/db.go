package db

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ToufiqQureshi/hakaishield/pkg/labels"
)

// Domain is one row of the tenants table as the dashboard cares about
// it: a protected origin owned by one account. It reuses the tenants
// table rather than a separate "domains" table — a domain is exactly
// what tenant.Store already models (one host mapped to one origin),
// so a second table for the same entity would just be a sync problem.
type Domain struct {
	ID        string
	Host      string
	Target    string
	Name      string
	Status    string
	CreatedAt time.Time
	// VerificationToken is the secret a new domain's owner must publish in
	// DNS to prove they control it. It is non-empty only while the domain
	// is awaiting verification, and must never be exposed for an active or
	// verified domain.
	VerificationToken string
}

var DB *pgxpool.Pool

func Init(databaseURL string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return fmt.Errorf("unable to connect to database: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		return fmt.Errorf("unable to ping database: %w", err)
	}

	if err := InitSchema(ctx, pool); err != nil {
		pool.Close()
		return err
	}
	DB = pool
	return nil
}

// InitSchema applies additive startup migrations to the selected database.
func InitSchema(ctx context.Context, pool *pgxpool.Pool) error {
	schema := `
	CREATE TABLE IF NOT EXISTS tenants (
		id VARCHAR(255) PRIMARY KEY,
		host VARCHAR(255) UNIQUE NOT NULL,
		target VARCHAR(255) NOT NULL,
		mode VARCHAR(50) NOT NULL,
		evidence_token VARCHAR(255)
	);
	ALTER TABLE tenants ADD COLUMN IF NOT EXISTS owner_user_id VARCHAR(255);
	ALTER TABLE tenants ADD COLUMN IF NOT EXISTS name VARCHAR(255);
	ALTER TABLE tenants ADD COLUMN IF NOT EXISTS status VARCHAR(50) NOT NULL DEFAULT 'active';
	ALTER TABLE tenants ADD COLUMN IF NOT EXISTS created_at TIMESTAMPTZ NOT NULL DEFAULT now();
	ALTER TABLE tenants ADD COLUMN IF NOT EXISTS verification_token VARCHAR(64);
	CREATE INDEX IF NOT EXISTS idx_tenants_owner ON tenants(owner_user_id);

	CREATE TABLE IF NOT EXISTS users (
		id VARCHAR(255) PRIMARY KEY,
		email VARCHAR(255) UNIQUE NOT NULL,
		password_hash VARCHAR(255) NOT NULL,
		full_name VARCHAR(255),
		company VARCHAR(255),
		onboarding_complete BOOLEAN NOT NULL DEFAULT false,
		created_at TIMESTAMPTZ NOT NULL DEFAULT now()
	);

	CREATE TABLE IF NOT EXISTS mitigation_rules (
		id VARCHAR(255) PRIMARY KEY,
		owner_user_id VARCHAR(255) NOT NULL,
		name VARCHAR(255) NOT NULL,
		conditions_json TEXT NOT NULL,
		action VARCHAR(50) NOT NULL,
		enabled BOOLEAN NOT NULL DEFAULT true,
		created_at TIMESTAMPTZ NOT NULL DEFAULT now()
	);
	CREATE INDEX IF NOT EXISTS idx_rules_owner ON mitigation_rules(owner_user_id);

	CREATE TABLE IF NOT EXISTS tenant_policy_revisions (
		tenant_id VARCHAR(255) NOT NULL,
		version INT NOT NULL CHECK (version > 0),
		owner_user_id VARCHAR(255) NOT NULL,
		actor_user_id VARCHAR(255) NOT NULL,
		document_json JSONB NOT NULL,
		created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
		PRIMARY KEY (tenant_id, version)
	);
	CREATE INDEX IF NOT EXISTS idx_tenant_policy_owner ON tenant_policy_revisions(owner_user_id, tenant_id, version DESC);

	-- Labelled traffic for the learned scorer (pkg/decide) to train on.
	-- Deliberately holds no IP, user agent, path or body: a model trains
	-- on which checks fired, and nothing else here is worth the storage
	-- or the retention argument. feature_version names the check list
	-- that produced the mask, which is positional and uninterpretable
	-- without it. See docs/ARCHITECTURE.md.
	CREATE TABLE IF NOT EXISTS training_samples (
		id BIGSERIAL PRIMARY KEY,
		tenant_id VARCHAR(255) NOT NULL,
		fired BIGINT NOT NULL,
		feature_version VARCHAR(32) NOT NULL,
		automated BOOLEAN NOT NULL,
		source VARCHAR(50) NOT NULL,
		created_at TIMESTAMPTZ NOT NULL DEFAULT now()
	);
	CREATE INDEX IF NOT EXISTS idx_training_samples_tenant ON training_samples(tenant_id, created_at);
	CREATE INDEX IF NOT EXISTS idx_training_samples_version ON training_samples(feature_version);
	CREATE INDEX IF NOT EXISTS idx_training_samples_retention ON training_samples(created_at, id);

	CREATE TABLE IF NOT EXISTS protection_settings (
		owner_user_id VARCHAR(255) PRIMARY KEY,
		block_threshold INT NOT NULL DEFAULT 90,
		challenge_threshold INT NOT NULL DEFAULT 50,
		challenge_type VARCHAR(50) NOT NULL DEFAULT 'pow',
		honeypot_enabled BOOLEAN NOT NULL DEFAULT true,
		updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
	);

	-- Durable per-request decisions, so a restart does not erase the
	-- history a customer can see. The whole Evidence record is stored as
	-- JSONB and only the query columns are indexed; nothing here is read
	-- on the request path.
	CREATE TABLE IF NOT EXISTS evidence_records (
		id BIGSERIAL PRIMARY KEY,
		tenant_id VARCHAR(255) NOT NULL,
		at TIMESTAMPTZ NOT NULL,
		record JSONB NOT NULL
	);
	CREATE INDEX IF NOT EXISTS idx_evidence_tenant_at ON evidence_records(tenant_id, at DESC);
	CREATE INDEX IF NOT EXISTS idx_evidence_retention ON evidence_records(at, id);
	`
	_, err := pool.Exec(ctx, schema)
	return err
}

const tenantByHostQuery = `SELECT id, target, mode, COALESCE(evidence_token, ''), status, COALESCE(owner_user_id::text, '') FROM tenants WHERE host = $1 LIMIT 1`

func GetTenant(ctx context.Context, host string) (id, target, mode, evidenceToken, status, ownerUserID string, err error) {
	if DB == nil {
		return "", "", "", "", "", "", fmt.Errorf("database not initialized")
	}

	err = DB.QueryRow(ctx, tenantByHostQuery, host).Scan(&id, &target, &mode, &evidenceToken, &status, &ownerUserID)
	return
}

// GetTenantByID is GetTenant's counterpart for lookups by internal ID
// rather than incoming Host header — the dashboard API knows a
// domain's ID, not the host a live request would carry.
const tenantByIDQuery = `SELECT host, target, mode, COALESCE(evidence_token, ''), status, COALESCE(owner_user_id::text, '') FROM tenants WHERE id = $1 LIMIT 1`

func GetTenantByID(ctx context.Context, id string) (host, target, mode, evidenceToken, status, ownerUserID string, err error) {
	if DB == nil {
		return "", "", "", "", "", "", fmt.Errorf("database not initialized")
	}

	err = DB.QueryRow(ctx, tenantByIDQuery, id).Scan(&host, &target, &mode, &evidenceToken, &status, &ownerUserID)
	return
}

// ListDomains returns every tenant owned by ownerUserID, newest first.
func ListDomains(ctx context.Context, ownerUserID string) ([]Domain, error) {
	if DB == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	const q = `SELECT id, host, target, COALESCE(name, host), status, created_at, COALESCE(verification_token, '')
		FROM tenants WHERE owner_user_id = $1 ORDER BY created_at DESC`
	rows, err := DB.Query(ctx, q, ownerUserID)
	if err != nil {
		return nil, fmt.Errorf("listing domains: %w", err)
	}
	defer rows.Close()

	out := []Domain{}
	for rows.Next() {
		var d Domain
		if err := rows.Scan(&d.ID, &d.Host, &d.Target, &d.Name, &d.Status, &d.CreatedAt, &d.VerificationToken); err != nil {
			return nil, fmt.Errorf("scanning domain row: %w", err)
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// SampleStore writes and reads labelled traffic for pkg/decide. It is
// the labels.Writer implementation; see docs/ARCHITECTURE.md.
type SampleStore struct{}

// WriteSamples inserts a batch of labelled requests.
//
// fired is stored as BIGINT rather than INT because a uint32 does not
// fit Postgres's signed 32-bit integer: the top bit would overflow once
// the checks list grows past 31 entries.
func (SampleStore) WriteSamples(ctx context.Context, samples []labels.Sample) error {
	if DB == nil {
		return fmt.Errorf("database not initialized")
	}
	if len(samples) == 0 {
		return nil
	}

	batch := &pgx.Batch{}
	for _, s := range samples {
		batch.Queue(
			`INSERT INTO training_samples (tenant_id, fired, feature_version, automated, source)
			 VALUES ($1, $2, $3, $4, $5)`,
			s.TenantID, int64(s.Fired), s.FeatureVersion, s.Automated, s.Source,
		)
	}

	results := DB.SendBatch(ctx, batch)
	defer func() { _ = results.Close() }()
	for range samples {
		if _, err := results.Exec(); err != nil {
			return fmt.Errorf("writing training samples: %w", err)
		}
	}
	return nil
}

// ReadSamples returns labelled traffic for training, newest first.
//
// featureVersion is required and filtered on, not merely reported: the
// fired mask is positional, so mixing rows captured against different
// check lists would train the model on signals it was never shown.
//
// tenantID empty means every tenant. That is a deliberate choice for a
// shared model and an explicit one - a per-tenant model passes the
// tenant here instead (CLAUDE.md Section 16).
func ReadSamples(ctx context.Context, tenantID, featureVersion string, limit int) ([]labels.Sample, error) {
	if DB == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	if featureVersion == "" {
		return nil, fmt.Errorf("feature version is required: a fired mask cannot be read without the check list that produced it")
	}
	if limit <= 0 {
		return nil, fmt.Errorf("limit must be positive")
	}

	query := `SELECT tenant_id, fired, feature_version, automated, source
	          FROM training_samples
	          WHERE feature_version = $1`
	args := []any{featureVersion}
	if tenantID != "" {
		query += ` AND tenant_id = $2`
		args = append(args, tenantID)
	}
	query += fmt.Sprintf(` ORDER BY created_at DESC LIMIT %d`, limit)

	rows, err := DB.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("reading training samples: %w", err)
	}
	defer rows.Close()

	var out []labels.Sample
	for rows.Next() {
		var s labels.Sample
		var fired int64
		if err := rows.Scan(&s.TenantID, &fired, &s.FeatureVersion, &s.Automated, &s.Source); err != nil {
			return nil, fmt.Errorf("scanning training sample: %w", err)
		}
		// fired is stored as BIGINT and read back into a uint32 mask. A
		// row outside that range did not come from this code, so it is
		// refused rather than truncated: a silently narrowed mask is a
		// sample that describes signals the request never fired.
		if fired < 0 || fired > math.MaxUint32 {
			return nil, fmt.Errorf("training sample has an out-of-range fired mask %d", fired)
		}
		s.Fired = uint32(fired)
		out = append(out, s)
	}
	return out, rows.Err()
}

// DeleteSamplesBefore drops one 1000-row batch older than cutoff. Callers
// schedule repeated bounded batches off the request path; old traffic is
// both less useful for training and subject to the pilot retention period.
func DeleteSamplesBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	if DB == nil {
		return 0, fmt.Errorf("database not initialized")
	}
	tag, err := DB.Exec(ctx, `DELETE FROM training_samples WHERE id IN (
		SELECT id FROM training_samples WHERE created_at < $1
		ORDER BY created_at, id LIMIT 1000
	)`, cutoff)
	if err != nil {
		return 0, fmt.Errorf("deleting old training samples: %w", err)
	}
	return tag.RowsAffected(), nil
}
