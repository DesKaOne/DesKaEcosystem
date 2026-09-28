-- DesKaProvider v0.1 payment correlation extension.
-- Extends the existing durable provider transaction/audit boundary; no parallel
-- payment persistence table is introduced.

ALTER TABLE provider_transactions
    ADD COLUMN IF NOT EXISTS transaction_kind TEXT NOT NULL DEFAULT 'ppob',
    ADD COLUMN IF NOT EXISTS payment_provider_reference TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS payment_currency TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS payment_customer_id TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS payment_description TEXT NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS provider_transactions_kind_status_idx
    ON provider_transactions (transaction_kind, status, updated_at);
