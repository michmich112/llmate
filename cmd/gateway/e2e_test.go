package main_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	chi "github.com/go-chi/chi/v5"
	"github.com/llmate/gateway/internal/admin"
	"github.com/llmate/gateway/internal/auth"
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

// ---------------------------------------------------------------------------
// E2E: API key authentication + per-key usage recording
// ---------------------------------------------------------------------------

// e2eSyncMetrics implements proxy.MetricsCollector and persists every RequestLog
// synchronously to the store so usage is immediately queryable (no goroutine
// timing like the production async collector).
type e2eSyncMetrics struct {
	ctx   context.Context
	store db.Store
}

func (m *e2eSyncMetrics) Record(log *models.RequestLog) {
	_ = m.store.InsertRequestLog(m.ctx, log)
}

func (m *e2eSyncMetrics) RecordStreaming(_ *models.RequestLog, _ []proxy.StreamingLogChunk, _ bool) {}

// e2eRouter routes every proxy request to a fixed backend, keeping the test
// focused on API-key auth + usage rather than provider routing (covered elsewhere).
type e2eRouter struct {
	target string
}

func (r *e2eRouter) Route(_ context.Context, _ string, _ string) (*proxy.RouteResult, error) {
	return &proxy.RouteResult{
		Provider:  models.Provider{ID: "e2e-provider", Name: "E2E"},
		ModelID:   "gpt-4o",
		TargetURL: r.target,
	}, nil
}

func (r *e2eRouter) ReportSuccess(_ string) {}
func (r *e2eRouter) ReportFailure(_ string) {}

