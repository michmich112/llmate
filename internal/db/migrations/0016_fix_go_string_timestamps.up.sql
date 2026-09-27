-- modernc.org/sqlite writes time.Time with time.Time.String() when the DSN
-- omits _time_format=sqlite. That text ("YYYY-MM-DD HH:MM:SS.nnn +0000 UTC")
-- does not compare equal to the sqlite format ("...+00:00") used everywhere
-- else, so range filters dropped those rows and scanners rejected them.
-- All application timestamps are UTC, so the suffix is always " +0000 UTC".
UPDATE request_logs SET timestamp = replace(timestamp, ' +0000 UTC', '') || '+00:00' WHERE timestamp LIKE '% +0000 UTC';
UPDATE request_logs SET created_at = replace(created_at, ' +0000 UTC', '') || '+00:00' WHERE created_at LIKE '% +0000 UTC';
UPDATE streaming_logs SET timestamp = replace(timestamp, ' +0000 UTC', '') || '+00:00' WHERE timestamp LIKE '% +0000 UTC';
UPDATE streaming_logs SET created_at = replace(created_at, ' +0000 UTC', '') || '+00:00' WHERE created_at LIKE '% +0000 UTC';
UPDATE providers SET created_at = replace(created_at, ' +0000 UTC', '') || '+00:00' WHERE created_at LIKE '% +0000 UTC';
UPDATE providers SET updated_at = replace(updated_at, ' +0000 UTC', '') || '+00:00' WHERE updated_at LIKE '% +0000 UTC';
UPDATE providers SET health_checked_at = replace(health_checked_at, ' +0000 UTC', '') || '+00:00' WHERE health_checked_at LIKE '% +0000 UTC';
UPDATE provider_endpoints SET created_at = replace(created_at, ' +0000 UTC', '') || '+00:00' WHERE created_at LIKE '% +0000 UTC';
UPDATE provider_models SET created_at = replace(created_at, ' +0000 UTC', '') || '+00:00' WHERE created_at LIKE '% +0000 UTC';
UPDATE model_aliases SET created_at = replace(created_at, ' +0000 UTC', '') || '+00:00' WHERE created_at LIKE '% +0000 UTC';
UPDATE model_aliases SET updated_at = replace(updated_at, ' +0000 UTC', '') || '+00:00' WHERE updated_at LIKE '% +0000 UTC';
UPDATE config SET updated_at = replace(updated_at, ' +0000 UTC', '') || '+00:00' WHERE updated_at LIKE '% +0000 UTC';
