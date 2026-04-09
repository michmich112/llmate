package db

import (
	"context"
	"fmt"
	"time"

	"github.com/llmate/gateway/internal/models"
)

// NewStore opens a database connection using the specified driver and DSN.
// driver must be "sqlite" (or empty, which defaults to SQLite).
func NewStore(driver, dsn string) (Store, error) {
	switch driver {
	case "sqlite", "":
		return NewSQLiteStore(dsn)
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
	// Results are ordered by timestamp desc. Does not populate RequestBody/ResponseBody.
	QueryRequestLogs(ctx context.Context, filter models.LogFilter) ([]models.RequestLog, int, error)

	// GetRequestLog returns a single request log by ID including request/response bodies.
	GetRequestLog(ctx context.Context, id string) (*models.RequestLog, error)

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

	// GetDashboardStats returns aggregated statistics since the given time.
	GetDashboardStats(ctx context.Context, since time.Time) (*models.DashboardStats, error)

	// GetTimeSeries returns request metrics bucketed by time.
	// granularity must be "hour" or "day".
	GetTimeSeries(ctx context.Context, since, until time.Time, granularity string) ([]models.TimeSeriesPoint, error)

	// --- Health ---

	// UpdateProviderHealth updates is_healthy and health_checked_at for a provider.
	UpdateProviderHealth(ctx context.Context, id string, healthy bool) error

	// --- Lifecycle ---

	// Close closes the database connection.
	Close() error
}
