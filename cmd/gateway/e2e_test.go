package main_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/llmate/gateway/internal/admin"
	"github.com/llmate/gateway/internal/db"
	"github.com/llmate/gateway/internal/models"
	"github.com/llmate/gateway/internal/proxy"
	"github.com/llmate/gateway/internal/stats"
)

func newE2EStore(t *testing.T) db.Store {
	t.Helper()
	s, err := db.NewSQLiteStore(":memory:")
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func newE2EServer(t *testing.T, store db.Store) *httptest.Server {
	t.Helper()
	qw := admin.NewQueryWorker(store, 4)
	qw.Start(context.Background())
	h := admin.NewHandler(store, admin.HandlerConfig{}, stats.NewAccumulator(), qw)
	ts := httptest.NewServer(h.Routes())
	t.Cleanup(func() { ts.Close() })
	return ts
}

func doJSON(t *testing.T, method, url string, body any) (int, map[string]json.RawMessage) {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatalf("encode body: %v", err)
		}
	}
	req, err := http.NewRequest(method, url, &buf)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, url, err)
	}
	defer resp.Body.Close()
	m := map[string]json.RawMessage{}
	if resp.StatusCode != http.StatusNoContent {
		if err := json.NewDecoder(resp.Body).Decode(&m); err != nil {
			t.Fatalf("decode response: %v", err)
		}
	}
	return resp.StatusCode, m
}

// TestAliasEditAndProviderRenameE2E exercises the admin HTTP API against a real
// in-memory SQLite store end to end: create a provider, register a model, create
// an alias, edit it, rename the provider, and finally verify the in-memory
// routing catalog reflects the edited alias mapping.
func TestAliasEditAndProviderRenameE2E(t *testing.T) {
	ctx := context.Background()
	store := newE2EStore(t)
	ts := newE2EServer(t, store)

	// 1. Create a provider.
	status, prov := doJSON(t, http.MethodPost, ts.URL+"/providers", map[string]string{
		"name": "Local", "base_url": "http://localhost:11434",
	})
	if status != http.StatusCreated {
		t.Fatalf("create provider: status %d", status)
	}
	var p models.Provider
	if err := json.Unmarshal(prov["provider"], &p); err != nil {
		t.Fatalf("decode provider: %v", err)
	}
	if p.ID == "" {
		t.Fatal("provider id empty")
	}

	// 2. Register a model on the provider.
	status, m := doJSON(t, http.MethodPost, ts.URL+"/providers/"+p.ID+"/models", map[string]string{
		"model_id": "llama3",
	})
	if status != http.StatusCreated {
		t.Fatalf("add model: status %d", status)
	}
	var pms []models.ProviderModel
	if err := json.Unmarshal(m["models"], &pms); err != nil {
		t.Fatalf("decode models: %v", err)
	}
	if len(pms) != 1 || pms[0].ModelID != "llama3" {
		t.Fatalf("unexpected models: %+v", pms)
	}

	// 3. Create an alias pointing at the provider/model.
	status, al := doJSON(t, http.MethodPost, ts.URL+"/aliases", map[string]any{
		"alias": "gpt-4", "provider_id": p.ID, "model_id": "llama3", "weight": 1, "priority": 0,
	})
	if status != http.StatusCreated {
		t.Fatalf("create alias: status %d", status)
	}
	var a models.ModelAlias
	if err := json.Unmarshal(al["alias"], &a); err != nil {
		t.Fatalf("decode alias: %v", err)
	}
	if a.ID == "" || a.Alias != "gpt-4" || a.ModelID != "llama3" {
		t.Fatalf("unexpected alias: %+v", a)
	}

	// 4. Edit the alias to point at a different model and rename it.
	status, upd := doJSON(t, http.MethodPut, ts.URL+"/aliases/"+a.ID, map[string]any{
		"alias": "claude", "provider_id": p.ID, "model_id": "claude-3", "weight": 3, "priority": 10,
	})
	if status != http.StatusOK {
		t.Fatalf("update alias: status %d", status)
	}
	var ua models.ModelAlias
	if err := json.Unmarshal(upd["alias"], &ua); err != nil {
		t.Fatalf("decode updated alias: %v", err)
	}
	if ua.Alias != "claude" || ua.ModelID != "claude-3" || ua.Weight != 3 || ua.Priority != 10 {
		t.Fatalf("unexpected updated alias: %+v", ua)
	}

	// 5. The store resolves the new alias name to the edited model mapping.
	got, err := store.ResolveAlias(ctx, "claude")
	if err != nil {
		t.Fatalf("resolve claude: %v", err)
	}
	if len(got) != 1 || got[0].ModelID != "claude-3" || got[0].ProviderID != p.ID {
		t.Fatalf("resolve claude: %+v", got)
	}
	// The old alias name no longer resolves (it was renamed away).
	stale, err := store.ResolveAlias(ctx, "gpt-4")
	if err != nil {
		t.Fatalf("resolve gpt-4: %v", err)
	}
	if len(stale) != 0 {
		t.Fatalf("old alias should not resolve: %+v", stale)
	}

	// 6. Rename the provider via the admin API.
	status, rn := doJSON(t, http.MethodPut, ts.URL+"/providers/"+p.ID, map[string]string{
		"name": "Renamed", "base_url": "http://localhost:11434",
	})
	if status != http.StatusOK {
		t.Fatalf("update provider: status %d", status)
	}
	var rp models.Provider
	if err := json.Unmarshal(rn["provider"], &rp); err != nil {
		t.Fatalf("decode renamed provider: %v", err)
	}
	if rp.Name != "Renamed" {
		t.Fatalf("unexpected provider name: %q", rp.Name)
	}

	// 7. The in-memory routing catalog (what the proxy hot path uses) reflects the
	// edited alias mapping after a reload from the same store.
	if err := store.UpdateProviderHealth(ctx, p.ID, true); err != nil {
		t.Fatalf("mark provider healthy: %v", err)
	}
	cat := proxy.NewRoutingCatalog(store)
	if err := cat.Reload(ctx); err != nil {
		t.Fatalf("reload catalog: %v", err)
	}
	cands, ok := cat.AliasCandidates("claude")
	if !ok || len(cands) != 1 || cands[0].ModelID != "claude-3" {
		t.Fatalf("catalog claude candidates: ok=%v %+v", ok, cands)
	}
	// The renamed-away alias is no longer routable through the catalog.
	if _, ok := cat.AliasCandidates("gpt-4"); ok {
		t.Fatal("catalog should not route renamed-away alias gpt-4")
	}
}
