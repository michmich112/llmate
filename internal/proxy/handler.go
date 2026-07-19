package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/llmate/gateway/internal/middleware"
	"github.com/llmate/gateway/internal/models"
)

// isClientCanceled reports whether err is a client-side request cancellation.
// These must not trip the provider circuit breaker.
func isClientCanceled(err error) bool {
	return err != nil && errors.Is(err, context.Canceled)
}

const maxProxyAttempts = 3

// Router selects a healthy provider and builds the backend URL for a given model and endpoint path.
type Router interface {
	Route(ctx context.Context, modelID string, endpointPath string) (*RouteResult, error)
	ReportSuccess(providerID string)
	ReportFailure(providerID string)
}

// RouteResult contains the selected provider, resolved model ID, and fully-qualified backend URL.
type RouteResult struct {
	Provider  models.Provider
	ModelID   string // resolved model ID after alias resolution
	TargetURL string // full URL: provider.BaseURL + endpointPath
	// RequestedViaAlias is true when routing used ResolveAlias (the client model name is a gateway alias).
	RequestedViaAlias bool
}

// StreamingLogChunk is one SSE chunk queued for async persistence.
type StreamingLogChunk struct {
	Raw   string
	Delta string
}

// MetricsCollector persists request logs asynchronously and must not block the proxy hot path.
type MetricsCollector interface {
	Record(log *models.RequestLog)
	RecordStreaming(log *models.RequestLog, chunks []StreamingLogChunk, prefixDropped bool)
}

// Handler is the main proxy handler that forwards requests to backend LLM providers.
type Handler struct {
	router  Router
	metrics MetricsCollector
	catalog *RoutingCatalog
	config  *ConfigSnapshot
	client  *http.Client
	logger  *slog.Logger
}

// NewHandler creates a new Handler. If client is nil, a default client with a 5-minute timeout
// is used. Production callers should inject a client with appropriate read/write timeouts.
func NewHandler(router Router, metrics MetricsCollector, catalog *RoutingCatalog, config *ConfigSnapshot, client *http.Client) *Handler {
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Minute}
	}
	return &Handler{
		router: router, metrics: metrics, catalog: catalog, config: config,
		client: client, logger: slog.Default(),
	}
}

// hopByHopHeaders lists headers that must not be forwarded upstream or downstream.
var hopByHopHeaders = map[string]bool{
	"connection":          true,
	"keep-alive":          true,
	"proxy-authenticate":  true,
	"proxy-authorization": true,
	"te":                  true,
	"trailers":            true,
	"transfer-encoding":   true,
	"upgrade":             true,
}

func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func respondError(w http.ResponseWriter, status int, msg string) {
	respondJSON(w, status, map[string]string{"error": msg})
}

func extractModelFromJSON(body []byte) (string, error) {
	var obj struct {
		Model string `json:"model"`
	}
	if err := json.Unmarshal(body, &obj); err != nil {
		return "", fmt.Errorf("parsing request body: %w", err)
	}
	if obj.Model == "" {
		return "", fmt.Errorf("model field is required")
	}
	return obj.Model, nil
}

// extractModelFromShowRequest reads the model from an Ollama /api/show request body.
// Ollama accepts "model" (current) or deprecated "name".
func extractModelFromShowRequest(body []byte) (string, error) {
	var obj struct {
		Model string `json:"model"`
		Name  string `json:"name"`
	}
	if err := json.Unmarshal(body, &obj); err != nil {
		return "", fmt.Errorf("parsing request body: %w", err)
	}
	if obj.Model != "" {
		return obj.Model, nil
	}
	if obj.Name != "" {
		return obj.Name, nil
	}
	return "", fmt.Errorf("model is required")
}

func extractModelFromMultipart(r *http.Request) (string, error) {
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		return "", fmt.Errorf("parsing multipart form: %w", err)
	}
	model := r.FormValue("model")
	if model == "" {
		return "", fmt.Errorf("model field is required")
	}
	return model, nil
}

