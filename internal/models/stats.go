package models

type DashboardStats struct {
	TotalRequests  int          `json:"total_requests"`
	AvgLatencyMs   float64      `json:"avg_latency_ms"`
	ErrorRate      float64      `json:"error_rate"`
	ByModel        []ModelStats `json:"by_model"`
	ByProvider     []ProviderStats `json:"by_provider"`
}

type ModelStats struct {
	Model         string  `json:"model"`
	RequestCount  int     `json:"request_count"`
	AvgLatencyMs  float64 `json:"avg_latency_ms"`
	ErrorCount    int     `json:"error_count"`
	TotalTokens   int     `json:"total_tokens"`
}

type ProviderStats struct {
	ProviderID   string  `json:"provider_id"`
	ProviderName string  `json:"provider_name"`
	RequestCount int     `json:"request_count"`
	AvgLatencyMs float64 `json:"avg_latency_ms"`
	ErrorCount   int     `json:"error_count"`
}

// TimeSeriesPoint holds aggregated metrics for a single time bucket.
// Bucket is an ISO 8601 string: "2006-01-02T15:00:00" for hourly, "2006-01-02" for daily.
type TimeSeriesPoint struct {
	Bucket           string  `json:"bucket"`
	Requests         int     `json:"requests"`
	SuccessCount     int     `json:"success_count"`
	ErrorCount       int     `json:"error_count"`
	InputTokens      int     `json:"input_tokens"`
	PromptTokens     int     `json:"prompt_tokens"`
	CompletionTokens int     `json:"completion_tokens"`
	TotalTokens      int     `json:"total_tokens"`
	CachedTokens     int     `json:"cached_tokens"`
	TotalCostUSD     float64 `json:"total_cost_usd"`
	InputCostUSD     float64 `json:"input_cost_usd"`
	OutputCostUSD    float64 `json:"output_cost_usd"`
	CachedCostUSD    float64 `json:"cached_cost_usd"`
}
