-- DesKaProvider v0.1 ledger constraint hardening for databases that have already applied migration 004.
CREATE UNIQUE INDEX IF NOT EXISTS ledger_transactions_transaction_currency_uq
    ON ledger_transactions (transaction_id, currency);

DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM ledger_entries
        WHERE direction NOT IN ('DEBIT', 'CREDIT')
    ) THEN
        RAISE EXCEPTION 'cannot add ledger direction constraint: invalid existing ledger entry direction';
    END IF;

    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'ledger_entry_direction_check'
          AND conrelid = 'ledger_entries'::regclass
    ) THEN
        ALTER TABLE ledger_entries
            ADD CONSTRAINT ledger_entry_direction_check
            CHECK (direction IN ('DEBIT', 'CREDIT'));
    END IF;
END $$;

DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM ledger_entries e
        JOIN ledger_transactions t ON t.transaction_id = e.transaction_id
        WHERE e.currency <> t.currency
    ) THEN
        RAISE EXCEPTION 'cannot add ledger currency constraint: invalid existing entry currency';
    END IF;

    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'ledger_entry_currency_matches_tx'
          AND conrelid = 'ledger_entries'::regclass
    ) THEN
        ALTER TABLE ledger_entries
            ADD CONSTRAINT ledger_entry_currency_matches_tx
            FOREIGN KEY (transaction_id, currency)
            REFERENCES ledger_transactions (transaction_id, currency);
    END IF;
END $$;
