package db

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/llmate/gateway/internal/models"
)

func (s *SQLiteStore) CreateAPIKey(ctx context.Context, k *models.APIKey) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO api_keys
		 (id, key_hash, name, is_active, rate_limit_rpm, rate_limit_tpm, created_at, last_used_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		k.ID, k.KeyHash, k.Name, k.IsActive,
		nullInt(k.RateLimitRPM), nullInt(k.RateLimitTPM),
		k.CreatedAt, nullTime(k.LastUsedAt),
	)
	if err != nil {
		return fmt.Errorf("create api key: %w", err)
	}
	return nil
}

func (s *SQLiteStore) GetAPIKeyByHash(ctx context.Context, keyHash string) (*models.APIKey, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, key_hash, name, is_active, rate_limit_rpm, rate_limit_tpm, created_at, last_used_at
		 FROM api_keys WHERE key_hash = ?`,
		keyHash,
	)
	k, err := scanAPIKey(row.Scan)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get api key by hash: %w", err)
	}
	return k, nil
}

func (s *SQLiteStore) GetAPIKey(ctx context.Context, id string) (*models.APIKey, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, key_hash, name, is_active, rate_limit_rpm, rate_limit_tpm, created_at, last_used_at
		 FROM api_keys WHERE id = ?`,
		id,
	)
	k, err := scanAPIKey(row.Scan)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, sql.ErrNoRows
		}
		return nil, fmt.Errorf("get api key: %w", err)
	}
	return k, nil
}

func (s *SQLiteStore) ListAPIKeys(ctx context.Context) ([]models.APIKey, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, key_hash, name, is_active, rate_limit_rpm, rate_limit_tpm, created_at, last_used_at
		 FROM api_keys ORDER BY created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("list api keys: %w", err)
	}
	defer rows.Close()

	var keys []models.APIKey
	for rows.Next() {
		k, err := scanAPIKey(rows.Scan)
		if err != nil {
			return nil, fmt.Errorf("list api keys scan: %w", err)
		}
		keys = append(keys, *k)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list api keys rows: %w", err)
	}
	return keys, nil
}

func (s *SQLiteStore) UpdateAPIKey(ctx context.Context, k *models.APIKey) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE api_keys
		 SET name = ?, is_active = ?, rate_limit_rpm = ?, rate_limit_tpm = ?
		 WHERE id = ?`,
		k.Name, k.IsActive, nullInt(k.RateLimitRPM), nullInt(k.RateLimitTPM), k.ID,
	)
	if err != nil {
		return fmt.Errorf("update api key: %w", err)
	}
	return nil
}

func (s *SQLiteStore) DeleteAPIKey(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM api_keys WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete api key: %w", err)
	}
	return nil
}

func (s *SQLiteStore) TouchAPIKeyLastUsed(ctx context.Context, id string, at time.Time) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE api_keys SET last_used_at = ? WHERE id = ?`,
		at, id,
	)
	if err != nil {
		return fmt.Errorf("touch api key last used: %w", err)
	}
	return nil
}

func (s *SQLiteStore) QueryLogsByAPIKey(ctx context.Context, apiKeyID string, filter models.LogFilter) ([]models.RequestLog, int, error) {
	conditions := []string{"api_key_id = ?"}
	args := []interface{}{apiKeyID}

	if filter.Since != nil {
		conditions = append(conditions, "timestamp >= ?")
		args = append(args, *filter.Since)
	}
	if filter.Until != nil {
		conditions = append(conditions, "timestamp <= ?")
		args = append(args, *filter.Until)
	}
	if filter.StatusMin > 0 {
		conditions = append(conditions, "status_code >= ?")
		args = append(args, filter.StatusMin)
	}
	if filter.StatusMax > 0 {
		conditions = append(conditions, "status_code <= ?")
		args = append(args, filter.StatusMax)
	}

	where := "WHERE " + strings.Join(conditions, " AND ")

	var total int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM request_logs "+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("query logs by api key count: %w", err)
	}

	limit := filter.Limit
	if limit <= 0 {
		limit = 100
	}

	dataArgs := append(args, limit, filter.Offset)
	rows, err := s.db.QueryContext(ctx,
		"SELECT "+requestLogCols+" FROM request_logs "+where+" ORDER BY timestamp DESC LIMIT ? OFFSET ?",
		dataArgs...,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("query logs by api key: %w", err)
	}
	defer rows.Close()

	var logs []models.RequestLog
	for rows.Next() {
		l, err := scanRequestLog(rows.Scan)
		if err != nil {
			return nil, 0, fmt.Errorf("query logs by api key scan: %w", err)
		}
		logs = append(logs, l)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("query logs by api key rows: %w", err)
	}
	return logs, total, nil
}

