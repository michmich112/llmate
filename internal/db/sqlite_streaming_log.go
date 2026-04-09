package db

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/llmate/gateway/internal/models"
)

func (s *SQLiteStore) InsertStreamingLog(ctx context.Context, log *models.StreamingLog) error {
	if log.ID == "" {
		log.ID = uuid.NewString()
	}
	if log.CreatedAt.IsZero() {
		log.CreatedAt = time.Now().UTC()
	}
	if log.Timestamp.IsZero() {
		log.Timestamp = log.CreatedAt
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO streaming_logs (id, request_log_id, chunk_index, timestamp, data, content_delta, is_truncated, created_at)
         VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		log.ID, log.RequestLogID, log.ChunkIndex,
		log.Timestamp, log.Data, log.ContentDelta, log.IsTruncated, log.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert streaming log: %w", err)
	}
	return nil
}

func (s *SQLiteStore) GetStreamingLogs(ctx context.Context, requestLogID string) ([]models.StreamingLog, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, request_log_id, chunk_index, timestamp, data, content_delta, body_purged, is_truncated, created_at
         FROM streaming_logs WHERE request_log_id = ? ORDER BY chunk_index ASC`,
		requestLogID,
	)
	if err != nil {
		return nil, fmt.Errorf("get streaming logs: %w", err)
	}
	defer rows.Close()
	var logs []models.StreamingLog
	for rows.Next() {
		var l models.StreamingLog
		var ts, ca timeScanner
		var bodyPurged int64
		if err := rows.Scan(&l.ID, &l.RequestLogID, &l.ChunkIndex, &ts, &l.Data, &l.ContentDelta, &bodyPurged, &l.IsTruncated, &ca); err != nil {
			return nil, fmt.Errorf("scan streaming log: %w", err)
		}
		l.Timestamp = ts.Time
		l.CreatedAt = ca.Time
		l.BodyPurged = bodyPurged != 0
		logs = append(logs, l)
	}
	return logs, rows.Err()
}

func (s *SQLiteStore) PurgeStreamingLogBodiesOlderThan(ctx context.Context, olderThan time.Time) (int64, error) {
	res, err := s.db.ExecContext(ctx, `
		UPDATE streaming_logs
		SET data = '', content_delta = '', body_purged = 1
		WHERE created_at < ?
		  AND (body_purged = 0 OR length(data) > 0 OR length(content_delta) > 0)`,
		olderThan.UTC(),
	)
	if err != nil {
		return 0, fmt.Errorf("purge streaming log bodies: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("purge streaming log bodies rows affected: %w", err)
	}
	return n, nil
}
