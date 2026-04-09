package proxy

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/llmate/gateway/internal/models"
)

// ---------------------------------------------------------------------------
// routerTestStore — configurable mock Store for router tests
// ---------------------------------------------------------------------------

// routerTestStore is a db.Store mock with injectable behaviour for the
// methods the SmartRouter exercises. All other methods are no-ops.
type routerTestStore struct {
	resolveAliasFn              func(ctx context.Context, alias string) ([]models.ModelAlias, error)
	getProviderFn               func(ctx context.Context, id string) (*models.Provider, error)
	getHealthyProvidersFn       func(ctx context.Context, modelID string) ([]models.Provider, error)
	getEnabledEndpointFn        func(ctx context.Context, providerID string, path string) (*models.ProviderEndpoint, error)
}

func (s *routerTestStore) ResolveAlias(ctx context.Context, alias string) ([]models.ModelAlias, error) {
	if s.resolveAliasFn != nil {
		return s.resolveAliasFn(ctx, alias)
	}
	return nil, nil
}
func (s *routerTestStore) GetProvider(ctx context.Context, id string) (*models.Provider, error) {
	if s.getProviderFn != nil {
		return s.getProviderFn(ctx, id)
	}
	return nil, nil
}
func (s *routerTestStore) GetHealthyProvidersForModel(ctx context.Context, modelID string) ([]models.Provider, error) {
	if s.getHealthyProvidersFn != nil {
		return s.getHealthyProvidersFn(ctx, modelID)
	}
	return nil, nil
}
func (s *routerTestStore) GetEnabledEndpoint(ctx context.Context, providerID, path string) (*models.ProviderEndpoint, error) {
	if s.getEnabledEndpointFn != nil {
		return s.getEnabledEndpointFn(ctx, providerID, path)
	}
	return nil, nil
}

// Satisfy remaining db.Store interface with no-ops.
func (s *routerTestStore) CreateProvider(_ context.Context, _ *models.Provider) error  { return nil }
func (s *routerTestStore) ListProviders(_ context.Context) ([]models.Provider, error)  { return nil, nil }
func (s *routerTestStore) UpdateProvider(_ context.Context, _ *models.Provider) error  { return nil }
func (s *routerTestStore) DeleteProvider(_ context.Context, _ string) error            { return nil }
func (s *routerTestStore) UpsertProviderEndpoints(_ context.Context, _ string, _ []models.ProviderEndpoint) error {
	return nil
}
func (s *routerTestStore) ListProviderEndpoints(_ context.Context, _ string) ([]models.ProviderEndpoint, error) {
	return nil, nil
}
func (s *routerTestStore) UpdateProviderEndpoint(_ context.Context, _ *models.ProviderEndpoint) error {
	return nil
}
func (s *routerTestStore) SyncProviderModels(_ context.Context, _ string, _ []string) error {
	return nil
}
func (s *routerTestStore) ListProviderModels(_ context.Context, _ string) ([]models.ProviderModel, error) {
	return nil, nil
}
func (s *routerTestStore) ListAllModels(_ context.Context) ([]models.ProviderModel, error) {
	return nil, nil
}
func (s *routerTestStore) CreateAlias(_ context.Context, _ *models.ModelAlias) error { return nil }
func (s *routerTestStore) ListAliases(_ context.Context) ([]models.ModelAlias, error) {
	return nil, nil
}
func (s *routerTestStore) UpdateAlias(_ context.Context, _ *models.ModelAlias) error { return nil }
func (s *routerTestStore) DeleteAlias(_ context.Context, _ string) error             { return nil }
func (s *routerTestStore) InsertRequestLog(_ context.Context, _ *models.RequestLog) error {
	return nil
}
func (s *routerTestStore) QueryRequestLogs(_ context.Context, _ models.LogFilter) ([]models.RequestLog, int, error) {
	return nil, 0, nil
}
func (s *routerTestStore) GetRequestLog(_ context.Context, _ string) (*models.RequestLog, error) {
	return nil, nil
}
func (s *routerTestStore) UpdateProviderModelCosts(_ context.Context, _ string, _ *models.ProviderModel) error {
	return nil
}
func (s *routerTestStore) GetProviderModelCosts(_ context.Context, _, _ string) (*models.ProviderModel, error) {
	return nil, nil
}
func (s *routerTestStore) GetDashboardStats(_ context.Context, _ time.Time) (*models.DashboardStats, error) {
	return nil, nil
}
func (s *routerTestStore) GetTimeSeries(_ context.Context, _, _ time.Time, _ string) ([]models.TimeSeriesPoint, error) {
	return nil, nil
}
func (s *routerTestStore) GetAllConfig(_ context.Context) (map[string]string, error) { return map[string]string{}, nil }
func (s *routerTestStore) SetConfig(_ context.Context, _, _ string) error              { return nil }
func (s *routerTestStore) InsertStreamingLog(_ context.Context, _ *models.StreamingLog) error {
	return nil
}
func (s *routerTestStore) GetStreamingLogs(_ context.Context, _ string) ([]models.StreamingLog, error) {
	return nil, nil
}
func (s *routerTestStore) PurgeStreamingLogBodiesOlderThan(_ context.Context, _ time.Time) (int64, error) {
	return 0, nil
}
func (s *routerTestStore) PurgeRequestLogRequestBodiesOlderThan(_ context.Context, _ time.Time) (int64, error) {
	return 0, nil
}
func (s *routerTestStore) PurgeRequestLogResponseBodiesOlderThan(_ context.Context, _ time.Time) (int64, error) {
	return 0, nil
}
func (s *routerTestStore) UpdateProviderHealth(_ context.Context, _ string, _ bool) error {
	return nil
}
func (s *routerTestStore) Close() error { return nil }

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