func (s *SQLiteStore) UsageByAPIKey(ctx context.Context, since, until time.Time) ([]models.APIKeyUsage, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT api_key_id, api_key_name,
		        COUNT(*) AS request_count,
		        SUM(IFNULL(total_tokens, 0)) AS total_tokens,
		        SUM(IFNULL(estimated_cost_usd, 0)) AS total_cost_usd,
		        MAX(timestamp) AS last_used_at
		 FROM request_logs
		 WHERE api_key_id IS NOT NULL AND api_key_id != ''
		   AND timestamp >= ? AND timestamp <= ?
		 GROUP BY api_key_id, api_key_name
		 ORDER BY total_cost_usd DESC`,
		since, until,
	)
	if err != nil {
		return nil, fmt.Errorf("usage by api key: %w", err)
	}
	defer rows.Close()

	var usage []models.APIKeyUsage
	for rows.Next() {
		var u models.APIKeyUsage
		var lastUsed nullTimeScanner
		if err := rows.Scan(&u.APIKeyID, &u.APIKeyName, &u.RequestCount, &u.TotalTokens, &u.TotalCostUSD, &lastUsed); err != nil {
			return nil, fmt.Errorf("usage by api key scan: %w", err)
		}
		if lastUsed.Valid {
			t := lastUsed.Time
			u.LastUsedAt = &t
		}
		usage = append(usage, u)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("usage by api key rows: %w", err)
	}
	return usage, nil
}

// UsageByAPIKeyModel aggregates per-model usage for a single API key.
func (s *SQLiteStore) UsageByAPIKeyModel(ctx context.Context, apiKeyID string, since, until time.Time) ([]models.ModelStats, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT
			COALESCE(NULLIF(resolved_model, ''), requested_model, '') AS model,
			COUNT(*) AS request_count,
			AVG(CAST(total_time_ms AS REAL)) AS avg_latency_ms,
			SUM(CASE WHEN status_code >= 400 THEN 1 ELSE 0 END) AS error_count,
			COALESCE(SUM(COALESCE(total_tokens, 0)), 0) AS total_tokens
		FROM request_logs
		WHERE api_key_id = ? AND timestamp >= ? AND timestamp <= ?
		GROUP BY COALESCE(NULLIF(resolved_model, ''), requested_model, '')
		ORDER BY request_count DESC
	`, apiKeyID, since, until)
	if err != nil {
		return nil, fmt.Errorf("usage by api key model: %w", err)
	}
	defer rows.Close()

	byModel := []models.ModelStats{}
	for rows.Next() {
		var ms models.ModelStats
		var avgLatency sql.NullFloat64
		if err := rows.Scan(&ms.Model, &ms.RequestCount, &avgLatency, &ms.ErrorCount, &ms.TotalTokens); err != nil {
			return nil, fmt.Errorf("usage by api key model scan: %w", err)
		}
		if avgLatency.Valid {
			ms.AvgLatencyMs = avgLatency.Float64
		}
		byModel = append(byModel, ms)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("usage by api key model rows: %w", err)
	}
	return byModel, nil
}

// scanAPIKey scans a single api_keys row into *models.APIKey.
func scanAPIKey(scan func(...any) error) (*models.APIKey, error) {
	var k models.APIKey
	var rpm, tpm sql.NullInt64
	var lastUsed nullTimeScanner
	if err := scan(&k.ID, &k.KeyHash, &k.Name, &k.IsActive, &rpm, &tpm, &k.CreatedAt, &lastUsed); err != nil {
		if err == sql.ErrNoRows {
			return nil, sql.ErrNoRows
		}
		return nil, err
	}
	if rpm.Valid {
		v := int(rpm.Int64)
		k.RateLimitRPM = &v
	}
	if tpm.Valid {
		v := int(tpm.Int64)
		k.RateLimitTPM = &v
	}
	if lastUsed.Valid {
		t := lastUsed.Time
		k.LastUsedAt = &t
	}
	return &k, nil
}
