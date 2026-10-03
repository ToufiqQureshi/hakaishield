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

// CreateDomain reserves a hostname for an account and returns it with a
// fresh verification token. It does not make the domain routable: the
// row starts as pending_verification, which pkg/tenant refuses to route
// until ownership is proven (MarkDomainVerified) and an operator
// completes TLS/DNS setup.
func CreateDomain(ctx context.Context, ownerUserID, host, target string) (Domain, error) {
	if DB == nil {
		return Domain{}, fmt.Errorf("database not initialized")
	}

	var pending int
	if err := DB.QueryRow(ctx,
		`SELECT count(*) FROM tenants WHERE owner_user_id = $1 AND status = $2`,
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
	err = DB.QueryRow(ctx, `
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
	return d, nil
}

// GetOwnedDomain returns one domain scoped to its owner. An id that
// exists but belongs to another account is reported as ErrDomainNotFound.
func GetOwnedDomain(ctx context.Context, ownerUserID, id string) (Domain, error) {
	if DB == nil {
		return Domain{}, fmt.Errorf("database not initialized")
	}

	var d Domain
	err := DB.QueryRow(ctx, `
		SELECT id, host, target, COALESCE(name, host), status, created_at, COALESCE(verification_token, '')
		FROM tenants WHERE id = $1 AND owner_user_id = $2`,
		id, ownerUserID,
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
		WHERE id = $1 AND owner_user_id = $2 AND status = $4`,
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
