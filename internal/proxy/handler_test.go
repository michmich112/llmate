package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	chi "github.com/go-chi/chi/v5"
	"github.com/llmate/gateway/internal/models"
)

// ---------------------------------------------------------------------------
// Mock Router
// ---------------------------------------------------------------------------

type mockRouter struct {
	mu           sync.Mutex
	routeFn      func(ctx context.Context, modelID string, endpointPath string) (*RouteResult, error)
	successCount int
	failureCount int
}

func (r *mockRouter) Route(ctx context.Context, modelID string, endpointPath string) (*RouteResult, error) {
	if r.routeFn != nil {
		return r.routeFn(ctx, modelID, endpointPath)
	}
	return nil, fmt.Errorf("no route configured")
}

func (r *mockRouter) ReportSuccess(_ string) {
	r.mu.Lock()
	r.successCount++
	r.mu.Unlock()
}

func (r *mockRouter) ReportFailure(_ string) {
	r.mu.Lock()
	r.failureCount++
	r.mu.Unlock()
}

// ---------------------------------------------------------------------------
// Mock MetricsCollector
// ---------------------------------------------------------------------------

type mockMetrics struct {
	mu   sync.Mutex
	logs []*models.RequestLog
}

func (m *mockMetrics) PersistSync(_ *models.RequestLog) error {
	return nil
}

func (m *mockMetrics) Record(log *models.RequestLog) {
	m.mu.Lock()
	m.logs = append(m.logs, log)
	m.mu.Unlock()
}

func (m *mockMetrics) last() *models.RequestLog {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.logs) == 0 {
		return nil
	}
	return m.logs[len(m.logs)-1]
}

// ---------------------------------------------------------------------------
// Mock Store (full db.Store interface)
// ---------------------------------------------------------------------------

type mockStore struct {
	allModels []models.ProviderModel
	aliases   []models.ModelAlias
}

func (s *mockStore) CreateProvider(_ context.Context, _ *models.Provider) error  { return nil }
func (s *mockStore) GetProvider(_ context.Context, _ string) (*models.Provider, error) {
	return nil, nil
}
func (s *mockStore) ListProviders(_ context.Context) ([]models.Provider, error) { return nil, nil }
func (s *mockStore) UpdateProvider(_ context.Context, _ *models.Provider) error  { return nil }
func (s *mockStore) DeleteProvider(_ context.Context, _ string) error             { return nil }
func (s *mockStore) UpsertProviderEndpoints(_ context.Context, _ string, _ []models.ProviderEndpoint) error {
	return nil
}
func (s *mockStore) ListProviderEndpoints(_ context.Context, _ string) ([]models.ProviderEndpoint, error) {
	return nil, nil
}
func (s *mockStore) UpdateProviderEndpoint(_ context.Context, _ *models.ProviderEndpoint) error {
	return nil
}
func (s *mockStore) SyncProviderModels(_ context.Context, _ string, _ []string) error { return nil }
func (s *mockStore) ListProviderModels(_ context.Context, _ string) ([]models.ProviderModel, error) {
	return nil, nil
}
func (s *mockStore) ListAllModels(_ context.Context) ([]models.ProviderModel, error) {
	return s.allModels, nil
}
func (s *mockStore) CreateAlias(_ context.Context, _ *models.ModelAlias) error { return nil }
func (s *mockStore) ListAliases(_ context.Context) ([]models.ModelAlias, error) {
	return s.aliases, nil
}
func (s *mockStore) UpdateAlias(_ context.Context, _ *models.ModelAlias) error { return nil }
func (s *mockStore) DeleteAlias(_ context.Context, _ string) error              { return nil }
func (s *mockStore) ResolveAlias(_ context.Context, _ string) ([]models.ModelAlias, error) {
	return nil, nil
}
func (s *mockStore) GetHealthyProvidersForModel(_ context.Context, _ string) ([]models.Provider, error) {
	return nil, nil
}
func (s *mockStore) GetEnabledEndpoint(_ context.Context, _, _ string) (*models.ProviderEndpoint, error) {
	return nil, nil
}
func (s *mockStore) InsertRequestLog(_ context.Context, _ *models.RequestLog) error { return nil }
func (s *mockStore) QueryRequestLogs(_ context.Context, _ models.LogFilter) ([]models.RequestLog, int, error) {
	return nil, 0, nil
}
func (s *mockStore) GetRequestLog(_ context.Context, _ string) (*models.RequestLog, error) {
	return nil, nil
}
func (s *mockStore) UpdateProviderModelCosts(_ context.Context, _ string, _ *models.ProviderModel) error {
	return nil
}
func (s *mockStore) GetProviderModelCosts(_ context.Context, _, _ string) (*models.ProviderModel, error) {
	return nil, nil
}
func (s *mockStore) GetDashboardStats(_ context.Context, _ time.Time) (*models.DashboardStats, error) {
	return nil, nil
}
func (s *mockStore) GetTimeSeries(_ context.Context, _, _ time.Time, _ string) ([]models.TimeSeriesPoint, error) {
	return nil, nil
}
func (s *mockStore) GetAllConfig(_ context.Context) (map[string]string, error) { return map[string]string{}, nil }
func (s *mockStore) SetConfig(_ context.Context, _, _ string) error              { return nil }
func (s *mockStore) InsertStreamingLog(_ context.Context, _ *models.StreamingLog) error {
	return nil
}
func (s *mockStore) GetStreamingLogs(_ context.Context, _ string) ([]models.StreamingLog, error) {
	return nil, nil
}
func (s *mockStore) PurgeStreamingLogBodiesOlderThan(_ context.Context, _ time.Time) (int64, error) {
	return 0, nil
}
func (s *mockStore) PurgeRequestLogRequestBodiesOlderThan(_ context.Context, _ time.Time) (int64, error) {
	return 0, nil
}
func (s *mockStore) PurgeRequestLogResponseBodiesOlderThan(_ context.Context, _ time.Time) (int64, error) {
	return 0, nil
}
func (s *mockStore) UpdateProviderHealth(_ context.Context, _ string, _ bool) error { return nil }
func (s *mockStore) Close() error                                                    { return nil }

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// withChiParam injects a chi URL parameter into the request context.
func withChiParam(r *http.Request, key, val string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add(key, val)
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}

