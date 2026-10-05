CREATE TABLE IF NOT EXISTS settlement_audit (
    event_id TEXT PRIMARY KEY,
    transaction_id TEXT NOT NULL REFERENCES ledger_transactions(transaction_id),
    reference_id TEXT NOT NULL,
    source_type TEXT NOT NULL,
    source_id TEXT NOT NULL,
    status TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS settlement_audit_transaction_id_uq
    ON settlement_audit(transaction_id);

CREATE INDEX IF NOT EXISTS settlement_audit_reference_idx
    ON settlement_audit(reference_id);

CREATE INDEX IF NOT EXISTS settlement_audit_source_idx
    ON settlement_audit(source_type, source_id);