// injectStreamOptions ensures stream_options.include_usage=true is set in the request body.
// If stream_options already exists as an object, it merges without removing other keys.
func injectStreamOptions(body []byte) ([]byte, error) {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(body, &obj); err != nil {
		return nil, fmt.Errorf("parsing request body: %w", err)
	}
	if existing, ok := obj["stream_options"]; ok {
		var streamOpts map[string]json.RawMessage
		if err := json.Unmarshal(existing, &streamOpts); err == nil {
			streamOpts["include_usage"] = json.RawMessage(`true`)
			merged, err := json.Marshal(streamOpts)
			if err != nil {
				return nil, fmt.Errorf("marshaling stream_options: %w", err)
			}
			obj["stream_options"] = json.RawMessage(merged)
		}
	} else {
		obj["stream_options"] = json.RawMessage(`{"include_usage":true}`)
	}
	out, err := json.Marshal(obj)
	if err != nil {
		return nil, fmt.Errorf("marshaling body: %w", err)
	}
	return out, nil
}

// usageOnly parses usage information from a JSON response body.
type usageOnly struct {
	Usage *struct {
		PromptTokens        int `json:"prompt_tokens"`
		CompletionTokens    int `json:"completion_tokens"`
		TotalTokens         int `json:"total_tokens"`
		PromptTokensDetails *struct {
			CachedTokens int `json:"cached_tokens"`
		} `json:"prompt_tokens_details,omitempty"`
	} `json:"usage"`
}

func parseUsageFromBody(body []byte) *usageOnly {
	var u usageOnly
	if err := json.Unmarshal(body, &u); err != nil {
		return nil
	}
	return &u
}

func copyResponseHeaders(w http.ResponseWriter, resp *http.Response) {
	for key, values := range resp.Header {
		if hopByHopHeaders[strings.ToLower(key)] {
			continue
		}
		for _, v := range values {
			w.Header().Add(key, v)
		}
	}
}

func setBackendAuth(req *http.Request, incomingReq *http.Request, provider models.Provider) {
	if provider.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+provider.APIKey)
	} else if auth := incomingReq.Header.Get("Authorization"); auth != "" {
		req.Header.Set("Authorization", auth)
	}
}

func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.SplitN(xff, ",", 2)
		return strings.TrimSpace(parts[0])
	}
	return r.RemoteAddr
}

// truncate returns s truncated to maxLen runes, appending "…" if cut.
func truncate(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "…"
}

// truncateBodyWithConfig returns body as a string, truncated to maxBytes.
// maxBytes == 0 means no truncation (store entire body).
// Appends "[truncated]" suffix when cut.
func truncateBodyWithConfig(b []byte, maxBytes int) string {
	if maxBytes == 0 || len(b) <= maxBytes {
		return string(b)
	}
	return string(b[:maxBytes]) + "\n[truncated]"
}

// getConfigInt extracts an integer value from config map, returning defaultValue if not found or invalid.
func getConfigInt(config map[string]string, key string, defaultValue int) int {
	if val, ok := config[key]; ok {
		if v, err := strconv.Atoi(val); err == nil {
			return v
		}
	}
	return defaultValue
}

// getConfigBool extracts a boolean value from config map, returning defaultValue if not found.
func getConfigBool(config map[string]string, key string, defaultValue bool) bool {
	if val, ok := config[key]; ok {
		return val == "true"
	}
	return defaultValue
}

// rewriteModelInBody replaces the "model" field in a JSON request body with resolvedModelID.
// Used to ensure the backend receives the real model identifier, not the alias name.
// Returns the original body unchanged on any parse error.
func rewriteModelInBody(body []byte, resolvedModelID string) ([]byte, error) {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(body, &obj); err != nil {
		return body, fmt.Errorf("parsing body for model rewrite: %w", err)
	}
	encoded, err := json.Marshal(resolvedModelID)
	if err != nil {
		return body, fmt.Errorf("encoding resolved model ID: %w", err)
	}
	if _, ok := obj["model"]; ok {
		obj["model"] = json.RawMessage(encoded)
	}
	if _, ok := obj["name"]; ok {
		obj["name"] = json.RawMessage(encoded)
	}
	out, err := json.Marshal(obj)
	if err != nil {
		return body, fmt.Errorf("re-encoding body after model rewrite: %w", err)
	}
	return out, nil
}

// rewriteResponseModelForClient sets top-level JSON "model" to clientModel when the body is a
// JSON object with a "model" key. Used when RouteResult.RequestedViaAlias is true so clients
// see the alias they requested. Returns body unchanged on parse or marshal errors.
func rewriteResponseModelForClient(body []byte, clientModel string) []byte {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(body, &obj); err != nil {
		return body
	}
	if _, ok := obj["model"]; !ok {
		return body
	}
	enc, err := json.Marshal(clientModel)
	if err != nil {
		return body
	}
	obj["model"] = json.RawMessage(enc)
	out, err := json.Marshal(obj)
	if err != nil {
		return body
	}
	return out
}

