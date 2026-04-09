-- Migration 0003 used substr(col, 1, 26) which assumed exactly 6 decimal places.
-- Go's time.String() uses the minimum digits needed (0, 3, 6, or 9), so values
-- with fewer than 6 decimals were corrupted (e.g. "...29.211 +0+00:00").
-- This migration fixes both remaining old-format values and 0003-corrupted values.

-- Pass 1: remaining old-format values ("... +0000 UTC").
-- Use replace() to strip the known suffix, preserving all decimal precision.
UPDATE request_logs SET timestamp     = replace(timestamp,     ' +0000 UTC', '') || '+00:00' WHERE timestamp     LIKE '% +0000 UTC';
UPDATE request_logs SET created_at    = replace(created_at,    ' +0000 UTC', '') || '+00:00' WHERE created_at    LIKE '% +0000 UTC';
UPDATE providers    SET created_at    = replace(created_at,    ' +0000 UTC', '') || '+00:00' WHERE created_at    LIKE '% +0000 UTC';
UPDATE providers    SET updated_at    = replace(updated_at,    ' +0000 UTC', '') || '+00:00' WHERE updated_at    LIKE '% +0000 UTC';
UPDATE providers    SET health_checked_at = replace(health_checked_at, ' +0000 UTC', '') || '+00:00' WHERE health_checked_at LIKE '% +0000 UTC';
UPDATE provider_models SET created_at = replace(created_at,   ' +0000 UTC', '') || '+00:00' WHERE created_at    LIKE '% +0000 UTC';
UPDATE model_aliases SET created_at   = replace(created_at,   ' +0000 UTC', '') || '+00:00' WHERE created_at    LIKE '% +0000 UTC';
UPDATE model_aliases SET updated_at   = replace(updated_at,   ' +0000 UTC', '') || '+00:00' WHERE updated_at    LIKE '% +0000 UTC';

-- Pass 2: values corrupted by migration 0003 (space + partial offset before +00:00).
-- Pattern "% +0%+00:00" identifies strings that have a " +0" fragment embedded before
-- the final +00:00, which valid converted values never have (their +00:00 is attached
-- directly to the fractional seconds with no preceding space).
-- substr(col, 1, 19) safely extracts "YYYY-MM-DD HH:MM:SS" (loses sub-second precision,
-- acceptable for all timestamp columns in this project).
UPDATE request_logs SET timestamp     = substr(timestamp,     1, 19) || '+00:00' WHERE timestamp     LIKE '% +0%+00:00';
UPDATE request_logs SET created_at    = substr(created_at,    1, 19) || '+00:00' WHERE created_at    LIKE '% +0%+00:00';
UPDATE providers    SET created_at    = substr(created_at,    1, 19) || '+00:00' WHERE created_at    LIKE '% +0%+00:00';
UPDATE providers    SET updated_at    = substr(updated_at,    1, 19) || '+00:00' WHERE updated_at    LIKE '% +0%+00:00';
UPDATE providers    SET health_checked_at = substr(health_checked_at, 1, 19) || '+00:00' WHERE health_checked_at LIKE '% +0%+00:00';
UPDATE provider_models SET created_at = substr(created_at,   1, 19) || '+00:00' WHERE created_at    LIKE '% +0%+00:00';
UPDATE model_aliases SET created_at   = substr(created_at,   1, 19) || '+00:00' WHERE created_at    LIKE '% +0%+00:00';
UPDATE model_aliases SET updated_at   = substr(updated_at,   1, 19) || '+00:00' WHERE updated_at    LIKE '% +0%+00:00';
