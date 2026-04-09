# Spec 00: Contracts (Phase 0)

## Goal

Define ALL interfaces, domain types, database schema, and configuration that Phase 1 agents will implement against. This phase produces zero business logic -- only type definitions, interface signatures, and schema.

## Files to Create

1. `go.mod` -- Go module initialization
2. `internal/models/provider.go` -- Provider, ProviderEndpoint, ProviderModel types
3. `internal/models/alias.go` -- ModelAlias type
4. `internal/models/request_log.go` -- RequestLog, LogFilter types
5. `internal/models/stats.go` -- DashboardStats and related types
6. `internal/db/store.go` -- Store interface
7. `internal/db/migrations/0001_initial.up.sql` -- Create tables
8. `internal/db/migrations/0001_initial.down.sql` -- Drop tables
9. `internal/config/config.go` -- Config struct and loader
10. `context/openai-proxy-endpoints.md` -- Already created (skip if exists)

## Dependencies

Initialize the Go module as:
```
module github.com/llmate/gateway
```

Required dependencies (add to go.mod):
- `github.com/go-chi/chi/v5` -- HTTP router
- `github.com/google/uuid` -- UUID generation
- `modernc.org/sqlite` -- Pure Go SQLite driver (no CGo required)

Run `go mod init` and `go mod tidy` after creating files.

---

## File: `internal/models/provider.go`

Package `models`. Contains provider-related domain types.

```go
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
}
```

## File: `internal/models/alias.go`

```go
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
```

## File: `internal/models/request_log.go`

```go
package models

import "time"

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
}

type LogFilter struct {
    Model      string     `json:"model,omitempty"`
    ProviderID string     `json:"provider_id,omitempty"`
    Since      *time.Time `json:"since,omitempty"`
    Until      *time.Time `json:"until,omitempty"`
    Limit      int        `json:"limit"`
    Offset     int        `json:"offset"`
}
```

## File: `internal/models/stats.go`

```go
package models

type DashboardStats struct {
    TotalRequests  int              `json:"total_requests"`
    AvgLatencyMs   float64          `json:"avg_latency_ms"`
    ErrorRate      float64          `json:"error_rate"`
    ByModel        []ModelStats     `json:"by_model"`
    ByProvider     []ProviderStats  `json:"by_provider"`
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
```

## File: `internal/db/store.go`

The Store interface is the central contract that all data access goes through. Phase 1A implements this against SQLite. Other agents consume it.

