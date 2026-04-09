CREATE TABLE IF NOT EXISTS config (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL,
    updated_at DATETIME NOT NULL DEFAULT (datetime('now'))
);

INSERT OR IGNORE INTO config (key, value, updated_at) VALUES
    ('request_body_max_bytes', '51200', datetime('now'));

INSERT OR IGNORE INTO config (key, value, updated_at) VALUES
    ('response_body_max_bytes', '51200', datetime('now'));

INSERT OR IGNORE INTO config (key, value, updated_at) VALUES
    ('track_streaming', 'false', datetime('now'));

INSERT OR IGNORE INTO config (key, value, updated_at) VALUES
    ('streaming_buffer_size', '10240', datetime('now'));
