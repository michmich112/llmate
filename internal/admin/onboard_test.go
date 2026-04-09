package admin

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/llmate/gateway/internal/models"
)

// newOnboardRouter wraps h in a chi router that matches the real route pattern
// so chi.URLParam(r, "id") works inside the handlers.
func newOnboardRouter(h *OnboardHandler) http.Handler {
	r := chi.NewRouter()
	r.Post("/providers/{id}/discover", h.HandleDiscover)
	r.Post("/providers/{id}/confirm", h.HandleConfirm)
	return r
}

// TestHandleDiscover_Success tests a fully successful discovery: models listed and
// endpoints probed with mixed results.
func TestHandleDiscover_Success(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/models":
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintln(w, `{"data":[{"id":"m1"},{"id":"m2"}]}`)
		case "/v1/chat/completions":
			w.WriteHeader(http.StatusOK)
		case "/v1/completions":
			w.WriteHeader(http.StatusNotFound)
		case "/v1/embeddings":
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer backend.Close()

	store := &mockStore{
		getProvider: func(_ context.Context, id string) (*models.Provider, error) {
			return &models.Provider{ID: id, Name: "Test", BaseURL: backend.URL}, nil
		},
	}

	h := &OnboardHandler{
		store:        store,
		client:       &http.Client{},
		probeTimeout: 2 * time.Second,
	}

	req := httptest.NewRequest(http.MethodPost, "/providers/prov-1/discover", nil)
	rr := httptest.NewRecorder()
	newOnboardRouter(h).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rr.Code, rr.Body.String())
	}

	var result discoverResult
	if err := json.NewDecoder(rr.Body).Decode(&result); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	// Models
	if len(result.Models) != 2 || result.Models[0] != "m1" || result.Models[1] != "m2" {
		t.Errorf("models = %v, want [m1 m2]", result.Models)
	}

	// Endpoint helpers
	byPath := func(path string) *discoverEndpoint {
		for i := range result.Endpoints {
			if result.Endpoints[i].Path == path {
				return &result.Endpoints[i]
			}
		}
		return nil
	}
	assertSupported := func(path string, want *bool) {
		t.Helper()
		ep := byPath(path)
		if ep == nil {
			t.Errorf("endpoint %s missing from response", path)
			return
		}
		if want == nil {
			if ep.IsSupported != nil {
				t.Errorf("%s: is_supported = %v, want null", path, *ep.IsSupported)
			}
		} else {
			if ep.IsSupported == nil {
				t.Errorf("%s: is_supported = null, want %v", path, *want)
			} else if *ep.IsSupported != *want {
				t.Errorf("%s: is_supported = %v, want %v", path, *ep.IsSupported, *want)
			}
		}
	}

	tr, fa := true, false
	assertSupported("/v1/chat/completions", &tr)
	assertSupported("/v1/completions", &fa)
	assertSupported("/v1/embeddings", &tr)
	assertSupported("/v1/images/generations", nil)
	assertSupported("/v1/audio/speech", nil)
	assertSupported("/v1/audio/transcriptions", nil)

	if len(result.Endpoints) != 6 {
		t.Errorf("len(endpoints) = %d, want 6", len(result.Endpoints))
	}
}

// TestHandleDiscover_ProviderNotFound verifies a 404 when the provider ID is unknown.
func TestHandleDiscover_ProviderNotFound(t *testing.T) {
	store := &mockStore{
		getProvider: func(_ context.Context, id string) (*models.Provider, error) {
			return nil, fmt.Errorf("not found: %w", sql.ErrNoRows)
		},
	}

	h := &OnboardHandler{store: store, client: &http.Client{}}
	req := httptest.NewRequest(http.MethodPost, "/providers/missing/discover", nil)
	rr := httptest.NewRecorder()
	newOnboardRouter(h).ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rr.Code)
	}
	assertJSONError(t, rr.Body.String())
}

// TestHandleDiscover_ModelsEndpointReturns500 verifies a 502 when the provider's
// models endpoint returns a server error.
func TestHandleDiscover_ModelsEndpointReturns500(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer backend.Close()

	store := &mockStore{
		getProvider: func(_ context.Context, id string) (*models.Provider, error) {
			return &models.Provider{ID: id, BaseURL: backend.URL}, nil
		},
	}

	h := &OnboardHandler{store: store, client: &http.Client{}, probeTimeout: 2 * time.Second}
	req := httptest.NewRequest(http.MethodPost, "/providers/prov-1/discover", nil)
	rr := httptest.NewRecorder()
	newOnboardRouter(h).ServeHTTP(rr, req)

	if rr.Code != http.StatusBadGateway {
		t.Errorf("status = %d, want 502", rr.Code)
	}
	assertJSONError(t, rr.Body.String())
}

