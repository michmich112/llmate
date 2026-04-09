package admin

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/llmate/gateway/internal/db"
	"github.com/llmate/gateway/internal/models"
	"github.com/llmate/gateway/internal/pricing"
)

// Handler holds admin API dependencies. The caller must not pass a nil store.
type Handler struct {
	store         db.Store
	configHandler *ConfigHandler
}

// NewHandler creates a new admin Handler backed by the given store.
func NewHandler(store db.Store) *Handler {
	return &Handler{
		store:         store,
		configHandler: NewConfigHandler(store),
	}
}

// Routes returns a chi.Router pre-configured with all admin routes.
// Phase 2 mounts this at /admin with ACCESS_KEY middleware.
// POST /providers/{id}/discover and POST /providers/{id}/confirm are intentionally
// absent here — they belong to Phase 1E (onboard.go).
func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()

	r.Post("/auth", h.HandleAuth)

	r.Get("/providers", h.HandleListProviders)
	r.Post("/providers", h.HandleCreateProvider)
	r.Get("/providers/{id}", h.HandleGetProvider)
	r.Put("/providers/{id}", h.HandleUpdateProvider)
	r.Delete("/providers/{id}", h.HandleDeleteProvider)
	r.Put("/providers/{id}/endpoints/{eid}", h.HandleUpdateEndpoint)

	r.Get("/aliases", h.HandleListAliases)
	r.Post("/aliases", h.HandleCreateAlias)
	r.Put("/aliases/{id}", h.HandleUpdateAlias)
	r.Delete("/aliases/{id}", h.HandleDeleteAlias)

	r.Get("/logs", h.HandleQueryLogs)
	r.Get("/logs/{id}", h.HandleGetLog)
	r.Get("/logs/{id}/streaming", h.HandleGetStreamingLogs)

	r.Put("/providers/{id}/models/{mid}", h.HandleUpdateProviderModel)

	r.Get("/config", h.configHandler.HandleGetConfig)
	r.Put("/config", h.configHandler.HandleUpdateConfig)
	r.Get("/config/definition", h.configHandler.HandleConfigDefinition)

	r.Get("/stats", h.HandleGetStats)
	r.Get("/stats/timeseries", h.HandleGetTimeSeries)

	return r
}

// respondJSON encodes data to a buffer first so a failed encode returns 500 rather
// than a partial response with an already-sent 2xx status line.
func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(data); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to encode response")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write(buf.Bytes()) //nolint:errcheck
}

// respondError writes a {"error":"<msg>"} JSON body with the given status code.
func respondError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": msg}) //nolint:errcheck
}

// parseDurationParam parses a dashboard window duration string.
// Supports standard Go durations (e.g. "24h", "1h30m") and a day shorthand
// with suffix "d" (e.g. "7d" → 168h, "30d" → 720h).
// Returns an error for empty input so callers can apply their own defaults.
func parseDurationParam(s string) (time.Duration, error) {
	if s == "" {
		return 0, fmt.Errorf("empty duration string")
	}
	if strings.HasSuffix(s, "d") {
		days, err := strconv.Atoi(s[:len(s)-1])
		if err != nil {
			return 0, fmt.Errorf("invalid day duration %q: %w", s, err)
		}
		return time.Duration(days) * 24 * time.Hour, nil
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, fmt.Errorf("invalid duration %q: %w", s, err)
	}
	return d, nil
}

// HandleAuth responds with {"valid":true}. Reaching this handler implies the
// ACCESS_KEY middleware has already authenticated the request.
func (h *Handler) HandleAuth(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, map[string]bool{"valid": true})
}

// providerListItem wraps a Provider with the list of model IDs registered for it.
type providerListItem struct {
	models.Provider
	Models []string `json:"models"`
}

// HandleListProviders returns all providers, each enriched with their registered model IDs.
func (h *Handler) HandleListProviders(w http.ResponseWriter, r *http.Request) {
	providers, err := h.store.ListProviders(r.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to list providers")
		return
	}

	allModels, err := h.store.ListAllModels(r.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to list models")
		return
	}

	// Group model IDs by provider in one pass to avoid N+1 queries.
	modelsByProvider := make(map[string][]string, len(providers))
	for _, m := range allModels {
		modelsByProvider[m.ProviderID] = append(modelsByProvider[m.ProviderID], m.ModelID)
	}

	items := make([]providerListItem, len(providers))
	for i, p := range providers {
		ms := modelsByProvider[p.ID]
		if ms == nil {
			ms = []string{}
		}
		items[i] = providerListItem{Provider: p, Models: ms}
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{"providers": items})
}