func applyUsageToLog(log *models.RequestLog, u *usageOnly) {
	if u == nil || u.Usage == nil {
		return
	}
	pt := u.Usage.PromptTokens
	ct := u.Usage.CompletionTokens
	tt := u.Usage.TotalTokens
	log.PromptTokens = &pt
	log.CompletionTokens = &ct
	log.TotalTokens = &tt
	if u.Usage.PromptTokensDetails != nil {
		cached := u.Usage.PromptTokensDetails.CachedTokens
		log.CachedTokens = &cached
	}
}

// proxyNonStreaming executes a non-streaming proxy request with up to maxProxyAttempts failovers.
// Failover occurs on 5xx responses and network/transport errors only.
func (h *Handler) proxyNonStreaming(w http.ResponseWriter, r *http.Request, body []byte, model, endpointPath string, isBinary bool, startTime time.Time) {
	log := &models.RequestLog{
		ID:             uuid.New().String(),
		Timestamp:      startTime.UTC(),
		ClientIP:       clientIP(r),
		Method:         r.Method,
		Path:           r.URL.Path,
		RequestedModel: model,
		CreatedAt:      startTime.UTC(),
	}

	reqID := middleware.GetRequestID(r.Context())
	h.logger.Debug("proxy request",
		"request_id", reqID,
		"model", model,
		"endpoint", endpointPath,
		"body_bytes", len(body),
	)

	for attempt := 1; attempt <= maxProxyAttempts; attempt++ {
		route, err := h.router.Route(r.Context(), model, endpointPath)
		if err != nil {
			h.logger.Warn("route resolution failed",
				"request_id", reqID,
				"attempt", attempt,
				"model", model,
				"error", err,
			)
			if attempt == maxProxyAttempts {
				log.StatusCode = 503
				log.ErrorMessage = fmt.Sprintf("no available provider: %v", err)
				log.TotalTimeMs = int(time.Since(startTime).Milliseconds())
				h.metrics.Record(log)
				respondError(w, 503, fmt.Sprintf("no available provider: %v", err))
				return
			}
			continue
		}

		h.logger.Debug("route selected",
			"request_id", reqID,
			"attempt", attempt,
			"provider", route.Provider.Name,
			"resolved_model", route.ModelID,
			"target_url", route.TargetURL,
		)

		// Rewrite the model field to the resolved backend model ID so the upstream
		// provider receives its own model identifier instead of the gateway alias.
		outBody := body
		if route.ModelID != model {
			if rewritten, rwErr := rewriteModelInBody(body, route.ModelID); rwErr == nil {
				outBody = rewritten
			}
		}

		req, err := http.NewRequestWithContext(r.Context(), http.MethodPost, route.TargetURL, bytes.NewReader(outBody))
		if err != nil {
			if attempt == maxProxyAttempts {
				log.StatusCode = 500
				log.ErrorMessage = fmt.Sprintf("building backend request: %v", err)
				log.TotalTimeMs = int(time.Since(startTime).Milliseconds())
				h.metrics.Record(log)
				respondError(w, 500, "internal error building request")
				return
			}
			continue
		}

		if ct := r.Header.Get("Content-Type"); ct != "" {
			req.Header.Set("Content-Type", ct)
		}
		if accept := r.Header.Get("Accept"); accept != "" {
			req.Header.Set("Accept", accept)
		}
		for key, values := range r.Header {
			if strings.HasPrefix(strings.ToLower(key), "openai-") {
				for _, v := range values {
					req.Header.Add(key, v)
				}
			}
		}
		setBackendAuth(req, r, route.Provider)

		resp, err := h.client.Do(req)
		if err != nil {
			if isClientCanceled(err) {
				log.StatusCode = 499
				log.ErrorMessage = "client canceled request"
				log.TotalTimeMs = int(time.Since(startTime).Milliseconds())
				h.metrics.Record(log)
				return
			}
			h.router.ReportFailure(route.Provider.ID)
			if attempt < maxProxyAttempts {
				continue
			}
			log.StatusCode = 502
			log.ErrorMessage = fmt.Sprintf("backend request failed: %v", err)
			log.TotalTimeMs = int(time.Since(startTime).Milliseconds())
			h.metrics.Record(log)
			respondError(w, 502, "backend request failed")
			return
		}

		respBody, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			if isClientCanceled(readErr) {
				log.StatusCode = 499
				log.ErrorMessage = "client canceled request"
				log.TotalTimeMs = int(time.Since(startTime).Milliseconds())
				h.metrics.Record(log)
				return
			}
			h.router.ReportFailure(route.Provider.ID)
			if attempt < maxProxyAttempts {
				continue
			}
			log.StatusCode = 502
			log.ErrorMessage = "failed to read backend response"
			log.TotalTimeMs = int(time.Since(startTime).Milliseconds())
			h.metrics.Record(log)
			respondError(w, 502, "failed to read backend response")
			return
		}

		respOut := respBody
		if route.RequestedViaAlias && !isBinary {
			respOut = rewriteResponseModelForClient(respBody, model)
		}

		if resp.StatusCode >= 500 {
			h.router.ReportFailure(route.Provider.ID)
			h.logger.Warn("backend 5xx, will failover if attempts remain",
				"request_id", reqID,
				"attempt", attempt,
				"provider", route.Provider.Name,
				"status", resp.StatusCode,
				"body_preview", truncate(string(respBody), 256),
			)
			if attempt < maxProxyAttempts {
				continue
			}
			log.StatusCode = resp.StatusCode
			log.ProviderID = route.Provider.ID
			log.ProviderName = route.Provider.Name
			log.ResolvedModel = route.ModelID
			log.TotalTimeMs = int(time.Since(startTime).Milliseconds())
			copyResponseHeaders(w, resp)
			w.WriteHeader(resp.StatusCode)
			_, _ = w.Write(respOut)
			h.metrics.Record(log)
			return
		}

		// 2xx or 4xx — no further failover
		log.StatusCode = resp.StatusCode
		log.ProviderID = route.Provider.ID
		log.ProviderName = route.Provider.Name
		log.ResolvedModel = route.ModelID
		log.TotalTimeMs = int(time.Since(startTime).Milliseconds())

		// Load logging config (fallback to defaults on error)
		logConfig := h.config.Get()
		reqMax := getConfigInt(logConfig, "request_body_max_bytes", models.DefaultRequestBodyMaxBytes)
		respMax := getConfigInt(logConfig, "response_body_max_bytes", models.DefaultResponseBodyMaxBytes)

		log.RequestBody = truncateBodyWithConfig(body, reqMax)
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			h.router.ReportSuccess(route.Provider.ID)
			if !isBinary {
				applyUsageToLog(log, parseUsageFromBody(respOut))
				log.ResponseBody = truncateBodyWithConfig(respOut, respMax)
			}
			h.logger.Debug("backend success",
				"request_id", reqID,
				"provider", route.Provider.Name,
				"resolved_model", route.ModelID,
				"status", resp.StatusCode,
				"duration_ms", log.TotalTimeMs,
			)
		} else {
			// 4xx from backend: log with body so routing/model issues are visible.
			h.logger.Warn("backend 4xx",
				"request_id", reqID,
				"provider", route.Provider.Name,
				"resolved_model", route.ModelID,
				"target_url", route.TargetURL,
				"status", resp.StatusCode,
				"body_preview", truncate(string(respBody), 512),
			)
		}
		// 4xx: no circuit reporting (client error, not provider fault)
		copyResponseHeaders(w, resp)
		w.WriteHeader(resp.StatusCode)
		_, _ = w.Write(respOut)
		h.metrics.Record(log)
		return
	}

	// Unreachable in normal flow, but guard against logic gaps
	log.StatusCode = 503
	log.ErrorMessage = "all attempts exhausted"
	log.TotalTimeMs = int(time.Since(startTime).Milliseconds())
	h.metrics.Record(log)
	respondError(w, 503, "all attempts exhausted")
}

