package admin

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/llmate/gateway/internal/db"
	"github.com/llmate/gateway/internal/models"
	"github.com/llmate/gateway/internal/proxy"
	"github.com/llmate/gateway/internal/stats"
)

// mockStore satisfies db.Store using optional function fields.
// Methods that are not needed by a specific test can be left nil;
// they panic with "unexpected call" to surface unintended invocations.
type mockStore struct {
	createProvider                  func(ctx context.Context, p *models.Provider) error
	getProvider                     func(ctx context.Context, id string) (*models.Provider, error)
	listProviders                   func(ctx context.Context) ([]models.Provider, error)
	updateProvider                  func(ctx context.Context, p *models.Provider) error
	deleteProvider                  func(ctx context.Context, id string) error
	upsertProviderEndpoints         func(ctx context.Context, providerID string, eps []models.ProviderEndpoint) error
	listProviderEndpoints           func(ctx context.Context, providerID string) ([]models.ProviderEndpoint, error)
	updateProviderEndpoint          func(ctx context.Context, ep *models.ProviderEndpoint) error
	syncProviderModels              func(ctx context.Context, providerID string, modelIDs []string) error
	createProviderModel             func(ctx context.Context, m *models.ProviderModel) error
	deleteProviderModel             func(ctx context.Context, providerID, recordID string) error
	setProviderModelsAvailability   func(ctx context.Context, providerID string, availableModelIDs []string) error
	updateProviderModelAvailability func(ctx context.Context, providerID, recordID string, available bool) error
	listProviderModels              func(ctx context.Context, providerID string) ([]models.ProviderModel, error)
	listAllModels                   func(ctx context.Context) ([]models.ProviderModel, error)
	createAlias                     func(ctx context.Context, a *models.ModelAlias) error
	listAliases                     func(ctx context.Context) ([]models.ModelAlias, error)
	updateAlias                     func(ctx context.Context, a *models.ModelAlias) error
	deleteAlias                     func(ctx context.Context, id string) error
	resolveAlias                    func(ctx context.Context, alias string) ([]models.ModelAlias, error)
	getHealthyProvidersForModel     func(ctx context.Context, modelID string) ([]models.Provider, error)
	getEnabledEndpoint              func(ctx context.Context, providerID string, path string) (*models.ProviderEndpoint, error)
	insertRequestLog                func(ctx context.Context, log *models.RequestLog) error
	queryRequestLogs                func(ctx context.Context, filter models.LogFilter) ([]models.RequestLog, int, error)
	getRequestLog                   func(ctx context.Context, id string) (*models.RequestLog, error)
	updateProviderModelCosts        func(ctx context.Context, id string, m *models.ProviderModel) error
	getProviderModelCosts           func(ctx context.Context, providerID, modelID string) (*models.ProviderModel, error)
	getDashboardStats               func(ctx context.Context, since, until time.Time) (*models.DashboardStats, error)
	getDashboardStatsForAPIKey      func(ctx context.Context, apiKeyID string, since, until time.Time) (*models.DashboardStats, error)
	getTimeSeries                   func(ctx context.Context, since, until time.Time, granularity string) ([]models.TimeSeriesPoint, error)
	timeSeriesByAPIKey              func(ctx context.Context, apiKeyID string, since, until time.Time, granularity string) ([]models.TimeSeriesPoint, error)
	getLifetimeCost                 func(ctx context.Context) (*models.LifetimeCost, error)
	getLifetimeCostForAPIKey        func(ctx context.Context, apiKeyID string) (*models.LifetimeCost, error)
	updateProviderHealth            func(ctx context.Context, id string, healthy bool) error
	createAPIKey                    func(ctx context.Context, k *models.APIKey) error
	getAPIKeyByHash                 func(ctx context.Context, keyHash string) (*models.APIKey, error)
	getAPIKey                       func(ctx context.Context, id string) (*models.APIKey, error)
	listAPIKeys                     func(ctx context.Context) ([]models.APIKey, error)
	updateAPIKey                    func(ctx context.Context, k *models.APIKey) error
	deleteAPIKey                    func(ctx context.Context, id string) error
	usageByAPIKey                   func(ctx context.Context, since, until time.Time) ([]models.APIKeyUsage, error)
	usageByAPIKeyModel              func(ctx context.Context, apiKeyID string, since, until time.Time) ([]models.ModelStats, error)
}