func fixedRoute(serverURL, endpointPath string) *RouteResult {
	return &RouteResult{
		Provider:  models.Provider{ID: "provider-1", Name: "Test Provider", APIKey: "test-key"},
		ModelID:   "gpt-4o",
		TargetURL: serverURL + endpointPath,
	}
}

// ---------------------------------------------------------------------------
// Test 1: Non-streaming chat completion
// ---------------------------------------------------------------------------

func TestHandleChatCompletions_NonStreaming(t *testing.T) {
	const respBody = `{"id":"chatcmpl-1","choices":[{"message":{"role":"assistant","content":"Hello"}}],"usage":{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15}}`

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		fmt.Fprint(w, respBody)
	}))
	defer backend.Close()

	router := &mockRouter{
		routeFn: func(_ context.Context, _ string, ep string) (*RouteResult, error) {
			return fixedRoute(backend.URL, ep), nil
		},
	}
	metrics := &mockMetrics{}
	h := NewHandler(router, metrics, &mockStore{}, nil)

	body := `{"model":"gpt-4o","messages":[{"role":"user","content":"Hi"}]}`
	req := httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.HandleChatCompletions(rr, req)

	if rr.Code != 200 {
		t.Fatalf("expected 200, got %d; body: %s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), `"chatcmpl-1"`) {
		t.Errorf("response body missing expected content: %s", rr.Body.String())
	}

	log := metrics.last()
	if log == nil {
		t.Fatal("expected metrics.Record to be called")
	}
	if log.RequestedModel != "gpt-4o" {
		t.Errorf("RequestedModel = %q, want gpt-4o", log.RequestedModel)
	}
	if log.ProviderID != "provider-1" {
		t.Errorf("ProviderID = %q, want provider-1", log.ProviderID)
	}
	if log.PromptTokens == nil || *log.PromptTokens != 10 {
		t.Errorf("PromptTokens = %v, want 10", log.PromptTokens)
	}
	if log.CompletionTokens == nil || *log.CompletionTokens != 5 {
		t.Errorf("CompletionTokens = %v, want 5", log.CompletionTokens)
	}
	if log.TotalTokens == nil || *log.TotalTokens != 15 {
		t.Errorf("TotalTokens = %v, want 15", log.TotalTokens)
	}
	if log.IsStreamed {
		t.Error("IsStreamed should be false")
	}

	router.mu.Lock()
	sc := router.successCount
	router.mu.Unlock()
	if sc != 1 {
		t.Errorf("ReportSuccess count = %d, want 1", sc)
	}
}

