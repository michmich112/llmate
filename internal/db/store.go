package db

import (
	"context"
	"fmt"
	"time"

	"github.com/llmate/gateway/internal/models"
)

// NewStore opens a database connection using the specified driver and DSN.
// driver must be "sqlite" (or empty, which defaults to SQLite).
func NewStore(driver, dsn, legacyPath string) (Store, error) {
	switch driver {
	case "sqlite":
		return NewSQLiteStore(dsn)
	case "libsql":
		return NewLibSQLStore(dsn, legacyPath)
	case "postgres":
		return nil, fmt.Errorf("postgres driver not yet implemented")
	default:
		return nil, fmt.Errorf("unsupported db driver %q", driver)
	}
}

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

	// UpsertProviderEndpoints inserts or updates endpoints for a provider.
	// Endpoints not in the given set are left unchanged.
	UpsertProviderEndpoints(ctx context.Context, providerID string, eps []models.ProviderEndpoint) error

	// ListProviderEndpoints returns all endpoints for a provider.
	ListProviderEndpoints(ctx context.Context, providerID string) ([]models.ProviderEndpoint, error)

	// UpdateProviderEndpoint updates is_enabled for a single endpoint. Identified by ep.ID.
	UpdateProviderEndpoint(ctx context.Context, ep *models.ProviderEndpoint) error

	// --- Provider Models ---

	// SyncProviderModels inserts any model IDs not yet registered for the provider.
	// Existing records (and their cost configuration) are never modified or removed.
	SyncProviderModels(ctx context.Context, providerID string, modelIDs []string) error

	// CreateProviderModel inserts a single provider model. The caller must set ID and timestamps.
	CreateProviderModel(ctx context.Context, m *models.ProviderModel) error

	// DeleteProviderModel removes a provider model record by ID scoped to the provider.
	DeleteProviderModel(ctx context.Context, providerID, recordID string) error

	// SetProviderModelsAvailability marks the given model IDs as available for a provider
	// and marks all other models on that provider as unavailable.
	SetProviderModelsAvailability(ctx context.Context, providerID string, availableModelIDs []string) error

	// UpdateProviderModelAvailability sets is_available on a single provider model record.
	UpdateProviderModelAvailability(ctx context.Context, providerID, recordID string, available bool) error

	// ListProviderModels returns all models for a provider.
	ListProviderModels(ctx context.Context, providerID string) ([]models.ProviderModel, error)

	// ListAllModels returns all models across all providers.
	ListAllModels(ctx context.Context) ([]models.ProviderModel, error)

	// --- Model Aliases ---

	// CreateAlias inserts a new alias. The caller must set ID and timestamps.
	CreateAlias(ctx context.Context, a *models.ModelAlias) error

	// ListAliases returns all aliases ordered by alias name, then priority desc.
	ListAliases(ctx context.Context) ([]models.ModelAlias, error)

	// UpdateAlias updates alias, provider_id, model_id, weight, priority, is_enabled, updated_at. Identified by a.ID.
	UpdateAlias(ctx context.Context, a *models.ModelAlias) error

	// DeleteAlias removes an alias by ID.
	DeleteAlias(ctx context.Context, id string) error

	// ResolveAlias returns all enabled alias entries for a given alias name,
	// ordered by priority desc. Used by the smart router.
	ResolveAlias(ctx context.Context, alias string) ([]models.ModelAlias, error)

	// --- Routing (hot path, read-only) ---

	// LoadRoutingData returns providers, models, aliases, and endpoints for the in-memory routing catalog.
	LoadRoutingData(ctx context.Context) (*models.RoutingData, error)

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
	// Results are ordered by timestamp desc. Does not populate RequestBody/ResponseBody.
	QueryRequestLogs(ctx context.Context, filter models.LogFilter) ([]models.RequestLog, int, error)

	// GetRequestLog returns a single request log by ID including request/response bodies.
	GetRequestLog(ctx context.Context, id string) (*models.RequestLog, error)

	// --- API Keys ---

	// CreateAPIKey inserts a new API key. The caller must set ID and timestamps, and must
	// set KeyHash to the sha256 of the raw key. Returns the stored key (KeyHash excluded).
	CreateAPIKey(ctx context.Context, k *models.APIKey) error

	// GetAPIKeyByHash returns the API key whose sha256 hash matches keyHash, or nil if not found.
	GetAPIKeyByHash(ctx context.Context, keyHash string) (*models.APIKey, error)

	// GetAPIKey returns the API key with the given ID, or sql.ErrNoRows if not found.
	GetAPIKey(ctx context.Context, id string) (*models.APIKey, error)

	// ListAPIKeys returns all API keys ordered by created_at desc.
	ListAPIKeys(ctx context.Context) ([]models.APIKey, error)

	// UpdateAPIKey updates the mutable fields of a key (name, is_active, rate limits). Identified by k.ID.
	UpdateAPIKey(ctx context.Context, k *models.APIKey) error

	// DeleteAPIKey removes an API key by ID.
	DeleteAPIKey(ctx context.Context, id string) error

	// TouchAPIKeyLastUsed records the last_used_at timestamp for a key.
	TouchAPIKeyLastUsed(ctx context.Context, id string, at time.Time) error

	// QueryLogsByAPIKey returns request logs stamped with the given API key, and total count.
	// Results are ordered by timestamp desc.
	QueryLogsByAPIKey(ctx context.Context, apiKeyID string, filter models.LogFilter) ([]models.RequestLog, int, error)

	// UsageByAPIKey returns per-key usage summaries from request logs.
	UsageByAPIKey(ctx context.Context, since, until time.Time) ([]models.APIKeyUsage, error)

	// UsageByAPIKeyModel returns per-model usage summaries aggregated from
	// request_logs stamped with the given API key in the [since, until] range.
	UsageByAPIKeyModel(ctx context.Context, apiKeyID string, since, until time.Time) ([]models.ModelStats, error)

	// --- Configuration ---

	// GetAllConfig returns all config key-value pairs.
	GetAllConfig(ctx context.Context) (map[string]string, error)

	// SetConfig upserts a single config key-value pair.
	SetConfig(ctx context.Context, key, value string) error

	// --- Streaming Logs ---

	// InsertStreamingLog inserts a single streaming log chunk.
	InsertStreamingLog(ctx context.Context, log *models.StreamingLog) error

	// GetStreamingLogs returns all streaming log chunks for a request, ordered by chunk_index.
	GetStreamingLogs(ctx context.Context, requestLogID string) ([]models.StreamingLog, error)

	// PurgeStreamingLogBodiesOlderThan clears data and content_delta for rows with created_at strictly before olderThan.
	// Returns the number of rows updated.
	PurgeStreamingLogBodiesOlderThan(ctx context.Context, olderThan time.Time) (int64, error)

	// PurgeRequestLogRequestBodiesOlderThan sets request_body to empty for request_logs with created_at strictly before olderThan and non-empty request_body.
	PurgeRequestLogRequestBodiesOlderThan(ctx context.Context, olderThan time.Time) (int64, error)

	// PurgeRequestLogResponseBodiesOlderThan sets response_body to empty for request_logs with created_at strictly before olderThan and non-empty response_body.
	PurgeRequestLogResponseBodiesOlderThan(ctx context.Context, olderThan time.Time) (int64, error)

	// --- Provider Model Costs ---

	// UpdateProviderModelCosts updates the four cost fields for a provider_models record by ID.
	UpdateProviderModelCosts(ctx context.Context, id string, m *models.ProviderModel) error

	// GetProviderModelCosts returns the ProviderModel record for a given provider+modelID pair,
	// used by the MetricsCollector to compute estimated cost. Returns nil (not error) if not found.
	GetProviderModelCosts(ctx context.Context, providerID, modelID string) (*models.ProviderModel, error)

	// --- Stats ---

	// GetDashboardStats returns aggregated statistics in [since, until].
	GetDashboardStats(ctx context.Context, since, until time.Time) (*models.DashboardStats, error)

	// GetTimeSeries returns request metrics bucketed by time.
	// granularity must be "hour" or "day".
	GetTimeSeries(ctx context.Context, since, until time.Time, granularity string) ([]models.TimeSeriesPoint, error)

	// GetLifetimeCost returns all-time estimated spend from request logs.
	GetLifetimeCost(ctx context.Context) (*models.LifetimeCost, error)

	// --- Health ---

	// UpdateProviderHealth updates is_healthy and health_checked_at for a provider.
	UpdateProviderHealth(ctx context.Context, id string, healthy bool) error

	// --- Lifecycle ---

	// Close closes the database connection.
	Close() error
}
