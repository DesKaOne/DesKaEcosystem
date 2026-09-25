-- DesKaProvider v0.1 transaction-store schema contract.
-- This migration is intentionally provider-neutral and does not wire a PostgreSQL driver.
-- The application integration must implement AtomicTransactionStore against this schema.

CREATE TABLE IF NOT EXISTS provider_transactions (
    reference_id TEXT PRIMARY KEY,
    product_code TEXT NOT NULL,
    customer_no TEXT NOT NULL,
    amount BIGINT NOT NULL CHECK (amount > 0),
    testing BOOLEAN NOT NULL DEFAULT FALSE,
    provider_name TEXT NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('pending', 'success', 'failed')),
    provider_code TEXT NOT NULL DEFAULT '',
    message TEXT NOT NULL DEFAULT '',
    serial_number TEXT NOT NULL DEFAULT '',
    price BIGINT NOT NULL DEFAULT 0,
    version BIGINT NOT NULL DEFAULT 1 CHECK (version > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT provider_transactions_identity_unique
        UNIQUE (reference_id, product_code, customer_no, provider_name)
);

CREATE INDEX IF NOT EXISTS provider_transactions_status_updated_idx
    ON provider_transactions (status, updated_at);

-- Atomic compare-and-transition primitive.
--
-- Preconditions:
--   * reference_id identifies the transaction.
--   * expected_version is the version read by the caller.
--   * the stored request identity and provider_name must match the expected values.
--   * only an existing pending row may transition.
--
-- A successful update increments version exactly once and returns the new row.
-- A zero-row result means the expected state was stale or the transaction was
-- already terminal; the caller must reload and reconcile rather than resubmit.
--
-- Example:
--
-- UPDATE provider_transactions
-- SET status = $2,
--     provider_code = $3,
--     message = $4,
--     serial_number = $5,
--     price = $6,
--     version = version + 1,
--     updated_at = CURRENT_TIMESTAMP
-- WHERE reference_id = $1
--   AND version = $7
--   AND product_code = $8
--   AND customer_no = $9
--   AND provider_name = $10
--   AND status = 'pending'
-- RETURNING *;
--
-- The request identity columns are deliberately part of the predicate so a
-- stale or mismatched caller cannot transition another request that happens
-- to reuse the same reference identifier.


-- Append-only transaction audit boundary.
-- Audit rows are never updated or deleted by the application lifecycle.
CREATE TABLE IF NOT EXISTS provider_transaction_audit (
    audit_id BIGSERIAL PRIMARY KEY,
    reference_id TEXT NOT NULL,
    action TEXT NOT NULL,
    previous_status TEXT NOT NULL DEFAULT '',
    next_status TEXT NOT NULL DEFAULT '',
    provider_name TEXT NOT NULL DEFAULT '',
    message TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS provider_transaction_audit_reference_created_idx
    ON provider_transaction_audit (reference_id, created_at, audit_id);

-- Audit insertion is append-only:
-- INSERT INTO provider_transaction_audit
--     (reference_id, action, previous_status, next_status, provider_name, message)
-- VALUES ($1, $2, $3, $4, $5, $6);
