package db

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/llmate/gateway/internal/models"
)

func newTestStore(t *testing.T) *SQLiteStore {
	t.Helper()
	store, err := NewSQLiteStore(":memory:")
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	return store
}

func newProvider(id, name, baseURL string, healthy bool, now time.Time) *models.Provider {
	return &models.Provider{
		ID:        id,
		Name:      name,
		BaseURL:   baseURL,
		IsHealthy: healthy,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func TestProviderCRUD(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)

	p := &models.Provider{
		ID:        uuid.NewString(),
		Name:      "OpenAI",
		BaseURL:   "https://api.openai.com",
		APIKey:    "sk-test",
		IsHealthy: false,
		CreatedAt: now,
		UpdatedAt: now,
	}

	// Create
	if err := store.CreateProvider(ctx, p); err != nil {
		t.Fatalf("CreateProvider: %v", err)
	}

	// Get
	got, err := store.GetProvider(ctx, p.ID)
	if err != nil {
		t.Fatalf("GetProvider: %v", err)
	}
	if got.Name != p.Name {
		t.Errorf("Name: got %q, want %q", got.Name, p.Name)
	}
	if got.BaseURL != p.BaseURL {
		t.Errorf("BaseURL: got %q, want %q", got.BaseURL, p.BaseURL)
	}
	if got.APIKey != p.APIKey {
		t.Errorf("APIKey: got %q, want %q", got.APIKey, p.APIKey)
	}

	// List
	list, err := store.ListProviders(ctx)
	if err != nil {
		t.Fatalf("ListProviders: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("ListProviders: got %d providers, want 1", len(list))
	}

	// Update
	p.Name = "OpenAI Updated"
	p.UpdatedAt = now.Add(time.Second)
	if err := store.UpdateProvider(ctx, p); err != nil {
		t.Fatalf("UpdateProvider: %v", err)
	}
	got2, err := store.GetProvider(ctx, p.ID)
	if err != nil {
		t.Fatalf("GetProvider after update: %v", err)
	}
	if got2.Name != "OpenAI Updated" {
		t.Errorf("UpdateProvider: got name %q, want %q", got2.Name, "OpenAI Updated")
	}

	// Delete
	if err := store.DeleteProvider(ctx, p.ID); err != nil {
		t.Fatalf("DeleteProvider: %v", err)
	}

	// Get after delete should return wrapped sql.ErrNoRows
	_, err = store.GetProvider(ctx, p.ID)
	if err == nil {
		t.Fatal("GetProvider after delete: expected error, got nil")
	}
	if !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("GetProvider after delete: expected sql.ErrNoRows wrapped, got %v", err)
	}
}

func TestCascadeDelete(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	now := time.Now().UTC()

	p := newProvider(uuid.NewString(), "Cascade", "https://cascade.test", false, now)
	if err := store.CreateProvider(ctx, p); err != nil {
		t.Fatalf("CreateProvider: %v", err)
	}

	// Upsert endpoints
	eps := []models.ProviderEndpoint{
		{ID: uuid.NewString(), ProviderID: p.ID, Path: "/v1/chat/completions", Method: "POST", IsSupported: true, IsEnabled: true, CreatedAt: now},
	}
	if err := store.UpsertProviderEndpoints(ctx, p.ID, eps); err != nil {
		t.Fatalf("UpsertProviderEndpoints: %v", err)
	}

	// Sync models
	if err := store.SyncProviderModels(ctx, p.ID, []string{"gpt-4", "gpt-3.5"}); err != nil {
		t.Fatalf("SyncProviderModels: %v", err)
	}

	// Create alias
	alias := &models.ModelAlias{
		ID: uuid.NewString(), Alias: "gpt", ProviderID: p.ID, ModelID: "gpt-4",
		Weight: 1, Priority: 0, IsEnabled: true, CreatedAt: now, UpdatedAt: now,
	}
	if err := store.CreateAlias(ctx, alias); err != nil {
		t.Fatalf("CreateAlias: %v", err)
	}

	// Delete provider (should cascade)
	if err := store.DeleteProvider(ctx, p.ID); err != nil {
		t.Fatalf("DeleteProvider: %v", err)
	}

	// Endpoints should be gone
	gotEps, err := store.ListProviderEndpoints(ctx, p.ID)
	if err != nil {
		t.Fatalf("ListProviderEndpoints after cascade: %v", err)
	}
	if len(gotEps) != 0 {
		t.Errorf("expected 0 endpoints after cascade delete, got %d", len(gotEps))
	}

	// Models should be gone
	gotModels, err := store.ListProviderModels(ctx, p.ID)
	if err != nil {
		t.Fatalf("ListProviderModels after cascade: %v", err)
	}
	if len(gotModels) != 0 {
		t.Errorf("expected 0 models after cascade delete, got %d", len(gotModels))
	}

	// Aliases should be gone
	aliases, err := store.ListAliases(ctx)
	if err != nil {
		t.Fatalf("ListAliases after cascade: %v", err)
	}
	for _, a := range aliases {
		if a.ProviderID == p.ID {
			t.Errorf("alias still present after cascade delete: %+v", a)
		}
	}
}

func TestUpsertProviderEndpoints(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	now := time.Now().UTC()

	p := newProvider(uuid.NewString(), "Endpoint", "https://endpoint.test", false, now)
	if err := store.CreateProvider(ctx, p); err != nil {
		t.Fatalf("CreateProvider: %v", err)
	}

	// Insert 2 endpoints
	eps1 := []models.ProviderEndpoint{
		{ID: uuid.NewString(), ProviderID: p.ID, Path: "/v1/chat/completions", Method: "POST", IsSupported: true, IsEnabled: true, CreatedAt: now},
		{ID: uuid.NewString(), ProviderID: p.ID, Path: "/v1/models", Method: "GET", IsSupported: true, IsEnabled: true, CreatedAt: now},
	}
	if err := store.UpsertProviderEndpoints(ctx, p.ID, eps1); err != nil {
		t.Fatalf("UpsertProviderEndpoints first: %v", err)
	}
	got1, err := store.ListProviderEndpoints(ctx, p.ID)
	if err != nil {
		t.Fatalf("ListProviderEndpoints: %v", err)
	}
	if len(got1) != 2 {
		t.Fatalf("expected 2 endpoints, got %d", len(got1))
	}

	// Upsert with 1 endpoint — replaces the previous set
	eps2 := []models.ProviderEndpoint{
		{ID: uuid.NewString(), ProviderID: p.ID, Path: "/v1/chat/completions", Method: "POST", IsSupported: true, IsEnabled: true, CreatedAt: now},
	}
	if err := store.UpsertProviderEndpoints(ctx, p.ID, eps2); err != nil {
		t.Fatalf("UpsertProviderEndpoints replace: %v", err)
	}
	got2, err := store.ListProviderEndpoints(ctx, p.ID)
	if err != nil {
		t.Fatalf("ListProviderEndpoints after replace: %v", err)
	}
	if len(got2) != 1 {
		t.Fatalf("expected 1 endpoint after upsert, got %d", len(got2))
	}
	if got2[0].Path != "/v1/chat/completions" {
		t.Errorf("wrong endpoint path: %s", got2[0].Path)
	}

	// Empty upsert clears all endpoints
	if err := store.UpsertProviderEndpoints(ctx, p.ID, nil); err != nil {
		t.Fatalf("UpsertProviderEndpoints empty: %v", err)
	}
	got3, err := store.ListProviderEndpoints(ctx, p.ID)
	if err != nil {
		t.Fatalf("ListProviderEndpoints after empty upsert: %v", err)
	}
	if len(got3) != 0 {
		t.Fatalf("expected 0 endpoints after empty upsert, got %d", len(got3))
	}
}

func TestUpdateProviderEndpoint(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	now := time.Now().UTC()

	p := newProvider(uuid.NewString(), "EP Update", "https://epupdate.test", false, now)
	if err := store.CreateProvider(ctx, p); err != nil {
		t.Fatalf("CreateProvider: %v", err)
	}

	ep := models.ProviderEndpoint{
		ID: uuid.NewString(), ProviderID: p.ID, Path: "/v1/chat/completions",
		Method: "POST", IsSupported: true, IsEnabled: true, CreatedAt: now,
	}
	if err := store.UpsertProviderEndpoints(ctx, p.ID, []models.ProviderEndpoint{ep}); err != nil {
		t.Fatalf("UpsertProviderEndpoints: %v", err)
	}

	ep.IsEnabled = false
	if err := store.UpdateProviderEndpoint(ctx, &ep); err != nil {
		t.Fatalf("UpdateProviderEndpoint: %v", err)
	}

	eps, err := store.ListProviderEndpoints(ctx, p.ID)
	if err != nil {
		t.Fatalf("ListProviderEndpoints: %v", err)
	}
	if len(eps) != 1 || eps[0].IsEnabled {
		t.Errorf("expected is_enabled=false, got %+v", eps)
	}
}

func TestSyncProviderModels(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	now := time.Now().UTC()

	p := newProvider(uuid.NewString(), "Model Sync", "https://modelsync.test", false, now)
	if err := store.CreateProvider(ctx, p); err != nil {
		t.Fatalf("CreateProvider: %v", err)
	}

	// Sync [a, b]
	if err := store.SyncProviderModels(ctx, p.ID, []string{"model-a", "model-b"}); err != nil {
		t.Fatalf("SyncProviderModels [a,b]: %v", err)
	}
	ms1, err := store.ListProviderModels(ctx, p.ID)
	if err != nil {
		t.Fatalf("ListProviderModels: %v", err)
	}
	if len(ms1) != 2 {
		t.Fatalf("expected 2 models, got %d", len(ms1))
	}
	ids1 := map[string]string{}
	for _, m := range ms1 {
		ids1[m.ModelID] = m.ID
	}

	// Sync [b, c] — a is removed, b keeps its existing record (preserving cost data), c is added
	if err := store.SyncProviderModels(ctx, p.ID, []string{"model-b", "model-c"}); err != nil {
		t.Fatalf("SyncProviderModels [b,c]: %v", err)
	}
	ms2, err := store.ListProviderModels(ctx, p.ID)
	if err != nil {
		t.Fatalf("ListProviderModels after sync: %v", err)
	}
	if len(ms2) != 2 {
		t.Fatalf("expected 2 models after sync, got %d", len(ms2))
	}

	modelSet := map[string]bool{}
	for _, m := range ms2 {
		modelSet[m.ModelID] = true
		// Existing model-b should retain its UUID so cost data is preserved.
		if m.ModelID == "model-b" {
			if m.ID != ids1["model-b"] {
				t.Errorf("model-b should retain its UUID across sync, got %s want %s", m.ID, ids1["model-b"])
			}
		}
	}
	if !modelSet["model-b"] || !modelSet["model-c"] {
		t.Errorf("expected [model-b, model-c], got %v", modelSet)
	}
	if modelSet["model-a"] {
		t.Error("model-a should have been removed by sync")
	}

	// ListAllModels should include models from all providers
	all, err := store.ListAllModels(ctx)
	if err != nil {
		t.Fatalf("ListAllModels: %v", err)
	}
	if len(all) != 2 {
		t.Errorf("expected 2 total models, got %d", len(all))
	}
}

func TestAliasesCRUD(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	now := time.Now().UTC()

	p := newProvider(uuid.NewString(), "Alias Provider", "https://aliasp.test", false, now)
	if err := store.CreateProvider(ctx, p); err != nil {
		t.Fatalf("CreateProvider: %v", err)
	}

	a1 := &models.ModelAlias{
		ID: uuid.NewString(), Alias: "gpt", ProviderID: p.ID, ModelID: "gpt-4",
		Weight: 1, Priority: 10, IsEnabled: true, CreatedAt: now, UpdatedAt: now,
	}
	a2 := &models.ModelAlias{
		ID: uuid.NewString(), Alias: "gpt", ProviderID: p.ID, ModelID: "gpt-3.5-turbo",
		Weight: 2, Priority: 5, IsEnabled: true, CreatedAt: now, UpdatedAt: now,
	}

	if err := store.CreateAlias(ctx, a1); err != nil {
		t.Fatalf("CreateAlias a1: %v", err)
	}
	if err := store.CreateAlias(ctx, a2); err != nil {
		t.Fatalf("CreateAlias a2: %v", err)
	}

	// List — ordered alias ASC, priority DESC
	list, err := store.ListAliases(ctx)
	if err != nil {
		t.Fatalf("ListAliases: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 aliases, got %d", len(list))
	}
	if list[0].Priority <= list[1].Priority {
		// list[0] should have the higher priority
		if list[0].Priority < list[1].Priority {
			t.Errorf("aliases not ordered by priority DESC: [0]=%d < [1]=%d", list[0].Priority, list[1].Priority)
		}
	}
	if list[0].Priority < list[1].Priority {
		t.Errorf("aliases not ordered by priority DESC: [0].Priority=%d < [1].Priority=%d", list[0].Priority, list[1].Priority)
	}

	// Update a1
	a1.Weight = 5
	a1.Priority = 20
	a1.UpdatedAt = now.Add(time.Second)
	if err := store.UpdateAlias(ctx, a1); err != nil {
		t.Fatalf("UpdateAlias: %v", err)
	}

	// Delete a2
	if err := store.DeleteAlias(ctx, a2.ID); err != nil {
		t.Fatalf("DeleteAlias: %v", err)
	}

	listAfter, err := store.ListAliases(ctx)
	if err != nil {
		t.Fatalf("ListAliases after delete: %v", err)
	}
	if len(listAfter) != 1 {
		t.Fatalf("expected 1 alias after delete, got %d", len(listAfter))
	}
	if listAfter[0].Priority != 20 {
		t.Errorf("expected updated priority 20, got %d", listAfter[0].Priority)
	}
}

func TestResolveAlias(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	now := time.Now().UTC()

	p := newProvider(uuid.NewString(), "Resolve Provider", "https://resolve.test", false, now)
	if err := store.CreateProvider(ctx, p); err != nil {
		t.Fatalf("CreateProvider: %v", err)
	}

	// Two enabled, one disabled
	a1 := &models.ModelAlias{ID: uuid.NewString(), Alias: "smart", ProviderID: p.ID, ModelID: "gpt-4", Weight: 1, Priority: 10, IsEnabled: true, CreatedAt: now, UpdatedAt: now}
	a2 := &models.ModelAlias{ID: uuid.NewString(), Alias: "smart", ProviderID: p.ID, ModelID: "gpt-3.5", Weight: 1, Priority: 5, IsEnabled: true, CreatedAt: now, UpdatedAt: now}
	a3 := &models.ModelAlias{ID: uuid.NewString(), Alias: "smart", ProviderID: p.ID, ModelID: "gpt-old", Weight: 1, Priority: 15, IsEnabled: false, CreatedAt: now, UpdatedAt: now}

	for _, a := range []*models.ModelAlias{a1, a2, a3} {
		if err := store.CreateAlias(ctx, a); err != nil {
			t.Fatalf("CreateAlias: %v", err)
		}
	}

	// Should return only enabled entries, ordered by priority DESC
	resolved, err := store.ResolveAlias(ctx, "smart")
	if err != nil {
		t.Fatalf("ResolveAlias: %v", err)
	}
	if len(resolved) != 2 {
		t.Fatalf("expected 2 resolved (enabled) aliases, got %d", len(resolved))
	}
	if resolved[0].Priority < resolved[1].Priority {
		t.Errorf("expected priority DESC: [0]=%d < [1]=%d", resolved[0].Priority, resolved[1].Priority)
	}
	if resolved[0].ModelID != "gpt-4" {
		t.Errorf("expected gpt-4 first (priority 10), got %s", resolved[0].ModelID)
	}

	// Non-existent alias returns empty slice, no error
	empty, err := store.ResolveAlias(ctx, "nonexistent")
	if err != nil {
		t.Fatalf("ResolveAlias nonexistent: %v", err)
	}
	if len(empty) != 0 {
		t.Errorf("expected 0 results for nonexistent alias, got %d", len(empty))
	}
}

func TestGetHealthyProvidersForModel(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	now := time.Now().UTC()

	pHealthy := newProvider(uuid.NewString(), "Healthy Provider", "https://healthy.test", true, now)
	pUnhealthy := newProvider(uuid.NewString(), "Unhealthy Provider", "https://unhealthy.test", false, now)

	for _, p := range []*models.Provider{pHealthy, pUnhealthy} {
		if err := store.CreateProvider(ctx, p); err != nil {
			t.Fatalf("CreateProvider: %v", err)
		}
		if err := store.SyncProviderModels(ctx, p.ID, []string{"gpt-4"}); err != nil {
			t.Fatalf("SyncProviderModels: %v", err)
		}
	}

	// Only the healthy provider should be returned
	providers, err := store.GetHealthyProvidersForModel(ctx, "gpt-4")
	if err != nil {
		t.Fatalf("GetHealthyProvidersForModel: %v", err)
	}
	if len(providers) != 1 {
		t.Fatalf("expected 1 healthy provider, got %d", len(providers))
	}
	if providers[0].ID != pHealthy.ID {
		t.Errorf("expected healthy provider, got %s", providers[0].Name)
	}

	// Model not in any provider
	none, err := store.GetHealthyProvidersForModel(ctx, "claude-3")
	if err != nil {
		t.Fatalf("GetHealthyProvidersForModel missing model: %v", err)
	}
	if len(none) != 0 {
		t.Errorf("expected 0 providers for missing model, got %d", len(none))
	}
}

func TestGetEnabledEndpoint(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	now := time.Now().UTC()

	p := newProvider(uuid.NewString(), "EP", "https://ep.test", false, now)
	if err := store.CreateProvider(ctx, p); err != nil {
		t.Fatalf("CreateProvider: %v", err)
	}

	eps := []models.ProviderEndpoint{
		{ID: uuid.NewString(), ProviderID: p.ID, Path: "/v1/chat/completions", Method: "POST", IsSupported: true, IsEnabled: true, CreatedAt: now},
		{ID: uuid.NewString(), ProviderID: p.ID, Path: "/v1/embeddings", Method: "POST", IsSupported: true, IsEnabled: false, CreatedAt: now},
		{ID: uuid.NewString(), ProviderID: p.ID, Path: "/v1/models", Method: "GET", IsSupported: false, IsEnabled: true, CreatedAt: now},
	}
	if err := store.UpsertProviderEndpoints(ctx, p.ID, eps); err != nil {
		t.Fatalf("UpsertProviderEndpoints: %v", err)
	}

	// Supported + enabled → returns endpoint
	ep, err := store.GetEnabledEndpoint(ctx, p.ID, "/v1/chat/completions")
	if err != nil {
		t.Fatalf("GetEnabledEndpoint: %v", err)
	}
	if ep == nil {
		t.Fatal("expected endpoint, got nil")
	}
	if ep.Path != "/v1/chat/completions" {
		t.Errorf("wrong path: %s", ep.Path)
	}

	// is_enabled = false → nil (no error)
	ep2, err := store.GetEnabledEndpoint(ctx, p.ID, "/v1/embeddings")
	if err != nil {
		t.Fatalf("GetEnabledEndpoint disabled: %v", err)
	}
	if ep2 != nil {
		t.Errorf("expected nil for disabled endpoint, got %+v", ep2)
	}

	// is_supported = false → nil (no error)
	ep3, err := store.GetEnabledEndpoint(ctx, p.ID, "/v1/models")
	if err != nil {
		t.Fatalf("GetEnabledEndpoint unsupported: %v", err)
	}
	if ep3 != nil {
		t.Errorf("expected nil for unsupported endpoint, got %+v", ep3)
	}
}

func TestInsertAndQueryRequestLogs(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)

	p := newProvider(uuid.NewString(), "Log Provider", "https://log.test", false, now)
	if err := store.CreateProvider(ctx, p); err != nil {
		t.Fatalf("CreateProvider: %v", err)
	}

	ttft := 150
	prompt := 100
	completion := 50
	total := 150

	log1 := &models.RequestLog{
		ID: uuid.NewString(), Timestamp: now.Add(-2 * time.Minute),
		ClientIP: "127.0.0.1", Method: "POST", Path: "/v1/chat/completions",
		RequestedModel: "gpt-4", ResolvedModel: "gpt-4",
		ProviderID: p.ID, ProviderName: "Log Provider",
		StatusCode: 200, IsStreamed: true,
		TTFTMs: &ttft, TotalTimeMs: 500,
		PromptTokens: &prompt, CompletionTokens: &completion,
		TotalTokens: &total, CreatedAt: now,
	}
	log2 := &models.RequestLog{
		ID: uuid.NewString(), Timestamp: now.Add(-1 * time.Minute),
		ClientIP: "10.0.0.1", Method: "POST", Path: "/v1/chat/completions",
		RequestedModel: "gpt-3.5", ProviderID: p.ID,
		StatusCode: 400, TotalTimeMs: 100, CreatedAt: now,
	}
	for _, l := range []*models.RequestLog{log1, log2} {
		if err := store.InsertRequestLog(ctx, l); err != nil {
			t.Fatalf("InsertRequestLog: %v", err)
		}
	}

	// Query all
	logs, count, err := store.QueryRequestLogs(ctx, models.LogFilter{Limit: 10})
	if err != nil {
		t.Fatalf("QueryRequestLogs all: %v", err)
	}
	if count != 2 {
		t.Errorf("expected total count 2, got %d", count)
	}
	if len(logs) != 2 {
		t.Errorf("expected 2 logs, got %d", len(logs))
	}

	// Filter by model
	byModel, countByModel, err := store.QueryRequestLogs(ctx, models.LogFilter{Model: "gpt-4", Limit: 10})
	if err != nil {
		t.Fatalf("QueryRequestLogs by model: %v", err)
	}
	if countByModel != 1 || len(byModel) != 1 {
		t.Errorf("expected 1 log for gpt-4, got count=%d len=%d", countByModel, len(byModel))
	}
	if byModel[0].RequestedModel != "gpt-4" {
		t.Errorf("wrong model: %s", byModel[0].RequestedModel)
	}

	// Filter by provider
	byProvider, countByProvider, err := store.QueryRequestLogs(ctx, models.LogFilter{ProviderID: p.ID, Limit: 10})
	if err != nil {
		t.Fatalf("QueryRequestLogs by provider: %v", err)
	}
	if countByProvider != 2 {
		t.Errorf("expected 2 logs for provider, got %d", countByProvider)
	}
	_ = byProvider

	// Filter by since (only log2 is within the last 90 seconds)
	since := now.Add(-90 * time.Second)
	bySince, countBySince, err := store.QueryRequestLogs(ctx, models.LogFilter{Since: &since, Limit: 10})
	if err != nil {
		t.Fatalf("QueryRequestLogs by since: %v", err)
	}
	if countBySince != 1 {
		t.Errorf("expected 1 log since -90s, got %d", countBySince)
	}
	_ = bySince

	// Filter by until (only log1 is older than 90 seconds ago)
	until := now.Add(-90 * time.Second)
	byUntil, countByUntil, err := store.QueryRequestLogs(ctx, models.LogFilter{Until: &until, Limit: 10})
	if err != nil {
		t.Fatalf("QueryRequestLogs by until: %v", err)
	}
	if countByUntil != 1 {
		t.Errorf("expected 1 log until -90s, got %d", countByUntil)
	}
	_ = byUntil

	// Verify nullable fields round-trip correctly for log1
	if byModel[0].TTFTMs == nil {
		t.Error("TTFTMs should not be nil")
	} else if *byModel[0].TTFTMs != ttft {
		t.Errorf("TTFTMs: got %d, want %d", *byModel[0].TTFTMs, ttft)
	}
	if byModel[0].PromptTokens == nil {
		t.Error("PromptTokens should not be nil")
	}
	// log2 should have nil TTFTMs
	if logs[0].TTFTMs != nil && logs[0].RequestedModel == "gpt-3.5" {
		t.Error("TTFTMs should be nil for log2")
	}
}

