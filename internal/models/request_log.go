package models

import "time"

// RequestLogCostBreakdown is per-component USD estimates using the same rules as persist-time pricing.
// Set only on admin GET /logs/{id} when provider model rates are available.
type RequestLogCostBreakdown struct {
	InputUSD      float64 `json:"input_usd"`
	OutputUSD     float64 `json:"output_usd"`
	CachedReadUSD float64 `json:"cached_read_usd"`
	TotalUSD      float64 `json:"total_usd"`
}

type RequestLog struct {
	ID               string    `json:"id"`
	Timestamp        time.Time `json:"timestamp"`
	ClientIP         string    `json:"client_ip"`
	Method           string    `json:"method"`
	Path             string    `json:"path"`
	RequestedModel   string    `json:"requested_model,omitempty"`
	ResolvedModel    string    `json:"resolved_model,omitempty"`
	ProviderID       string    `json:"provider_id,omitempty"`
	ProviderName     string    `json:"provider_name,omitempty"`
	StatusCode       int       `json:"status_code"`
	IsStreamed       bool      `json:"is_streamed"`
	TTFTMs           *int      `json:"ttft_ms,omitempty"`
	TotalTimeMs      int       `json:"total_time_ms"`
	PromptTokens     *int      `json:"prompt_tokens,omitempty"`
	CompletionTokens *int      `json:"completion_tokens,omitempty"`
	TotalTokens      *int      `json:"total_tokens,omitempty"`
	CachedTokens     *int      `json:"cached_tokens,omitempty"`
	ErrorMessage     string    `json:"error_message,omitempty"`
	CreatedAt        time.Time `json:"created_at"`

	// Estimated cost computed asynchronously from model pricing at log time.
	EstimatedCostUSD *float64 `json:"estimated_cost_usd,omitempty"`
	// CostBreakdown is populated on admin detail fetch when pricing exists (see internal/pricing).
	CostBreakdown *RequestLogCostBreakdown `json:"cost_breakdown,omitempty"`
	// Request and response bodies, truncated per admin config. May be populated for streaming responses.
	RequestBody  string `json:"request_body,omitempty"`
	ResponseBody string `json:"response_body,omitempty"`
}

type LogFilter struct {
	Model      string     `json:"model,omitempty"`
	ProviderID string     `json:"provider_id,omitempty"`
	Since      *time.Time `json:"since,omitempty"`
	Until      *time.Time `json:"until,omitempty"`
	// StatusMin/StatusMax filter by HTTP status code range (inclusive). Zero means no bound.
	StatusMin int `json:"status_min,omitempty"`
	StatusMax int `json:"status_max,omitempty"`
	Limit     int `json:"limit"`
	Offset    int `json:"offset"`
}
