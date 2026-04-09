ALTER TABLE provider_models ADD COLUMN cost_per_million_input REAL;
ALTER TABLE provider_models ADD COLUMN cost_per_million_output REAL;
ALTER TABLE provider_models ADD COLUMN cost_per_million_cache_read REAL;
ALTER TABLE provider_models ADD COLUMN cost_per_million_cache_write REAL;

ALTER TABLE request_logs ADD COLUMN estimated_cost_usd REAL;
ALTER TABLE request_logs ADD COLUMN request_body TEXT;
ALTER TABLE request_logs ADD COLUMN response_body TEXT;
