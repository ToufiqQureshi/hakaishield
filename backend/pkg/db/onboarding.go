package db

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// Domain lifecycle statuses. Only StatusActive routes visitor traffic
// (see pkg/tenant); the others describe setup progress so the dashboard
// can tell a customer exactly what is still missing.
const (
	StatusPendingVerification = "pending_verification"
	StatusVerified            = "verified"
	StatusActive              = "active"
	StatusExpired             = "expired"
)

var (
	// ErrHostTaken means another tenant already reserved this hostname.
	ErrHostTaken = errors.New("domain host is already registered")
	// ErrDomainNotFound means no domain with that id belongs to the owner.
	// It is deliberately the same error for "does not exist" and "owned by
	// someone else" so an authenticated caller cannot probe another
	// account's domains (CLAUDE.md Section 16).
	ErrDomainNotFound = errors.New("domain not found")
	// ErrTooManyPendingDomains caps unverified domains per account so one
	// account cannot reserve an unbounded number of hostnames.
	ErrTooManyPendingDomains = errors.New("too many domains awaiting verification")
)

const maxPendingDomainsPerOwner = 5

// A pending reservation is not ownership proof. Expiry prevents an account
// from holding another customer's hostname indefinitely without a TXT record.
const pendingDomainTTL = "7 days"

