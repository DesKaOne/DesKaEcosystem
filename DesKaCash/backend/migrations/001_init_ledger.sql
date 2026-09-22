CREATE TABLE accounts (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    asset TEXT NOT NULL,
    balance_base_units BIGINT NOT NULL DEFAULT 0,
    version BIGINT NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT accounts_asset_check CHECK (asset = 'dIDR'),
    CONSTRAINT accounts_balance_check CHECK (balance_base_units >= 0)
);

CREATE INDEX accounts_user_id_idx ON accounts (user_id);

CREATE TABLE transactions (
    id TEXT PRIMARY KEY,
    account_id TEXT NOT NULL REFERENCES accounts(id),
    asset TEXT NOT NULL,
    amount_base_units BIGINT NOT NULL,
    type TEXT NOT NULL,
    status TEXT NOT NULL,
    provider_id TEXT,
    reference TEXT,
    created_at TIMESTAMPTZ NOT NULL,

    CONSTRAINT transactions_asset_check CHECK (asset = 'dIDR'),
    CONSTRAINT transactions_amount_check CHECK (amount_base_units > 0),
    CONSTRAINT transactions_status_check CHECK (
        status IN ('pending', 'succeeded', 'failed', 'reversed')
    )
);

CREATE INDEX transactions_account_id_idx ON transactions (account_id);
CREATE INDEX transactions_provider_id_idx ON transactions (provider_id);

CREATE TABLE ledger_entries (
    id TEXT PRIMARY KEY,
    account_id TEXT NOT NULL REFERENCES accounts(id),
    transaction_id TEXT NOT NULL REFERENCES transactions(id),
    type TEXT NOT NULL,
    asset TEXT NOT NULL,
    amount_base_units BIGINT NOT NULL,
    reference TEXT,
    created_at TIMESTAMPTZ NOT NULL,

    CONSTRAINT ledger_entries_asset_check CHECK (asset = 'dIDR'),
    CONSTRAINT ledger_entries_amount_check CHECK (amount_base_units > 0),
    CONSTRAINT ledger_entries_type_check CHECK (
        type IN ('credit', 'debit')
    ),
    CONSTRAINT ledger_entries_transaction_unique UNIQUE (transaction_id)
);

CREATE INDEX ledger_entries_account_id_created_at_idx
    ON ledger_entries (account_id, created_at);


CREATE TABLE ledger_postings (
    id TEXT PRIMARY KEY,
    transaction_id TEXT NOT NULL REFERENCES transactions(id),
    account_id TEXT NOT NULL REFERENCES accounts(id),
    asset TEXT NOT NULL,
    type TEXT NOT NULL,
    amount_base_units BIGINT NOT NULL,
    reference TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT ledger_postings_asset_check CHECK (asset = 'dIDR'),
    CONSTRAINT ledger_postings_amount_check CHECK (amount_base_units > 0),
    CONSTRAINT ledger_postings_type_check CHECK (
        type IN ('debit', 'credit')
    )
);

CREATE INDEX ledger_postings_transaction_id_idx
    ON ledger_postings (transaction_id);

CREATE INDEX ledger_postings_account_id_idx
    ON ledger_postings (account_id);