func (m *mockStore) CreateProvider(ctx context.Context, p *models.Provider) error {
	if m.createProvider != nil {
		return m.createProvider(ctx, p)
	}
	return nil
}
func (m *mockStore) GetProvider(ctx context.Context, id string) (*models.Provider, error) {
	if m.getProvider != nil {
		return m.getProvider(ctx, id)
	}
	panic("unexpected call to GetProvider")
}
func (m *mockStore) ListProviders(ctx context.Context) ([]models.Provider, error) {
	if m.listProviders != nil {
		return m.listProviders(ctx)
	}
	return []models.Provider{}, nil
}
func (m *mockStore) UpdateProvider(ctx context.Context, p *models.Provider) error {
	if m.updateProvider != nil {
		return m.updateProvider(ctx, p)
	}
	return nil
}
func (m *mockStore) DeleteProvider(ctx context.Context, id string) error {
	if m.deleteProvider != nil {
		return m.deleteProvider(ctx, id)
	}
	return nil
}
func (m *mockStore) UpsertProviderEndpoints(ctx context.Context, providerID string, eps []models.ProviderEndpoint) error {
	if m.upsertProviderEndpoints != nil {
		return m.upsertProviderEndpoints(ctx, providerID, eps)
	}
	return nil
}
func (m *mockStore) ListProviderEndpoints(ctx context.Context, providerID string) ([]models.ProviderEndpoint, error) {
	if m.listProviderEndpoints != nil {
		return m.listProviderEndpoints(ctx, providerID)
	}
	return []models.ProviderEndpoint{}, nil
}
func (m *mockStore) UpdateProviderEndpoint(ctx context.Context, ep *models.ProviderEndpoint) error {
	if m.updateProviderEndpoint != nil {
		return m.updateProviderEndpoint(ctx, ep)
	}
	return nil
}
func (m *mockStore) SyncProviderModels(ctx context.Context, providerID string, modelIDs []string) error {
	if m.syncProviderModels != nil {
		return m.syncProviderModels(ctx, providerID, modelIDs)
	}
	return nil
}
func (m *mockStore) CreateProviderModel(ctx context.Context, pm *models.ProviderModel) error {
	if m.createProviderModel != nil {
		return m.createProviderModel(ctx, pm)
	}
	return nil
}
func (m *mockStore) DeleteProviderModel(ctx context.Context, providerID, recordID string) error {
	if m.deleteProviderModel != nil {
		return m.deleteProviderModel(ctx, providerID, recordID)
	}
	return nil
}
func (m *mockStore) SetProviderModelsAvailability(ctx context.Context, providerID string, availableModelIDs []string) error {
	if m.setProviderModelsAvailability != nil {
		return m.setProviderModelsAvailability(ctx, providerID, availableModelIDs)
	}
	return nil
}
func (m *mockStore) UpdateProviderModelAvailability(ctx context.Context, providerID, recordID string, available bool) error {
	if m.updateProviderModelAvailability != nil {
		return m.updateProviderModelAvailability(ctx, providerID, recordID, available)
	}
	return nil
}
func (m *mockStore) ListProviderModels(ctx context.Context, providerID string) ([]models.ProviderModel, error) {
	if m.listProviderModels != nil {
		return m.listProviderModels(ctx, providerID)
	}
	return []models.ProviderModel{}, nil
}
func (m *mockStore) ListAllModels(ctx context.Context) ([]models.ProviderModel, error) {
	if m.listAllModels != nil {
		return m.listAllModels(ctx)
	}
	return []models.ProviderModel{}, nil
}
func (m *mockStore) CreateAlias(ctx context.Context, a *models.ModelAlias) error {
	if m.createAlias != nil {
		return m.createAlias(ctx, a)
	}
	return nil
}
func (m *mockStore) ListAliases(ctx context.Context) ([]models.ModelAlias, error) {
	if m.listAliases != nil {
		return m.listAliases(ctx)
	}
	return []models.ModelAlias{}, nil
}
func (m *mockStore) UpdateAlias(ctx context.Context, a *models.ModelAlias) error {
	if m.updateAlias != nil {
		return m.updateAlias(ctx, a)
	}
	return nil
}
func (m *mockStore) DeleteAlias(ctx context.Context, id string) error {
	if m.deleteAlias != nil {
		return m.deleteAlias(ctx, id)
	}
	return nil
}
func (m *mockStore) ResolveAlias(ctx context.Context, alias string) ([]models.ModelAlias, error) {
	if m.resolveAlias != nil {
		return m.resolveAlias(ctx, alias)
	}
	return []models.ModelAlias{}, nil
}
func (m *mockStore) GetHealthyProvidersForModel(ctx context.Context, modelID string) ([]models.Provider, error) {
	if m.getHealthyProvidersForModel != nil {
		return m.getHealthyProvidersForModel(ctx, modelID)
	}
	return []models.Provider{}, nil
}
func (m *mockStore) GetEnabledEndpoint(ctx context.Context, providerID string, path string) (*models.ProviderEndpoint, error) {
	if m.getEnabledEndpoint != nil {
		return m.getEnabledEndpoint(ctx, providerID, path)
	}
	return nil, nil
}
func (m *mockStore) InsertRequestLog(ctx context.Context, log *models.RequestLog) error {
	if m.insertRequestLog != nil {
		return m.insertRequestLog(ctx, log)
	}
	return nil
}
func (m *mockStore) QueryRequestLogs(ctx context.Context, filter models.LogFilter) ([]models.RequestLog, int, error) {
	if m.queryRequestLogs != nil {
		return m.queryRequestLogs(ctx, filter)
	}
	return []models.RequestLog{}, 0, nil
}
func (m *mockStore) GetRequestLog(ctx context.Context, id string) (*models.RequestLog, error) {
	if m.getRequestLog != nil {
		return m.getRequestLog(ctx, id)
	}
	panic("unexpected call to GetRequestLog")
}
func (m *mockStore) UpdateProviderModelCosts(ctx context.Context, id string, pm *models.ProviderModel) error {
	if m.updateProviderModelCosts != nil {
		return m.updateProviderModelCosts(ctx, id, pm)
	}
	return nil
}
func (m *mockStore) GetProviderModelCosts(ctx context.Context, providerID, modelID string) (*models.ProviderModel, error) {
	if m.getProviderModelCosts != nil {
		return m.getProviderModelCosts(ctx, providerID, modelID)
	}
	return nil, nil
}
func (m *mockStore) GetDashboardStats(ctx context.Context, since, until time.Time) (*models.DashboardStats, error) {
	if m.getDashboardStats != nil {
		return m.getDashboardStats(ctx, since, until)
	}
	return &models.DashboardStats{}, nil
}
func (m *mockStore) GetDashboardStatsForAPIKey(ctx context.Context, apiKeyID string, since, until time.Time) (*models.DashboardStats, error) {
	if m.getDashboardStatsForAPIKey != nil {
		return m.getDashboardStatsForAPIKey(ctx, apiKeyID, since, until)
	}
	return &models.DashboardStats{ByModel: []models.ModelStats{}, ByProvider: []models.ProviderStats{}}, nil
}
func (m *mockStore) GetTimeSeries(ctx context.Context, since, until time.Time, granularity string) ([]models.TimeSeriesPoint, error) {
	if m.getTimeSeries != nil {
		return m.getTimeSeries(ctx, since, until, granularity)
	}
	return []models.TimeSeriesPoint{}, nil
}
func (m *mockStore) TimeSeriesByAPIKey(ctx context.Context, apiKeyID string, since, until time.Time, granularity string) ([]models.TimeSeriesPoint, error) {
	if m.timeSeriesByAPIKey != nil {
		return m.timeSeriesByAPIKey(ctx, apiKeyID, since, until, granularity)
	}
	return []models.TimeSeriesPoint{}, nil
}
func (m *mockStore) GetLifetimeCost(ctx context.Context) (*models.LifetimeCost, error) {
	if m.getLifetimeCost != nil {
		return m.getLifetimeCost(ctx)
	}
	return &models.LifetimeCost{}, nil
}
func (m *mockStore) GetLifetimeCostForAPIKey(ctx context.Context, apiKeyID string) (*models.LifetimeCost, error) {
	if m.getLifetimeCostForAPIKey != nil {
		return m.getLifetimeCostForAPIKey(ctx, apiKeyID)
	}
	return &models.LifetimeCost{}, nil
}
func (m *mockStore) UpdateProviderHealth(ctx context.Context, id string, healthy bool) error {
	if m.updateProviderHealth != nil {
		return m.updateProviderHealth(ctx, id, healthy)
	}
	return nil
}
func (m *mockStore) GetAllConfig(_ context.Context) (map[string]string, error) {
	return map[string]string{}, nil
}
func (m *mockStore) SetConfig(_ context.Context, _, _ string) error {
	return nil
}
func (m *mockStore) InsertStreamingLog(_ context.Context, _ *models.StreamingLog) error {
	return nil
}
func (m *mockStore) GetStreamingLogs(_ context.Context, _ string) ([]models.StreamingLog, error) {
	return nil, nil
}
func (m *mockStore) PurgeStreamingLogBodiesOlderThan(_ context.Context, _ time.Time) (int64, error) {
	return 0, nil
}
func (m *mockStore) PurgeRequestLogRequestBodiesOlderThan(_ context.Context, _ time.Time) (int64, error) {
	return 0, nil
}
func (m *mockStore) PurgeRequestLogResponseBodiesOlderThan(_ context.Context, _ time.Time) (int64, error) {
	return 0, nil
}
func (m *mockStore) LoadRoutingData(_ context.Context) (*models.RoutingData, error) {
	return &models.RoutingData{}, nil
}
func (m *mockStore) CreateAPIKey(ctx context.Context, k *models.APIKey) error {
	if m.createAPIKey != nil {
		return m.createAPIKey(ctx, k)
	}
	return nil
}
func (m *mockStore) GetAPIKeyByHash(ctx context.Context, keyHash string) (*models.APIKey, error) {
	if m.getAPIKeyByHash != nil {
		return m.getAPIKeyByHash(ctx, keyHash)
	}
	return nil, nil
}
func (m *mockStore) GetAPIKey(ctx context.Context, id string) (*models.APIKey, error) {
	if m.getAPIKey != nil {
		return m.getAPIKey(ctx, id)
	}
	return nil, nil
}
func (m *mockStore) ListAPIKeys(ctx context.Context) ([]models.APIKey, error) {
	if m.listAPIKeys != nil {
		return m.listAPIKeys(ctx)
	}
	return nil, nil
}
func (m *mockStore) UpdateAPIKey(ctx context.Context, k *models.APIKey) error {
	if m.updateAPIKey != nil {
		return m.updateAPIKey(ctx, k)
	}
	return nil
}
func (m *mockStore) DeleteAPIKey(ctx context.Context, id string) error {
	if m.deleteAPIKey != nil {
		return m.deleteAPIKey(ctx, id)
	}
	return nil
}
func (m *mockStore) TouchAPIKeyLastUsed(_ context.Context, _ string, _ time.Time) error {
	return nil
}
func (m *mockStore) QueryLogsByAPIKey(_ context.Context, _ string, _ models.LogFilter) ([]models.RequestLog, int, error) {
	return nil, 0, nil
}
func (m *mockStore) UsageByAPIKey(ctx context.Context, since, until time.Time) ([]models.APIKeyUsage, error) {
	if m.usageByAPIKey != nil {
		return m.usageByAPIKey(ctx, since, until)
	}
	return nil, nil
}
func (m *mockStore) UsageByAPIKeyModel(ctx context.Context, apiKeyID string, since, until time.Time) ([]models.ModelStats, error) {
	if m.usageByAPIKeyModel != nil {
		return m.usageByAPIKeyModel(ctx, apiKeyID, since, until)
	}
	panic("unexpected call to UsageByAPIKeyModel")
}
func (m *mockStore) Close() error { return nil }