func TestHandleChatCompletions_NonStreaming_AliasRewritesResponseModel(t *testing.T) {
	const backendModel = "llama-3"
	respBody := `{"id":"chatcmpl-1","model":"` + backendModel + `","choices":[{"message":{"role":"assistant","content":"Hi"}}]}`

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		fmt.Fprint(w, respBody)
	}))
	defer backend.Close()

	router := &mockRouter{
		routeFn: func(_ context.Context, _ string, ep string) (*RouteResult, error) {
			res := fixedRoute(backend.URL, ep)
			res.ModelID = backendModel
			res.RequestedViaAlias = true
			return res, nil
		},
	}
	metrics := &mockMetrics{}
	h := NewHandler(router, metrics, &mockStore{}, nil)

	body := `{"model":"fast","messages":[{"role":"user","content":"Hi"}]}`
	req := httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.HandleChatCompletions(rr, req)

	if rr.Code != 200 {
		t.Fatalf("expected 200, got %d; body: %s", rr.Code, rr.Body.String())
	}
	var out struct {
		Model string `json:"model"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if out.Model != "fast" {
		t.Errorf("response model = %q, want fast (alias name)", out.Model)
	}
	log := metrics.last()
	if log == nil {
		t.Fatal("expected metrics.Record to be called")
	}
	if !strings.Contains(log.ResponseBody, `"model":"fast"`) {
		t.Errorf("logged ResponseBody should use client model name: %s", log.ResponseBody)
	}
}

// ---------------------------------------------------------------------------
// Test 2: Streaming chat completion
// ---------------------------------------------------------------------------

func TestHandleChatCompletions_Streaming(t *testing.T) {
	sseLines := []string{
		`data: {"id":"chatcmpl-2","choices":[{"delta":{"content":"Hello"}}]}`,
		`data: {"id":"chatcmpl-2","choices":[{"delta":{"content":" world"}}],"usage":{"prompt_tokens":5,"completion_tokens":10,"total_tokens":15}}`,
		`data: [DONE]`,
	}

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher := w.(http.Flusher)
		for _, line := range sseLines {
			fmt.Fprintf(w, "%s\n", line)
			flusher.Flush()
		}
	}))
	defer backend.Close()

	router := &mockRouter{
		routeFn: func(_ context.Context, _ string, ep string) (*RouteResult, error) {
			return fixedRoute(backend.URL, ep), nil
		},
	}
	metrics := &mockMetrics{}
	h := NewHandler(router, metrics, &mockStore{}, nil)

	body := `{"model":"gpt-4o","messages":[{"role":"user","content":"Hi"}],"stream":true}`
	req := httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.HandleChatCompletions(rr, req)

	if rr.Code != 200 {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	out := rr.Body.String()
	for _, line := range sseLines {
		if !strings.Contains(out, line) {
			t.Errorf("response missing SSE line %q\nfull body:\n%s", line, out)
		}
	}

	log := metrics.last()
	if log == nil {
		t.Fatal("expected metrics.Record to be called")
	}
	if !log.IsStreamed {
		t.Error("IsStreamed should be true")
	}
	if log.TTFTMs == nil {
		t.Error("TTFTMs should be non-nil for streaming response")
	}
	if log.PromptTokens == nil || *log.PromptTokens != 5 {
		t.Errorf("PromptTokens = %v, want 5", log.PromptTokens)
	}
	if log.CompletionTokens == nil || *log.CompletionTokens != 10 {
		t.Errorf("CompletionTokens = %v, want 10", log.CompletionTokens)
	}
	if log.TotalTokens == nil || *log.TotalTokens != 15 {
		t.Errorf("TotalTokens = %v, want 15", log.TotalTokens)
	}

	router.mu.Lock()
	sc := router.successCount
	router.mu.Unlock()
	if sc != 1 {
		t.Errorf("ReportSuccess count = %d, want 1", sc)
	}
}

func TestHandleChatCompletions_Streaming_AliasRewritesResponseModel(t *testing.T) {
	sseLines := []string{
		`data: {"id":"chatcmpl-2","model":"llama-3","choices":[{"delta":{"content":"Hello"}}]}`,
		`data: [DONE]`,
	}

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher := w.(http.Flusher)
		for _, line := range sseLines {
			fmt.Fprintf(w, "%s\n", line)
			flusher.Flush()
		}
	}))
	defer backend.Close()

	router := &mockRouter{
		routeFn: func(_ context.Context, _ string, ep string) (*RouteResult, error) {
			res := fixedRoute(backend.URL, ep)
			res.ModelID = "llama-3"
			res.RequestedViaAlias = true
			return res, nil
		},
	}
	metrics := &mockMetrics{}
	h := NewHandler(router, metrics, &mockStore{}, nil)

	body := `{"model":"fast","messages":[{"role":"user","content":"Hi"}],"stream":true}`
	req := httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.HandleChatCompletions(rr, req)

	if rr.Code != 200 {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	out := rr.Body.String()
	if strings.Contains(out, `"model":"llama-3"`) {
		t.Errorf("client stream should not expose backend model id; body:\n%s", out)
	}
	if !strings.Contains(out, `"model":"fast"`) {
		t.Errorf("expected alias in streamed JSON; body:\n%s", out)
	}
}

// ---------------------------------------------------------------------------
// Test 3: extractModelFromJSON
// ---------------------------------------------------------------------------

func TestExtractModelFromJSON(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		want    string
		wantErr bool
	}{
		{name: "valid model", body: `{"model":"gpt-4o","messages":[]}`, want: "gpt-4o"},
		{name: "missing model field", body: `{"messages":[]}`, wantErr: true},
		{name: "empty model field", body: `{"model":"","messages":[]}`, wantErr: true},
		{name: "invalid JSON", body: `not json`, wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := extractModelFromJSON([]byte(tc.body))
			if tc.wantErr {
				if err == nil {
					t.Errorf("expected error, got model=%q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestRewriteResponseModelForClient(t *testing.T) {
	in := []byte(`{"id":"1","model":"backend-id","choices":[]}`)
	out := rewriteResponseModelForClient(in, "my-alias")
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(out, &obj); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	var model string
	if err := json.Unmarshal(obj["model"], &model); err != nil {
		t.Fatalf("model field: %v", err)
	}
	if model != "my-alias" {
		t.Errorf("model = %q, want my-alias", model)
	}
	noModel := []byte(`{"id":"1"}`)
	if got := rewriteResponseModelForClient(noModel, "x"); string(got) != string(noModel) {
		t.Errorf("expected unchanged body without model key")
	}
	bad := []byte(`not-json`)
	if got := rewriteResponseModelForClient(bad, "x"); string(got) != string(bad) {
		t.Errorf("expected unchanged invalid JSON")
	}
}

// ---------------------------------------------------------------------------
// Test 4: extractModelFromMultipart
// ---------------------------------------------------------------------------

func TestExtractModelFromMultipart(t *testing.T) {
	t.Run("valid model and file", func(t *testing.T) {
		var buf bytes.Buffer
		mw := multipart.NewWriter(&buf)
		_ = mw.WriteField("model", "whisper-1")
		fw, _ := mw.CreateFormFile("file", "audio.mp3")
		_, _ = fw.Write([]byte("fake audio data"))
		mw.Close()

		req := httptest.NewRequest("POST", "/v1/audio/transcriptions", &buf)
		req.Header.Set("Content-Type", mw.FormDataContentType())

		model, err := extractModelFromMultipart(req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if model != "whisper-1" {
			t.Errorf("got %q, want %q", model, "whisper-1")
		}
	})

	t.Run("missing model field", func(t *testing.T) {
		var buf bytes.Buffer
		mw := multipart.NewWriter(&buf)
		fw, _ := mw.CreateFormFile("file", "audio.mp3")
		_, _ = fw.Write([]byte("data"))
		mw.Close()

		req := httptest.NewRequest("POST", "/v1/audio/transcriptions", &buf)
		req.Header.Set("Content-Type", mw.FormDataContentType())

		_, err := extractModelFromMultipart(req)
		if err == nil {
			t.Error("expected error for missing model field")
		}
	})
}

// ---------------------------------------------------------------------------
// Test 5: Backend error failover
// ---------------------------------------------------------------------------

func TestFailover_FirstBackend503_SecondBackend200(t *testing.T) {
	const successBody = `{"id":"chatcmpl-ok","choices":[],"usage":{"prompt_tokens":2,"completion_tokens":3,"total_tokens":5}}`

	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(503)
		fmt.Fprint(w, `{"error":"overloaded"}`)
	}))
	defer bad.Close()

	good := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		fmt.Fprint(w, successBody)
	}))
	defer good.Close()

	callCount := 0
	var mu sync.Mutex
	router := &mockRouter{
		routeFn: func(_ context.Context, _ string, ep string) (*RouteResult, error) {
			mu.Lock()
			callCount++
			n := callCount
			mu.Unlock()
			if n == 1 {
				return &RouteResult{
					Provider:  models.Provider{ID: "p-bad", Name: "Bad"},
					ModelID:   "gpt-4o",
					TargetURL: bad.URL + ep,
				}, nil
			}
			return &RouteResult{
				Provider:  models.Provider{ID: "p-good", Name: "Good"},
				ModelID:   "gpt-4o",
				TargetURL: good.URL + ep,
			}, nil
		},
	}
	metrics := &mockMetrics{}
	h := NewHandler(router, metrics, &mockStore{}, nil)

	body := `{"model":"gpt-4o","messages":[]}`
	req := httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.HandleChatCompletions(rr, req)

	if rr.Code != 200 {
		t.Fatalf("expected 200 after failover, got %d; body: %s", rr.Code, rr.Body.String())
	}

	router.mu.Lock()
	fc, sc := router.failureCount, router.successCount
	router.mu.Unlock()

	if fc != 1 {
		t.Errorf("ReportFailure count = %d, want 1", fc)
	}
	if sc != 1 {
		t.Errorf("ReportSuccess count = %d, want 1", sc)
	}

	log := metrics.last()
	if log == nil {
		t.Fatal("expected metrics.Record to be called")
	}
	if log.ProviderID != "p-good" {
		t.Errorf("final ProviderID = %q, want p-good", log.ProviderID)
	}
}

// ---------------------------------------------------------------------------
// Test 6: HandleListModels
// ---------------------------------------------------------------------------

func TestHandleListModels(t *testing.T) {
	store := &mockStore{
		allModels: []models.ProviderModel{
			{ID: "pm1", ProviderID: "p1", ModelID: "gpt-4o"},
			{ID: "pm2", ProviderID: "p1", ModelID: "gpt-3.5-turbo"},
		},
		aliases: []models.ModelAlias{
			{ID: "a1", Alias: "smart", ModelID: "gpt-4o", ProviderID: "p1", IsEnabled: true},
			{ID: "a2", Alias: "disabled-alias", ModelID: "gpt-3.5-turbo", ProviderID: "p1", IsEnabled: false},
		},
	}
	h := NewHandler(&mockRouter{}, &mockMetrics{}, store, nil)

	req := httptest.NewRequest("GET", "/v1/models", nil)
	rr := httptest.NewRecorder()
	h.HandleListModels(rr, req)

	if rr.Code != 200 {
		t.Fatalf("expected 200, got %d; body: %s", rr.Code, rr.Body.String())
	}

	var resp struct {
		Object string        `json:"object"`
		Data   []modelObject `json:"data"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if resp.Object != "list" {
		t.Errorf("object = %q, want list", resp.Object)
	}

	ids := make(map[string]bool)
	for _, d := range resp.Data {
		ids[d.ID] = true
	}
	for _, want := range []string{"gpt-4o", "gpt-3.5-turbo", "smart"} {
		if !ids[want] {
			t.Errorf("expected %q in model list, got: %v", want, resp.Data)
		}
	}
	if ids["disabled-alias"] {
		t.Error("disabled alias should not appear in model list")
	}
	if len(resp.Data) != 3 {
		t.Errorf("expected 3 entries, got %d: %v", len(resp.Data), resp.Data)
	}
}

