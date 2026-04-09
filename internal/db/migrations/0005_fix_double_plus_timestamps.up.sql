-- Migration 0003 produced "YYYY-MM-DD HH:MM:SS.SSS ++00:00" (double-plus) for
-- timestamps whose fractional seconds happened to start with a digit that placed
-- the 27th character at the '+' of "+0000 UTC". The pattern used by migration
-- 0004 ("% +0%+00:00") does not match "++00:00", so those rows were not fixed.
-- Strip everything after the 19-character datetime prefix and append the correct
-- UTC offset.  Sub-second precision is lost but is acceptable for all timestamp
-- columns in this project.
UPDATE request_logs SET timestamp  = substr(timestamp,  1, 19) || '+00:00' WHERE timestamp  LIKE '%++%';
UPDATE request_logs SET created_at = substr(created_at, 1, 19) || '+00:00' WHERE created_at LIKE '%++%';
UPDATE providers    SET created_at = substr(created_at, 1, 19) || '+00:00' WHERE created_at LIKE '%++%';
UPDATE providers    SET updated_at = substr(updated_at, 1, 19) || '+00:00' WHERE updated_at LIKE '%++%';
UPDATE providers    SET health_checked_at = substr(health_checked_at, 1, 19) || '+00:00' WHERE health_checked_at LIKE '%++%';
UPDATE provider_models SET created_at = substr(created_at, 1, 19) || '+00:00' WHERE created_at LIKE '%++%';
UPDATE model_aliases SET created_at = substr(created_at, 1, 19) || '+00:00' WHERE created_at LIKE '%++%';
UPDATE model_aliases SET updated_at = substr(updated_at, 1, 19) || '+00:00' WHERE updated_at LIKE '%++%';
