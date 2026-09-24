package postgres

import (
	"context"
	"database/sql"
	"errors"

	"github.com/DesKaOne/DesKaEcosystem/DesKaCash/internal/ledger"
)

// ApplyCredit atomically applies a successful credit to an account.
func (r *Repository) ApplyCredit(ctx context.Context, tx ledger.Transaction, reference string) error {
	return r.applyBalanceChange(ctx, tx, reference, ledger.EntryCredit)
}

// ApplyDebit atomically applies a successful debit to an account.
func (r *Repository) ApplyDebit(ctx context.Context, tx ledger.Transaction, reference string) error {
	return r.applyBalanceChange(ctx, tx, reference, ledger.EntryDebit)
}

func (r *Repository) applyBalanceChange(
	ctx context.Context,
	tx ledger.Transaction,
	reference string,
	entryType ledger.EntryType,
) error {
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

	if entryType == ledger.EntryDebit && tx.Amount.BaseUnits > account.Balance.BaseUnits {
		return ledger.ErrInsufficientFunds
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

	if entryType == ledger.EntryDebit {
		account.Balance.BaseUnits -= tx.Amount.BaseUnits
	} else {
		account.Balance.BaseUnits += tx.Amount.BaseUnits
	}
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
	`, tx.ID+":"+string(entryType), tx.AccountID, tx.ID, entryType,
		tx.Asset, tx.Amount.BaseUnits, reference, tx.CreatedAt)
	if err != nil {
		return mapDBError(err)
	}

	return dbtx.Commit()
}