// HandleCreateProvider creates a new provider from the JSON body.
func (h *Handler) HandleCreateProvider(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name    string `json:"name"`
		BaseURL string `json:"base_url"`
		APIKey  string `json:"api_key"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respondError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if strings.TrimSpace(body.Name) == "" {
		respondError(w, http.StatusBadRequest, "name is required")
		return
	}
	if strings.TrimSpace(body.BaseURL) == "" {
		respondError(w, http.StatusBadRequest, "base_url is required")
		return
	}

	now := time.Now().UTC()
	p := models.Provider{
		ID:        uuid.NewString(),
		Name:      body.Name,
		BaseURL:   body.BaseURL,
		APIKey:    body.APIKey,
		IsHealthy: false,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := h.store.CreateProvider(r.Context(), &p); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to create provider")
		return
	}
	respondJSON(w, http.StatusCreated, map[string]interface{}{"provider": p})
}

// HandleGetProvider returns a provider along with its endpoints and models.
func (h *Handler) HandleGetProvider(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		respondError(w, http.StatusBadRequest, "id is required")
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

	endpoints, err := h.store.ListProviderEndpoints(r.Context(), id)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to list endpoints")
		return
	}
	if endpoints == nil {
		endpoints = []models.ProviderEndpoint{}
	}

	providerModels, err := h.store.ListProviderModels(r.Context(), id)
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

// HandleUpdateProvider replaces mutable fields on an existing provider.
func (h *Handler) HandleUpdateProvider(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		respondError(w, http.StatusBadRequest, "id is required")
		return
	}

	var body struct {
		Name    string `json:"name"`
		BaseURL string `json:"base_url"`
		APIKey  string `json:"api_key"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respondError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if strings.TrimSpace(body.Name) == "" {
		respondError(w, http.StatusBadRequest, "name is required")
		return
	}
	if strings.TrimSpace(body.BaseURL) == "" {
		respondError(w, http.StatusBadRequest, "base_url is required")
		return
	}

	existing, err := h.store.GetProvider(r.Context(), id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			respondError(w, http.StatusNotFound, "provider not found")
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to get provider")
		return
	}

	merged := *existing
	merged.Name = body.Name
	merged.BaseURL = body.BaseURL
	merged.APIKey = body.APIKey
	merged.UpdatedAt = time.Now().UTC()

	if err := h.store.UpdateProvider(r.Context(), &merged); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to update provider")
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"provider": merged})
}

