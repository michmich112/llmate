package models

import "time"

type ModelAlias struct {
	ID         string    `json:"id"`
	Alias      string    `json:"alias"`
	ProviderID string    `json:"provider_id"`
	ModelID    string    `json:"model_id"`
	Weight     int       `json:"weight"`
	Priority   int       `json:"priority"`
	IsEnabled  bool      `json:"is_enabled"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}