var endpointOK = &models.ProviderEndpoint{IsSupported: true, IsEnabled: true}

func alwaysEndpoint(_ context.Context, _, _ string) (*models.ProviderEndpoint, error) {
	return endpointOK, nil
}

func noEndpoint(_ context.Context, _, _ string) (*models.ProviderEndpoint, error) {
	return nil, nil
}

func makeProvider(id, name, baseURL string) models.Provider {
	return models.Provider{ID: id, Name: name, BaseURL: baseURL, IsHealthy: true}
}

// ---------------------------------------------------------------------------
// Test 1: Alias resolution
// ---------------------------------------------------------------------------

func TestRouter_AliasResolution(t *testing.T) {
	provA := makeProvider("prov-a", "Provider A", "https://a.example.com")
	store := &routerTestStore{
		resolveAliasFn: func(_ context.Context, alias string) ([]models.ModelAlias, error) {
			if alias == "fast" {
				return []models.ModelAlias{
					{ProviderID: "prov-a", ModelID: "llama-3", Weight: 1, Priority: 0, IsEnabled: true},
				}, nil
			}
			return nil, nil
		},
		getProviderFn: func(_ context.Context, id string) (*models.Provider, error) {
			if id == "prov-a" {
				return &provA, nil
			}
			return nil, nil
		},
		getEnabledEndpointFn: alwaysEndpoint,
	}

	router := NewSmartRouter(store)
	result, err := router.Route(context.Background(), "fast", "/v1/chat/completions")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Provider.ID != "prov-a" {
		t.Errorf("expected provider prov-a, got %s", result.Provider.ID)
	}
	if result.ModelID != "llama-3" {
		t.Errorf("expected ModelID llama-3, got %s", result.ModelID)
	}
	if result.TargetURL != "https://a.example.com/v1/chat/completions" {
		t.Errorf("unexpected TargetURL: %s", result.TargetURL)
	}
	if !result.RequestedViaAlias {
		t.Error("expected RequestedViaAlias true when routing by alias")
	}
}

// ---------------------------------------------------------------------------
// Test 2: Direct model (no alias)
// ---------------------------------------------------------------------------

