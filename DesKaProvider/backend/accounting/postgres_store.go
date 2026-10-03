package accounting

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

type PostgresStore struct {
	db *sql.DB
}

func NewPostgresStore(db *sql.DB) (*PostgresStore, error) {
	if db == nil {
		return nil, errors.New("postgres ledger database is required")
	}
	return &PostgresStore{db: db}, nil
}

func (s *PostgresStore) AppendContext(ctx context.Context, tx LedgerTransaction) error {
	return s.Append(ctx, tx)
}

func (s *PostgresStore) GetContext(ctx context.Context, id string) (LedgerTransaction, bool, error) {
	return s.Get(ctx, id)
}

func (s *PostgresStore) Append(ctx context.Context, tx LedgerTransaction) error {
	if err := tx.Validate(); err != nil {
		return err
	}
	dbtx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin ledger append: %w", err)
	}
	defer func() { _ = dbtx.Rollback() }()

	var inserted string
	err = dbtx.QueryRowContext(ctx,
		"INSERT INTO ledger_transactions (transaction_id, reference_id, source_type, source_id, currency, description, created_at) VALUES ($1,$2,$3,$4,$5,$6,$7) ON CONFLICT (transaction_id) DO NOTHING RETURNING transaction_id",
		tx.ID, tx.ReferenceID, tx.SourceType, tx.SourceID, tx.Currency, tx.Description, tx.CreatedAt,
	).Scan(&inserted)
	if errors.Is(err, sql.ErrNoRows) {
		if err := dbtx.Rollback(); err != nil {
			return fmt.Errorf("rollback duplicate ledger append: %w", err)
		}
		return s.checkExisting(ctx, tx)
	}
	if err != nil {
		return fmt.Errorf("insert ledger transaction: %w", err)
	}
	for _, entry := range tx.Entries {
		if _, err := dbtx.ExecContext(ctx,
			"INSERT INTO ledger_entries (transaction_id,line_id,account_id,direction,amount,currency,memo) VALUES ($1,$2,$3,$4,$5,$6,$7)",
			tx.ID, entry.LineID, entry.AccountID, entry.Direction, entry.Amount, entry.Currency, entry.Memo,
		); err != nil {
			return fmt.Errorf("insert ledger entry %d: %w", entry.LineID, err)
		}
	}
	if err := dbtx.Commit(); err != nil {
		return fmt.Errorf("commit ledger append: %w", err)
	}
	return nil
}

func (s *PostgresStore) checkExisting(ctx context.Context, wanted LedgerTransaction) error {
	current, ok, err := s.Get(ctx, wanted.ID)
	if err != nil {
		return err
	}
	if !ok {
		return ErrLedgerConflict
	}
	if sameLedgerTransaction(current, wanted) {
		return nil
	}
	return ErrLedgerConflict
}

func (s *PostgresStore) Get(ctx context.Context, id string) (LedgerTransaction, bool, error) {
	var tx LedgerTransaction
	err := s.db.QueryRowContext(ctx,
		"SELECT transaction_id,reference_id,source_type,source_id,currency,description,created_at FROM ledger_transactions WHERE transaction_id=$1",
		id,
	).Scan(&tx.ID,&tx.ReferenceID,&tx.SourceType,&tx.SourceID,&tx.Currency,&tx.Description,&tx.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return LedgerTransaction{}, false, nil
	}
	if err != nil {
		return LedgerTransaction{}, false, fmt.Errorf("get ledger transaction: %w", err)
	}
	rows, err := s.db.QueryContext(ctx,
		"SELECT line_id,account_id,direction,amount,currency,memo FROM ledger_entries WHERE transaction_id=$1 ORDER BY line_id",
		id,
	)
	if err != nil {
		return LedgerTransaction{}, false, fmt.Errorf("get ledger entries: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var entry Entry
		if err := rows.Scan(&entry.LineID,&entry.AccountID,&entry.Direction,&entry.Amount,&entry.Currency,&entry.Memo); err != nil {
			return LedgerTransaction{}, false, fmt.Errorf("scan ledger entry: %w", err)
		}
		tx.Entries = append(tx.Entries, entry)
	}
	if err := rows.Err(); err != nil {
		return LedgerTransaction{}, false, fmt.Errorf("iterate ledger entries: %w", err)
	}
	return tx, true, nil
}

func (s *PostgresStore) All(ctx context.Context) ([]LedgerTransaction, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT transaction_id FROM ledger_transactions ORDER BY created_at,transaction_id",
	)
	if err != nil {
		return nil, fmt.Errorf("list ledger transactions: %w", err)
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scan ledger transaction id: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, fmt.Errorf("iterate ledger transaction ids: %w", err)
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("close ledger transaction ids: %w", err)
	}

	out := make([]LedgerTransaction, 0, len(ids))
	for _, id := range ids {
		tx, ok, err := s.Get(ctx, id)
		if err != nil {
			return nil, err
		}
		if ok {
			out = append(out, tx)
		}
	}
	return out, nil
}

func (s *PostgresStore) AllContext(ctx context.Context) ([]LedgerTransaction, error) {
	return s.All(ctx)
}

func (s *PostgresStore) AllSettlementAudits(ctx context.Context) ([]SettlementAudit, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT event_id,transaction_id,reference_id,source_type,source_id,status,created_at FROM settlement_audit ORDER BY created_at,event_id",
	)
	if err != nil {
		return nil, fmt.Errorf("list settlement audits: %w", err)
	}
	defer rows.Close()
	out := make([]SettlementAudit, 0)
	for rows.Next() {
		var audit SettlementAudit
		if err := rows.Scan(&audit.EventID,&audit.TransactionID,&audit.ReferenceID,&audit.SourceType,&audit.SourceID,&audit.Status,&audit.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan settlement audit: %w", err)
		}
		out = append(out, audit)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate settlement audits: %w", err)
	}
	return out, nil
}

