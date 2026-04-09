INSERT OR IGNORE INTO config (key, value, updated_at) VALUES
    ('request_log_body_retention_days', '30', datetime('now'));

INSERT OR IGNORE INTO config (key, value, updated_at) VALUES
    ('response_log_body_retention_days', '30', datetime('now'));
