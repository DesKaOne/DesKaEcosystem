CREATE TABLE IF NOT EXISTS provider_operational_snapshots (
    provider_name TEXT PRIMARY KEY,
    balance BIGINT NOT NULL,
    currency TEXT NOT NULL,
    health TEXT NOT NULL,
    last_checked_at TIMESTAMPTZ NOT NULL,
    last_success_at TIMESTAMPTZ NOT NULL,
    last_error TEXT NOT NULL DEFAULT '',
    consecutive_failures INTEGER NOT NULL DEFAULT 0
);
