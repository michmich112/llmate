package admin

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/llmate/gateway/internal/models"
)

// mockStore satisfies db.Store using optional function fields.
// Methods that are not needed by a specific test can be left nil;
// they panic with "unexpected call" to surface unintended invocations.
type mockStore struct {
	createProvider              func(ctx context.Context, p *models.Provider) error
	getProvider                 func(ctx context.Context, id string) (*models.Provider, error)
	listProviders               func(ctx context.Context) ([]models.Provider, error)
	updateProvider              func(ctx context.Context, p *models.Provider) error
	deleteProvider              func(ctx context.Context, id string) error
	upsertProviderEndpoints     func(ctx context.Context, providerID string, eps []models.ProviderEndpoint) error
	listProviderEndpoints       func(ctx context.Context, providerID string) ([]models.ProviderEndpoint, error)
	updateProviderEndpoint      func(ctx context.Context, ep *models.ProviderEndpoint) error
	syncProviderModels          func(ctx context.Context, providerID string, modelIDs []string) error
	listProviderModels          func(ctx context.Context, providerID string) ([]models.ProviderModel, error)
	listAllModels               func(ctx context.Context) ([]models.ProviderModel, error)
	createAlias                 func(ctx context.Context, a *models.ModelAlias) error
	listAliases                 func(ctx context.Context) ([]models.ModelAlias, error)
	updateAlias                 func(ctx context.Context, a *models.ModelAlias) error
	deleteAlias                 func(ctx context.Context, id string) error
	resolveAlias                func(ctx context.Context, alias string) ([]models.ModelAlias, error)
	getHealthyProvidersForModel func(ctx context.Context, modelID string) ([]models.Provider, error)
	getEnabledEndpoint          func(ctx context.Context, providerID string, path string) (*models.ProviderEndpoint, error)
	insertRequestLog            func(ctx context.Context, log *models.RequestLog) error
	queryRequestLogs            func(ctx context.Context, filter models.LogFilter) ([]models.RequestLog, int, error)
	getRequestLog               func(ctx context.Context, id string) (*models.RequestLog, error)
	updateProviderModelCosts    func(ctx context.Context, id string, m *models.ProviderModel) error
	getProviderModelCosts       func(ctx context.Context, providerID, modelID string) (*models.ProviderModel, error)
	getDashboardStats           func(ctx context.Context, since time.Time) (*models.DashboardStats, error)
	getTimeSeries               func(ctx context.Context, since, until time.Time, granularity string) ([]models.TimeSeriesPoint, error)
	updateProviderHealth        func(ctx context.Context, id string, healthy bool) error
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
func (m *mockStore) GetDashboardStats(ctx context.Context, since time.Time) (*models.DashboardStats, error) {
	if m.getDashboardStats != nil {
		return m.getDashboardStats(ctx, since)
	}
	return &models.DashboardStats{}, nil
}
func (m *mockStore) GetTimeSeries(ctx context.Context, since, until time.Time, granularity string) ([]models.TimeSeriesPoint, error) {
	if m.getTimeSeries != nil {
		return m.getTimeSeries(ctx, since, until, granularity)
	}
	return []models.TimeSeriesPoint{}, nil
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

func TestCreateProvider_Valid(t *testing.T) {
	store := &mockStore{}
	h := NewHandler(store)

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
	h := NewHandler(&mockStore{})
	body := `{"base_url":"http://localhost:11434"}`
	req := httptest.NewRequest(http.MethodPost, "/providers", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := serve(h, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestCreateProvider_MissingBaseURL(t *testing.T) {
	h := NewHandler(&mockStore{})
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
	h := NewHandler(store)

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
	h := NewHandler(store)

	req := httptest.NewRequest(http.MethodGet, "/providers/nonexistent", nil)
	rec := serve(h, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestDeleteProvider_OK(t *testing.T) {
	h := NewHandler(&mockStore{
		deleteProvider: func(_ context.Context, id string) error { return nil },
	})
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
	h := NewHandler(store)

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
	h := NewHandler(store)

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
	h := NewHandler(store)

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
	h := NewHandler(store)

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
	h := NewHandler(&mockStore{})
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
	wantStats := &models.DashboardStats{
		TotalRequests: 100,
		AvgLatencyMs:  250.5,
		ErrorRate:     0.02,
		ByModel: []models.ModelStats{
			{Model: "llama3", RequestCount: 80, AvgLatencyMs: 200.0, TotalTokens: 40000},
		},
		ByProvider: []models.ProviderStats{
			{ProviderID: "p1", ProviderName: "Local", RequestCount: 100},
		},
	}
	store := &mockStore{
		getDashboardStats: func(_ context.Context, since time.Time) (*models.DashboardStats, error) {
			return wantStats, nil
		},
	}
	h := NewHandler(store)

	req := httptest.NewRequest(http.MethodGet, "/stats?since=24h", nil)
	rec := serve(h, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var got models.DashboardStats
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.TotalRequests != wantStats.TotalRequests {
		t.Errorf("TotalRequests: want %d, got %d", wantStats.TotalRequests, got.TotalRequests)
	}
	if got.AvgLatencyMs != wantStats.AvgLatencyMs {
		t.Errorf("AvgLatencyMs: want %f, got %f", wantStats.AvgLatencyMs, got.AvgLatencyMs)
	}
	if len(got.ByModel) != 1 || got.ByModel[0].Model != "llama3" {
		t.Errorf("ByModel mismatch: %+v", got.ByModel)
	}
}

func TestGetStats_InvalidSince(t *testing.T) {
	h := NewHandler(&mockStore{})
	req := httptest.NewRequest(http.MethodGet, "/stats?since=notaduration", nil)
	rec := serve(h, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid since, got %d", rec.Code)
	}
}

func TestGetStats_DayShorthand(t *testing.T) {
	var capturedSince time.Time
	store := &mockStore{
		getDashboardStats: func(_ context.Context, since time.Time) (*models.DashboardStats, error) {
			capturedSince = since
			return &models.DashboardStats{}, nil
		},
	}
	h := NewHandler(store)

	before := time.Now().Add(-7 * 24 * time.Hour)
	req := httptest.NewRequest(http.MethodGet, "/stats?since=7d", nil)
	rec := serve(h, req)
	after := time.Now().Add(-7 * 24 * time.Hour)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	// capturedSince should be approximately 7 days ago
	if capturedSince.Before(before.Add(-time.Second)) || capturedSince.After(after.Add(time.Second)) {
		t.Errorf("since=%v is outside expected 7d window [%v, %v]", capturedSince, before, after)
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