// serve is a helper that routes a test request through the handler's chi router.
func serve(h *Handler, req *http.Request) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, req)
	return rec
}

// ------------------------------------------------------------------
// Provider tests
// ------------------------------------------------------------------

func testAdminHandler(store *mockStore, cfg HandlerConfig) *Handler {
	acc := stats.NewAccumulator()
	qw := NewQueryWorker(store, 4)
	qw.Start(context.Background())
	return NewHandler(store, cfg, acc, qw)
}

func testAdminHandlerWithStats(store *mockStore, cfg HandlerConfig, acc *stats.Accumulator) *Handler {
	qw := NewQueryWorker(store, 4)
	qw.Start(context.Background())
	return NewHandler(store, cfg, acc, qw)
}

func TestHandleActive(t *testing.T) {
	store := &mockStore{}
	acc := stats.NewAccumulator()
	qw := NewQueryWorker(store, 4)
	qw.Start(context.Background())
	h := NewHandler(store, HandlerConfig{}, acc, qw)
	reg := proxy.NewActiveRegistry()
	h.SetActiveRegistry(reg)

	req := httptest.NewRequest(http.MethodGet, "/admin/active", nil)
	rec := httptest.NewRecorder()
	h.HandleActive(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Active   int                   `json:"active"`
		Requests []proxy.ActiveRequest `json:"requests"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Active != 0 || len(resp.Requests) != 0 {
		t.Errorf("expected 0 active requests, got %d (%d)", resp.Active, len(resp.Requests))
	}

	reg.Begin("gpt-4o", "/v1/chat/completions", "127.0.0.1")
	rec = httptest.NewRecorder()
	h.HandleActive(rec, req)
	resp.Active = 0
	resp.Requests = nil
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Active != 1 || len(resp.Requests) != 1 {
		t.Errorf("expected 1 active request, got %d (%d)", resp.Active, len(resp.Requests))
	}
	if resp.Requests[0].Model != "gpt-4o" || resp.Requests[0].Endpoint != "/v1/chat/completions" {
		t.Errorf("unexpected request record: %+v", resp.Requests[0])
	}
}

func TestCreateProvider_Valid(t *testing.T) {
	store := &mockStore{}
	h := testAdminHandler(store, HandlerConfig{})

	body := `{"name":"Local Ollama","base_url":"http://localhost:11434"}`
	req := httptest.NewRequest(http.MethodPost, "/providers", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := serve(h, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]json.RawMessage
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	var p models.Provider
	if err := json.Unmarshal(resp["provider"], &p); err != nil {
		t.Fatalf("decode provider: %v", err)
	}
	if p.ID == "" {
		t.Error("expected non-empty provider ID (UUID)")
	}
	if p.Name != "Local Ollama" {
		t.Errorf("expected name %q, got %q", "Local Ollama", p.Name)
	}
}

func TestCreateProvider_MissingName(t *testing.T) {
	h := testAdminHandler(&mockStore{}, HandlerConfig{})
	body := `{"base_url":"http://localhost:11434"}`
	req := httptest.NewRequest(http.MethodPost, "/providers", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := serve(h, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestCreateProvider_MissingBaseURL(t *testing.T) {
	h := testAdminHandler(&mockStore{}, HandlerConfig{})
	body := `{"name":"Test"}`
	req := httptest.NewRequest(http.MethodPost, "/providers", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := serve(h, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestGetProvider_OK(t *testing.T) {
	now := time.Now().UTC()
	store := &mockStore{
		getProvider: func(_ context.Context, id string) (*models.Provider, error) {
			return &models.Provider{
				ID:        id,
				Name:      "Test",
				BaseURL:   "http://test",
				CreatedAt: now,
				UpdatedAt: now,
			}, nil
		},
		listProviderEndpoints: func(_ context.Context, _ string) ([]models.ProviderEndpoint, error) {
			return []models.ProviderEndpoint{{ID: "ep1", ProviderID: "p1", Path: "/v1/chat/completions", Method: "POST"}}, nil
		},
		listProviderModels: func(_ context.Context, _ string) ([]models.ProviderModel, error) {
			return []models.ProviderModel{{ID: "m1", ProviderID: "p1", ModelID: "llama3"}}, nil
		},
	}
	h := testAdminHandler(store, HandlerConfig{})

	req := httptest.NewRequest(http.MethodGet, "/providers/p1", nil)
	rec := serve(h, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]json.RawMessage
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	for _, key := range []string{"provider", "endpoints", "models"} {
		if _, ok := resp[key]; !ok {
			t.Errorf("missing key %q in response", key)
		}
	}
}

func TestGetProvider_NotFound(t *testing.T) {
	store := &mockStore{
		getProvider: func(_ context.Context, id string) (*models.Provider, error) {
			return nil, sql.ErrNoRows
		},
	}
	h := testAdminHandler(store, HandlerConfig{})

	req := httptest.NewRequest(http.MethodGet, "/providers/nonexistent", nil)
	rec := serve(h, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestDeleteProvider_OK(t *testing.T) {
	h := testAdminHandler(&mockStore{
		deleteProvider: func(_ context.Context, id string) error { return nil },
	}, HandlerConfig{})
	req := httptest.NewRequest(http.MethodDelete, "/providers/p1", nil)
	rec := serve(h, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", rec.Code, rec.Body.String())
	}
	if rec.Body.Len() != 0 {
		t.Errorf("expected empty body on 204, got %q", rec.Body.String())
	}
}

// ------------------------------------------------------------------
// Alias tests
// ------------------------------------------------------------------

func TestCreateAlias_Valid(t *testing.T) {
	store := &mockStore{
		getProvider: func(_ context.Context, id string) (*models.Provider, error) {
			return &models.Provider{ID: id}, nil
		},
		createAlias: func(_ context.Context, a *models.ModelAlias) error { return nil },
	}
	h := testAdminHandler(store, HandlerConfig{})

	body := `{"alias":"gpt-4","provider_id":"prov1","model_id":"llama3","weight":1,"priority":0}`
	req := httptest.NewRequest(http.MethodPost, "/aliases", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := serve(h, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]json.RawMessage
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	var a models.ModelAlias
	if err := json.Unmarshal(resp["alias"], &a); err != nil {
		t.Fatalf("decode alias: %v", err)
	}
	if a.ID == "" {
		t.Error("expected non-empty alias ID")
	}
	if !a.IsEnabled {
		t.Error("new alias should default to is_enabled=true")
	}
}

func TestUpdateAlias_IdentityFields(t *testing.T) {
	now := time.Now().UTC()
	var updated *models.ModelAlias
	store := &mockStore{
		getProvider: func(_ context.Context, id string) (*models.Provider, error) {
			if id == "p2" {
				return &models.Provider{ID: id}, nil
			}
			return nil, sql.ErrNoRows
		},
		listAliases: func(_ context.Context) ([]models.ModelAlias, error) {
			return []models.ModelAlias{{
				ID: "a1", Alias: "gpt-4", ProviderID: "p1", ModelID: "llama3",
				Weight: 1, Priority: 0, IsEnabled: true, CreatedAt: now, UpdatedAt: now,
			}}, nil
		},
		updateAlias: func(_ context.Context, a *models.ModelAlias) error {
			cp := *a
			updated = &cp
			return nil
		},
	}
	h := testAdminHandler(store, HandlerConfig{})

	body := `{"alias":"claude","provider_id":"p2","model_id":"claude-3","weight":3,"priority":10}`
	req := httptest.NewRequest(http.MethodPut, "/aliases/a1", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := serve(h, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if updated == nil {
		t.Fatal("expected UpdateAlias to be called")
	}
	if updated.Alias != "claude" {
		t.Errorf("expected alias %q, got %q", "claude", updated.Alias)
	}
	if updated.ProviderID != "p2" {
		t.Errorf("expected provider_id %q, got %q", "p2", updated.ProviderID)
	}
	if updated.ModelID != "claude-3" {
		t.Errorf("expected model_id %q, got %q", "claude-3", updated.ModelID)
	}
	if updated.Weight != 3 || updated.Priority != 10 {
		t.Errorf("expected weight=3 priority=10, got weight=%d priority=%d", updated.Weight, updated.Priority)
	}

	var resp map[string]json.RawMessage
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	var a models.ModelAlias
	if err := json.Unmarshal(resp["alias"], &a); err != nil {
		t.Fatalf("decode alias: %v", err)
	}
	if a.Alias != "claude" || a.ProviderID != "p2" || a.ModelID != "claude-3" {
		t.Errorf("response alias mismatch: %+v", a)
	}
}

func TestNotifyRoutingChanged_OnMutations(t *testing.T) {
	now := time.Now().UTC()
	enabled := true
	cases := []struct {
		name string
		run  func(h *Handler) *httptest.ResponseRecorder
	}{
		{
			name: "create provider",
			run: func(h *Handler) *httptest.ResponseRecorder {
				body := `{"name":"Local","base_url":"http://localhost:11434"}`
				req := httptest.NewRequest(http.MethodPost, "/providers", strings.NewReader(body))
				req.Header.Set("Content-Type", "application/json")
				return serve(h, req)
			},
		},
		{
			name: "update provider",
			run: func(h *Handler) *httptest.ResponseRecorder {
				body := `{"name":"Renamed","base_url":"http://localhost:11434"}`
				req := httptest.NewRequest(http.MethodPut, "/providers/p1", strings.NewReader(body))
				req.Header.Set("Content-Type", "application/json")
				return serve(h, req)
			},
		},
		{
			name: "delete provider",
			run: func(h *Handler) *httptest.ResponseRecorder {
				return serve(h, httptest.NewRequest(http.MethodDelete, "/providers/p1", nil))
			},
		},
		{
			name: "update endpoint",
			run: func(h *Handler) *httptest.ResponseRecorder {
				body := `{"is_enabled":false}`
				req := httptest.NewRequest(http.MethodPut, "/providers/p1/endpoints/ep1", strings.NewReader(body))
				req.Header.Set("Content-Type", "application/json")
				return serve(h, req)
			},
		},
		{
			name: "create alias",
			run: func(h *Handler) *httptest.ResponseRecorder {
				body := `{"alias":"gpt-4","provider_id":"p1","model_id":"llama3"}`
				req := httptest.NewRequest(http.MethodPost, "/aliases", strings.NewReader(body))
				req.Header.Set("Content-Type", "application/json")
				return serve(h, req)
			},
		},
		{
			name: "update alias",
			run: func(h *Handler) *httptest.ResponseRecorder {
				body := `{"alias":"claude","provider_id":"p1","model_id":"claude-3","weight":5}`
				req := httptest.NewRequest(http.MethodPut, "/aliases/a1", strings.NewReader(body))
				req.Header.Set("Content-Type", "application/json")
				return serve(h, req)
			},
		},
		{
			name: "delete alias",
			run: func(h *Handler) *httptest.ResponseRecorder {
				return serve(h, httptest.NewRequest(http.MethodDelete, "/aliases/a1", nil))
			},
		},
	}

	store := &mockStore{
		getProvider: func(_ context.Context, id string) (*models.Provider, error) {
			return &models.Provider{ID: id, Name: "Test", BaseURL: "http://test", CreatedAt: now, UpdatedAt: now}, nil
		},
		updateProvider: func(_ context.Context, _ *models.Provider) error { return nil },
		deleteProvider: func(_ context.Context, _ string) error { return nil },
		listProviderEndpoints: func(_ context.Context, _ string) ([]models.ProviderEndpoint, error) {
			return []models.ProviderEndpoint{{ID: "ep1", ProviderID: "p1", Path: "/v1/chat/completions", Method: "POST", IsEnabled: enabled}}, nil
		},
		updateProviderEndpoint: func(_ context.Context, _ *models.ProviderEndpoint) error { return nil },
		createAlias:            func(_ context.Context, _ *models.ModelAlias) error { return nil },
		listAliases: func(_ context.Context) ([]models.ModelAlias, error) {
			return []models.ModelAlias{{ID: "a1", Alias: "gpt-4", ProviderID: "p1", ModelID: "llama3", Weight: 1, IsEnabled: true, CreatedAt: now, UpdatedAt: now}}, nil
		},
		updateAlias: func(_ context.Context, _ *models.ModelAlias) error { return nil },
		deleteAlias: func(_ context.Context, _ string) error { return nil },
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var reloads int
			h := testAdminHandler(store, HandlerConfig{
				OnRoutingChanged: func() { reloads++ },
			})
			rec := tc.run(h)
			if rec.Code >= 400 {
				t.Fatalf("unexpected status %d: %s", rec.Code, rec.Body.String())
			}
			if reloads != 1 {
				t.Fatalf("expected 1 routing reload, got %d", reloads)
			}
		})
	}
}

func TestNotifyRoutingChanged_NotCalledOnValidationError(t *testing.T) {
	var reloads int
	h := testAdminHandler(&mockStore{}, HandlerConfig{
		OnRoutingChanged: func() { reloads++ },
	})
	body := `{"base_url":"http://localhost:11434"}`
	req := httptest.NewRequest(http.MethodPost, "/providers", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := serve(h, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
	if reloads != 0 {
		t.Fatalf("expected no routing reload on validation error, got %d", reloads)
	}
}

func TestListAliases_OK(t *testing.T) {
	now := time.Now().UTC()
	expected := []models.ModelAlias{
		{ID: "a1", Alias: "gpt-4", ProviderID: "p1", ModelID: "llama3", Weight: 1, IsEnabled: true, CreatedAt: now, UpdatedAt: now},
		{ID: "a2", Alias: "gpt-3.5", ProviderID: "p1", ModelID: "llama2", Weight: 1, IsEnabled: true, CreatedAt: now, UpdatedAt: now},
	}
	store := &mockStore{
		listAliases: func(_ context.Context) ([]models.ModelAlias, error) {
			return expected, nil
		},
	}
	h := testAdminHandler(store, HandlerConfig{})

	req := httptest.NewRequest(http.MethodGet, "/aliases", nil)
	rec := serve(h, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Aliases []models.ModelAlias `json:"aliases"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Aliases) != len(expected) {
		t.Errorf("expected %d aliases, got %d", len(expected), len(resp.Aliases))
	}
	for i, a := range resp.Aliases {
		if a.ID != expected[i].ID {
			t.Errorf("alias[%d].ID: want %q, got %q", i, expected[i].ID, a.ID)
		}
	}
}

