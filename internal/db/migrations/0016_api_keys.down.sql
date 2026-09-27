DROP INDEX IF EXISTS idx_request_logs_api_key_id;
ALTER TABLE request_logs DROP COLUMN api_key_name;
ALTER TABLE request_logs DROP COLUMN api_key_id;
DROP TABLE IF EXISTS api_keys;