```go
package db

import (
    "context"
    "time"

    "github.com/llmate/gateway/internal/models"
)

type Store interface {
    // --- Providers ---

    // CreateProvider inserts a new provider. The caller must set ID and timestamps.
    CreateProvider(ctx context.Context, p *models.Provider) error

    // GetProvider returns a provider by ID. Returns error if not found.
    GetProvider(ctx context.Context, id string) (*models.Provider, error)

    // ListProviders returns all providers ordered by created_at desc.
    ListProviders(ctx context.Context) ([]models.Provider, error)

    // UpdateProvider updates name, base_url, api_key, updated_at. Identified by p.ID.
    UpdateProvider(ctx context.Context, p *models.Provider) error

    // DeleteProvider removes a provider and cascades to endpoints, models, and aliases.
    DeleteProvider(ctx context.Context, id string) error

    // --- Provider Endpoints ---

    // UpsertProviderEndpoints replaces all endpoints for a provider with the given set.
    UpsertProviderEndpoints(ctx context.Context, providerID string, eps []models.ProviderEndpoint) error

    // ListProviderEndpoints returns all endpoints for a provider.
    ListProviderEndpoints(ctx context.Context, providerID string) ([]models.ProviderEndpoint, error)

    // UpdateProviderEndpoint updates is_enabled for a single endpoint. Identified by ep.ID.
    UpdateProviderEndpoint(ctx context.Context, ep *models.ProviderEndpoint) error

    // --- Provider Models ---

    // SyncProviderModels replaces all models for a provider with the given model IDs.
    // Generates new UUIDs for new models, removes models not in the list.
    SyncProviderModels(ctx context.Context, providerID string, modelIDs []string) error

    // ListProviderModels returns all models for a provider.
    ListProviderModels(ctx context.Context, providerID string) ([]models.ProviderModel, error)

    // ListAllModels returns all models across all providers.
    ListAllModels(ctx context.Context) ([]models.ProviderModel, error)

    // --- Model Aliases ---

    // CreateAlias inserts a new alias. The caller must set ID and timestamps.
    CreateAlias(ctx context.Context, a *models.ModelAlias) error

    // ListAliases returns all aliases ordered by alias name, then priority desc.
    ListAliases(ctx context.Context) ([]models.ModelAlias, error)

    // UpdateAlias updates weight, priority, is_enabled, updated_at. Identified by a.ID.
    UpdateAlias(ctx context.Context, a *models.ModelAlias) error

    // DeleteAlias removes an alias by ID.
    DeleteAlias(ctx context.Context, id string) error

    // ResolveAlias returns all enabled alias entries for a given alias name,
    // ordered by priority desc. Used by the smart router.
    ResolveAlias(ctx context.Context, alias string) ([]models.ModelAlias, error)

    // --- Routing (hot path, read-only) ---

    // GetHealthyProvidersForModel returns all healthy providers that have
    // the given model_id in their provider_models table.
    GetHealthyProvidersForModel(ctx context.Context, modelID string) ([]models.Provider, error)

    // GetEnabledEndpoint returns the endpoint for a provider+path if it exists
    // and is both supported and enabled. Returns nil (not error) if not found.
    GetEnabledEndpoint(ctx context.Context, providerID string, path string) (*models.ProviderEndpoint, error)

    // --- Request Logs ---

    // InsertRequestLog inserts a request log entry.
    InsertRequestLog(ctx context.Context, log *models.RequestLog) error

    // QueryRequestLogs returns filtered request logs and total count.
    // Results are ordered by timestamp desc.
    QueryRequestLogs(ctx context.Context, filter models.LogFilter) ([]models.RequestLog, int, error)

    // --- Stats ---

    // GetDashboardStats returns aggregated statistics since the given time.
    GetDashboardStats(ctx context.Context, since time.Time) (*models.DashboardStats, error)

    // --- Health ---

    // UpdateProviderHealth updates is_healthy and health_checked_at for a provider.
    UpdateProviderHealth(ctx context.Context, id string, healthy bool) error

    // --- Lifecycle ---

    // Close closes the database connection.
    Close() error
}
```

## File: `internal/db/migrations/0001_initial.up.sql`