// HandleChatCompletions handles POST /v1/chat/completions.
func (h *Handler) HandleChatCompletions(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()
	r.Body = http.MaxBytesReader(w, r.Body, 32<<20)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		respondError(w, 400, "failed to read request body")
		return
	}
	model, err := extractModelFromJSON(body)
	if err != nil {
		respondError(w, 400, err.Error())
		return
	}
	var streamCheck struct {
		Stream bool `json:"stream"`
	}
	_ = json.Unmarshal(body, &streamCheck)
	if streamCheck.Stream {
		h.handleStreamingRequest(w, r, body, model, "/v1/chat/completions", startTime)
		return
	}
	h.proxyNonStreaming(w, r, body, model, "/v1/chat/completions", false, startTime)
}

// HandleCompletions handles POST /v1/completions.
func (h *Handler) HandleCompletions(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()
	r.Body = http.MaxBytesReader(w, r.Body, 32<<20)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		respondError(w, 400, "failed to read request body")
		return
	}
	model, err := extractModelFromJSON(body)
	if err != nil {
		respondError(w, 400, err.Error())
		return
	}
	var streamCheck struct {
		Stream bool `json:"stream"`
	}
	_ = json.Unmarshal(body, &streamCheck)
	if streamCheck.Stream {
		h.handleStreamingRequest(w, r, body, model, "/v1/completions", startTime)
		return
	}
	h.proxyNonStreaming(w, r, body, model, "/v1/completions", false, startTime)
}

