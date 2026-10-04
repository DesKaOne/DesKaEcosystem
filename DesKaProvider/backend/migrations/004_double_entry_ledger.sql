-- DesKaProvider v0.1 double-entry ledger foundation.
CREATE TABLE IF NOT EXISTS ledger_accounts (
    account_id TEXT PRIMARY KEY,
    account_type TEXT NOT NULL,
    owner_id TEXT NOT NULL DEFAULT '',
    currency TEXT NOT NULL,
    name TEXT NOT NULL,
    active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS ledger_transactions (
    transaction_id TEXT PRIMARY KEY,
    reference_id TEXT NOT NULL,
    source_type TEXT NOT NULL,
    source_id TEXT NOT NULL,
    currency TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL,
    UNIQUE (transaction_id, currency)
);

CREATE TABLE IF NOT EXISTS ledger_entries (
    transaction_id TEXT NOT NULL REFERENCES ledger_transactions(transaction_id),
    line_id INTEGER NOT NULL,
    account_id TEXT NOT NULL REFERENCES ledger_accounts(account_id),
    direction TEXT NOT NULL CHECK (direction IN ('DEBIT', 'CREDIT')),
    amount BIGINT NOT NULL CHECK (amount > 0),
    currency TEXT NOT NULL,
    memo TEXT NOT NULL DEFAULT '',
    PRIMARY KEY (transaction_id, line_id),
    CONSTRAINT ledger_entry_currency_matches_tx FOREIGN KEY (transaction_id, currency) REFERENCES ledger_transactions (transaction_id, currency)
);

CREATE INDEX IF NOT EXISTS ledger_transactions_reference_idx
    ON ledger_transactions (reference_id, created_at);

CREATE INDEX IF NOT EXISTS ledger_entries_account_idx
    ON ledger_entries (account_id, transaction_id);

CREATE INDEX IF NOT EXISTS ledger_entries_direction_idx
    ON ledger_entries (direction, currency);
