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

	// Circuit breaker (per provider). Defaults applied on create.
	CircuitBreakerEnabled         bool    `json:"circuit_breaker_enabled"`
	CircuitBreakerErrorThreshold  float64 `json:"circuit_breaker_error_threshold"`
	CircuitBreakerWindowSeconds   int     `json:"circuit_breaker_window_seconds"`
	CircuitBreakerCooldownSeconds int     `json:"circuit_breaker_cooldown_seconds"`
}

const (
	DefaultCircuitBreakerErrorThreshold  = 0.5
	DefaultCircuitBreakerWindowSeconds   = 60
	DefaultCircuitBreakerCooldownSeconds = 30
)

// NormalizeCircuitBreaker fills zero/invalid numeric CB settings with defaults.
// It does not change CircuitBreakerEnabled.
func NormalizeCircuitBreaker(p *Provider) {
	if p.CircuitBreakerErrorThreshold <= 0 || p.CircuitBreakerErrorThreshold > 1 {
		p.CircuitBreakerErrorThreshold = DefaultCircuitBreakerErrorThreshold
	}
	if p.CircuitBreakerWindowSeconds <= 0 {
		p.CircuitBreakerWindowSeconds = DefaultCircuitBreakerWindowSeconds
	}
	if p.CircuitBreakerCooldownSeconds <= 0 {
		p.CircuitBreakerCooldownSeconds = DefaultCircuitBreakerCooldownSeconds
	}
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
	// IsAvailable controls whether the model appears in GET /v1/models and is routable.
	IsAvailable bool `json:"is_available"`

	// Per-million-token pricing in USD. Nil means no price configured.
	CostPerMillionInput      *float64 `json:"cost_per_million_input,omitempty"`
	CostPerMillionOutput     *float64 `json:"cost_per_million_output,omitempty"`
	CostPerMillionCacheRead  *float64 `json:"cost_per_million_cache_read,omitempty"`
	CostPerMillionCacheWrite *float64 `json:"cost_per_million_cache_write,omitempty"`
}
