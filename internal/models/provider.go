package models

import "time"

type Provider struct {
	ID              string     `json:"id"`
	Name            string     `json:"name"`
	BaseURL         string     `json:"base_url"`
	APIKey          string     `json:"api_key,omitempty"`
	IsHealthy       bool       `json:"is_healthy"`
	HealthCheckedAt *time.Time `json:"health_checked_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

type ProviderEndpoint struct {
	ID          string    `json:"id"`
	ProviderID  string    `json:"provider_id"`
	Path        string    `json:"path"`
	Method      string    `json:"method"`
	IsSupported bool      `json:"is_supported"`
	IsEnabled   bool      `json:"is_enabled"`
	CreatedAt   time.Time `json:"created_at"`
}

type ProviderModel struct {
	ID         string    `json:"id"`
	ProviderID string    `json:"provider_id"`
	ModelID    string    `json:"model_id"`
	CreatedAt  time.Time `json:"created_at"`

	// Per-million-token pricing in USD. Nil means no price configured.
	CostPerMillionInput      *float64 `json:"cost_per_million_input,omitempty"`
	CostPerMillionOutput     *float64 `json:"cost_per_million_output,omitempty"`
	CostPerMillionCacheRead  *float64 `json:"cost_per_million_cache_read,omitempty"`
	CostPerMillionCacheWrite *float64 `json:"cost_per_million_cache_write,omitempty"`
}