func (s *PostgresStore) CreateAccount(ctx context.Context, account Account) (Account, bool, error) {
	if err := account.Validate(); err != nil {
		return Account{}, false, err
	}
	var inserted string
	err := s.db.QueryRowContext(ctx,
		"INSERT INTO ledger_accounts (account_id,account_type,owner_id,currency,name,active) VALUES ($1,$2,$3,$4,$5,$6) ON CONFLICT (account_id) DO NOTHING RETURNING account_id",
		account.ID,account.Type,account.OwnerID,account.Currency,account.Name,account.Active,
	).Scan(&inserted)
	if err == nil {
		return account, true, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return Account{}, false, fmt.Errorf("create ledger account: %w", err)
	}
	var current Account
	err=s.db.QueryRowContext(ctx,
		"SELECT account_id,account_type,owner_id,currency,name,active FROM ledger_accounts WHERE account_id=$1",
		account.ID,
	).Scan(&current.ID,&current.Type,&current.OwnerID,&current.Currency,&current.Name,&current.Active)
	if err != nil {
		return Account{}, false, fmt.Errorf("reload ledger account: %w", err)
	}
	if current != account {
		return Account{}, false, ErrAccountConflict
	}
	return current, false, nil
}


func (s *PostgresStore) AppendSettlement(ctx context.Context, ledger LedgerTransaction, audit SettlementAudit) error {
	if err := ledger.Validate(); err != nil {
		return err
	}
	if err := audit.Validate(); err != nil {
		return err
	}
	if audit.TransactionID != ledger.ID || audit.ReferenceID != ledger.ReferenceID ||
		audit.SourceType != ledger.SourceType || audit.SourceID != ledger.SourceID {
		return ErrLedgerConflict
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin settlement append: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var inserted string
	err = tx.QueryRowContext(ctx,
		"INSERT INTO ledger_transactions (transaction_id, reference_id, source_type, source_id, currency, description, created_at) VALUES ($1,$2,$3,$4,$5,$6,$7) ON CONFLICT (transaction_id) DO NOTHING RETURNING transaction_id",
		ledger.ID, ledger.ReferenceID, ledger.SourceType, ledger.SourceID, ledger.Currency, ledger.Description, ledger.CreatedAt,
	).Scan(&inserted)
	if errors.Is(err, sql.ErrNoRows) {
		if err := tx.Rollback(); err != nil {
			return fmt.Errorf("rollback duplicate settlement append: %w", err)
		}
		current, ok, err := s.Get(ctx, ledger.ID)
		if err != nil { return err }
		currentAudit, auditOK, err := s.GetSettlementAudit(ctx, ledger.ID)
		if err != nil { return err }
		if ok && auditOK && sameLedgerTransaction(current, ledger) && sameSettlementAudit(currentAudit, audit) {
			return nil
		}
		return ErrLedgerConflict
	}
	if err != nil {
		return fmt.Errorf("insert settlement ledger transaction: %w", err)
	}
	for _, entry := range ledger.Entries {
		if _, err := tx.ExecContext(ctx,
			"INSERT INTO ledger_entries (transaction_id,line_id,account_id,direction,amount,currency,memo) VALUES ($1,$2,$3,$4,$5,$6,$7)",
			ledger.ID, entry.LineID, entry.AccountID, entry.Direction, entry.Amount, entry.Currency, entry.Memo,
		); err != nil {
			return fmt.Errorf("insert settlement ledger entry %d: %w", entry.LineID, err)
		}
	}
	var insertedAuditID string
	err = tx.QueryRowContext(ctx,
		"INSERT INTO settlement_audit (event_id, transaction_id, reference_id, source_type, source_id, status, created_at) VALUES ($1,$2,$3,$4,$5,$6,$7) ON CONFLICT (event_id) DO NOTHING RETURNING event_id",
		audit.EventID, audit.TransactionID, audit.ReferenceID, audit.SourceType, audit.SourceID, audit.Status, audit.CreatedAt,
	).Scan(&insertedAuditID)
	if errors.Is(err, sql.ErrNoRows) {
		var existing SettlementAudit
		lookupErr := tx.QueryRowContext(ctx,
			"SELECT event_id,transaction_id,reference_id,source_type,source_id,status,created_at FROM settlement_audit WHERE event_id=$1",
			audit.EventID,
		).Scan(&existing.EventID,&existing.TransactionID,&existing.ReferenceID,&existing.SourceType,&existing.SourceID,&existing.Status,&existing.CreatedAt)
		if lookupErr != nil {
			return fmt.Errorf("check existing settlement audit event: %w", lookupErr)
		}
		if sameSettlementAudit(existing, audit) {
			return ErrSettlementAuditConflict
		}
		return ErrSettlementAuditConflict
	}
	if err != nil {
		return fmt.Errorf("insert settlement audit: %w", err)
	}
	_ = insertedAuditID
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("%w: %v", ErrSettlementPersistenceAmbiguous, err)
	}
	return nil
}

func (s *PostgresStore) GetSettlementAudit(ctx context.Context, transactionID string) (SettlementAudit, bool, error) {
	var audit SettlementAudit
	err := s.db.QueryRowContext(ctx,
		"SELECT event_id,transaction_id,reference_id,source_type,source_id,status,created_at FROM settlement_audit WHERE transaction_id=$1",
		transactionID,
	).Scan(&audit.EventID,&audit.TransactionID,&audit.ReferenceID,&audit.SourceType,&audit.SourceID,&audit.Status,&audit.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) { return SettlementAudit{}, false, nil }
	if err != nil { return SettlementAudit{}, false, fmt.Errorf("get settlement audit: %w", err) }
	return audit, true, nil
}
