package db

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/ToufiqQureshi/hakaishield/pkg/evidence"
	"github.com/jackc/pgx/v5"
)

// EvidenceStore persists per-request decisions and reads them back. It is
// evidence.Sink's database implementation; pkg/evidence stays free of any
// database dependency.
type EvidenceStore struct{}

// AppendBatch writes one tenant's records in a single round trip. The
// time column is duplicated out of the JSON record so retention and
// per-tenant reads can use an index without parsing JSON.
func (EvidenceStore) AppendBatch(ctx context.Context, tenantID string, records []evidence.Evidence) error {
	if DB == nil {
		return fmt.Errorf("database not initialized")
	}
	if len(records) == 0 {
		return nil
	}

	batch := &pgx.Batch{}
	for _, r := range records {
		raw, err := json.Marshal(r)
		if err != nil {
			return fmt.Errorf("encoding evidence record: %w", err)
		}
		batch.Queue(
			`INSERT INTO evidence_records (tenant_id, at, record) VALUES ($1, $2, $3)`,
			tenantID, r.Time, raw,
		)
	}

	results := DB.SendBatch(ctx, batch)
	defer func() { _ = results.Close() }()
	for range records {
		if _, err := results.Exec(); err != nil {
			return fmt.Errorf("writing evidence records: %w", err)
		}
	}
	return nil
}

// Recent returns a tenant's newest records down to since, newest first.
func (EvidenceStore) Recent(ctx context.Context, tenantID string, limit int, since time.Time) ([]evidence.Evidence, error) {
	if DB == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	if limit <= 0 {
		return nil, fmt.Errorf("limit must be positive")
	}

	rows, err := DB.Query(ctx, `
		SELECT record FROM evidence_records
		WHERE tenant_id = $1 AND at >= $2
		ORDER BY at DESC LIMIT $3`,
		tenantID, since, limit)
	if err != nil {
		return nil, fmt.Errorf("reading evidence records: %w", err)
	}
	defer rows.Close()

	out := []evidence.Evidence{}
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			return nil, fmt.Errorf("scanning evidence record: %w", err)
		}
		var e evidence.Evidence
		if err := json.Unmarshal(raw, &e); err != nil {
			return nil, fmt.Errorf("decoding evidence record: %w", err)
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// DeleteEvidenceBefore drops one 1000-row batch older than cutoff, the
// same bounded pattern as training-sample retention. Callers schedule
// repeated batches off the request path.
func DeleteEvidenceBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	if DB == nil {
		return 0, fmt.Errorf("database not initialized")
	}
	tag, err := DB.Exec(ctx, `DELETE FROM evidence_records WHERE id IN (
		SELECT id FROM evidence_records WHERE at < $1
		ORDER BY at, id LIMIT 1000
	)`, cutoff)
	if err != nil {
		return 0, fmt.Errorf("deleting old evidence records: %w", err)
	}
	return tag.RowsAffected(), nil
}
