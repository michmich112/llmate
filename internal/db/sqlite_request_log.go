package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/llmate/gateway/internal/models"
)

const requestLogCols = `id, timestamp, client_ip, method, path, requested_model, resolved_model,
	provider_id, provider_name, status_code, is_streamed, ttft_ms, total_time_ms,
	prompt_tokens, completion_tokens, total_tokens, cached_tokens, error_message, created_at,
	estimated_cost_usd`

// applyRequestLogFields populates the nullable fields of log from the scanned null-wrapper values.
// Shared between scanRequestLog and scanRequestLogFull to avoid duplication.
func applyRequestLogFields(
	log *models.RequestLog,
	timestamp, createdAt timeScanner,
	requestedModel, resolvedModel, providerID, providerName, errorMessage sql.NullString,
	ttftMs, promptTokens, completionTokens, totalTokens, cachedTokens sql.NullInt64,
) {
	log.Timestamp = timestamp.Time
	log.CreatedAt = createdAt.Time
	if requestedModel.Valid {
		log.RequestedModel = requestedModel.String
	}
	if resolvedModel.Valid {
		log.ResolvedModel = resolvedModel.String
	}
	if providerID.Valid {
		log.ProviderID = providerID.String
	}
	if providerName.Valid {
		log.ProviderName = providerName.String
	}
	if errorMessage.Valid {
		log.ErrorMessage = errorMessage.String
	}
	if ttftMs.Valid {
		v := int(ttftMs.Int64)
		log.TTFTMs = &v
	}
	if promptTokens.Valid {
		v := int(promptTokens.Int64)
		log.PromptTokens = &v
	}
	if completionTokens.Valid {
		v := int(completionTokens.Int64)
		log.CompletionTokens = &v
	}
	if totalTokens.Valid {
		v := int(totalTokens.Int64)
		log.TotalTokens = &v
	}
	if cachedTokens.Valid {
		v := int(cachedTokens.Int64)
		log.CachedTokens = &v
	}
}

func scanRequestLog(scan func(...any) error) (models.RequestLog, error) {
	var log models.RequestLog
	var requestedModel, resolvedModel, providerID, providerName, errorMessage sql.NullString
	var ttftMs, promptTokens, completionTokens, totalTokens, cachedTokens sql.NullInt64
	var estimatedCost sql.NullFloat64
	var timestamp, createdAt timeScanner

	err := scan(
		&log.ID, &timestamp, &log.ClientIP, &log.Method, &log.Path,
		&requestedModel, &resolvedModel, &providerID, &providerName,
		&log.StatusCode, &log.IsStreamed, &ttftMs, &log.TotalTimeMs,
		&promptTokens, &completionTokens, &totalTokens, &cachedTokens,
		&errorMessage, &createdAt,
		&estimatedCost,
	)
	if err != nil {
		return models.RequestLog{}, err
	}
	applyRequestLogFields(&log, timestamp, createdAt,
		requestedModel, resolvedModel, providerID, providerName, errorMessage,
		ttftMs, promptTokens, completionTokens, totalTokens, cachedTokens)
	if estimatedCost.Valid {
		log.EstimatedCostUSD = &estimatedCost.Float64
	}
	return log, nil
}

// scanRequestLogFull extends scanRequestLog with request_body and response_body. Used only in GetRequestLog (detail view).
func scanRequestLogFull(scan func(...any) error) (models.RequestLog, error) {
	var log models.RequestLog
	var requestedModel, resolvedModel, providerID, providerName, errorMessage sql.NullString
	var requestBody, responseBody sql.NullString
	var ttftMs, promptTokens, completionTokens, totalTokens, cachedTokens sql.NullInt64
	var estimatedCost sql.NullFloat64
	var timestamp, createdAt timeScanner

	err := scan(
		&log.ID, &timestamp, &log.ClientIP, &log.Method, &log.Path,
		&requestedModel, &resolvedModel, &providerID, &providerName,
		&log.StatusCode, &log.IsStreamed, &ttftMs, &log.TotalTimeMs,
		&promptTokens, &completionTokens, &totalTokens, &cachedTokens,
		&errorMessage, &createdAt,
		&estimatedCost,
		&requestBody, &responseBody,
	)
	if err != nil {
		return models.RequestLog{}, err
	}
	applyRequestLogFields(&log, timestamp, createdAt,
		requestedModel, resolvedModel, providerID, providerName, errorMessage,
		ttftMs, promptTokens, completionTokens, totalTokens, cachedTokens)
	if estimatedCost.Valid {
		log.EstimatedCostUSD = &estimatedCost.Float64
	}
	if requestBody.Valid {
		log.RequestBody = requestBody.String
	}
	if responseBody.Valid {
		log.ResponseBody = responseBody.String
	}
	return log, nil
}

