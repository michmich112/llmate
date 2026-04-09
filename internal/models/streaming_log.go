package models

import "time"

type StreamingLog struct {
	ID           string    `json:"id"`
	RequestLogID string    `json:"request_log_id"`
	ChunkIndex   int       `json:"chunk_index"`
	Data         string    `json:"data"`
	ContentDelta string    `json:"content_delta"`
	BodyPurged   bool      `json:"body_purged"`
	IsTruncated  bool      `json:"is_truncated"`
	Timestamp    time.Time `json:"timestamp"`
	CreatedAt    time.Time `json:"created_at"`
	// CumulativeBody is the assistant text after applying this chunk and all prior deltas (OpenAI-style).
	// Filled by the admin API when listing chunks; not stored in the database.
	CumulativeBody string `json:"cumulative_body"`
}
