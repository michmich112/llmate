-- Replace global chunk_index index with a composite suited to per-request lookups.
DROP INDEX IF EXISTS idx_streaming_logs_chunk;

CREATE INDEX IF NOT EXISTS idx_streaming_logs_request_chunk ON streaming_logs(request_log_id, chunk_index);