// HandleDeleteProvider removes a provider and cascades to its related records.
func (h *Handler) HandleDeleteProvider(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		respondError(w, http.StatusBadRequest, "id is required")
		return
	}

	if err := h.store.DeleteProvider(r.Context(), id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			respondError(w, http.StatusNotFound, "provider not found")
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to delete provider")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// HandleUpdateEndpoint toggles is_enabled on a single provider endpoint.
func (h *Handler) HandleUpdateEndpoint(w http.ResponseWriter, r *http.Request) {
	providerID := chi.URLParam(r, "id")
	endpointID := chi.URLParam(r, "eid")
	if providerID == "" || endpointID == "" {
		respondError(w, http.StatusBadRequest, "provider id and endpoint id are required")
		return
	}

	var body struct {
		IsEnabled *bool `json:"is_enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respondError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if body.IsEnabled == nil {
		respondError(w, http.StatusBadRequest, "is_enabled is required")
		return
	}

	endpoints, err := h.store.ListProviderEndpoints(r.Context(), providerID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to list endpoints")
		return
	}

	var target *models.ProviderEndpoint
	for i := range endpoints {
		if endpoints[i].ID == endpointID {
			target = &endpoints[i]
			break
		}
	}
	if target == nil {
		respondError(w, http.StatusNotFound, "endpoint not found")
		return
	}

	target.IsEnabled = *body.IsEnabled
	if err := h.store.UpdateProviderEndpoint(r.Context(), target); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to update endpoint")
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"endpoint": target})
}

// HandleListAliases returns all model aliases.
func (h *Handler) HandleListAliases(w http.ResponseWriter, r *http.Request) {
	aliases, err := h.store.ListAliases(r.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to list aliases")
		return
	}
	if aliases == nil {
		aliases = []models.ModelAlias{}
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"aliases": aliases})
}

// HandleCreateAlias creates a new model alias. Performs a pre-check that the
// referenced provider exists, returning 400 for an unknown provider.
func (h *Handler) HandleCreateAlias(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Alias      string `json:"alias"`
		ProviderID string `json:"provider_id"`
		ModelID    string `json:"model_id"`
		Weight     *int   `json:"weight"`
		Priority   *int   `json:"priority"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respondError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if strings.TrimSpace(body.Alias) == "" {
		respondError(w, http.StatusBadRequest, "alias is required")
		return
	}
	if strings.TrimSpace(body.ProviderID) == "" {
		respondError(w, http.StatusBadRequest, "provider_id is required")
		return
	}
	if strings.TrimSpace(body.ModelID) == "" {
		respondError(w, http.StatusBadRequest, "model_id is required")
		return
	}

	if _, err := h.store.GetProvider(r.Context(), body.ProviderID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			respondError(w, http.StatusBadRequest, "unknown provider")
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to verify provider")
		return
	}

	weight := 1
	if body.Weight != nil {
		weight = *body.Weight
	}
	priority := 0
	if body.Priority != nil {
		priority = *body.Priority
	}

	now := time.Now().UTC()
	a := models.ModelAlias{
		ID:         uuid.NewString(),
		Alias:      body.Alias,
		ProviderID: body.ProviderID,
		ModelID:    body.ModelID,
		Weight:     weight,
		Priority:   priority,
		IsEnabled:  true,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	if err := h.store.CreateAlias(r.Context(), &a); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to create alias")
		return
	}
	respondJSON(w, http.StatusCreated, map[string]interface{}{"alias": a})
}

// HandleUpdateAlias updates weight, priority, and/or is_enabled on an alias.
// Uses ListAliases + linear search to avoid adding a GetAlias store method (O(n), v1 acceptable).
func (h *Handler) HandleUpdateAlias(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		respondError(w, http.StatusBadRequest, "id is required")
		return
	}

	var body struct {
		Weight    *int  `json:"weight"`
		Priority  *int  `json:"priority"`
		IsEnabled *bool `json:"is_enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respondError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	aliases, err := h.store.ListAliases(r.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to list aliases")
		return
	}

	var target *models.ModelAlias
	for i := range aliases {
		if aliases[i].ID == id {
			target = &aliases[i]
			break
		}
	}
	if target == nil {
		respondError(w, http.StatusNotFound, "alias not found")
		return
	}

	if body.Weight != nil {
		target.Weight = *body.Weight
	}
	if body.Priority != nil {
		target.Priority = *body.Priority
	}
	if body.IsEnabled != nil {
		target.IsEnabled = *body.IsEnabled
	}
	target.UpdatedAt = time.Now().UTC()

	if err := h.store.UpdateAlias(r.Context(), target); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to update alias")
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"alias": target})
}

// HandleDeleteAlias removes an alias by ID.
func (h *Handler) HandleDeleteAlias(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		respondError(w, http.StatusBadRequest, "id is required")
		return
	}

	if err := h.store.DeleteAlias(r.Context(), id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			respondError(w, http.StatusNotFound, "alias not found")
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to delete alias")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// HandleQueryLogs returns paginated, filtered request logs.
func (h *Handler) HandleQueryLogs(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	filter := models.LogFilter{
		Model:      q.Get("model"),
		ProviderID: q.Get("provider_id"),
		Limit:      50,
		Offset:     0,
	}

	if s := q.Get("since"); s != "" {
		t, err := time.Parse(time.RFC3339, s)
		if err != nil {
			respondError(w, http.StatusBadRequest, "invalid since: must be RFC3339")
			return
		}
		filter.Since = &t
	}

	if s := q.Get("until"); s != "" {
		t, err := time.Parse(time.RFC3339, s)
		if err != nil {
			respondError(w, http.StatusBadRequest, "invalid until: must be RFC3339")
			return
		}
		filter.Until = &t
	}

	if s := q.Get("limit"); s != "" {
		v, err := strconv.Atoi(s)
		if err != nil || v < 1 {
			respondError(w, http.StatusBadRequest, "invalid limit")
			return
		}
		if v > 1000 {
			v = 1000
		}
		filter.Limit = v
	}

	if s := q.Get("offset"); s != "" {
		v, err := strconv.Atoi(s)
		if err != nil || v < 0 {
			respondError(w, http.StatusBadRequest, "invalid offset")
			return
		}
		filter.Offset = v
	}

	// status param: "2xx" | "4xx" | "5xx" | "error" (4xx+5xx) | "success" (2xx+3xx)
	switch q.Get("status") {
	case "2xx", "success":
		filter.StatusMin, filter.StatusMax = 200, 299
	case "3xx":
		filter.StatusMin, filter.StatusMax = 300, 399
	case "4xx":
		filter.StatusMin, filter.StatusMax = 400, 499
	case "5xx":
		filter.StatusMin, filter.StatusMax = 500, 599
	case "error":
		filter.StatusMin = 400 // no upper bound — catches all errors
	}

	logs, total, err := h.store.QueryRequestLogs(r.Context(), filter)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to query logs")
		return
	}
	if logs == nil {
		logs = []models.RequestLog{}
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"logs": logs, "total": total})
}

// HandleGetLog returns a single request log by ID including request/response bodies.
func (h *Handler) HandleGetLog(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		respondError(w, http.StatusBadRequest, "id is required")
		return
	}
	log, err := h.store.GetRequestLog(r.Context(), id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			respondError(w, http.StatusNotFound, "log not found")
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to get log")
		return
	}
	if log.ProviderID != "" && log.ResolvedModel != "" {
		if pm, perr := h.store.GetProviderModelCosts(r.Context(), log.ProviderID, log.ResolvedModel); perr == nil && pm != nil {
			b := pricing.ForRequestLog(log, pm)
			log.CostBreakdown = &models.RequestLogCostBreakdown{
				InputUSD:      b.InputUSD,
				OutputUSD:     b.OutputUSD,
				CachedReadUSD: b.CachedReadUSD,
				TotalUSD:      b.TotalUSD,
			}
		}
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"log": log})
}

// HandleUpdateProviderModel updates the cost fields for a provider_models record.
func (h *Handler) HandleUpdateProviderModel(w http.ResponseWriter, r *http.Request) {
	providerID := chi.URLParam(r, "id")
	modelRecordID := chi.URLParam(r, "mid")
	if providerID == "" || modelRecordID == "" {
		respondError(w, http.StatusBadRequest, "provider id and model id are required")
		return
	}

	var body struct {
		CostPerMillionInput      *float64 `json:"cost_per_million_input"`
		CostPerMillionOutput     *float64 `json:"cost_per_million_output"`
		CostPerMillionCacheRead  *float64 `json:"cost_per_million_cache_read"`
		CostPerMillionCacheWrite *float64 `json:"cost_per_million_cache_write"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respondError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	m := &models.ProviderModel{
		CostPerMillionInput:      body.CostPerMillionInput,
		CostPerMillionOutput:     body.CostPerMillionOutput,
		CostPerMillionCacheRead:  body.CostPerMillionCacheRead,
		CostPerMillionCacheWrite: body.CostPerMillionCacheWrite,
	}
	if err := h.store.UpdateProviderModelCosts(r.Context(), modelRecordID, m); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			respondError(w, http.StatusNotFound, "model not found")
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to update model costs")
		return
	}

	// Return updated provider model list so the frontend can refresh in one round trip.
	providerModels, err := h.store.ListProviderModels(r.Context(), providerID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to list models")
		return
	}
	if providerModels == nil {
		providerModels = []models.ProviderModel{}
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"models": providerModels})
}

func (h *Handler) HandleGetStreamingLogs(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		respondError(w, http.StatusBadRequest, "id is required")
		return
	}

	logs, err := h.store.GetStreamingLogs(r.Context(), id)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to get streaming logs")
		return
	}
	if logs == nil {
		logs = []models.StreamingLog{}
	}
	var acc strings.Builder
	for i := range logs {
		acc.WriteString(logs[i].ContentDelta)
		logs[i].CumulativeBody = acc.String()
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"streaming_logs": logs})
}