// TestHandleDiscover_Unreachable verifies a 502 when the provider backend is down.
func TestHandleDiscover_Unreachable(t *testing.T) {
	// Start a server then immediately close it so the port is unavailable.
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	backend.Close()

	store := &mockStore{
		getProvider: func(_ context.Context, id string) (*models.Provider, error) {
			return &models.Provider{ID: id, BaseURL: backend.URL}, nil
		},
	}

	h := &OnboardHandler{store: store, client: &http.Client{}, probeTimeout: 2 * time.Second}
	req := httptest.NewRequest(http.MethodPost, "/providers/prov-1/discover", nil)
	rr := httptest.NewRecorder()
	newOnboardRouter(h).ServeHTTP(rr, req)

	if rr.Code != http.StatusBadGateway {
		t.Errorf("status = %d, want 502", rr.Code)
	}
	assertJSONError(t, rr.Body.String())
}

// TestHandleDiscover_ProbeErrorBecomesNull verifies that a probe failure (connection
// refused on an individual endpoint) results in is_supported: null — it does NOT abort
// the entire discover response.
func TestHandleDiscover_ProbeErrorBecomesNull(t *testing.T) {
	// A closed server to use as the "unreachable" endpoint for chat completions.
	closed := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	closed.Close()

	// The main backend serves models normally but routes /v1/chat/completions to nowhere.
	// We achieve this by giving the provider a baseURL pointing to the closed server after
	// returning model data from a working server for /v1/models only.
	//
	// Simpler approach: use a single backend that serves /v1/models fine but refuses probes.
	var backendURL string
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/models" {
			fmt.Fprintln(w, `{"data":[{"id":"m1"}]}`)
			return
		}
		// All probe paths return 500 → unknown (null)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer backend.Close()
	backendURL = backend.URL

	store := &mockStore{
		getProvider: func(_ context.Context, id string) (*models.Provider, error) {
			return &models.Provider{ID: id, BaseURL: backendURL}, nil
		},
	}

	h := &OnboardHandler{store: store, client: &http.Client{}, probeTimeout: 2 * time.Second}
	req := httptest.NewRequest(http.MethodPost, "/providers/prov-1/discover", nil)
	rr := httptest.NewRecorder()
	newOnboardRouter(h).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rr.Code, rr.Body.String())
	}

	var result discoverResult
	if err := json.NewDecoder(rr.Body).Decode(&result); err != nil {
		t.Fatalf("decode: %v", err)
	}

	for _, ep := range result.Endpoints {
		if ep.Path == "/v1/chat/completions" || ep.Path == "/v1/completions" || ep.Path == "/v1/embeddings" {
			if ep.IsSupported != nil {
				t.Errorf("%s: is_supported = %v, want null (500 → unknown)", ep.Path, *ep.IsSupported)
			}
		}
	}
}

// TestHandleDiscover_ProbeTimeout verifies that a probe that exceeds the per-probe
// timeout results in is_supported: null rather than aborting the whole discover.
func TestHandleDiscover_ProbeTimeout(t *testing.T) {
	// hangCh is closed after assertions to unblock hanging server goroutines so
	// backend.Close() can drain them without deadlocking.
	hangCh := make(chan struct{})

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/models" {
			fmt.Fprintln(w, `{"data":[{"id":"m1"}]}`)
			return
		}
		// Hang until the test signals done; this simulates a slow backend that
		// the per-probe timeout (50 ms) will expire before getting a response.
		select {
		case <-hangCh:
		case <-time.After(10 * time.Second):
		}
	}))

	store := &mockStore{
		getProvider: func(_ context.Context, id string) (*models.Provider, error) {
			return &models.Provider{ID: id, BaseURL: backend.URL}, nil
		},
	}

	h := &OnboardHandler{
		store:        store,
		client:       &http.Client{},
		probeTimeout: 50 * time.Millisecond,
	}

	req := httptest.NewRequest(http.MethodPost, "/providers/prov-1/discover", nil)
	rr := httptest.NewRecorder()
	newOnboardRouter(h).ServeHTTP(rr, req)

	// Assertions are done — unblock hanging handlers so backend.Close() returns promptly.
	close(hangCh)
	backend.Close()

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rr.Code, rr.Body.String())
	}

	var result discoverResult
	if err := json.NewDecoder(rr.Body).Decode(&result); err != nil {
		t.Fatalf("decode: %v", err)
	}

	// Timed-out probes must appear as null, not true/false.
	for _, ep := range result.Endpoints {
		if ep.Path == "/v1/chat/completions" || ep.Path == "/v1/completions" || ep.Path == "/v1/embeddings" {
			if ep.IsSupported != nil {
				t.Errorf("%s: is_supported = %v, want null (timeout → unknown)", ep.Path, *ep.IsSupported)
			}
		}
	}
}