// HandleShow handles POST /api/show (Ollama-compatible model metadata).
func (h *Handler) HandleShow(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()
	r.Body = http.MaxBytesReader(w, r.Body, 32<<20)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		respondError(w, 400, "failed to read request body")
		return
	}
	model, err := extractModelFromShowRequest(body)
	if err != nil {
		respondError(w, 400, err.Error())
		return
	}
	h.proxyNonStreaming(w, r, body, model, "/api/show", false, startTime)
}

// HandleEmbeddings handles POST /v1/embeddings.
func (h *Handler) HandleEmbeddings(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()
	r.Body = http.MaxBytesReader(w, r.Body, 32<<20)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		respondError(w, 400, "failed to read request body")
		return
	}
	model, err := extractModelFromJSON(body)
	if err != nil {
		respondError(w, 400, err.Error())
		return
	}
	h.proxyNonStreaming(w, r, body, model, "/v1/embeddings", false, startTime)
}

// HandleImageGenerations handles POST /v1/images/generations.
func (h *Handler) HandleImageGenerations(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()
	r.Body = http.MaxBytesReader(w, r.Body, 32<<20)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		respondError(w, 400, "failed to read request body")
		return
	}
	model, err := extractModelFromJSON(body)
	if err != nil {
		respondError(w, 400, err.Error())
		return
	}
	h.proxyNonStreaming(w, r, body, model, "/v1/images/generations", false, startTime)
}

// HandleAudioSpeech handles POST /v1/audio/speech. Response is binary (audio/mpeg etc.).
func (h *Handler) HandleAudioSpeech(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()
	r.Body = http.MaxBytesReader(w, r.Body, 32<<20)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		respondError(w, 400, "failed to read request body")
		return
	}
	model, err := extractModelFromJSON(body)
	if err != nil {
		respondError(w, 400, err.Error())
		return
	}
	h.proxyNonStreaming(w, r, body, model, "/v1/audio/speech", true, startTime)
}

