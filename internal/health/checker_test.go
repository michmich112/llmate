package health

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/llmate/gateway/internal/models"
)

// mockStore is a minimal db.Store implementation for testing.
// Only ListProviders and UpdateProviderHealth record state; all other methods are no-ops.
type mockStore struct {
	providers []models.Provider

	mu        sync.Mutex
	updates   map[string]bool // providerID -> last health value
	updateCnt map[string]int  // providerID -> update call count
}

func newMockStore(providers []models.Provider) *mockStore {
	return &mockStore{
		providers: providers,
		updates:   make(map[string]bool),
		updateCnt: make(map[string]int),
	}
}

func (m *mockStore) ListProviders(_ context.Context) ([]models.Provider, error) {
	return m.providers, nil
}

func (m *mockStore) UpdateProviderHealth(_ context.Context, id string, healthy bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.updates[id] = healthy
	m.updateCnt[id]++
	return nil
}

// No-op implementations of remaining Store interface methods.

func (m *mockStore) CreateProvider(_ context.Context, _ *models.Provider) error { return nil }
func (m *mockStore) GetProvider(_ context.Context, _ string) (*models.Provider, error) {
	return nil, nil
}
func (m *mockStore) UpdateProvider(_ context.Context, _ *models.Provider) error { return nil }
func (m *mockStore) DeleteProvider(_ context.Context, _ string) error           { return nil }
func (m *mockStore) UpsertProviderEndpoints(_ context.Context, _ string, _ []models.ProviderEndpoint) error {
	return nil
}
func (m *mockStore) ListProviderEndpoints(_ context.Context, _ string) ([]models.ProviderEndpoint, error) {
	return nil, nil
}
func (m *mockStore) UpdateProviderEndpoint(_ context.Context, _ *models.ProviderEndpoint) error {
	return nil
}
func (m *mockStore) SyncProviderModels(_ context.Context, _ string, _ []string) error { return nil }
func (m *mockStore) ListProviderModels(_ context.Context, _ string) ([]models.ProviderModel, error) {
	return nil, nil
}
func (m *mockStore) ListAllModels(_ context.Context) ([]models.ProviderModel, error) { return nil, nil }
func (m *mockStore) CreateAlias(_ context.Context, _ *models.ModelAlias) error       { return nil }
func (m *mockStore) ListAliases(_ context.Context) ([]models.ModelAlias, error)      { return nil, nil }
func (m *mockStore) UpdateAlias(_ context.Context, _ *models.ModelAlias) error       { return nil }
func (m *mockStore) DeleteAlias(_ context.Context, _ string) error                   { return nil }
func (m *mockStore) ResolveAlias(_ context.Context, _ string) ([]models.ModelAlias, error) {
	return nil, nil
}
func (m *mockStore) GetHealthyProvidersForModel(_ context.Context, _ string) ([]models.Provider, error) {
	return nil, nil
}
func (m *mockStore) GetEnabledEndpoint(_ context.Context, _, _ string) (*models.ProviderEndpoint, error) {
	return nil, nil
}
func (m *mockStore) InsertRequestLog(_ context.Context, _ *models.RequestLog) error { return nil }
func (m *mockStore) QueryRequestLogs(_ context.Context, _ models.LogFilter) ([]models.RequestLog, int, error) {
	return nil, 0, nil
}
func (m *mockStore) GetRequestLog(_ context.Context, _ string) (*models.RequestLog, error) {
	return nil, nil
}
func (m *mockStore) UpdateProviderModelCosts(_ context.Context, _ string, _ *models.ProviderModel) error {
	return nil
}
func (m *mockStore) GetProviderModelCosts(_ context.Context, _, _ string) (*models.ProviderModel, error) {
	return nil, nil
}
func (m *mockStore) GetDashboardStats(_ context.Context, _ time.Time) (*models.DashboardStats, error) {
	return nil, nil
}
func (m *mockStore) GetTimeSeries(_ context.Context, _, _ time.Time, _ string) ([]models.TimeSeriesPoint, error) {
	return nil, nil
}
func (m *mockStore) GetAllConfig(_ context.Context) (map[string]string, error) { return map[string]string{}, nil }
func (m *mockStore) SetConfig(_ context.Context, _, _ string) error              { return nil }
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

