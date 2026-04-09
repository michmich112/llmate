package db

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/llmate/gateway/internal/models"
)

func (s *SQLiteStore) GetDashboardStats(ctx context.Context, since time.Time) (*models.DashboardStats, error) {
	stats := &models.DashboardStats{
		ByModel:    []models.ModelStats{},
		ByProvider: []models.ProviderStats{},
	}

	if err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM request_logs WHERE timestamp >= ?`, since,
	).Scan(&stats.TotalRequests); err != nil {
		return nil, fmt.Errorf("dashboard stats total requests: %w", err)
	}

	if stats.TotalRequests > 0 {
		var avgLatency sql.NullFloat64
		if err := s.db.QueryRowContext(ctx,
			`SELECT AVG(CAST(total_time_ms AS REAL)) FROM request_logs WHERE timestamp >= ?`, since,
		).Scan(&avgLatency); err != nil {
			return nil, fmt.Errorf("dashboard stats avg latency: %w", err)
		}
		if avgLatency.Valid {
			stats.AvgLatencyMs = avgLatency.Float64
		}

		var errorCount sql.NullInt64
		if err := s.db.QueryRowContext(ctx,
			`SELECT SUM(CASE WHEN status_code >= 400 THEN 1 ELSE 0 END) FROM request_logs WHERE timestamp >= ?`, since,
		).Scan(&errorCount); err != nil {
			return nil, fmt.Errorf("dashboard stats error count: %w", err)
		}
		if errorCount.Valid {
			stats.ErrorRate = float64(errorCount.Int64) / float64(stats.TotalRequests)
		}
	}

	modelRows, err := s.db.QueryContext(ctx, `
		SELECT
			COALESCE(requested_model, '') AS model,
			COUNT(*) AS request_count,
			AVG(CAST(total_time_ms AS REAL)) AS avg_latency_ms,
			SUM(CASE WHEN status_code >= 400 THEN 1 ELSE 0 END) AS error_count,
			COALESCE(SUM(COALESCE(total_tokens, 0)), 0) AS total_tokens
		FROM request_logs
		WHERE timestamp >= ?
		GROUP BY requested_model
		ORDER BY request_count DESC
	`, since)
	if err != nil {
		return nil, fmt.Errorf("dashboard stats by model: %w", err)
	}
	defer modelRows.Close()

	for modelRows.Next() {
		var ms models.ModelStats
		var avgLatency sql.NullFloat64
		if err := modelRows.Scan(&ms.Model, &ms.RequestCount, &avgLatency, &ms.ErrorCount, &ms.TotalTokens); err != nil {
			return nil, fmt.Errorf("dashboard stats by model scan: %w", err)
		}
		if avgLatency.Valid {
			ms.AvgLatencyMs = avgLatency.Float64
		}
		stats.ByModel = append(stats.ByModel, ms)
	}
	if err := modelRows.Err(); err != nil {
		return nil, fmt.Errorf("dashboard stats by model rows: %w", err)
	}

	provRows, err := s.db.QueryContext(ctx, `
		SELECT
			COALESCE(r.provider_id, '') AS provider_id,
			COALESCE(p.name, r.provider_name, '') AS provider_name,
			COUNT(*) AS request_count,
			AVG(CAST(r.total_time_ms AS REAL)) AS avg_latency_ms,
			SUM(CASE WHEN r.status_code >= 400 THEN 1 ELSE 0 END) AS error_count
		FROM request_logs r
		LEFT JOIN providers p ON p.id = r.provider_id
		WHERE r.timestamp >= ?
		GROUP BY r.provider_id
		ORDER BY request_count DESC
	`, since)
	if err != nil {
		return nil, fmt.Errorf("dashboard stats by provider: %w", err)
	}
	defer provRows.Close()

	for provRows.Next() {
		var ps models.ProviderStats
		var avgLatency sql.NullFloat64
		if err := provRows.Scan(&ps.ProviderID, &ps.ProviderName, &ps.RequestCount, &avgLatency, &ps.ErrorCount); err != nil {
			return nil, fmt.Errorf("dashboard stats by provider scan: %w", err)
		}
		if avgLatency.Valid {
			ps.AvgLatencyMs = avgLatency.Float64
		}
		stats.ByProvider = append(stats.ByProvider, ps)
	}
	if err := provRows.Err(); err != nil {
		return nil, fmt.Errorf("dashboard stats by provider rows: %w", err)
	}

	return stats, nil
}

// GetTimeSeries returns request metrics bucketed by time between since and until.
// granularity must be "hour" or "day".
// Hourly buckets use format "2006-01-02T15:00:00"; daily buckets use "2006-01-02".
func (s *SQLiteStore) GetTimeSeries(ctx context.Context, since, until time.Time, granularity string) ([]models.TimeSeriesPoint, error) {
	// substr(timestamp, 1, 19) extracts "YYYY-MM-DD HH:MM:SS" which SQLite's
	// date functions always parse correctly, regardless of what follows (timezone
	// offset variants, fractional seconds, corrupt "++" sequences from old
	// migration runs, etc.).
	var bucketExpr string
	switch granularity {
	case "hour":
		bucketExpr = `strftime('%Y-%m-%dT%H:00:00', substr(timestamp, 1, 19))`
	case "day":
		bucketExpr = `strftime('%Y-%m-%d', substr(timestamp, 1, 10))`
	default:
		return nil, fmt.Errorf("invalid granularity %q: must be hour or day", granularity)
	}

	// Query with cost breakdown calculation using a join to provider_models.
	// Per-request cost rules must match internal/pricing.ForRequestLog (subtract cached from prompt for input rate; cache-read rate on cached_tokens).
	rows, err := s.db.QueryContext(ctx, `
		SELECT
			`+bucketExpr+` AS bucket,
			COUNT(*) AS requests,
			COUNT(CASE WHEN status_code >= 200 AND status_code < 300 THEN 1 END) AS success_count,
			COUNT(CASE WHEN status_code >= 400 THEN 1 END) AS error_count,
			COALESCE(SUM(COALESCE(prompt_tokens, 0)), 0) - COALESCE(SUM(COALESCE(cached_tokens, 0)), 0) AS input_tokens,
			COALESCE(SUM(COALESCE(prompt_tokens, 0)), 0) AS prompt_tokens,
			COALESCE(SUM(COALESCE(completion_tokens, 0)), 0) AS completion_tokens,
			COALESCE(SUM(COALESCE(total_tokens, 0)), 0) AS total_tokens,
			-- Recalculate total cost from token counts to avoid rounding errors from stored estimates
			COALESCE(SUM(
				CASE 
				 WHEN pm.cost_per_million_input IS NOT NULL AND prompt_tokens IS NOT NULL 
				 THEN CAST(MAX(0, prompt_tokens - COALESCE(cached_tokens, 0)) AS REAL) / 1000000 * pm.cost_per_million_input
				 ELSE 0 
				END
			), 0)
			+
			COALESCE(SUM(
				CASE 
				 WHEN pm.cost_per_million_output IS NOT NULL AND completion_tokens IS NOT NULL 
				 THEN CAST(completion_tokens AS REAL) / 1000000 * pm.cost_per_million_output
				 ELSE 0 
				END
			), 0)
			+
			COALESCE(SUM(
				CASE 
				 WHEN pm.cost_per_million_cache_read IS NOT NULL AND cached_tokens IS NOT NULL AND cached_tokens > 0
				 THEN CAST(cached_tokens AS REAL) / 1000000 * pm.cost_per_million_cache_read
				 ELSE 0 
				END
			), 0) AS total_cost_usd,
			COALESCE(SUM(
				CASE 
				 WHEN pm.cost_per_million_input IS NOT NULL AND prompt_tokens IS NOT NULL 
				 THEN CAST(MAX(0, prompt_tokens - COALESCE(cached_tokens, 0)) AS REAL) / 1000000 * pm.cost_per_million_input
				 ELSE 0 
				END
			), 0) AS input_cost_usd,
			COALESCE(SUM(
				CASE 
				 WHEN pm.cost_per_million_output IS NOT NULL AND completion_tokens IS NOT NULL 
				 THEN CAST(completion_tokens AS REAL) / 1000000 * pm.cost_per_million_output
				 ELSE 0 
				END
			), 0) AS output_cost_usd,
			COALESCE(SUM(
				CASE 
				 WHEN pm.cost_per_million_cache_read IS NOT NULL AND cached_tokens IS NOT NULL AND cached_tokens > 0
				 THEN CAST(cached_tokens AS REAL) / 1000000 * pm.cost_per_million_cache_read
				 ELSE 0 
				END
			), 0) AS cached_cost_usd,
			COALESCE(SUM(COALESCE(cached_tokens, 0)), 0) AS cached_tokens
		FROM request_logs r
		LEFT JOIN provider_models pm ON r.provider_id = pm.provider_id AND r.resolved_model = pm.model_id
		WHERE r.timestamp >= ? AND r.timestamp <= ?
		GROUP BY bucket
		ORDER BY bucket ASC
	`, since, until)
	if err != nil {
		return nil, fmt.Errorf("get time series: %w", err)
	}
	defer rows.Close()

	// Collect SQL results into a map keyed by bucket string.
	dataByBucket := make(map[string]models.TimeSeriesPoint)
	for rows.Next() {
		var p models.TimeSeriesPoint
		if err := rows.Scan(&p.Bucket, &p.Requests, &p.SuccessCount, &p.ErrorCount, &p.InputTokens, &p.PromptTokens, &p.CompletionTokens, &p.TotalTokens, &p.TotalCostUSD, &p.InputCostUSD, &p.OutputCostUSD, &p.CachedCostUSD, &p.CachedTokens); err != nil {
			return nil, fmt.Errorf("get time series scan: %w", err)
		}
		dataByBucket[p.Bucket] = p
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("get time series rows: %w", err)
	}

	// Generate every expected bucket in [since, until] and merge in real data,
	// filling gaps with zero-value points so the chart always shows a full axis.
	var points []models.TimeSeriesPoint
	switch granularity {
	case "hour":
		for t := since.UTC().Truncate(time.Hour); !t.After(until.UTC()); t = t.Add(time.Hour) {
			bucket := t.Format("2006-01-02T15:00:00")
			if p, ok := dataByBucket[bucket]; ok {
				points = append(points, p)
			} else {
				points = append(points, models.TimeSeriesPoint{Bucket: bucket})
			}
		}
	case "day":
		for t := since.UTC().Truncate(24 * time.Hour); !t.After(until.UTC()); t = t.Add(24 * time.Hour) {
			bucket := t.Format("2006-01-02")
			if p, ok := dataByBucket[bucket]; ok {
				points = append(points, p)
			} else {
				points = append(points, models.TimeSeriesPoint{Bucket: bucket})
			}
		}
	}
	return points, nil
}
