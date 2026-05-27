INSERT OR IGNORE INTO config (key, value, updated_at) VALUES
    ('http_idle_conn_timeout_seconds', '90', datetime('now'));