// mockBreaker records ReportSuccess and ReportFailure calls per provider ID.
type mockBreaker struct {
	mu       sync.Mutex
	successes map[string]int
	failures  map[string]int
}

func newMockBreaker() *mockBreaker {
	return &mockBreaker{
		successes: make(map[string]int),
		failures:  make(map[string]int),
	}
}

func (b *mockBreaker) ReportSuccess(providerID string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.successes[providerID]++
}

func (b *mockBreaker) ReportFailure(providerID string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.failures[providerID]++
}

func discardLogger() *slog.Logger {
	return slog.New(slog.DiscardHandler)
}

// TestCheckAll_SingleProvider tests healthy (200) and unhealthy (500) server responses.
func TestCheckAll_SingleProvider(t *testing.T) {
	tests := []struct {
		name          string
		serverStatus  int
		wantHealthy   bool
		wantSuccesses int
		wantFailures  int
	}{
		{
			name:          "healthy_200",
			serverStatus:  http.StatusOK,
			wantHealthy:   true,
			wantSuccesses: 1,
			wantFailures:  0,
		},
		{
			name:          "unhealthy_500",
			serverStatus:  http.StatusInternalServerError,
			wantHealthy:   false,
			wantSuccesses: 0,
			wantFailures:  1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tc.serverStatus)
			}))
			defer srv.Close()

			const providerID = "p1"
			provider := models.Provider{
				ID:      providerID,
				Name:    "test-provider",
				BaseURL: srv.URL,
			}

			store := newMockStore([]models.Provider{provider})
			breaker := newMockBreaker()
			checker := NewChecker(store, breaker, &http.Client{}, time.Minute, discardLogger())

			checker.checkAll(context.Background())

			store.mu.Lock()
			gotHealthy := store.updates[providerID]
			updateCnt := store.updateCnt[providerID]
			store.mu.Unlock()

			if updateCnt != 1 {
				t.Errorf("UpdateProviderHealth call count: got %d, want 1", updateCnt)
			}
			if gotHealthy != tc.wantHealthy {
				t.Errorf("UpdateProviderHealth healthy=%v, want %v", gotHealthy, tc.wantHealthy)
			}

			breaker.mu.Lock()
			gotSuccesses := breaker.successes[providerID]
			gotFailures := breaker.failures[providerID]
			breaker.mu.Unlock()

			if gotSuccesses != tc.wantSuccesses {
				t.Errorf("ReportSuccess count: got %d, want %d", gotSuccesses, tc.wantSuccesses)
			}
			if gotFailures != tc.wantFailures {
				t.Errorf("ReportFailure count: got %d, want %d", gotFailures, tc.wantFailures)
			}
		})
	}
}

// TestCheckAll_Unreachable tests that a provider whose server is not reachable is marked unhealthy.
func TestCheckAll_Unreachable(t *testing.T) {
	// Start a server to get a valid address, then immediately close it so connections are refused.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {}))
	unreachableURL := srv.URL
	srv.Close()

	const providerID = "p-unreachable"
	provider := models.Provider{
		ID:      providerID,
		Name:    "unreachable",
		BaseURL: unreachableURL,
	}

	store := newMockStore([]models.Provider{provider})
	breaker := newMockBreaker()
	checker := NewChecker(store, breaker, &http.Client{}, time.Minute, discardLogger())

	checker.checkAll(context.Background())

	store.mu.Lock()
	updateCnt := store.updateCnt[providerID]
	gotHealthy := store.updates[providerID]
	store.mu.Unlock()

	if updateCnt != 1 {
		t.Errorf("UpdateProviderHealth call count: got %d, want 1", updateCnt)
	}
	if gotHealthy {
		t.Error("expected unreachable provider to be marked unhealthy, got healthy=true")
	}

	breaker.mu.Lock()
	failures := breaker.failures[providerID]
	successes := breaker.successes[providerID]
	breaker.mu.Unlock()

	if failures != 1 {
		t.Errorf("ReportFailure count: got %d, want 1", failures)
	}
	if successes != 0 {
		t.Errorf("ReportSuccess count: got %d, want 0", successes)
	}
}

