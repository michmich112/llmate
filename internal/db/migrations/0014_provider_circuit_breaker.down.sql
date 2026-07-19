ALTER TABLE providers DROP COLUMN circuit_breaker_enabled;
ALTER TABLE providers DROP COLUMN circuit_breaker_error_threshold;
ALTER TABLE providers DROP COLUMN circuit_breaker_window_seconds;
ALTER TABLE providers DROP COLUMN circuit_breaker_cooldown_seconds;
