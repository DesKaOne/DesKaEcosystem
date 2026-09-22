package postgres

import (
	"context"
	"database/sql"
	"errors"

	"github.com/DesKaOne/DesKaEcosystem/DesKaCash/backend/internal/ledger"
)

// ApplyCredit atomically applies a successful credit to an account.
//
// The account row is locked for the duration of the transaction. The
// transaction ID is inserted before the balance mutation, while the unique
// constraints in PostgreSQL protect against duplicate application.
func (r *Repository) ApplyCredit(ctx context.Context, tx ledger.Transaction, reference string) error {
	dbtx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = dbtx.Rollback() }()

	var account ledger.Account
	err = dbtx.QueryRowContext(ctx, `
		SELECT id, user_id, asset, balance_base_units, version
		FROM accounts
		WHERE id = $1
		FOR UPDATE
	`, tx.AccountID).Scan(
		&account.ID,
		&account.UserID,
		&account.Asset,
		&account.Balance.BaseUnits,
		&account.Version,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return ledger.ErrNotFound
	}
	if err != nil {
		return err
	}

	if tx.Amount.BaseUnits <= 0 {
		return ledger.ErrInvalidAmount
	}

	_, err = dbtx.ExecContext(ctx, `
		INSERT INTO transactions
			(id, account_id, asset, amount_base_units, type, status, provider_id, reference, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, NULLIF($7, ''), NULLIF($8, ''), $9)
	`, tx.ID, tx.AccountID, tx.Asset, tx.Amount.BaseUnits,
		tx.Type, ledger.StatusSucceeded, tx.ProviderID, reference, tx.CreatedAt)
	if err != nil {
		return mapDBError(err)
	}

	account.Balance.BaseUnits += tx.Amount.BaseUnits
	account.Version++

	_, err = dbtx.ExecContext(ctx, `
		UPDATE accounts
		SET balance_base_units = $2, version = $3, updated_at = NOW()
		WHERE id = $1
	`, account.ID, account.Balance.BaseUnits, account.Version)
	if err != nil {
		return err
	}

	_, err = dbtx.ExecContext(ctx, `
		INSERT INTO ledger_entries
			(id, account_id, transaction_id, type, asset, amount_base_units, reference, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, NULLIF($7, ''), $8)
	`, tx.ID+":credit", tx.AccountID, tx.ID, ledger.EntryCredit,
		tx.Asset, tx.Amount.BaseUnits, reference, tx.CreatedAt)
	if err != nil {
		return mapDBError(err)
	}

	return dbtx.Commit()
}