// HandleAudioTranscriptions handles POST /v1/audio/transcriptions (multipart/form-data).
func (h *Handler) HandleAudioTranscriptions(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()

	model, err := extractModelFromMultipart(r)
	if err != nil {
		respondError(w, 400, err.Error())
		return
	}

	// Reconstruct the multipart body so we can send fresh readers on each retry.
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	if r.MultipartForm != nil {
		for key, values := range r.MultipartForm.Value {
			for _, v := range values {
				if wErr := mw.WriteField(key, v); wErr != nil {
					respondError(w, 500, "failed to build multipart body")
					return
				}
			}
		}
		for key, files := range r.MultipartForm.File {
			for _, fh := range files {
				fw, wErr := mw.CreateFormFile(key, fh.Filename)
				if wErr != nil {
					respondError(w, 500, "failed to create form file")
					return
				}
				f, oErr := fh.Open()
				if oErr != nil {
					respondError(w, 500, "failed to open uploaded file")
					return
				}
				_, _ = io.Copy(fw, f)
				f.Close()
			}
		}
	}
	if closeErr := mw.Close(); closeErr != nil {
		respondError(w, 500, "failed to finalize multipart body")
		return
	}
	multipartBody := buf.Bytes()
	contentType := mw.FormDataContentType()

	log := &models.RequestLog{
		ID:             uuid.New().String(),
		Timestamp:      startTime.UTC(),
		ClientIP:       clientIP(r),
		Method:         r.Method,
		Path:           r.URL.Path,
		RequestedModel: model,
		CreatedAt:      startTime.UTC(),
	}

	for attempt := 1; attempt <= maxProxyAttempts; attempt++ {
		route, routeErr := h.router.Route(r.Context(), model, "/v1/audio/transcriptions")
		if routeErr != nil {
			if attempt == maxProxyAttempts {
				log.StatusCode = 503
				log.ErrorMessage = fmt.Sprintf("no available provider: %v", routeErr)
				log.TotalTimeMs = int(time.Since(startTime).Milliseconds())
				h.metrics.Record(log)
				respondError(w, 503, fmt.Sprintf("no available provider: %v", routeErr))
				return
			}
			continue
		}

		req, reqErr := http.NewRequestWithContext(r.Context(), http.MethodPost, route.TargetURL, bytes.NewReader(multipartBody))
		if reqErr != nil {
			if attempt == maxProxyAttempts {
				log.StatusCode = 500
				log.ErrorMessage = fmt.Sprintf("building backend request: %v", reqErr)
				log.TotalTimeMs = int(time.Since(startTime).Milliseconds())
				h.metrics.Record(log)
				respondError(w, 500, "internal error building request")
				return
			}
			continue
		}
		req.Header.Set("Content-Type", contentType)
		setBackendAuth(req, r, route.Provider)

		resp, doErr := h.client.Do(req)
		if doErr != nil {
			if isClientCanceled(doErr) {
				log.StatusCode = 499
				log.ErrorMessage = "client canceled request"
				log.TotalTimeMs = int(time.Since(startTime).Milliseconds())
				h.metrics.Record(log)
				return
			}
			h.router.ReportFailure(route.Provider.ID)
			if attempt < maxProxyAttempts {
				continue
			}
			log.StatusCode = 502
			log.ErrorMessage = fmt.Sprintf("backend request failed: %v", doErr)
			log.TotalTimeMs = int(time.Since(startTime).Milliseconds())
			h.metrics.Record(log)
			respondError(w, 502, "backend request failed")
			return
		}

		respBody, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			if isClientCanceled(readErr) {
				log.StatusCode = 499
				log.ErrorMessage = "client canceled request"
				log.TotalTimeMs = int(time.Since(startTime).Milliseconds())
				h.metrics.Record(log)
				return
			}
			h.router.ReportFailure(route.Provider.ID)
			if attempt < maxProxyAttempts {
				continue
			}
			log.StatusCode = 502
			log.ErrorMessage = "failed to read backend response"
			log.TotalTimeMs = int(time.Since(startTime).Milliseconds())
			h.metrics.Record(log)
			respondError(w, 502, "failed to read backend response")
			return
		}

		respOut := respBody
		if route.RequestedViaAlias {
			respOut = rewriteResponseModelForClient(respBody, model)
		}

		if resp.StatusCode >= 500 {
			h.router.ReportFailure(route.Provider.ID)
			if attempt < maxProxyAttempts {
				continue
			}
			log.StatusCode = resp.StatusCode
			log.ProviderID = route.Provider.ID
			log.ProviderName = route.Provider.Name
			log.ResolvedModel = route.ModelID
			log.TotalTimeMs = int(time.Since(startTime).Milliseconds())
			copyResponseHeaders(w, resp)
			w.WriteHeader(resp.StatusCode)
			_, _ = w.Write(respOut)
			h.metrics.Record(log)
			return
		}

		log.StatusCode = resp.StatusCode
		log.ProviderID = route.Provider.ID
		log.ProviderName = route.Provider.Name
		log.ResolvedModel = route.ModelID
		log.TotalTimeMs = int(time.Since(startTime).Milliseconds())
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			h.router.ReportSuccess(route.Provider.ID)
			applyUsageToLog(log, parseUsageFromBody(respOut))
		}
		copyResponseHeaders(w, resp)
		w.WriteHeader(resp.StatusCode)
		_, _ = w.Write(respOut)
		h.metrics.Record(log)
		return
	}

	log.StatusCode = 503
	log.ErrorMessage = "all attempts exhausted"
	log.TotalTimeMs = int(time.Since(startTime).Milliseconds())
	h.metrics.Record(log)
	respondError(w, 503, "all attempts exhausted")
}

// modelObject represents a single entry in the OpenAI-compatible /v1/models response.
type modelObject struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int    `json:"created"`
	OwnedBy string `json:"owned_by"`
}

