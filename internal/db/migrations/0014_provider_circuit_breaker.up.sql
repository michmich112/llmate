ALTER TABLE providers ADD COLUMN circuit_breaker_enabled BOOLEAN NOT NULL DEFAULT 1;
ALTER TABLE providers ADD COLUMN circuit_breaker_error_threshold REAL NOT NULL DEFAULT 0.5;
ALTER TABLE providers ADD COLUMN circuit_breaker_window_seconds INTEGER NOT NULL DEFAULT 60;
ALTER TABLE providers ADD COLUMN circuit_breaker_cooldown_seconds INTEGER NOT NULL DEFAULT 30;