func TestDashboardStats(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)

	p := newProvider(uuid.NewString(), "Stats Provider", "https://stats.test", false, now)
	if err := store.CreateProvider(ctx, p); err != nil {
		t.Fatalf("CreateProvider: %v", err)
	}

	total := 100
	// Insert 5 logs: 3 success (200), 2 error (500)
	for i := 0; i < 5; i++ {
		sc := 200
		if i >= 3 {
			sc = 500
		}
		l := &models.RequestLog{
			ID:             uuid.NewString(),
			Timestamp:      now.Add(-time.Duration(i) * time.Minute),
			ClientIP:       "127.0.0.1",
			Method:         "POST",
			Path:           "/v1/chat/completions",
			RequestedModel: "gpt-4",
			ProviderID:     p.ID,
			ProviderName:   "Stats Provider",
			StatusCode:     sc,
			TotalTimeMs:    100 * (i + 1),
			TotalTokens:    &total,
			CreatedAt:      now,
		}
		if err := store.InsertRequestLog(ctx, l); err != nil {
			t.Fatalf("InsertRequestLog: %v", err)
		}
	}

	since := now.Add(-10 * time.Minute)
	stats, err := store.GetDashboardStats(ctx, since)
	if err != nil {
		t.Fatalf("GetDashboardStats: %v", err)
	}

	if stats.TotalRequests != 5 {
		t.Errorf("TotalRequests: got %d, want 5", stats.TotalRequests)
	}
	// 2 errors / 5 total = 0.4
	if stats.ErrorRate < 0.39 || stats.ErrorRate > 0.41 {
		t.Errorf("ErrorRate: got %f, want ~0.4", stats.ErrorRate)
	}
	if stats.AvgLatencyMs == 0 {
		t.Error("AvgLatencyMs should be non-zero")
	}
	if len(stats.ByModel) == 0 {
		t.Error("ByModel should be non-empty")
	}
	if len(stats.ByProvider) == 0 {
		t.Error("ByProvider should be non-empty")
	}
	// gpt-4 should appear in ByModel
	if stats.ByModel[0].Model != "gpt-4" {
		t.Errorf("ByModel[0].Model: got %q, want %q", stats.ByModel[0].Model, "gpt-4")
	}
	if stats.ByModel[0].RequestCount != 5 {
		t.Errorf("ByModel[0].RequestCount: got %d, want 5", stats.ByModel[0].RequestCount)
	}

	// Empty stats when since is in the future
	future := now.Add(time.Hour)
	empty, err := store.GetDashboardStats(ctx, future)
	if err != nil {
		t.Fatalf("GetDashboardStats empty: %v", err)
	}
	if empty.TotalRequests != 0 {
		t.Errorf("expected 0 total requests for future since, got %d", empty.TotalRequests)
	}
	if empty.ByModel == nil {
		t.Error("ByModel should not be nil (must be empty slice)")
	}
	if empty.ByProvider == nil {
		t.Error("ByProvider should not be nil (must be empty slice)")
	}
}