// listModelObjects builds a deduplicated, sorted list of model objects from the store.
// It includes all provider model IDs and all enabled alias names.
func (h *Handler) listModelObjects() []modelObject {
	ids := h.catalog.PublicModelIDs()
	result := make([]modelObject, 0, len(ids))
	for _, id := range ids {
		result = append(result, modelObject{ID: id, Object: "model", Created: 0, OwnedBy: "llmate"})
	}
	return result
}

// HandleListModels handles GET /v1/models.
func (h *Handler) HandleListModels(w http.ResponseWriter, r *http.Request) {
	objects := h.listModelObjects()
	respondJSON(w, 200, map[string]interface{}{
		"object": "list",
		"data":   objects,
	})
}

// HandleGetModel handles GET /v1/models/{model}.
func (h *Handler) HandleGetModel(w http.ResponseWriter, r *http.Request) {
	modelID := chi.URLParam(r, "model")
	if modelID == "" {
		respondError(w, 400, "model parameter required")
		return
	}
	decoded, err := url.PathUnescape(modelID)
	if err != nil {
		decoded = modelID
	}

	objects := h.listModelObjects()
	for _, obj := range objects {
		if obj.ID == decoded {
			respondJSON(w, 200, obj)
			return
		}
	}
	respondError(w, 404, fmt.Sprintf("model %q not found", decoded))
}