func TestRouter_DirectModel(t *testing.T) {
	provA := makeProvider("prov-a", "Provider A", "https://a.example.com")
	store := &routerTestStore{
		resolveAliasFn: func(_ context.Context, _ string) ([]models.ModelAlias, error) {
			return nil, nil // no aliases
		},
		getHealthyProvidersFn: func(_ context.Context, modelID string) ([]models.Provider, error) {
			if modelID == "gpt-4o" {
				return []models.Provider{provA}, nil
			}
			return nil, nil
		},
		getEnabledEndpointFn: alwaysEndpoint,
	}

	router := NewSmartRouter(store)
	result, err := router.Route(context.Background(), "gpt-4o", "/v1/chat/completions")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Provider.ID != "prov-a" {
		t.Errorf("expected prov-a, got %s", result.Provider.ID)
	}
	if result.ModelID != "gpt-4o" {
		t.Errorf("expected ModelID gpt-4o, got %s", result.ModelID)
	}
	if result.RequestedViaAlias {
		t.Error("expected RequestedViaAlias false for direct model id")
	}
}

// ---------------------------------------------------------------------------
// Test 3: Priority selection
// ---------------------------------------------------------------------------

func TestRouter_PrioritySelection(t *testing.T) {
	provLow := makeProvider("prov-low", "Low Priority", "https://low.example.com")
	provHigh := makeProvider("prov-high", "High Priority", "https://high.example.com")

	store := &routerTestStore{
		resolveAliasFn: func(_ context.Context, alias string) ([]models.ModelAlias, error) {
			return []models.ModelAlias{
				{ProviderID: "prov-low", ModelID: "model-x", Weight: 100, Priority: 1, IsEnabled: true},
				{ProviderID: "prov-high", ModelID: "model-x", Weight: 1, Priority: 10, IsEnabled: true},
			}, nil
		},
		getProviderFn: func(_ context.Context, id string) (*models.Provider, error) {
			switch id {
			case "prov-low":
				return &provLow, nil
			case "prov-high":
				return &provHigh, nil
			}
			return nil, nil
		},
		getEnabledEndpointFn: alwaysEndpoint,
	}

	router := NewSmartRouter(store)
	// Run many times — high-priority provider must always win regardless of weight
	for i := 0; i < 50; i++ {
		result, err := router.Route(context.Background(), "anything", "/v1/chat/completions")
		if err != nil {
			t.Fatalf("iteration %d: unexpected error: %v", i, err)
		}
		if result.Provider.ID != "prov-high" {
			t.Fatalf("iteration %d: expected prov-high (priority 10), got %s", i, result.Provider.ID)
		}
		if i == 0 && !result.RequestedViaAlias {
			t.Error("expected RequestedViaAlias true for alias-based route")
		}
	}
}

// ---------------------------------------------------------------------------
// Test 4: Weighted selection
// ---------------------------------------------------------------------------

func TestRouter_WeightedSelection(t *testing.T) {
	provA := makeProvider("prov-a", "Provider A", "https://a.example.com")
	provB := makeProvider("prov-b", "Provider B", "https://b.example.com")

	// Weight 3:1 in favour of prov-a
	store := &routerTestStore{
		resolveAliasFn: func(_ context.Context, _ string) ([]models.ModelAlias, error) {
			return []models.ModelAlias{
				{ProviderID: "prov-a", ModelID: "m", Weight: 3, Priority: 0, IsEnabled: true},
				{ProviderID: "prov-b", ModelID: "m", Weight: 1, Priority: 0, IsEnabled: true},
			}, nil
		},
		getProviderFn: func(_ context.Context, id string) (*models.Provider, error) {
			switch id {
			case "prov-a":
				return &provA, nil
			case "prov-b":
				return &provB, nil
			}
			return nil, nil
		},
		getEnabledEndpointFn: alwaysEndpoint,
	}

	router := NewSmartRouter(store)
	counts := map[string]int{}
	const iterations = 1000
	for i := 0; i < iterations; i++ {
		result, err := router.Route(context.Background(), "model", "/v1/chat/completions")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if i == 0 && !result.RequestedViaAlias {
			t.Error("expected RequestedViaAlias true for alias-based route")
		}
		counts[result.Provider.ID]++
	}

	// Expect ~75% prov-a, ~25% prov-b. Allow generous tolerance (±15%).
	ratioA := float64(counts["prov-a"]) / float64(iterations)
	if ratioA < 0.60 || ratioA > 0.90 {
		t.Errorf("prov-a selected %.0f%% of the time, expected ~75%% (±15%%)", ratioA*100)
	}
}