func TestNotFound(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	// GetProvider with unknown ID should return wrapped sql.ErrNoRows
	_, err := store.GetProvider(ctx, "does-not-exist")
	if err == nil {
		t.Fatal("expected error for missing provider, got nil")
	}
	if !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("expected sql.ErrNoRows, got %v", err)
	}
}

func TestUpdateProviderMissing(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	now := time.Now().UTC()

	err := store.UpdateProvider(ctx, &models.Provider{
		ID:        "missing-id",
		Name:      "ghost",
		BaseURL:   "https://ghost.test",
		UpdatedAt: now,
	})
	if err == nil {
		t.Fatal("expected error updating nonexistent provider, got nil")
	}
}

func TestUpdateProviderHealth(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	now := time.Now().UTC()

	p := newProvider(uuid.NewString(), "Health Test", "https://health.test", false, now)
	if err := store.CreateProvider(ctx, p); err != nil {
		t.Fatalf("CreateProvider: %v", err)
	}

	if err := store.UpdateProviderHealth(ctx, p.ID, true); err != nil {
		t.Fatalf("UpdateProviderHealth: %v", err)
	}

	got, err := store.GetProvider(ctx, p.ID)
	if err != nil {
		t.Fatalf("GetProvider after health update: %v", err)
	}
	if !got.IsHealthy {
		t.Error("expected is_healthy=true after UpdateProviderHealth")
	}
	if got.HealthCheckedAt == nil {
		t.Error("expected health_checked_at to be set")
	}

	// Update to unhealthy
	if err := store.UpdateProviderHealth(ctx, p.ID, false); err != nil {
		t.Fatalf("UpdateProviderHealth false: %v", err)
	}
	got2, err := store.GetProvider(ctx, p.ID)
	if err != nil {
		t.Fatalf("GetProvider after health update 2: %v", err)
	}
	if got2.IsHealthy {
		t.Error("expected is_healthy=false")
	}

	// Missing provider
	if err := store.UpdateProviderHealth(ctx, "missing", true); err == nil {
		t.Error("expected error for missing provider health update, got nil")
	}
}

