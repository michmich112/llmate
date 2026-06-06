package models

import "strconv"

const (
	DefaultRequestBodyMaxBytes           = 51200 // 50KB
	DefaultResponseBodyMaxBytes          = 51200 // 50KB
	DefaultTrackStreaming                = false
	DefaultStreamingBufferSize           = 10240      // 10KB
	MaxBodyMaxBytes                      = 1073741824 // 1GB
	MinStreamingBufferSize               = 1024       // 1KB
	MaxStreamingBufferSize               = 1048576    // 1MB
	DefaultHTTPIdleConnTimeoutSeconds    = 90
	MinHTTPIdleConnTimeoutSeconds        = 10
	MaxHTTPIdleConnTimeoutSeconds        = 86400 // 24h; upper bound for pooled idle sockets
	DefaultStreamingLogBodyRetentionDays = 30
	DefaultRequestLogBodyRetentionDays   = 30
	DefaultResponseLogBodyRetentionDays  = 30
	// MinStreamingLogBodyRetentionDays and Max apply to all persisted body retention settings (streaming chunks, request bodies, response bodies).
	MinStreamingLogBodyRetentionDays = 1
	MaxStreamingLogBodyRetentionDays = 3650
)

type Configuration struct {
	RequestBodyMaxBytes           int  `json:"request_body_max_bytes"`
	ResponseBodyMaxBytes          int  `json:"response_body_max_bytes"`
	TrackStreaming                bool `json:"track_streaming"`
	StreamingBufferSize           int  `json:"streaming_buffer_size"`
	HTTPIdleConnTimeoutSeconds    int  `json:"http_idle_conn_timeout_seconds"`
	StreamingLogBodyRetentionDays int  `json:"streaming_log_body_retention_days"`
	RequestLogBodyRetentionDays   int  `json:"request_log_body_retention_days"`
	ResponseLogBodyRetentionDays  int  `json:"response_log_body_retention_days"`
}

func DefaultConfiguration() Configuration {
	return Configuration{
		RequestBodyMaxBytes:           DefaultRequestBodyMaxBytes,
		ResponseBodyMaxBytes:          DefaultResponseBodyMaxBytes,
		TrackStreaming:                DefaultTrackStreaming,
		StreamingBufferSize:           DefaultStreamingBufferSize,
		HTTPIdleConnTimeoutSeconds:    DefaultHTTPIdleConnTimeoutSeconds,
		StreamingLogBodyRetentionDays: DefaultStreamingLogBodyRetentionDays,
		RequestLogBodyRetentionDays:   DefaultRequestLogBodyRetentionDays,
		ResponseLogBodyRetentionDays:  DefaultResponseLogBodyRetentionDays,
	}
}

// ClampHTTPIdleConnTimeoutSeconds returns v bounded to the allowed range.
func ClampHTTPIdleConnTimeoutSeconds(v int) int {
	if v < MinHTTPIdleConnTimeoutSeconds {
		return MinHTTPIdleConnTimeoutSeconds
	}
	if v > MaxHTTPIdleConnTimeoutSeconds {
		return MaxHTTPIdleConnTimeoutSeconds
	}
	return v
}

// HTTPIdleConnTimeoutSecondsFromConfig reads http_idle_conn_timeout_seconds from a
// config map (e.g. GetAllConfig at startup). Invalid or missing values yield the default.
func HTTPIdleConnTimeoutSecondsFromConfig(m map[string]string) int {
	if m == nil {
		return DefaultHTTPIdleConnTimeoutSeconds
	}
	raw, ok := m["http_idle_conn_timeout_seconds"]
	if !ok {
		return DefaultHTTPIdleConnTimeoutSeconds
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return DefaultHTTPIdleConnTimeoutSeconds
	}
	return ClampHTTPIdleConnTimeoutSeconds(v)
}