func (s *SQLiteStore) InsertRequestLog(ctx context.Context, log *models.RequestLog) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO request_logs
		 (id, timestamp, client_ip, method, path, requested_model, resolved_model,
		  provider_id, provider_name, status_code, is_streamed, ttft_ms, total_time_ms,
		  prompt_tokens, completion_tokens, total_tokens, cached_tokens, error_message, created_at,
		  estimated_cost_usd, request_body, response_body)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		log.ID, log.Timestamp, log.ClientIP, log.Method, log.Path,
		nullStr(log.RequestedModel), nullStr(log.ResolvedModel),
		nullStr(log.ProviderID), nullStr(log.ProviderName),
		log.StatusCode, log.IsStreamed,
		nullInt(log.TTFTMs), log.TotalTimeMs,
		nullInt(log.PromptTokens), nullInt(log.CompletionTokens),
		nullInt(log.TotalTokens), nullInt(log.CachedTokens),
		nullStr(log.ErrorMessage), log.CreatedAt,
		nullFloat64(log.EstimatedCostUSD), nullStr(log.RequestBody), nullStr(log.ResponseBody),
	)
	if err != nil {
		return fmt.Errorf("insert request log: %w", err)
	}
	return nil
}

// GetRequestLog returns a single request log by ID, including request/response bodies.
// Returns sql.ErrNoRows (wrapped) if not found.
func (s *SQLiteStore) GetRequestLog(ctx context.Context, id string) (*models.RequestLog, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT `+requestLogCols+`, request_body, response_body
		 FROM request_logs WHERE id = ?`,
		id,
	)
	log, err := scanRequestLogFull(row.Scan)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("get request log: %w", sql.ErrNoRows)
		}
		return nil, fmt.Errorf("get request log: %w", err)
	}
	return &log, nil
}

func (s *SQLiteStore) QueryRequestLogs(ctx context.Context, filter models.LogFilter) ([]models.RequestLog, int, error) {
	var conditions []string
	var args []interface{}

	if filter.Model != "" {
		conditions = append(conditions, "requested_model = ?")
		args = append(args, filter.Model)
	}
	if filter.ProviderID != "" {
		conditions = append(conditions, "provider_id = ?")
		args = append(args, filter.ProviderID)
	}
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

	where := ""
	if len(conditions) > 0 {
		where = "WHERE " + strings.Join(conditions, " AND ")
	}

	var total int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM request_logs "+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("query request logs count: %w", err)
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
		return nil, 0, fmt.Errorf("query request logs: %w", err)
	}
	defer rows.Close()

	var logs []models.RequestLog
	for rows.Next() {
		l, err := scanRequestLog(rows.Scan)
		if err != nil {
			return nil, 0, fmt.Errorf("query request logs scan: %w", err)
		}
		logs = append(logs, l)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("query request logs rows: %w", err)
	}
	return logs, total, nil
}

func (s *SQLiteStore) PurgeRequestLogRequestBodiesOlderThan(ctx context.Context, olderThan time.Time) (int64, error) {
	res, err := s.db.ExecContext(ctx, `
		UPDATE request_logs
		SET request_body = ''
		WHERE created_at < ?
		  AND IFNULL(request_body, '') != ''`,
		olderThan.UTC(),
	)
	if err != nil {
		return 0, fmt.Errorf("purge request log request bodies: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("purge request log request bodies rows affected: %w", err)
	}
	return n, nil
}

func (s *SQLiteStore) PurgeRequestLogResponseBodiesOlderThan(ctx context.Context, olderThan time.Time) (int64, error) {
	res, err := s.db.ExecContext(ctx, `
		UPDATE request_logs
		SET response_body = ''
		WHERE created_at < ?
		  AND IFNULL(response_body, '') != ''`,
		olderThan.UTC(),
	)
	if err != nil {
		return 0, fmt.Errorf("purge request log response bodies: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("purge request log response bodies rows affected: %w", err)
	}
	return n, nil
}