// ---------------------------------------------------------------------------
// Test 5: Circuit breaker filtering
// ---------------------------------------------------------------------------

func TestRouter_CircuitBreakerFiltering(t *testing.T) {
	provA := makeProvider("prov-a", "Provider A", "https://a.example.com")
	provB := makeProvider("prov-b", "Provider B", "https://b.example.com")

	store := &routerTestStore{
		resolveAliasFn: func(_ context.Context, _ string) ([]models.ModelAlias, error) {
			return nil, nil
		},
		getHealthyProvidersFn: func(_ context.Context, _ string) ([]models.Provider, error) {
			return []models.Provider{provA, provB}, nil
		},
		getEnabledEndpointFn: alwaysEndpoint,
	}

	router := NewSmartRouter(store)

	// Trip prov-a's breaker by reporting many failures
	for i := 0; i < 20; i++ {
		router.ReportFailure("prov-a")
	}
	if router.getBreaker("prov-a").State() != StateOpen {
		t.Fatal("expected prov-a breaker to be Open")
	}

	// All routes must now go to prov-b
	for i := 0; i < 20; i++ {
		result, err := router.Route(context.Background(), "model", "/v1/chat/completions")
		if err != nil {
			t.Fatalf("iteration %d: unexpected error: %v", i, err)
		}
		if result.Provider.ID != "prov-b" {
			t.Fatalf("iteration %d: expected prov-b (prov-a open), got %s", i, result.Provider.ID)
		}
	}
}

// ---------------------------------------------------------------------------
// Test 6: Failover when endpoint is missing
// ---------------------------------------------------------------------------

func TestRouter_Failover_MissingEndpoint(t *testing.T) {
	provA := makeProvider("prov-a", "Provider A", "https://a.example.com")
	provB := makeProvider("prov-b", "Provider B", "https://b.example.com")

	store := &routerTestStore{
		resolveAliasFn: func(_ context.Context, _ string) ([]models.ModelAlias, error) {
			return nil, nil
		},
		getHealthyProvidersFn: func(_ context.Context, _ string) ([]models.Provider, error) {
			return []models.Provider{provA, provB}, nil
		},
		getEnabledEndpointFn: func(_ context.Context, providerID, _ string) (*models.ProviderEndpoint, error) {
			if providerID == "prov-a" {
				return nil, nil // no endpoint for prov-a
			}
			return endpointOK, nil
		},
	}

	router := NewSmartRouter(store)
	for i := 0; i < 10; i++ {
		result, err := router.Route(context.Background(), "model", "/v1/chat/completions")
		if err != nil {
			t.Fatalf("iteration %d: unexpected error: %v", i, err)
		}
		if result.Provider.ID != "prov-b" {
			t.Fatalf("iteration %d: expected prov-b (prov-a has no endpoint), got %s", i, result.Provider.ID)
		}
	}
}

// ---------------------------------------------------------------------------
// Test 7: No providers → ErrNoAvailableProvider
// ---------------------------------------------------------------------------

func TestRouter_NoProviders(t *testing.T) {
	store := &routerTestStore{
		resolveAliasFn: func(_ context.Context, _ string) ([]models.ModelAlias, error) {
			return nil, nil
		},
		getHealthyProvidersFn: func(_ context.Context, _ string) ([]models.Provider, error) {
			return nil, nil
		},
	}

	router := NewSmartRouter(store)
	_, err := router.Route(context.Background(), "model", "/v1/chat/completions")
	if !errors.Is(err, ErrNoAvailableProvider) {
		t.Fatalf("expected ErrNoAvailableProvider, got %v", err)
	}
}

