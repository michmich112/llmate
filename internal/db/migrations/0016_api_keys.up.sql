CREATE TABLE IF NOT EXISTS api_keys (
    id TEXT PRIMARY KEY,
    key_hash TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT 1,
    rate_limit_rpm INTEGER,
    rate_limit_tpm INTEGER,
    created_at DATETIME NOT NULL,
    last_used_at DATETIME
);

ALTER TABLE request_logs ADD COLUMN api_key_id TEXT;
ALTER TABLE request_logs ADD COLUMN api_key_name TEXT;

CREATE INDEX IF NOT EXISTS idx_request_logs_api_key_id ON request_logs(api_key_id);
