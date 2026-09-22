package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/DesKaOne/DesKaEcosystem/DesKaCash/backend/internal/ledger"
)

var ErrDuplicate = errors.New("duplicate record")

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) GetAccount(ctx context.Context, id string) (ledger.Account, error) {
	const query = `SELECT id, user_id, asset, balance_base_units, version
		FROM accounts WHERE id = $1`

	var account ledger.Account
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&account.ID, &account.UserID, &account.Asset,
		&account.Balance.BaseUnits, &account.Version,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return ledger.Account{}, ledger.ErrNotFound
	}
	return account, err
}

func (r *Repository) CreateAccount(ctx context.Context, account ledger.Account) error {
	const query = `INSERT INTO accounts
		(id, user_id, asset, balance_base_units, version)
		VALUES ($1, $2, $3, $4, $5)`

	_, err := r.db.ExecContext(ctx, query,
		account.ID, account.UserID, account.Asset,
		account.Balance.BaseUnits, account.Version,
	)
	return mapDBError(err)
}

func (r *Repository) SaveAccount(ctx context.Context, account ledger.Account) error {
	const query = `UPDATE accounts
		SET balance_base_units = $2, version = $3, updated_at = NOW()
		WHERE id = $1`

	result, err := r.db.ExecContext(ctx, query,
		account.ID, account.Balance.BaseUnits, account.Version,
	)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows != 1 {
		return ledger.ErrNotFound
	}
	return nil
}

func (r *Repository) GetTransaction(ctx context.Context, id string) (ledger.Transaction, error) {
	const query = `SELECT id, account_id, asset, amount_base_units, type,
		status, COALESCE(provider_id, ''), COALESCE(reference, ''), created_at
		FROM transactions WHERE id = $1`

	var tx ledger.Transaction
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&tx.ID, &tx.AccountID, &tx.Asset, &tx.Amount.BaseUnits,
		&tx.Type, &tx.Status, &tx.ProviderID, &tx.Reference, &tx.CreatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return ledger.Transaction{}, ledger.ErrNotFound
	}
	return tx, err
}

func (r *Repository) CreateTransaction(ctx context.Context, tx ledger.Transaction) error {
	const query = `INSERT INTO transactions
		(id, account_id, asset, amount_base_units, type, status, provider_id, reference, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, NULLIF($7, ''), NULLIF($8, ''), $9)`

	_, err := r.db.ExecContext(ctx, query,
		tx.ID, tx.AccountID, tx.Asset, tx.Amount.BaseUnits,
		tx.Type, tx.Status, tx.ProviderID, tx.Reference, tx.CreatedAt,
	)
	return mapDBError(err)
}

func (r *Repository) CreateEntry(ctx context.Context, entry ledger.Entry) error {
	const query = `INSERT INTO ledger_entries
		(id, account_id, transaction_id, type, asset, amount_base_units, reference, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, NULLIF($7, ''), $8)`

	_, err := r.db.ExecContext(ctx, query,
		entry.ID, entry.AccountID, entry.TransactionID, entry.Type,
		entry.Asset, entry.Amount.BaseUnits, entry.Reference, entry.CreatedAt,
	)
	return mapDBError(err)
}

func (r *Repository) ListEntries(ctx context.Context, accountID string) ([]ledger.Entry, error) {
	const query = `SELECT id, account_id, transaction_id, type, asset,
		amount_base_units, COALESCE(reference, ''), created_at
		FROM ledger_entries
		WHERE account_id = $1
		ORDER BY created_at ASC, id ASC`

	rows, err := r.db.QueryContext(ctx, query, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []ledger.Entry
	for rows.Next() {
		var entry ledger.Entry
		if err := rows.Scan(
			&entry.ID, &entry.AccountID, &entry.TransactionID, &entry.Type,
			&entry.Asset, &entry.Amount.BaseUnits, &entry.Reference, &entry.CreatedAt,
		); err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return entries, nil
}

func mapDBError(err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("postgres: %w", err)
}
