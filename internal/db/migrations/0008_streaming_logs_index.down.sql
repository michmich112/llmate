DROP INDEX IF EXISTS idx_streaming_logs_request_chunk;

CREATE INDEX IF NOT EXISTS idx_streaming_logs_chunk ON streaming_logs(chunk_index);
