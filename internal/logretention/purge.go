package logretention

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/llmate/gateway/internal/db"
	"github.com/llmate/gateway/internal/models"
)

func validateRetentionDays(days int) error {
	if days < models.MinStreamingLogBodyRetentionDays || days > models.MaxStreamingLogBodyRetentionDays {
		return fmt.Errorf("retention days out of range")
	}
	return nil
}

// PurgeStreamingChunkBodies clears streaming_logs chunk payloads older than the retention window.
func PurgeStreamingChunkBodies(ctx context.Context, store db.Store, days int) (int64, error) {
	if err := validateRetentionDays(days); err != nil {
		return 0, err
	}
	cutoff := time.Now().UTC().AddDate(0, 0, -days)
	return store.PurgeStreamingLogBodiesOlderThan(ctx, cutoff)
}

// PurgeRequestLogRequestBodies clears stored request_body text on request_logs older than the retention window.
func PurgeRequestLogRequestBodies(ctx context.Context, store db.Store, days int) (int64, error) {
	if err := validateRetentionDays(days); err != nil {
		return 0, err
	}
	cutoff := time.Now().UTC().AddDate(0, 0, -days)
	return store.PurgeRequestLogRequestBodiesOlderThan(ctx, cutoff)
}

// PurgeRequestLogResponseBodies clears stored response_body text on request_logs older than the retention window.
func PurgeRequestLogResponseBodies(ctx context.Context, store db.Store, days int) (int64, error) {
	if err := validateRetentionDays(days); err != nil {
		return 0, err
	}
	cutoff := time.Now().UTC().AddDate(0, 0, -days)
	return store.PurgeRequestLogResponseBodiesOlderThan(ctx, cutoff)
}

// StreamingRetentionDaysFromConfig returns effective streaming chunk body retention days.
// When the key is missing, returns (DefaultStreamingLogBodyRetentionDays, true).
func StreamingRetentionDaysFromConfig(config map[string]string) (days int, ok bool) {
	return retentionDaysFromKey(config, "streaming_log_body_retention_days", models.DefaultStreamingLogBodyRetentionDays)
}

// RequestLogBodyRetentionDaysFromConfig returns effective request body retention days on request_logs.
func RequestLogBodyRetentionDaysFromConfig(config map[string]string) (days int, ok bool) {
	return retentionDaysFromKey(config, "request_log_body_retention_days", models.DefaultRequestLogBodyRetentionDays)
}

// ResponseLogBodyRetentionDaysFromConfig returns effective response body retention days on request_logs.
func ResponseLogBodyRetentionDaysFromConfig(config map[string]string) (days int, ok bool) {
	return retentionDaysFromKey(config, "response_log_body_retention_days", models.DefaultResponseLogBodyRetentionDays)
}

func retentionDaysFromKey(config map[string]string, key string, defaultDays int) (days int, ok bool) {
	val, exists := config[key]
	if !exists {
		return defaultDays, true
	}
	d, err := strconv.Atoi(val)
	if err != nil {
		return 0, false
	}
	if d < models.MinStreamingLogBodyRetentionDays || d > models.MaxStreamingLogBodyRetentionDays {
		return 0, false
	}
	return d, true
}