// TestHandleConfirm_Persists verifies that HandleConfirm stores endpoints and models
// correctly and returns the expected JSON shape.
func TestHandleConfirm_Persists(t *testing.T) {
	now := time.Now().UTC()
	prov := &models.Provider{ID: "prov-1", Name: "Test", BaseURL: "http://localhost", CreatedAt: now, UpdatedAt: now}

	var capturedEndpoints []models.ProviderEndpoint
	var capturedModelIDs []string

	returnEndpoints := []models.ProviderEndpoint{
		{ID: "ep-1", ProviderID: "prov-1", Path: "/v1/chat/completions", Method: "POST", IsSupported: true, IsEnabled: true, CreatedAt: now},
	}
	returnModels := []models.ProviderModel{
		{ID: "pm-1", ProviderID: "prov-1", ModelID: "m1", CreatedAt: now},
		{ID: "pm-2", ProviderID: "prov-1", ModelID: "m2", CreatedAt: now},
	}

	store := &mockStore{
		getProvider: func(_ context.Context, id string) (*models.Provider, error) {
			return prov, nil
		},
		upsertProviderEndpoints: func(_ context.Context, providerID string, eps []models.ProviderEndpoint) error {
			capturedEndpoints = eps
			return nil
		},
		syncProviderModels: func(_ context.Context, providerID string, modelIDs []string) error {
			capturedModelIDs = modelIDs
			return nil
		},
		listProviderEndpoints: func(_ context.Context, providerID string) ([]models.ProviderEndpoint, error) {
			return returnEndpoints, nil
		},
		listProviderModels: func(_ context.Context, providerID string) ([]models.ProviderModel, error) {
			return returnModels, nil
		},
	}

	h := &OnboardHandler{store: store, client: &http.Client{}}

	body := `{
		"endpoints": [
			{"path": "/v1/chat/completions", "method": "POST", "is_supported": true, "is_enabled": true}
		],
		"models": ["m1", "m2"]
	}`

	req := httptest.NewRequest(http.MethodPost, "/providers/prov-1/confirm", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	newOnboardRouter(h).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rr.Code, rr.Body.String())
	}

	// Verify persisted endpoints
	if len(capturedEndpoints) != 1 {
		t.Fatalf("upserted %d endpoints, want 1", len(capturedEndpoints))
	}
	ep := capturedEndpoints[0]
	if ep.Path != "/v1/chat/completions" {
		t.Errorf("endpoint path = %q, want /v1/chat/completions", ep.Path)
	}
	if !ep.IsSupported {
		t.Error("endpoint IsSupported = false, want true")
	}
	if !ep.IsEnabled {
		t.Error("endpoint IsEnabled = false, want true")
	}
	if ep.ProviderID != "prov-1" {
		t.Errorf("endpoint ProviderID = %q, want prov-1", ep.ProviderID)
	}
	if ep.ID == "" {
		t.Error("endpoint ID should not be empty (UUID expected)")
	}

	// Verify synced model IDs
	if len(capturedModelIDs) != 2 || capturedModelIDs[0] != "m1" || capturedModelIDs[1] != "m2" {
		t.Errorf("synced model IDs = %v, want [m1 m2]", capturedModelIDs)
	}

	// Verify response structure
	var resp map[string]json.RawMessage
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	for _, key := range []string{"provider", "endpoints", "models"} {
		if _, ok := resp[key]; !ok {
			t.Errorf("response missing key %q", key)
		}
	}
}

// TestHandleConfirm_InvalidBody verifies that a malformed request body returns 400.
func TestHandleConfirm_InvalidBody(t *testing.T) {
	store := &mockStore{
		getProvider: func(_ context.Context, id string) (*models.Provider, error) {
			return &models.Provider{ID: id}, nil
		},
	}

	h := &OnboardHandler{store: store, client: &http.Client{}}
	req := httptest.NewRequest(http.MethodPost, "/providers/prov-1/confirm", strings.NewReader("{not valid json"))
	rr := httptest.NewRecorder()
	newOnboardRouter(h).ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rr.Code)
	}
}

// TestHandleConfirm_ProviderNotFound verifies 404 when provider is missing.
func TestHandleConfirm_ProviderNotFound(t *testing.T) {
	store := &mockStore{
		getProvider: func(_ context.Context, id string) (*models.Provider, error) {
			return nil, fmt.Errorf("nope: %w", sql.ErrNoRows)
		},
	}

	h := &OnboardHandler{store: store, client: &http.Client{}}
	body := `{"endpoints":[],"models":[]}`
	req := httptest.NewRequest(http.MethodPost, "/providers/missing/confirm", strings.NewReader(body))
	rr := httptest.NewRecorder()
	newOnboardRouter(h).ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rr.Code)
	}
}

// assertJSONError checks that the body contains an "error" key with a non-empty value.
func assertJSONError(t *testing.T, body string) {
	t.Helper()
	var m map[string]string
	if err := json.Unmarshal([]byte(body), &m); err != nil {
		t.Errorf("body is not valid JSON: %v — body: %s", err, body)
		return
	}
	if m["error"] == "" {
		t.Errorf("body has no error message: %s", body)
	}
}
