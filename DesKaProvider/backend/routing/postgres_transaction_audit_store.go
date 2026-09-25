package routing

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// PostgresTransactionAuditStore is an append-only PostgreSQL implementation
// of TransactionAuditStore. It never updates or deletes existing audit rows.
type PostgresTransactionAuditStore struct {
	db DBTX
}

func NewPostgresTransactionAuditStore(db DBTX) (*PostgresTransactionAuditStore, error) {
	if db == nil {
		return nil, errors.New("postgres transaction audit store database is required")
	}
	return &PostgresTransactionAuditStore{db: db}, nil
}

var _ TransactionAuditStore = (*PostgresTransactionAuditStore)(nil)

const postgresAuditAppendSQL = `INSERT INTO provider_transaction_audit
	(reference_id, action, previous_status, next_status, provider_name, message, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7)`

const postgresAuditAllSQL = `SELECT reference_id, action, previous_status, next_status, provider_name, message, created_at
FROM provider_transaction_audit
WHERE reference_id = $1
ORDER BY created_at, audit_id`

func (s *PostgresTransactionAuditStore) Append(event TransactionAuditEvent) error {
	return s.AppendContext(context.Background(), event)
}

func (s *PostgresTransactionAuditStore) AppendContext(ctx context.Context, event TransactionAuditEvent) error {
	if err := validateTransactionAuditEvent(event); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if _, err := s.db.ExecContext(ctx, postgresAuditAppendSQL,
		event.ReferenceID,
		event.Action,
		event.Previous,
		event.Next,
		event.ProviderName,
		event.Message,
		event.CreatedAt.UTC(),
	); err != nil {
		return fmt.Errorf("append transaction audit: %w", err)
	}
	return nil
}

func (s *PostgresTransactionAuditStore) All(referenceID string) []TransactionAuditEvent {
	result, err := s.AllContext(context.Background(), referenceID)
	if err != nil {
		return nil
	}
	return result
}

func (s *PostgresTransactionAuditStore) AllContext(ctx context.Context, referenceID string) ([]TransactionAuditEvent, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, postgresAuditAllSQL, referenceID)
	if err != nil {
		return nil, fmt.Errorf("list transaction audit: %w", err)
	}
	defer rows.Close()

	var result []TransactionAuditEvent
	for rows.Next() {
		var event TransactionAuditEvent
		if err := rows.Scan(
			&event.ReferenceID,
			&event.Action,
			&event.Previous,
			&event.Next,
			&event.ProviderName,
			&event.Message,
			&event.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan transaction audit: %w", err)
		}
		result = append(result, event)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate transaction audit: %w", err)
	}
	return result, nil
}

func validateTransactionAuditEvent(event TransactionAuditEvent) error {
	if event.ReferenceID == "" {
		return errors.New("audit reference ID is required")
	}
	if event.Action == "" {
		return errors.New("audit action is required")
	}
	if event.CreatedAt.IsZero() {
		return errors.New("audit created_at is required")
	}
	return nil
}

// Keep the sql package referenced here as part of the scanner contract used by
// database/sql-backed implementations and deterministic test doubles.
var _ interface{ Scan(...any) error } = (*sql.Row)(nil)

var _ = time.Time{}