// CreateDomain reserves a hostname for an account and returns it with a
// fresh verification token. It does not make the domain routable: the
// row starts as pending_verification, which pkg/tenant refuses to route
// until ownership is proven (MarkDomainVerified) and an operator
// completes TLS/DNS setup.
func CreateDomain(ctx context.Context, ownerUserID, host, target string) (Domain, error) {
	if DB == nil {
		return Domain{}, fmt.Errorf("database not initialized")
	}

	tx, err := DB.Begin(ctx)
	if err != nil {
		return Domain{}, fmt.Errorf("beginning domain reservation: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Serialize quota checks for one owner. Without this, concurrent requests
	// can each see four pending domains and both insert a sixth reservation.
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(738133, hashtext($1))`, ownerUserID); err != nil {
		return Domain{}, fmt.Errorf("locking domain reservations: %w", err)
	}
	// Reclaim only expired, unverified reservations for this exact host. A
	// verified or active domain can never be displaced by a new signup.
	var expiredID string
	err = tx.QueryRow(ctx,
		`DELETE FROM tenants WHERE host = $1 AND status = $2 AND created_at < now() - interval '`+pendingDomainTTL+`' RETURNING id`,
		host, StatusPendingVerification,
	).Scan(&expiredID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return Domain{}, fmt.Errorf("reclaiming expired reservation: %w", err)
	}
	if err == nil {
		// These tables intentionally have no tenant FK, so reclaiming the
		// hostname must remove its old policy and telemetry in the same
		// transaction. Otherwise old account data survives an expired claim.
		for _, table := range []string{"tenant_policy_revisions", "evidence_records", "training_samples"} {
			if _, err := tx.Exec(ctx, `DELETE FROM `+table+` WHERE tenant_id = $1`, expiredID); err != nil {
				return Domain{}, fmt.Errorf("cleaning expired domain data: %w", err)
			}
		}
	}

	var pending int
	if err := tx.QueryRow(ctx,
		`SELECT count(*) FROM tenants WHERE owner_user_id = $1 AND status = $2 AND created_at >= now() - interval '`+pendingDomainTTL+`'`,
		ownerUserID, StatusPendingVerification,
	).Scan(&pending); err != nil {
		return Domain{}, fmt.Errorf("counting pending domains: %w", err)
	}
	if pending >= maxPendingDomainsPerOwner {
		return Domain{}, ErrTooManyPendingDomains
	}

	id, err := randomToken()
	if err != nil {
		return Domain{}, fmt.Errorf("generating domain id: %w", err)
	}
	token, err := randomToken()
	if err != nil {
		return Domain{}, fmt.Errorf("generating verification token: %w", err)
	}

	var d Domain
	err = tx.QueryRow(ctx, `
		INSERT INTO tenants (id, host, target, mode, name, status, owner_user_id, verification_token)
		VALUES ($1, $2, $3, 'shadow', $2, $4, $5, $6)
		ON CONFLICT (host) DO NOTHING
		RETURNING id, host, target, COALESCE(name, host), status, created_at, COALESCE(verification_token, '')`,
		id, host, target, StatusPendingVerification, ownerUserID, token,
	).Scan(&d.ID, &d.Host, &d.Target, &d.Name, &d.Status, &d.CreatedAt, &d.VerificationToken)
	if errors.Is(err, pgx.ErrNoRows) {
		return Domain{}, ErrHostTaken
	}
	if err != nil {
		return Domain{}, fmt.Errorf("creating domain: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return Domain{}, fmt.Errorf("committing domain reservation: %w", err)
	}
	return d, nil
}

// MarkDomainActive atomically enables routing only after provisioning has
// completed. A concurrent request cannot reactivate a removed/replaced row.
func MarkDomainActive(ctx context.Context, ownerUserID, id string) error {
	if DB == nil {
		return fmt.Errorf("database not initialized")
	}
	// New customer traffic starts in shadow mode. A human must review
	// observations and explicitly activate an enforcement policy later.
	tag, err := DB.Exec(ctx, `UPDATE tenants SET status = $3, mode = 'shadow' WHERE id = $1 AND owner_user_id = $2 AND status = $4`, id, ownerUserID, StatusActive, StatusVerified)
	if err != nil {
		return fmt.Errorf("activating domain: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrDomainNotFound
	}
	return nil
}

// GetOwnedDomain returns one domain scoped to its owner. An id that
// exists but belongs to another account is reported as ErrDomainNotFound.
func GetOwnedDomain(ctx context.Context, ownerUserID, id string) (Domain, error) {
	if DB == nil {
		return Domain{}, fmt.Errorf("database not initialized")
	}

	var d Domain
	err := DB.QueryRow(ctx, `
		SELECT id, host, target, COALESCE(name, host),
			CASE WHEN status = $3 AND created_at < now() - interval '`+pendingDomainTTL+`' THEN $4 ELSE status END,
			created_at,
			CASE WHEN status = $3 AND created_at < now() - interval '`+pendingDomainTTL+`' THEN '' ELSE COALESCE(verification_token, '') END
		FROM tenants WHERE id = $1 AND owner_user_id = $2`,
		id, ownerUserID, StatusPendingVerification, StatusExpired,
	).Scan(&d.ID, &d.Host, &d.Target, &d.Name, &d.Status, &d.CreatedAt, &d.VerificationToken)
	if errors.Is(err, pgx.ErrNoRows) {
		return Domain{}, ErrDomainNotFound
	}
	if err != nil {
		return Domain{}, fmt.Errorf("loading domain: %w", err)
	}
	return d, nil
}

// MarkDomainVerified moves an owned, pending domain to verified and
// clears its verification token so it cannot be reused. The status is
// part of the WHERE clause so a concurrent activation cannot be undone.
func MarkDomainVerified(ctx context.Context, ownerUserID, id string) error {
	if DB == nil {
		return fmt.Errorf("database not initialized")
	}

	tag, err := DB.Exec(ctx, `
		UPDATE tenants SET status = $3, verification_token = NULL
		WHERE id = $1 AND owner_user_id = $2 AND status = $4
			AND created_at >= now() - interval '`+pendingDomainTTL+`'`,
		id, ownerUserID, StatusVerified, StatusPendingVerification,
	)
	if err != nil {
		return fmt.Errorf("verifying domain: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrDomainNotFound
	}
	return nil
}

// randomToken returns 128 bits of randomness as hex, used for both the
// domain id and the DNS verification token.
func randomToken() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