// doJSONAuth is doJSON plus an Authorization: Bearer <key> header.
func doJSONAuth(t *testing.T, method, url string, key string, body any) (int, map[string]json.RawMessage) {
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
	req.Header.Set("Authorization", "Bearer "+key)
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

// TestAPIKeyAuthAndUsageE2E wires the full stack (real SQLite store, admin handler
// behind ACCESS_KEY middleware, proxy handler behind RequireAPIKey, mock backend)
// and verifies: admin creates an API key, a proxied chat completion authenticated
// with that key succeeds, and usage shows up for the key via both the admin
// /usage endpoint and the user-facing /me/usage endpoint.
func TestAPIKeyAuthAndUsageE2E(t *testing.T) {
	ctx := context.Background()
	store := newE2EStore(t)

	// Mock LLM backend returns a normal chat completion.
	const respBody = `{"id":"chatcmpl-e2e","object":"chat.completion","choices":[{"index":0,"message":{"role":"assistant","content":"hi"},"finish_reason":"stop"}],"usage":{"prompt_tokens":8,"completion_tokens":5,"total_tokens":13}}`
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, respBody)
	}))
	t.Cleanup(func() { backend.Close() })

	// Proxy handler wired like production (RequireAPIKey wrapper).
	metrics := &e2eSyncMetrics{ctx: ctx, store: store}
	router := &e2eRouter{target: backend.URL + "/v1/chat/completions"}
	cat := proxy.NewRoutingCatalog(store)
	configSnap := proxy.NewConfigSnapshot(store)
	proxyHandler := proxy.NewHandler(router, metrics, cat, configSnap, nil, store)

	// Admin handler; /auth and /me/usage are open (reachable with an API key),
	// everything else is gated by ACCESS_KEY middleware (mirrors main.go).
	qw := admin.NewQueryWorker(store, 4)
	qw.Start(ctx)
	adminHandler := admin.NewHandler(store, admin.HandlerConfig{AccessKey: "admin-secret"}, stats.NewAccumulator(), qw)

	adminRouter := chi.NewRouter()
	adminRouter.Post("/auth", adminHandler.HandleAuth)
	adminRouter.Get("/me/usage", adminHandler.HandleMyUsage)
	adminRouter.Route("/", func(r chi.Router) {
		r.Use(auth.AccessKeyMiddleware("admin-secret"))
		r.Mount("/", adminHandler.Routes())
	})
	adminTS := httptest.NewServer(adminRouter)
	t.Cleanup(func() { adminTS.Close() })

	proxyRouter := chi.NewRouter()
	proxyRouter.Post("/v1/chat/completions", proxyHandler.RequireAPIKey(proxyHandler.HandleChatCompletions))
	proxyTS := httptest.NewServer(proxyRouter)
	t.Cleanup(func() { proxyTS.Close() })

	// 1. Create an API key via the admin API (ACCESS_KEY required).
	status, createResp := doJSONAuth(t, http.MethodPost, adminTS.URL+"/keys", "admin-secret", map[string]string{"name": "app-key"})
	if status != http.StatusCreated {
		t.Fatalf("create key: status %d", status)
	}
	var key models.APIKey
	if err := json.Unmarshal(createResp["api_key"], &key); err != nil {
		t.Fatalf("decode api_key: %v", err)
	}
	var rawKey string
	if err := json.Unmarshal(createResp["key"], &rawKey); err != nil {
		t.Fatalf("decode raw key: %v", err)
	}
	if key.ID == "" || key.Name != "app-key" || rawKey == "" {
		t.Fatalf("unexpected key: %+v raw=%q", key, rawKey)
	}

	// 2. Proxied chat completion authenticated with the raw API key succeeds.
	status, chatResp := doJSONAuth(t, http.MethodPost, proxyTS.URL+"/v1/chat/completions", rawKey, map[string]any{
		"model": "gpt-4o",
		"messages": []map[string]string{{"role": "user", "content": "hello"}},
	})
	if status != http.StatusOK {
		t.Fatalf("chat completion: status %d", status)
	}
	if !bytes.Contains(chatResp["id"], []byte("chatcmpl-e2e")) {
		t.Fatalf("unexpected completion response: %s", chatResp["id"])
	}

	// 3. Reject requests with an invalid API key.
	status, _ = doJSONAuth(t, http.MethodPost, proxyTS.URL+"/v1/chat/completions", "not-a-real-key", map[string]any{
		"model": "gpt-4o",
		"messages": []map[string]string{{"role": "user", "content": "hello"}},
	})
	if status != http.StatusUnauthorized {
		t.Fatalf("invalid key should be unauthorized, got %d", status)
	}

	// 4. Admin sees per-key usage via /usage (ACCESS_KEY).
	status, usageResp := doJSONAuth(t, http.MethodGet, adminTS.URL+"/usage", "admin-secret", nil)
	if status != http.StatusOK {
		t.Fatalf("admin usage: status %d", status)
	}
	var usage []models.APIKeyUsage
	if err := json.Unmarshal(usageResp["usage"], &usage); err != nil {
		t.Fatalf("decode usage: %v", err)
	}
	if len(usage) != 1 || usage[0].APIKeyID != key.ID || usage[0].RequestCount != 1 || usage[0].TotalTokens != 13 {
		t.Fatalf("unexpected admin usage: %+v", usage)
	}

	// 5. The API key user sees their own usage via /me/usage (open route, key auth).
	status, myResp := doJSONAuth(t, http.MethodGet, adminTS.URL+"/me/usage", rawKey, nil)
	if status != http.StatusOK {
		t.Fatalf("my usage: status %d", status)
	}
	var my models.APIKeyUsage
	if err := json.Unmarshal(myResp["usage"], &my); err != nil {
		t.Fatalf("decode my usage: %v", err)
	}
	if my.APIKeyID != key.ID || my.RequestCount != 1 || my.TotalTokens != 13 {
		t.Fatalf("unexpected my usage: %+v", my)
	}
	var myName string
	if err := json.Unmarshal(myResp["api_key"], &myName); err != nil {
		t.Fatalf("decode my key name: %v", err)
	}
	if myName != "app-key" {
		t.Fatalf("unexpected my key name: %q", myName)
	}
}
