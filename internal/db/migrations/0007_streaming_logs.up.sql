CREATE TABLE IF NOT EXISTS streaming_logs (
    id TEXT PRIMARY KEY,
    request_log_id TEXT NOT NULL REFERENCES request_logs(id) ON DELETE CASCADE,
    chunk_index INTEGER NOT NULL,
    timestamp DATETIME NOT NULL,
    data TEXT NOT NULL,
    is_truncated BOOLEAN NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_streaming_logs_request ON streaming_logs(request_log_id);
CREATE INDEX IF NOT EXISTS idx_streaming_logs_request_chunk ON streaming_logs(request_log_id, chunk_index);
CREATE INDEX IF NOT EXISTS idx_streaming_logs_timestamp ON streaming_logs(timestamp);