// TestCheckAll_ConcurrentProviders tests that checkAll checks multiple providers concurrently
// and records correct health state for each.
func TestCheckAll_ConcurrentProviders(t *testing.T) {
	srv1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv1.Close()

	srv2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv2.Close()

	providers := []models.Provider{
		{ID: "p1", Name: "healthy", BaseURL: srv1.URL},
		{ID: "p2", Name: "unhealthy", BaseURL: srv2.URL},
	}

	store := newMockStore(providers)
	breaker := newMockBreaker()
	checker := NewChecker(store, breaker, &http.Client{}, time.Minute, discardLogger())

	checker.checkAll(context.Background())

	// checkAll uses a WaitGroup, so all goroutines have finished by the time it returns.
	store.mu.Lock()
	p1Healthy := store.updates["p1"]
	p1Cnt := store.updateCnt["p1"]
	p2Healthy := store.updates["p2"]
	p2Cnt := store.updateCnt["p2"]
	store.mu.Unlock()

	if p1Cnt != 1 || !p1Healthy {
		t.Errorf("p1: updateCnt=%d healthy=%v; want updateCnt=1 healthy=true", p1Cnt, p1Healthy)
	}
	if p2Cnt != 1 || p2Healthy {
		t.Errorf("p2: updateCnt=%d healthy=%v; want updateCnt=1 healthy=false", p2Cnt, p2Healthy)
	}

	breaker.mu.Lock()
	p1Successes := breaker.successes["p1"]
	p2Failures := breaker.failures["p2"]
	p1Failures := breaker.failures["p1"]
	p2Successes := breaker.successes["p2"]
	breaker.mu.Unlock()

	if p1Successes != 1 {
		t.Errorf("p1 ReportSuccess count: got %d, want 1", p1Successes)
	}
	if p2Failures != 1 {
		t.Errorf("p2 ReportFailure count: got %d, want 1", p2Failures)
	}
	if p1Failures != 0 {
		t.Errorf("p1 ReportFailure count: got %d, want 0", p1Failures)
	}
	if p2Successes != 0 {
		t.Errorf("p2 ReportSuccess count: got %d, want 0", p2Successes)
	}
}

// TestCheckAll_ContextCancellation verifies that checkAll returns without deadlock
// when the context is already cancelled before it is called.
func TestCheckAll_ContextCancellation(t *testing.T) {
	store := newMockStore(nil) // no providers to avoid HTTP requests with cancelled ctx
	breaker := newMockBreaker()
	checker := NewChecker(store, breaker, &http.Client{}, time.Minute, discardLogger())

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancelled before checkAll is called

	done := make(chan struct{})
	go func() {
		checker.checkAll(ctx)
		close(done)
	}()

	select {
	case <-done:
		// returned without deadlock
	case <-time.After(5 * time.Second):
		t.Fatal("checkAll hung with a cancelled context")
	}
}

// TestCheckProvider_APIKey verifies that the Authorization: Bearer header is sent
// when the provider has a non-empty APIKey.
func TestCheckProvider_APIKey(t *testing.T) {
	const apiKey = "secret-token-abc123"

	var receivedAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	const providerID = "p-keyed"
	provider := models.Provider{
		ID:      providerID,
		Name:    "keyed-provider",
		BaseURL: srv.URL,
		APIKey:  apiKey,
	}

	store := newMockStore([]models.Provider{provider})
	breaker := newMockBreaker()
	checker := NewChecker(store, breaker, &http.Client{}, time.Minute, discardLogger())

	checker.checkAll(context.Background())

	expected := "Bearer " + apiKey
	if receivedAuth != expected {
		t.Errorf("Authorization header: got %q, want %q", receivedAuth, expected)
	}

	store.mu.Lock()
	gotHealthy := store.updates[providerID]
	updateCnt := store.updateCnt[providerID]
	store.mu.Unlock()

	if updateCnt != 1 {
		t.Errorf("UpdateProviderHealth call count: got %d, want 1", updateCnt)
	}
	if !gotHealthy {
		t.Error("expected provider with valid API key to be marked healthy")
	}
}