// handleStreamingRequest handles streaming chat/completions with up to maxProxyAttempts
// before the first byte is written to the client. Once streaming starts, no failover.
func (h *Handler) handleStreamingRequest(w http.ResponseWriter, r *http.Request, body []byte, model, endpointPath string, startTime time.Time) {
	modifiedBody, err := injectStreamOptions(body)
	if err != nil {
		respondError(w, 400, fmt.Sprintf("failed to inject stream options: %v", err))
		return
	}

	logConfig := h.config.Get()
	reqMax := getConfigInt(logConfig, "request_body_max_bytes", models.DefaultRequestBodyMaxBytes)

	log := &models.RequestLog{
		ID:             uuid.New().String(),
		Timestamp:      startTime.UTC(),
		ClientIP:       clientIP(r),
		Method:         r.Method,
		Path:           r.URL.Path,
		RequestedModel: model,
		IsStreamed:     true,
		CreatedAt:      startTime.UTC(),
		RequestBody:    truncateBodyWithConfig(body, reqMax),
	}

	var resp *http.Response
	var route *RouteResult

	reqID := middleware.GetRequestID(r.Context())
	h.logger.Debug("proxy streaming request",
		"request_id", reqID,
		"model", model,
		"endpoint", endpointPath,
		"body_bytes", len(body),
	)

	for attempt := 1; attempt <= maxProxyAttempts; attempt++ {
		route, err = h.router.Route(r.Context(), model, endpointPath)
		if err != nil {
			h.logger.Warn("route resolution failed (streaming)",
				"request_id", reqID,
				"attempt", attempt,
				"model", model,
				"error", err,
			)
			if attempt == maxProxyAttempts {
				log.StatusCode = 503
				log.ErrorMessage = fmt.Sprintf("no available provider: %v", err)
				log.TotalTimeMs = int(time.Since(startTime).Milliseconds())
				h.metrics.Record(log)
				respondError(w, 503, fmt.Sprintf("no available provider: %v", err))
				return
			}
			continue
		}

		h.logger.Debug("route selected (streaming)",
			"request_id", reqID,
			"attempt", attempt,
			"provider", route.Provider.Name,
			"resolved_model", route.ModelID,
			"target_url", route.TargetURL,
		)

		// Rewrite model in the (already stream-options-injected) body.
		streamBody := modifiedBody
		if route.ModelID != model {
			if rewritten, rwErr := rewriteModelInBody(modifiedBody, route.ModelID); rwErr == nil {
				streamBody = rewritten
			}
		}

		req, reqErr := http.NewRequestWithContext(r.Context(), http.MethodPost, route.TargetURL, bytes.NewReader(streamBody))
		if reqErr != nil {
			if attempt == maxProxyAttempts {
				log.StatusCode = 500
				log.ErrorMessage = fmt.Sprintf("building backend request: %v", reqErr)
				log.TotalTimeMs = int(time.Since(startTime).Milliseconds())
				h.metrics.Record(log)
				respondError(w, 500, "internal error building request")
				return
			}
			continue
		}

		req.Header.Set("Content-Type", "application/json")
		if accept := r.Header.Get("Accept"); accept != "" {
			req.Header.Set("Accept", accept)
		} else {
			req.Header.Set("Accept", "text/event-stream")
		}
		for key, values := range r.Header {
			if strings.HasPrefix(strings.ToLower(key), "openai-") {
				for _, v := range values {
					req.Header.Add(key, v)
				}
			}
		}
		setBackendAuth(req, r, route.Provider)

		resp, err = h.client.Do(req)
		if err != nil {
			if isClientCanceled(err) {
				log.StatusCode = 499
				log.ErrorMessage = "client canceled request"
				log.TotalTimeMs = int(time.Since(startTime).Milliseconds())
				h.metrics.Record(log)
				return
			}
			h.router.ReportFailure(route.Provider.ID)
			if attempt < maxProxyAttempts {
				continue
			}
			log.StatusCode = 502
			log.ErrorMessage = fmt.Sprintf("backend request failed: %v", err)
			log.TotalTimeMs = int(time.Since(startTime).Milliseconds())
			h.metrics.Record(log)
			respondError(w, 502, "backend request failed")
			return
		}

		if resp.StatusCode >= 500 {
			statusCode := resp.StatusCode
			body5xx, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			resp = nil
			h.router.ReportFailure(route.Provider.ID)
			h.logger.Warn("backend 5xx on streaming attempt",
				"request_id", reqID,
				"attempt", attempt,
				"provider", route.Provider.Name,
				"status", statusCode,
				"body_preview", truncate(string(body5xx), 256),
			)
			if attempt < maxProxyAttempts {
				continue
			}
			log.StatusCode = 503
			log.ProviderID = route.Provider.ID
			log.ProviderName = route.Provider.Name
			log.TotalTimeMs = int(time.Since(startTime).Milliseconds())
			h.metrics.Record(log)
			respondError(w, 503, "backend error")
			return
		}

		// Valid response — begin streaming
		break
	}

	if resp == nil {
		log.StatusCode = 503
		log.ErrorMessage = "no backend response available"
		log.TotalTimeMs = int(time.Since(startTime).Milliseconds())
		h.metrics.Record(log)
		respondError(w, 503, "no backend response available")
		return
	}
	defer resp.Body.Close()

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}

	usage, ttftMs, reconstructedBody, streamChunks, streamChunksPrefixDropped, streamErr := h.proxyStreaming(w, resp, startTime, logConfig, model, route.RequestedViaAlias)

	log.ProviderID = route.Provider.ID
	log.ProviderName = route.Provider.Name
	log.ResolvedModel = route.ModelID
	log.TTFTMs = ttftMs
	log.TotalTimeMs = int(time.Since(startTime).Milliseconds())

	if streamErr != nil {
		if !isClientCanceled(streamErr) {
			h.router.ReportFailure(route.Provider.ID)
		}
		log.StatusCode = http.StatusOK // headers already sent
		log.ErrorMessage = fmt.Sprintf("stream error: %v", streamErr)
	} else {
		h.router.ReportSuccess(route.Provider.ID)
		log.StatusCode = resp.StatusCode
	}

	if usage != nil {
		pt := usage.PromptTokens
		ct := usage.CompletionTokens
		tt := usage.TotalTokens
		log.PromptTokens = &pt
		log.CompletionTokens = &ct
		log.TotalTokens = &tt
		if usage.CachedTokensReported {
			cached := usage.CachedTokens
			log.CachedTokens = &cached
		}
	}

	// After usage is applied to log, add reconstructed response body:
	respMax := getConfigInt(logConfig, "response_body_max_bytes", models.DefaultResponseBodyMaxBytes)
	if reconstructedBody != "" {
		log.ResponseBody = truncateBodyWithConfig([]byte(reconstructedBody), respMax)
	}

	// streaming_logs FK request_logs(id). The metrics worker may persist the log after this
	// handler returns, so we must insert the request log before async chunk writes.
	if len(streamChunks) > 0 {
		chunks := make([]StreamingLogChunk, len(streamChunks))
		for i, ch := range streamChunks {
			chunks[i] = StreamingLogChunk{Raw: ch.raw, Delta: ch.delta}
		}
		h.metrics.RecordStreaming(log, chunks, streamChunksPrefixDropped)
	} else {
		h.metrics.Record(log)
	}
}
