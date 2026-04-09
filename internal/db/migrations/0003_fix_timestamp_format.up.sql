UPDATE request_logs SET timestamp = substr(timestamp, 1, 26) || '+00:00' WHERE timestamp LIKE '% +0000 UTC%';
UPDATE request_logs SET created_at = substr(created_at, 1, 26) || '+00:00' WHERE created_at LIKE '% +0000 UTC%';
UPDATE providers SET created_at = substr(created_at, 1, 26) || '+00:00' WHERE created_at LIKE '% +0000 UTC%';
UPDATE providers SET updated_at = substr(updated_at, 1, 26) || '+00:00' WHERE updated_at LIKE '% +0000 UTC%';
UPDATE providers SET health_checked_at = substr(health_checked_at, 1, 26) || '+00:00' WHERE health_checked_at LIKE '% +0000 UTC%';
UPDATE provider_models SET created_at = substr(created_at, 1, 26) || '+00:00' WHERE created_at LIKE '% +0000 UTC%';
UPDATE model_aliases SET created_at = substr(created_at, 1, 26) || '+00:00' WHERE created_at LIKE '% +0000 UTC%';
UPDATE model_aliases SET updated_at = substr(updated_at, 1, 26) || '+00:00' WHERE updated_at LIKE '% +0000 UTC%';