// ---------------------------------------------------------------------------
// Test 7: injectStreamOptions
// ---------------------------------------------------------------------------

func TestInjectStreamOptions(t *testing.T) {
	t.Run("no existing stream_options", func(t *testing.T) {
		out, err := injectStreamOptions([]byte(`{"model":"gpt-4o","messages":[]}`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var obj map[string]json.RawMessage
		if err := json.Unmarshal(out, &obj); err != nil {
			t.Fatalf("output not valid JSON: %v", err)
		}
		var so map[string]interface{}
		json.Unmarshal(obj["stream_options"], &so)
		if so["include_usage"] != true {
			t.Errorf("include_usage = %v, want true", so["include_usage"])
		}
	})

	t.Run("existing stream_options — merge preserves other keys", func(t *testing.T) {
		out, err := injectStreamOptions([]byte(`{"model":"gpt-4o","stream_options":{"other_key":"value"}}`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var obj map[string]json.RawMessage
		json.Unmarshal(out, &obj)
		var so map[string]interface{}
		json.Unmarshal(obj["stream_options"], &so)
		if so["include_usage"] != true {
			t.Errorf("include_usage = %v, want true", so["include_usage"])
		}
		if so["other_key"] != "value" {
			t.Errorf("other_key = %v, want value (must be preserved)", so["other_key"])
		}
	})

	t.Run("existing include_usage false is overridden to true", func(t *testing.T) {
		out, err := injectStreamOptions([]byte(`{"model":"gpt-4o","stream_options":{"include_usage":false}}`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var obj map[string]json.RawMessage
		json.Unmarshal(out, &obj)
		var so map[string]interface{}
		json.Unmarshal(obj["stream_options"], &so)
		if so["include_usage"] != true {
			t.Errorf("include_usage = %v, want true", so["include_usage"])
		}
	})

	t.Run("invalid JSON returns error", func(t *testing.T) {
		_, err := injectStreamOptions([]byte(`not json`))
		if err == nil {
			t.Error("expected error for invalid JSON")
		}
	})
}

// ---------------------------------------------------------------------------
// Test 8: Binary passthrough (HandleAudioSpeech)
// ---------------------------------------------------------------------------

func TestHandleAudioSpeech_BinaryPassthrough(t *testing.T) {
	fakeAudio := []byte{0xFF, 0xFB, 0x90, 0x00, 0x01, 0x02, 0x03}

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "audio/mpeg")
		w.WriteHeader(200)
		w.Write(fakeAudio)
	}))
	defer backend.Close()

	router := &mockRouter{
		routeFn: func(_ context.Context, _ string, ep string) (*RouteResult, error) {
			return fixedRoute(backend.URL, ep), nil
		},
	}
	metrics := &mockMetrics{}
	h := NewHandler(router, metrics, &mockStore{}, nil)

	body := `{"model":"tts-1","input":"Hello world","voice":"alloy"}`
	req := httptest.NewRequest("POST", "/v1/audio/speech", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.HandleAudioSpeech(rr, req)

	if rr.Code != 200 {
		t.Fatalf("expected 200, got %d; body: %s", rr.Code, rr.Body.String())
	}
	if ct := rr.Header().Get("Content-Type"); ct != "audio/mpeg" {
		t.Errorf("Content-Type = %q, want audio/mpeg", ct)
	}
	if !bytes.Equal(rr.Body.Bytes(), fakeAudio) {
		t.Error("response body does not match expected binary audio bytes")
	}

	log := metrics.last()
	if log == nil {
		t.Fatal("expected metrics.Record to be called")
	}
	// Token fields must be nil for binary responses (no JSON parsing).
	if log.PromptTokens != nil {
		t.Errorf("PromptTokens should be nil for binary response, got %d", *log.PromptTokens)
	}
}

// ---------------------------------------------------------------------------
// Test 9: HandleGetModel via chi router
// ---------------------------------------------------------------------------

func TestHandleGetModel(t *testing.T) {
	store := &mockStore{
		allModels: []models.ProviderModel{
			{ID: "pm1", ProviderID: "p1", ModelID: "gpt-4o"},
		},
	}
	h := NewHandler(&mockRouter{}, &mockMetrics{}, store, nil)

	r := chi.NewRouter()
	r.Get("/v1/models/{model}", h.HandleGetModel)
	r.Get("/v1/models", h.HandleListModels)
	ts := httptest.NewServer(r)
	defer ts.Close()

	t.Run("found", func(t *testing.T) {
		resp, err := ts.Client().Get(ts.URL + "/v1/models/gpt-4o")
		if err != nil {
			t.Fatalf("request error: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
		var obj modelObject
		if err := json.NewDecoder(resp.Body).Decode(&obj); err != nil {
			t.Fatalf("decode error: %v", err)
		}
		if obj.ID != "gpt-4o" {
			t.Errorf("id = %q, want gpt-4o", obj.ID)
		}
		if obj.Object != "model" {
			t.Errorf("object = %q, want model", obj.Object)
		}
		if obj.OwnedBy != "llmate" {
			t.Errorf("owned_by = %q, want llmate", obj.OwnedBy)
		}
	})

	t.Run("not found", func(t *testing.T) {
		resp, err := ts.Client().Get(ts.URL + "/v1/models/nonexistent-model")
		if err != nil {
			t.Fatalf("request error: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 404 {
			t.Errorf("expected 404, got %d", resp.StatusCode)
		}
	})
}