func TestPurgeStreamingLogBodiesOlderThan(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Millisecond)

	p := newProvider(uuid.NewString(), "Stream Purge", "https://streampurge.test", false, now)
	if err := store.CreateProvider(ctx, p); err != nil {
		t.Fatalf("CreateProvider: %v", err)
	}

	rid := uuid.NewString()
	log := &models.RequestLog{
		ID: rid, Timestamp: now,
		ClientIP: "127.0.0.1", Method: "POST", Path: "/v1/chat/completions",
		RequestedModel: "gpt-4", ProviderID: p.ID,
		StatusCode: 200, IsStreamed: true, TotalTimeMs: 100, CreatedAt: now,
	}
	if err := store.InsertRequestLog(ctx, log); err != nil {
		t.Fatalf("InsertRequestLog: %v", err)
	}

	old := now.Add(-45 * 24 * time.Hour)
	if err := store.InsertStreamingLog(ctx, &models.StreamingLog{
		RequestLogID: rid, ChunkIndex: 0,
		Data: "raw-old", ContentDelta: "delta-old",
		CreatedAt: old, Timestamp: old,
	}); err != nil {
		t.Fatalf("InsertStreamingLog old: %v", err)
	}
	if err := store.InsertStreamingLog(ctx, &models.StreamingLog{
		RequestLogID: rid, ChunkIndex: 1,
		Data: "raw-new", ContentDelta: "delta-new",
		CreatedAt: now, Timestamp: now,
	}); err != nil {
		t.Fatalf("InsertStreamingLog new: %v", err)
	}

	cutoff := now.AddDate(0, 0, -30)
	n, err := store.PurgeStreamingLogBodiesOlderThan(ctx, cutoff)
	if err != nil {
		t.Fatalf("PurgeStreamingLogBodiesOlderThan: %v", err)
	}
	if n != 1 {
		t.Fatalf("expected 1 row purged, got %d", n)
	}

	chunks, err := store.GetStreamingLogs(ctx, rid)
	if err != nil {
		t.Fatalf("GetStreamingLogs: %v", err)
	}
	if len(chunks) != 2 {
		t.Fatalf("expected 2 chunks, got %d", len(chunks))
	}
	if chunks[0].Data != "" || chunks[0].ContentDelta != "" || !chunks[0].BodyPurged {
		t.Errorf("old chunk not purged: %+v", chunks[0])
	}
	if chunks[1].Data != "raw-new" || chunks[1].ContentDelta != "delta-new" || chunks[1].BodyPurged {
		t.Errorf("new chunk should be untouched: %+v", chunks[1])
	}
}