```sql
CREATE TABLE IF NOT EXISTS providers (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    base_url TEXT NOT NULL UNIQUE,
    api_key TEXT,
    is_healthy BOOLEAN NOT NULL DEFAULT 0,
    health_checked_at DATETIME,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS provider_endpoints (
    id TEXT PRIMARY KEY,
    provider_id TEXT NOT NULL,
    path TEXT NOT NULL,
    method TEXT NOT NULL,
    is_supported BOOLEAN NOT NULL,
    is_enabled BOOLEAN NOT NULL DEFAULT 1,
    created_at DATETIME NOT NULL,
    FOREIGN KEY (provider_id) REFERENCES providers(id) ON DELETE CASCADE,
    UNIQUE(provider_id, path, method)
);

CREATE TABLE IF NOT EXISTS provider_models (
    id TEXT PRIMARY KEY,
    provider_id TEXT NOT NULL,
    model_id TEXT NOT NULL,
    created_at DATETIME NOT NULL,
    FOREIGN KEY (provider_id) REFERENCES providers(id) ON DELETE CASCADE,
    UNIQUE(provider_id, model_id)
);

CREATE TABLE IF NOT EXISTS model_aliases (
    id TEXT PRIMARY KEY,
    alias TEXT NOT NULL,
    provider_id TEXT NOT NULL,
    model_id TEXT NOT NULL,
    weight INTEGER NOT NULL DEFAULT 1,
    priority INTEGER NOT NULL DEFAULT 0,
    is_enabled BOOLEAN NOT NULL DEFAULT 1,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL,
    FOREIGN KEY (provider_id) REFERENCES providers(id) ON DELETE CASCADE,
    UNIQUE(alias, provider_id, model_id)
);

CREATE TABLE IF NOT EXISTS request_logs (
    id TEXT PRIMARY KEY,
    timestamp DATETIME NOT NULL,
    client_ip TEXT NOT NULL,
    method TEXT NOT NULL,
    path TEXT NOT NULL,
    requested_model TEXT,
    resolved_model TEXT,
    provider_id TEXT,
    provider_name TEXT,
    status_code INTEGER NOT NULL,
    is_streamed BOOLEAN NOT NULL DEFAULT 0,
    ttft_ms INTEGER,
    total_time_ms INTEGER NOT NULL,
    prompt_tokens INTEGER,
    completion_tokens INTEGER,
    total_tokens INTEGER,
    cached_tokens INTEGER,
    error_message TEXT,
    created_at DATETIME NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_request_logs_timestamp ON request_logs(timestamp);
CREATE INDEX IF NOT EXISTS idx_request_logs_requested_model ON request_logs(requested_model);
CREATE INDEX IF NOT EXISTS idx_request_logs_provider_id ON request_logs(provider_id);
```

## File: `internal/db/migrations/0001_initial.down.sql`

```sql
DROP TABLE IF EXISTS request_logs;
DROP TABLE IF EXISTS model_aliases;
DROP TABLE IF EXISTS provider_models;
DROP TABLE IF EXISTS provider_endpoints;
DROP TABLE IF EXISTS providers;
```

## File: `internal/config/config.go`

```go
package config

import (
    "fmt"
    "os"
    "strconv"
    "time"
)

type Config struct {
    AccessKey      string
    Port           string
    DBPath         string
    HealthInterval time.Duration
    LogLevel       string
    MaxBodySize    int64
}

// Load reads configuration from environment variables with defaults.
func Load() (*Config, error) {
    accessKey := os.Getenv("ACCESS_KEY")
    if accessKey == "" {
        return nil, fmt.Errorf("ACCESS_KEY environment variable is required")
    }

    cfg := &Config{
        AccessKey:      accessKey,
        Port:           getEnvOrDefault("PORT", "8080"),
        DBPath:         getEnvOrDefault("DB_PATH", "./llmate.db"),
        HealthInterval: parseDurationOrDefault("HEALTH_INTERVAL", 30*time.Second),
        LogLevel:       getEnvOrDefault("LOG_LEVEL", "info"),
        MaxBodySize:    parseIntOrDefault("MAX_BODY_SIZE", 10*1024*1024),
    }

    return cfg, nil
}

func getEnvOrDefault(key, defaultVal string) string {
    if v := os.Getenv(key); v != "" {
        return v
    }
    return defaultVal
}

func parseDurationOrDefault(key string, defaultVal time.Duration) time.Duration {
    if v := os.Getenv(key); v != "" {
        if d, err := time.ParseDuration(v); err == nil {
            return d
        }
    }
    return defaultVal
}

func parseIntOrDefault(key string, defaultVal int64) int64 {
    if v := os.Getenv(key); v != "" {
        if n, err := strconv.ParseInt(v, 10, 64); err == nil {
            return n
        }
    }
    return defaultVal
}
```

## Done Criteria

- [ ] `go.mod` exists with module name `github.com/llmate/gateway` and required dependencies
- [ ] All 4 model files exist in `internal/models/` with correct struct definitions and JSON tags
- [ ] `internal/db/store.go` contains the Store interface with all method signatures
- [ ] Migration files exist in `internal/db/migrations/`
- [ ] `internal/config/config.go` loads from environment with defaults
- [ ] `go build ./...` succeeds (types compile)
- [ ] No circular imports