// ------------------------------------------------------------------
// Log tests
// ------------------------------------------------------------------

func TestQueryLogs_Filters(t *testing.T) {
	var capturedFilter models.LogFilter
	store := &mockStore{
		queryRequestLogs: func(_ context.Context, f models.LogFilter) ([]models.RequestLog, int, error) {
			capturedFilter = f
			return []models.RequestLog{}, 0, nil
		},
	}
	h := testAdminHandler(store, HandlerConfig{})

	req := httptest.NewRequest(http.MethodGet, "/logs?model=gpt-4&provider_id=prov1", nil)
	rec := serve(h, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if capturedFilter.Model != "gpt-4" {
		t.Errorf("expected Model=%q, got %q", "gpt-4", capturedFilter.Model)
	}
	if capturedFilter.ProviderID != "prov1" {
		t.Errorf("expected ProviderID=%q, got %q", "prov1", capturedFilter.ProviderID)
	}
}

func TestQueryLogs_DefaultPagination(t *testing.T) {
	var capturedFilter models.LogFilter
	store := &mockStore{
		queryRequestLogs: func(_ context.Context, f models.LogFilter) ([]models.RequestLog, int, error) {
			capturedFilter = f
			return []models.RequestLog{}, 42, nil
		},
	}
	h := testAdminHandler(store, HandlerConfig{})

	req := httptest.NewRequest(http.MethodGet, "/logs", nil)
	rec := serve(h, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if capturedFilter.Limit != 50 {
		t.Errorf("expected default Limit=50, got %d", capturedFilter.Limit)
	}
	if capturedFilter.Offset != 0 {
		t.Errorf("expected default Offset=0, got %d", capturedFilter.Offset)
	}

	var resp struct {
		Logs  []models.RequestLog `json:"logs"`
		Total int                 `json:"total"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Total != 42 {
		t.Errorf("expected total=42, got %d", resp.Total)
	}
}

func TestQueryLogs_InvalidLimit(t *testing.T) {
	h := testAdminHandler(&mockStore{}, HandlerConfig{})
	req := httptest.NewRequest(http.MethodGet, "/logs?limit=0", nil)
	rec := serve(h, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for limit=0, got %d", rec.Code)
	}
}

// ------------------------------------------------------------------
// Stats test
// ------------------------------------------------------------------

func TestGetStats_OK(t *testing.T) {
	acc := stats.NewAccumulator()
	now := time.Now().UTC()
	pt, ct, tt := 10, 5, 15
	for i := 0; i < 100; i++ {
		acc.Record(&models.RequestLog{
			Timestamp: now, RequestedModel: "llama3", ProviderID: "p1", ProviderName: "Local",
			StatusCode: 200, TotalTimeMs: 250, PromptTokens: &pt, CompletionTokens: &ct, TotalTokens: &tt,
		}, nil)
	}
	h := testAdminHandlerWithStats(&mockStore{}, HandlerConfig{}, acc)

	req := httptest.NewRequest(http.MethodGet, "/stats?since=24h", nil)
	rec := serve(h, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var got models.DashboardStats
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.TotalRequests != 100 {
		t.Errorf("TotalRequests: want 100, got %d", got.TotalRequests)
	}
	if got.AvgLatencyMs != 250 {
		t.Errorf("AvgLatencyMs: want 250, got %f", got.AvgLatencyMs)
	}
	if len(got.ByModel) != 1 || got.ByModel[0].Model != "llama3" {
		t.Errorf("ByModel mismatch: %+v", got.ByModel)
	}
}

func TestGetStats_InvalidSince(t *testing.T) {
	h := testAdminHandler(&mockStore{}, HandlerConfig{})
	req := httptest.NewRequest(http.MethodGet, "/stats?since=notaduration", nil)
	rec := serve(h, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid since, got %d", rec.Code)
	}
}

func TestGetStats_DayShorthand(t *testing.T) {
	acc := stats.NewAccumulator()
	acc.Record(&models.RequestLog{Timestamp: time.Now().UTC(), StatusCode: 200, TotalTimeMs: 1}, nil)
	h := testAdminHandlerWithStats(&mockStore{}, HandlerConfig{}, acc)
	req := httptest.NewRequest(http.MethodGet, "/stats?since=7d", nil)
	rec := serve(h, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestGetStats_FromTo(t *testing.T) {
	from := time.Now().UTC().Add(-48 * time.Hour).Format(time.RFC3339)
	to := time.Now().UTC().Format(time.RFC3339)
	called := false
	h := testAdminHandlerWithStats(&mockStore{
		getDashboardStats: func(_ context.Context, since, until time.Time) (*models.DashboardStats, error) {
			called = true
			if since.IsZero() || until.IsZero() {
				t.Fatal("expected non-zero since/until")
			}
			return &models.DashboardStats{TotalRequests: 3, ByModel: []models.ModelStats{}, ByProvider: []models.ProviderStats{}}, nil
		},
	}, HandlerConfig{}, stats.NewAccumulator())

	req := httptest.NewRequest(http.MethodGet, "/stats?from="+from+"&to="+to, nil)
	rec := serve(h, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if !called {
		t.Fatal("expected GetDashboardStats to be called for from/to window")
	}

	// Missing to
	req = httptest.NewRequest(http.MethodGet, "/stats?from="+from, nil)
	rec = serve(h, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing to, got %d", rec.Code)
	}
}

func TestGetTimeSeries_FromTo(t *testing.T) {
	from := time.Now().UTC().Add(-24 * time.Hour).Format(time.RFC3339Nano)
	to := time.Now().UTC().Format(time.RFC3339Nano)
	called := false
	h := testAdminHandlerWithStats(&mockStore{
		getTimeSeries: func(_ context.Context, since, until time.Time, granularity string) ([]models.TimeSeriesPoint, error) {
			called = true
			if granularity != "hour" {
				t.Fatalf("granularity: got %q, want hour", granularity)
			}
			if until.Before(since) {
				t.Fatal("until before since")
			}
			return []models.TimeSeriesPoint{{Bucket: "2026-01-01T00:00:00", Requests: 1}}, nil
		},
	}, HandlerConfig{}, stats.NewAccumulator())

	req := httptest.NewRequest(http.MethodGet, "/stats/timeseries?from="+url.QueryEscape(from)+"&to="+url.QueryEscape(to)+"&granularity=hour", nil)
	rec := serve(h, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if !called {
		t.Fatal("expected GetTimeSeries to be called for from/to window")
	}
}

func TestParseStatsWindow(t *testing.T) {
	from := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)

	cases := []struct {
		name    string
		q       url.Values
		wantDB  bool
		wantErr bool
	}{
		{"relative since", url.Values{"since": []string{"7d"}}, false, false},
		{"absolute from/to", url.Values{"from": []string{from.Format(time.RFC3339)}, "to": []string{to.Format(time.RFC3339)}}, true, false},
		{"nano timestamps", url.Values{"from": []string{from.Format(time.RFC3339Nano)}, "to": []string{to.Format(time.RFC3339Nano)}}, true, false},
		{"missing to", url.Values{"from": []string{from.Format(time.RFC3339)}}, false, true},
		{"from after to", url.Values{"from": []string{to.Format(time.RFC3339)}, "to": []string{from.Format(time.RFC3339)}}, false, true},
		{"invalid from", url.Values{"from": []string{"not-a-date"}, "to": []string{to.Format(time.RFC3339)}}, false, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, useDB, err := parseStatsWindow(tc.q)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if useDB != tc.wantDB {
				t.Fatalf("useDB: got %v, want %v", useDB, tc.wantDB)
			}
		})
	}
}

func TestGetLifetimeCost(t *testing.T) {
	h := testAdminHandlerWithStats(&mockStore{
		getLifetimeCost: func(_ context.Context) (*models.LifetimeCost, error) {
			return &models.LifetimeCost{TotalCostUSD: 1.25, TotalRequests: 10}, nil
		},
	}, HandlerConfig{}, stats.NewAccumulator())
	req := httptest.NewRequest(http.MethodGet, "/stats/lifetime", nil)
	rec := serve(h, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var body models.LifetimeCost
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.TotalCostUSD != 1.25 || body.TotalRequests != 10 {
		t.Fatalf("unexpected body: %+v", body)
	}
}

// ------------------------------------------------------------------
// parseDurationParam unit tests
// ------------------------------------------------------------------

func TestParseDurationParam(t *testing.T) {
	cases := []struct {
		input   string
		wantErr bool
		wantDur time.Duration
	}{
		{"", true, 0},
		{"24h", false, 24 * time.Hour},
		{"1h30m", false, 90 * time.Minute},
		{"7d", false, 7 * 24 * time.Hour},
		{"30d", false, 30 * 24 * time.Hour},
		{"notaduration", true, 0},
		{"xd", true, 0},
	}
	for _, tc := range cases {
		d, err := parseDurationParam(tc.input)
		if tc.wantErr {
			if err == nil {
				t.Errorf("parseDurationParam(%q): expected error, got nil", tc.input)
			}
		} else {
			if err != nil {
				t.Errorf("parseDurationParam(%q): unexpected error: %v", tc.input, err)
			} else if d != tc.wantDur {
				t.Errorf("parseDurationParam(%q): want %v, got %v", tc.input, tc.wantDur, d)
			}
		}
	}
}

func TestHandleListAPIKeys(t *testing.T) {
	store := &mockStore{}
	store.listAPIKeys = func(ctx context.Context) ([]models.APIKey, error) {
		return []models.APIKey{
			{ID: "k1", Name: "key one", IsActive: true, CreatedAt: time.Now().UTC()},
			{ID: "k2", Name: "key two", IsActive: false, CreatedAt: time.Now().UTC()},
		}, nil
	}
	h := testAdminHandler(store, HandlerConfig{})

	req := httptest.NewRequest(http.MethodGet, "/keys", nil)
	rec := serve(h, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Keys []models.APIKey `json:"keys"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Keys) != 2 || resp.Keys[0].Name != "key one" {
		t.Fatalf("unexpected keys: %+v", resp.Keys)
	}
}

func TestHandleCreateAPIKey(t *testing.T) {
	store := &mockStore{}
	store.createAPIKey = func(ctx context.Context, k *models.APIKey) error {
		return nil
	}
	h := testAdminHandler(store, HandlerConfig{})

	body := `{"name":"my key","rate_limit_rpm":10}`
	req := httptest.NewRequest(http.MethodPost, "/keys", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := serve(h, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp struct {
		APIKey models.APIKey `json:"api_key"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.APIKey.Name != "my key" || resp.APIKey.KeyHash != "" {
		t.Fatalf("unexpected api key: %+v", resp.APIKey)
	}
}

func TestHandleCreateAPIKey_DuplicateName(t *testing.T) {
	store, err := db.NewSQLiteStore(":memory:")
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	h := testAdminHandlerReal(store, HandlerConfig{})

	post := func(name string) *httptest.ResponseRecorder {
		t.Helper()
		body := fmt.Sprintf(`{"name":%q}`, name)
		req := httptest.NewRequest(http.MethodPost, "/keys", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		return serve(h, req)
	}

	if rec := post("dup"); rec.Code != http.StatusCreated {
		t.Fatalf("first create: %d %s", rec.Code, rec.Body.String())
	}
	rec := post("DUP")
	if rec.Code != http.StatusConflict {
		t.Fatalf("duplicate: %d %s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Error string `json:"error"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(resp.Error, "already exists") {
		t.Fatalf("error: %q", resp.Error)
	}
}

func TestHandleCreateAPIKey_MissingName(t *testing.T) {
	h := testAdminHandler(&mockStore{}, HandlerConfig{})

	body := `{"name":""}`
	req := httptest.NewRequest(http.MethodPost, "/keys", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := serve(h, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleUpdateAPIKey(t *testing.T) {
	store := &mockStore{}
	store.getAPIKey = func(ctx context.Context, id string) (*models.APIKey, error) {
		return &models.APIKey{ID: id, Name: "old", IsActive: true}, nil
	}
	store.updateAPIKey = func(ctx context.Context, k *models.APIKey) error {
		return nil
	}
	h := testAdminHandler(store, HandlerConfig{})

	body := `{"name":"renamed","is_active":false}`
	req := httptest.NewRequest(http.MethodPut, "/keys/k1", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := serve(h, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleUpdateAPIKey_ToggleKeepsLimits(t *testing.T) {
	rpm, tpm := 10, 20
	store := &mockStore{}
	store.getAPIKey = func(ctx context.Context, id string) (*models.APIKey, error) {
		return &models.APIKey{ID: id, Name: "old", IsActive: true, RateLimitRPM: &rpm, RateLimitTPM: &tpm}, nil
	}
	var saved *models.APIKey
	store.updateAPIKey = func(ctx context.Context, k *models.APIKey) error {
		cp := *k
		if k.RateLimitRPM != nil {
			v := *k.RateLimitRPM
			cp.RateLimitRPM = &v
		}
		if k.RateLimitTPM != nil {
			v := *k.RateLimitTPM
			cp.RateLimitTPM = &v
		}
		saved = &cp
		return nil
	}
	h := testAdminHandler(store, HandlerConfig{})

	body := `{"name":"old","is_active":false,"rate_limit_rpm":10,"rate_limit_tpm":20}`
	req := httptest.NewRequest(http.MethodPut, "/keys/k1", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := serve(h, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if saved == nil || saved.IsActive || saved.RateLimitRPM == nil || *saved.RateLimitRPM != 10 || saved.RateLimitTPM == nil || *saved.RateLimitTPM != 20 {
		t.Fatalf("toggle should keep both limits, got %+v", saved)
	}
}

func TestHandleUpdateAPIKey_NullClearsLimits(t *testing.T) {
	rpm, tpm := 10, 20
	store := &mockStore{}
	store.getAPIKey = func(ctx context.Context, id string) (*models.APIKey, error) {
		return &models.APIKey{ID: id, Name: "old", IsActive: true, RateLimitRPM: &rpm, RateLimitTPM: &tpm}, nil
	}
	var saved *models.APIKey
	store.updateAPIKey = func(ctx context.Context, k *models.APIKey) error {
		cp := *k
		saved = &cp
		return nil
	}
	h := testAdminHandler(store, HandlerConfig{})

	body := `{"name":"old","is_active":true,"rate_limit_rpm":null,"rate_limit_tpm":null}`
	req := httptest.NewRequest(http.MethodPut, "/keys/k1", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := serve(h, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if saved == nil || saved.RateLimitRPM != nil || saved.RateLimitTPM != nil {
		t.Fatalf("explicit null should clear limits, got %+v", saved)
	}
}

func TestHandleMyStats_OmitsProvider(t *testing.T) {
	store := &mockStore{}
	store.getAPIKeyByHash = func(ctx context.Context, keyHash string) (*models.APIKey, error) {
		if keyHash == "" {
			return nil, nil
		}
		return &models.APIKey{ID: "k1", Name: "my key", IsActive: true}, nil
	}
	store.getDashboardStatsForAPIKey = func(ctx context.Context, apiKeyID string, since, until time.Time) (*models.DashboardStats, error) {
		if apiKeyID != "k1" {
			t.Fatalf("expected key k1, got %q", apiKeyID)
		}
		return &models.DashboardStats{
			TotalRequests: 4,
			ByModel:       []models.ModelStats{{Model: "llama3", RequestCount: 4}},
			ByProvider:    []models.ProviderStats{{ProviderID: "p1", ProviderName: "secret", RequestCount: 4}},
		}, nil
	}
	h := testAdminHandler(store, HandlerConfig{})

	req := httptest.NewRequest(http.MethodGet, "/me/stats?since=24h", nil)
	req.Header.Set("Authorization", "Bearer testtoken")
	rec := httptest.NewRecorder()
	h.HandleMyStats(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var raw map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&raw); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if _, ok := raw["by_provider"]; ok {
		t.Fatalf("key stats must omit by_provider: %v", raw)
	}
	if _, ok := raw["by_api_key"]; ok {
		t.Fatalf("key stats must omit by_api_key: %v", raw)
	}
	if _, ok := raw["active_requests"]; ok {
		t.Fatalf("key stats must omit active_requests: %v", raw)
	}
	if raw["total_requests"] != float64(4) {
		t.Fatalf("total_requests: %v", raw["total_requests"])
	}
}

func TestHandleMyStats_Unauthorized(t *testing.T) {
	h := testAdminHandler(&mockStore{}, HandlerConfig{})
	req := httptest.NewRequest(http.MethodGet, "/me/stats", nil)
	rec := httptest.NewRecorder()
	h.HandleMyStats(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestHandleDeleteAPIKey(t *testing.T) {
	store := &mockStore{}
	store.deleteAPIKey = func(ctx context.Context, id string) error {
		return nil
	}
	h := testAdminHandler(store, HandlerConfig{})

	req := httptest.NewRequest(http.MethodDelete, "/keys/k1", nil)
	rec := serve(h, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleUsageByKey(t *testing.T) {
	store := &mockStore{}
	store.usageByAPIKey = func(ctx context.Context, since, until time.Time) ([]models.APIKeyUsage, error) {
		return []models.APIKeyUsage{
			{APIKeyID: "k1", APIKeyName: "key one", RequestCount: 5, TotalTokens: 100, TotalCostUSD: 1.0},
		}, nil
	}
	h := testAdminHandler(store, HandlerConfig{})

	req := httptest.NewRequest(http.MethodGet, "/usage?since=2026-01-01T00:00:00Z", nil)
	rec := serve(h, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleMyUsage_Unauthorized(t *testing.T) {
	store := &mockStore{}
	store.getAPIKeyByHash = func(ctx context.Context, keyHash string) (*models.APIKey, error) {
		return nil, nil
	}
	h := testAdminHandler(store, HandlerConfig{})

	req := httptest.NewRequest(http.MethodGet, "/me/usage", nil)
	rec := httptest.NewRecorder()
	h.HandleMyUsage(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleMyUsage_OK(t *testing.T) {
	store := &mockStore{}
	store.getAPIKeyByHash = func(ctx context.Context, keyHash string) (*models.APIKey, error) {
		if keyHash == "" {
			return nil, nil
		}
		return &models.APIKey{ID: "k1", Name: "my key", IsActive: true}, nil
	}
	store.usageByAPIKey = func(ctx context.Context, since, until time.Time) ([]models.APIKeyUsage, error) {
		return []models.APIKeyUsage{
			{APIKeyID: "k1", APIKeyName: "my key", RequestCount: 5, TotalTokens: 100, TotalCostUSD: 1.0},
			{APIKeyID: "k2", APIKeyName: "other", RequestCount: 9, TotalTokens: 200, TotalCostUSD: 2.0},
		}, nil
	}
	store.usageByAPIKeyModel = func(ctx context.Context, apiKeyID string, since, until time.Time) ([]models.ModelStats, error) {
		if apiKeyID != "k1" {
			t.Fatalf("expected api key id k1, got %q", apiKeyID)
		}
		return []models.ModelStats{
			{Model: "gpt-4", RequestCount: 3, TotalTokens: 60, ErrorCount: 1, AvgLatencyMs: 250},
			{Model: "gpt-3.5", RequestCount: 2, TotalTokens: 40, ErrorCount: 0, AvgLatencyMs: 150},
		}, nil
	}
	h := testAdminHandler(store, HandlerConfig{})

	req := httptest.NewRequest(http.MethodGet, "/me/usage?since=2026-01-01T00:00:00Z", nil)
	req.Header.Set("Authorization", "Bearer testtoken")
	rec := httptest.NewRecorder()
	h.HandleMyUsage(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Usage   models.APIKeyUsage  `json:"usage"`
		ByModel []models.ModelStats `json:"by_model"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Usage.APIKeyID != "k1" || resp.Usage.RequestCount != 5 {
		t.Fatalf("unexpected usage: %+v", resp.Usage)
	}
	if len(resp.ByModel) != 2 || resp.ByModel[0].Model != "gpt-4" || resp.ByModel[0].RequestCount != 3 {
		t.Fatalf("unexpected by_model: %+v", resp.ByModel)
	}
}

func TestHandleMyTimeSeries_Unauthorized(t *testing.T) {
	store := &mockStore{}
	store.getAPIKeyByHash = func(ctx context.Context, keyHash string) (*models.APIKey, error) {
		return nil, nil
	}
	h := testAdminHandler(store, HandlerConfig{})

	req := httptest.NewRequest(http.MethodGet, "/me/usage/timeseries", nil)
	rec := httptest.NewRecorder()
	h.HandleMyTimeSeries(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleMyTimeSeries_OK(t *testing.T) {
	store := &mockStore{}
	store.getAPIKeyByHash = func(ctx context.Context, keyHash string) (*models.APIKey, error) {
		if keyHash == "" {
			return nil, nil
		}
		return &models.APIKey{ID: "k1", Name: "my key", IsActive: true}, nil
	}
	store.timeSeriesByAPIKey = func(ctx context.Context, apiKeyID string, since, until time.Time, granularity string) ([]models.TimeSeriesPoint, error) {
		if apiKeyID != "k1" {
			t.Fatalf("expected api key id k1, got %q", apiKeyID)
		}
		if granularity != "hour" {
			t.Fatalf("expected default granularity hour, got %q", granularity)
		}
		return []models.TimeSeriesPoint{
			{Bucket: since.Format("2006-01-02T15:00:00"), Requests: 3, TotalTokens: 60},
			{Bucket: since.Add(time.Hour).Format("2006-01-02T15:00:00"), Requests: 2, TotalTokens: 40},
		}, nil
	}
	h := testAdminHandler(store, HandlerConfig{})

	req := httptest.NewRequest(http.MethodGet, "/me/usage/timeseries?since=2026-01-01T00:00:00Z&until=2026-01-01T23:00:00Z", nil)
	req.Header.Set("Authorization", "Bearer testtoken")
	rec := httptest.NewRecorder()
	h.HandleMyTimeSeries(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Points []models.TimeSeriesPoint `json:"points"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Points) != 2 || resp.Points[0].Requests != 3 {
		t.Fatalf("unexpected points: %+v", resp.Points)
	}
}

func TestHandleMyTimeSeries_BadGranularity(t *testing.T) {
	store := &mockStore{}
	store.getAPIKeyByHash = func(ctx context.Context, keyHash string) (*models.APIKey, error) {
		return &models.APIKey{ID: "k1", Name: "my key", IsActive: true}, nil
	}
	h := testAdminHandler(store, HandlerConfig{})

	req := httptest.NewRequest(http.MethodGet, "/me/usage/timeseries?since=2026-01-01T00:00:00Z&granularity=week", nil)
	req.Header.Set("Authorization", "Bearer testtoken")
	rec := httptest.NewRecorder()
	h.HandleMyTimeSeries(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleMe(t *testing.T) {
	adminKey := "secret-admin-key"
	activeKey := "sk-active"
	hash := proxy.HashKey(activeKey)
	store := &mockStore{
		getAPIKeyByHash: func(ctx context.Context, keyHash string) (*models.APIKey, error) {
			if keyHash == hash {
				return &models.APIKey{ID: "k1", Name: "my key", IsActive: true}, nil
			}
			return nil, nil
		},
	}
	h := testAdminHandler(store, HandlerConfig{AccessKey: adminKey})

	req := httptest.NewRequest(http.MethodGet, "/admin/me", nil)
	req.Header.Set("X-Access-Key", adminKey)
	rec := httptest.NewRecorder()
	h.HandleMe(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for admin, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["role"] != "admin" {
		t.Fatalf("expected role admin, got %+v", resp)
	}

	req = httptest.NewRequest(http.MethodGet, "/admin/me", nil)
	req.Header.Set("Authorization", "Bearer "+activeKey)
	rec = httptest.NewRecorder()
	h.HandleMe(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for key, got %d: %s", rec.Code, rec.Body.String())
	}
	var keyResp struct {
		Role       string `json:"role"`
		APIKeyID   string `json:"api_key_id"`
		APIKeyName string `json:"api_key_name"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&keyResp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if keyResp.Role != "key" || keyResp.APIKeyID != "k1" || keyResp.APIKeyName != "my key" {
		t.Fatalf("unexpected key scope: %+v", keyResp)
	}

	req = httptest.NewRequest(http.MethodGet, "/admin/me", nil)
	rec = httptest.NewRecorder()
	h.HandleMe(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", rec.Code, rec.Body.String())
	}
}
