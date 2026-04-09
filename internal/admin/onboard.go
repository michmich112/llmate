package admin

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/llmate/gateway/internal/db"
	"github.com/llmate/gateway/internal/models"
)

// OnboardHandler handles provider discovery and confirmation endpoints.
type OnboardHandler struct {
	store  db.Store
	client *http.Client
	// probeTimeout overrides the per-probe context deadline; zero means 10s (production default).
	probeTimeout time.Duration
}

// NewOnboardHandler creates an OnboardHandler. If client is nil a default 30s-timeout
// client is used for the outer operations (model list fetch).
func NewOnboardHandler(store db.Store, client *http.Client) *OnboardHandler {
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	return &OnboardHandler{store: store, client: client}
}

// discoverEndpoint holds the result for a single endpoint probe.
type discoverEndpoint struct {
	Path        string `json:"path"`
	Method      string `json:"method"`
	IsSupported *bool  `json:"is_supported"`
}

// discoverResult is the response body for HandleDiscover.
type discoverResult struct {
	Models    []string           `json:"models"`
	Endpoints []discoverEndpoint `json:"endpoints"`
}

// HandleDiscover probes a provider's backend for supported models and endpoints.
// It does NOT persist anything — results are returned to the UI for confirmation.
func (h *OnboardHandler) HandleDiscover(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	provider, err := h.store.GetProvider(r.Context(), id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			respondError(w, http.StatusNotFound, "provider not found")
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to get provider")
		return
	}

	baseURL := strings.TrimRight(provider.BaseURL, "/")

	modelIDs, err := h.discoverModels(r.Context(), baseURL, provider.APIKey)
	if err != nil {
		respondError(w, http.StatusBadGateway, fmt.Sprintf("failed to reach provider: %s", err.Error()))
		return
	}
	if len(modelIDs) == 0 {
		respondError(w, http.StatusBadGateway, "provider returned no models")
		return
	}

	firstModel := modelIDs[0]

	chatBody, _ := json.Marshal(map[string]interface{}{
		"model":      firstModel,
		"messages":   []map[string]string{{"role": "user", "content": "hi"}},
		"max_tokens": 1,
	})
	completionsBody, _ := json.Marshal(map[string]interface{}{
		"model":      firstModel,
		"prompt":     "hi",
		"max_tokens": 1,
	})
	embeddingsBody, _ := json.Marshal(map[string]interface{}{
		"model": firstModel,
		"input": "test",
	})

	type probeSpec struct {
		path  string
		body  []byte
		skip  bool
	}

	specs := []probeSpec{
		{path: "/v1/chat/completions", body: chatBody},
		{path: "/v1/completions", body: completionsBody},
		{path: "/v1/embeddings", body: embeddingsBody},
		{path: "/v1/images/generations", skip: true},
		{path: "/v1/audio/speech", skip: true},
		{path: "/v1/audio/transcriptions", skip: true},
	}

	endpoints := make([]discoverEndpoint, len(specs))
	for i, spec := range specs {
		endpoints[i] = discoverEndpoint{
			Path:   spec.path,
			Method: "POST",
		}
		if spec.skip {
			// is_supported stays nil → encodes as JSON null
			continue
		}
		supported, _ := h.probeEndpoint(r.Context(), baseURL, provider.APIKey, spec.path, spec.body)
		endpoints[i].IsSupported = supported
	}

	respondJSON(w, http.StatusOK, discoverResult{
		Models:    modelIDs,
		Endpoints: endpoints,
	})
}

// confirmRequest is the JSON body expected by HandleConfirm.
type confirmRequest struct {
	Endpoints []confirmEndpointEntry `json:"endpoints"`
	Models    []string               `json:"models"`
}

type confirmEndpointEntry struct {
	Path        string `json:"path"`
	Method      string `json:"method"`
	IsSupported bool   `json:"is_supported"`
	IsEnabled   bool   `json:"is_enabled"`
}

// HandleConfirm persists the user-confirmed endpoints and models for a provider.
func (h *OnboardHandler) HandleConfirm(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var body confirmRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respondError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	provider, err := h.store.GetProvider(r.Context(), id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			respondError(w, http.StatusNotFound, "provider not found")
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to get provider")
		return
	}

	now := time.Now().UTC()
	eps := make([]models.ProviderEndpoint, len(body.Endpoints))
	for i, e := range body.Endpoints {
		eps[i] = models.ProviderEndpoint{
			ID:          uuid.NewString(),
			ProviderID:  provider.ID,
			Path:        e.Path,
			Method:      e.Method,
			IsSupported: e.IsSupported,
			IsEnabled:   e.IsEnabled,
			CreatedAt:   now,
		}
	}

	if err := h.store.UpsertProviderEndpoints(r.Context(), provider.ID, eps); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to upsert endpoints")
		return
	}

	if err := h.store.SyncProviderModels(r.Context(), provider.ID, body.Models); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to sync models")
		return
	}

	endpoints, err := h.store.ListProviderEndpoints(r.Context(), provider.ID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to list endpoints")
		return
	}
	if endpoints == nil {
		endpoints = []models.ProviderEndpoint{}
	}

	providerModels, err := h.store.ListProviderModels(r.Context(), provider.ID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to list models")
		return
	}
	if providerModels == nil {
		providerModels = []models.ProviderModel{}
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"provider":  provider,
		"endpoints": endpoints,
		"models":    providerModels,
	})
}

// discoverModels fetches model IDs from GET {baseURL}/v1/models.
func (h *OnboardHandler) discoverModels(ctx context.Context, baseURL string, apiKey string) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/v1/models", nil)
	if err != nil {
		return nil, fmt.Errorf("building models request: %w", err)
	}
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	resp, err := h.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching models: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("models endpoint returned status %d", resp.StatusCode)
	}

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading models response: %w", err)
	}

	var modelsResp struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &modelsResp); err != nil {
		return nil, fmt.Errorf("parsing models response: %w", err)
	}
	if len(modelsResp.Data) == 0 {
		return nil, fmt.Errorf("provider returned no models")
	}

	ids := make([]string, len(modelsResp.Data))
	for i, m := range modelsResp.Data {
		ids[i] = m.ID
	}
	return ids, nil
}

// probeEndpoint sends a POST to baseURL+path with probeBody and reports endpoint support.
//
// Return semantics:
//   - (ptr(true), nil)  — 2xx response
//   - (ptr(false), nil) — 404 response
//   - (nil, nil)        — unknown: 5xx, other 4xx (not 404), or non-nil err
//   - (nil, err)        — transport/timeout error; caller should treat as unknown
func (h *OnboardHandler) probeEndpoint(ctx context.Context, baseURL string, apiKey string, path string, probeBody []byte) (supported *bool, err error) {
	timeout := h.probeTimeout
	if timeout == 0 {
		timeout = 10 * time.Second
	}
	probeCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(probeCtx, http.MethodPost, baseURL+path, bytes.NewReader(probeBody))
	if err != nil {
		return nil, fmt.Errorf("building probe request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	resp, err := h.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body) //nolint:errcheck

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		t := true
		return &t, nil
	}
	if resp.StatusCode == http.StatusNotFound {
		f := false
		return &f, nil
	}
	// Unknown: 5xx, other 4xx, etc.
	return nil, nil
}
