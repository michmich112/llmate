package models

import "time"

// APIKey represents an API key used to authenticate proxy requests.
// Only the sha256 hash of the key is ever stored; the raw key is returned
// once at creation time and never persisted.
type APIKey struct {
	ID           string     `json:"id"`
	KeyHash      string     `json:"-"`
	Name         string     `json:"name"`
	IsActive     bool       `json:"is_active"`
	RateLimitRPM *int       `json:"rate_limit_rpm,omitempty"`
	RateLimitTPM *int       `json:"rate_limit_tpm,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	LastUsedAt   *time.Time `json:"last_used_at,omitempty"`
}

// APIKeyUsage is a per-key usage summary aggregated from request_logs.
type APIKeyUsage struct {
	APIKeyID     string  `json:"api_key_id"`
	APIKeyName   string  `json:"api_key_name"`
	RequestCount int     `json:"request_count"`
	TotalTokens  int     `json:"total_tokens"`
	TotalCostUSD float64 `json:"total_cost_usd"`
	LastUsedAt   *time.Time `json:"last_used_at,omitempty"`
}

// APIKeyCreateRequest is the admin input for creating a new API key.
type APIKeyCreateRequest struct {
	Name         string `json:"name"`
	RateLimitRPM *int   `json:"rate_limit_rpm,omitempty"`
	RateLimitTPM *int   `json:"rate_limit_tpm,omitempty"`
}

// APIKeyUpdateRequest carries the desired state of an existing key; the client
// sends the full state. A nil RateLimitRPM/TPM means "remove" the limit.
type APIKeyUpdateRequest struct {
	Name         string `json:"name"`
	IsActive     *bool  `json:"is_active"`
	RateLimitRPM *int   `json:"rate_limit_rpm,omitempty"`
	RateLimitTPM *int   `json:"rate_limit_tpm,omitempty"`
}