func TestPurgeRequestLogRequestAndResponseBodiesOlderThan(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Millisecond)

	p := newProvider(uuid.NewString(), "Body Purge", "https://bodypurge.test", false, now)
	if err := store.CreateProvider(ctx, p); err != nil {
		t.Fatalf("CreateProvider: %v", err)
	}

	old := now.Add(-60 * 24 * time.Hour)
	newLog := now.Add(-1 * time.Hour)

	oldID := uuid.NewString()
	if err := store.InsertRequestLog(ctx, &models.RequestLog{
		ID: oldID, Timestamp: old, ClientIP: "127.0.0.1", Method: "POST", Path: "/v1/chat/completions",
		RequestedModel: "m", ProviderID: p.ID, StatusCode: 200, TotalTimeMs: 10, CreatedAt: old,
		RequestBody: `{"old":"req"}`, ResponseBody: `{"old":"resp"}`,
	}); err != nil {
		t.Fatalf("InsertRequestLog old: %v", err)
	}
	newID := uuid.NewString()
	if err := store.InsertRequestLog(ctx, &models.RequestLog{
		ID: newID, Timestamp: newLog, ClientIP: "127.0.0.1", Method: "POST", Path: "/v1/chat/completions",
		RequestedModel: "m", ProviderID: p.ID, StatusCode: 200, TotalTimeMs: 10, CreatedAt: newLog,
		RequestBody: `{"new":"req"}`, ResponseBody: `{"new":"resp"}`,
	}); err != nil {
		t.Fatalf("InsertRequestLog new: %v", err)
	}

	cutoff := now.AddDate(0, 0, -30)
	nReq, err := store.PurgeRequestLogRequestBodiesOlderThan(ctx, cutoff)
	if err != nil {
		t.Fatalf("PurgeRequestLogRequestBodiesOlderThan: %v", err)
	}
	if nReq != 1 {
		t.Fatalf("expected 1 request body purged, got %d", nReq)
	}
	nResp, err := store.PurgeRequestLogResponseBodiesOlderThan(ctx, cutoff)
	if err != nil {
		t.Fatalf("PurgeRequestLogResponseBodiesOlderThan: %v", err)
	}
	if nResp != 1 {
		t.Fatalf("expected 1 response body purged, got %d", nResp)
	}

	gotOld, err := store.GetRequestLog(ctx, oldID)
	if err != nil {
		t.Fatalf("GetRequestLog old: %v", err)
	}
	if gotOld.RequestBody != "" || gotOld.ResponseBody != "" {
		t.Fatalf("old log bodies should be empty: req=%q resp=%q", gotOld.RequestBody, gotOld.ResponseBody)
	}
	gotNew, err := store.GetRequestLog(ctx, newID)
	if err != nil {
		t.Fatalf("GetRequestLog new: %v", err)
	}
	if gotNew.RequestBody != `{"new":"req"}` || gotNew.ResponseBody != `{"new":"resp"}` {
		t.Fatalf("new log should be untouched: %+v", gotNew)
	}
}