// TestRouter_AllProvidersFiltered verifies error when all candidates are filtered out.
func TestRouter_AllProvidersFiltered(t *testing.T) {
	provA := makeProvider("prov-a", "Provider A", "https://a.example.com")
	store := &routerTestStore{
		resolveAliasFn: func(_ context.Context, _ string) ([]models.ModelAlias, error) {
			return nil, nil
		},
		getHealthyProvidersFn: func(_ context.Context, _ string) ([]models.Provider, error) {
			return []models.Provider{provA}, nil
		},
		getEnabledEndpointFn: noEndpoint, // no endpoints supported
	}

	router := NewSmartRouter(store)
	_, err := router.Route(context.Background(), "model", "/v1/chat/completions")
	if !errors.Is(err, ErrNoAvailableProvider) {
		t.Fatalf("expected ErrNoAvailableProvider, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// Test 8: Endpoint filtering
// ---------------------------------------------------------------------------

func TestRouter_EndpointFiltering(t *testing.T) {
	provA := makeProvider("prov-a", "Provider A", "https://a.example.com")
	provB := makeProvider("prov-b", "Provider B", "https://b.example.com")

	store := &routerTestStore{
		resolveAliasFn: func(_ context.Context, _ string) ([]models.ModelAlias, error) {
			return nil, nil
		},
		getHealthyProvidersFn: func(_ context.Context, _ string) ([]models.Provider, error) {
			return []models.Provider{provA, provB}, nil
		},
		getEnabledEndpointFn: func(_ context.Context, providerID, path string) (*models.ProviderEndpoint, error) {
			// Only prov-b supports /v1/embeddings
			if providerID == "prov-b" && path == "/v1/embeddings" {
				return endpointOK, nil
			}
			return nil, nil
		},
	}

	router := NewSmartRouter(store)
	result, err := router.Route(context.Background(), "embed-model", "/v1/embeddings")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Provider.ID != "prov-b" {
		t.Errorf("expected prov-b for /v1/embeddings, got %s", result.Provider.ID)
	}
}

// ---------------------------------------------------------------------------
// Test 9: TargetURL construction (no double slashes)
// ---------------------------------------------------------------------------

func TestRouter_TargetURLConstruction(t *testing.T) {
	tests := []struct {
		baseURL      string
		endpointPath string
		wantURL      string
	}{
		{"https://api.example.com", "/v1/chat/completions", "https://api.example.com/v1/chat/completions"},
		{"https://api.example.com/", "/v1/chat/completions", "https://api.example.com/v1/chat/completions"},
		{"https://api.example.com", "v1/chat/completions", "https://api.example.com/v1/chat/completions"},
		{"https://api.example.com/", "v1/chat/completions", "https://api.example.com/v1/chat/completions"},
	}

	for _, tc := range tests {
		prov := makeProvider("prov-x", "X", tc.baseURL)
		store := &routerTestStore{
			resolveAliasFn: func(_ context.Context, _ string) ([]models.ModelAlias, error) {
				return nil, nil
			},
			getHealthyProvidersFn: func(_ context.Context, _ string) ([]models.Provider, error) {
				return []models.Provider{prov}, nil
			},
			getEnabledEndpointFn: alwaysEndpoint,
		}
		router := NewSmartRouter(store)
		result, err := router.Route(context.Background(), "m", tc.endpointPath)
		if err != nil {
			t.Errorf("base=%q path=%q: unexpected error: %v", tc.baseURL, tc.endpointPath, err)
			continue
		}
		if result.TargetURL != tc.wantURL {
			t.Errorf("base=%q path=%q: got %q, want %q", tc.baseURL, tc.endpointPath, result.TargetURL, tc.wantURL)
		}
	}
}

// ---------------------------------------------------------------------------
// Test 10: ReportSuccess / ReportFailure update circuit breaker
// ---------------------------------------------------------------------------

func TestRouter_ReportSuccessFailureUpdateBreaker(t *testing.T) {
	router := NewSmartRouter(&routerTestStore{})

	// Repeated failures should eventually trip the breaker
	for i := 0; i < 20; i++ {
		router.ReportFailure("provider-x")
	}
	if router.getBreaker("provider-x").State() != StateOpen {
		t.Fatal("expected breaker to be Open after many failures")
	}

	// Wait for cooldown so we can get to HalfOpen, then RecordSuccess → Closed
	router.getBreaker("provider-x").mu.Lock()
	router.getBreaker("provider-x").lastStateChange = time.Now().Add(-31 * time.Second)
	router.getBreaker("provider-x").mu.Unlock()

	router.getBreaker("provider-x").Allow() // transitions to HalfOpen
	router.ReportSuccess("provider-x")
	if router.getBreaker("provider-x").State() != StateClosed {
		t.Fatal("expected breaker Closed after success in HalfOpen")
	}
}
