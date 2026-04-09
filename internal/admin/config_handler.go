package admin

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/llmate/gateway/internal/db"
	"github.com/llmate/gateway/internal/models"
	"github.com/llmate/gateway/internal/logretention"
)

type ConfigHandler struct {
	store db.Store
}

func NewConfigHandler(store db.Store) *ConfigHandler {
	return &ConfigHandler{store: store}
}

func (h *ConfigHandler) HandleGetConfig(w http.ResponseWriter, r *http.Request) {
	config, err := h.store.GetAllConfig(r.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to load configuration")
		return
	}

	result := models.DefaultConfiguration()
	if val, ok := config["request_body_max_bytes"]; ok {
		if v, err := strconv.Atoi(val); err == nil {
			result.RequestBodyMaxBytes = v
		}
	}
	if val, ok := config["response_body_max_bytes"]; ok {
		if v, err := strconv.Atoi(val); err == nil {
			result.ResponseBodyMaxBytes = v
		}
	}
	if val, ok := config["track_streaming"]; ok {
		result.TrackStreaming = val == "true"
	}
	if val, ok := config["streaming_buffer_size"]; ok {
		if v, err := strconv.Atoi(val); err == nil {
			result.StreamingBufferSize = v
		}
	}
	if val, ok := config["streaming_log_body_retention_days"]; ok {
		if v, err := strconv.Atoi(val); err == nil {
			result.StreamingLogBodyRetentionDays = v
		}
	}
	if val, ok := config["request_log_body_retention_days"]; ok {
		if v, err := strconv.Atoi(val); err == nil {
			result.RequestLogBodyRetentionDays = v
		}
	}
	if val, ok := config["response_log_body_retention_days"]; ok {
		if v, err := strconv.Atoi(val); err == nil {
			result.ResponseLogBodyRetentionDays = v
		}
	}

	respondJSON(w, http.StatusOK, result)
}

func (h *ConfigHandler) HandleUpdateConfig(w http.ResponseWriter, r *http.Request) {
	var updates map[string]json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		respondError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	config := make(map[string]string)
	for key, raw := range updates {
		switch key {
		case "request_body_max_bytes":
			var v int
			if err := json.Unmarshal(raw, &v); err != nil {
				respondError(w, http.StatusBadRequest, "request_body_max_bytes must be an integer")
				return
			}
			if v < 0 || v > models.MaxBodyMaxBytes {
				respondError(w, http.StatusBadRequest, fmt.Sprintf("request_body_max_bytes must be between 0 and %d", models.MaxBodyMaxBytes))
				return
			}
			config[key] = strconv.Itoa(v)

		case "response_body_max_bytes":
			var v int
			if err := json.Unmarshal(raw, &v); err != nil {
				respondError(w, http.StatusBadRequest, "response_body_max_bytes must be an integer")
				return
			}
			if v < 0 || v > models.MaxBodyMaxBytes {
				respondError(w, http.StatusBadRequest, fmt.Sprintf("response_body_max_bytes must be between 0 and %d", models.MaxBodyMaxBytes))
				return
			}
			config[key] = strconv.Itoa(v)

		case "track_streaming":
			var v bool
			if err := json.Unmarshal(raw, &v); err != nil {
				respondError(w, http.StatusBadRequest, "track_streaming must be a boolean")
				return
			}
			config[key] = strconv.FormatBool(v)

		case "streaming_buffer_size":
			var v int
			if err := json.Unmarshal(raw, &v); err != nil {
				respondError(w, http.StatusBadRequest, "streaming_buffer_size must be an integer")
				return
			}
			if v < models.MinStreamingBufferSize || v > models.MaxStreamingBufferSize {
				respondError(w, http.StatusBadRequest, fmt.Sprintf("streaming_buffer_size must be between %d and %d", models.MinStreamingBufferSize, models.MaxStreamingBufferSize))
				return
			}
			config[key] = strconv.Itoa(v)

		case "streaming_log_body_retention_days":
			var v int
			if err := json.Unmarshal(raw, &v); err != nil {
				respondError(w, http.StatusBadRequest, "streaming_log_body_retention_days must be an integer")
				return
			}
			if v < models.MinStreamingLogBodyRetentionDays || v > models.MaxStreamingLogBodyRetentionDays {
				respondError(w, http.StatusBadRequest, fmt.Sprintf("streaming_log_body_retention_days must be between %d and %d", models.MinStreamingLogBodyRetentionDays, models.MaxStreamingLogBodyRetentionDays))
				return
			}
			config[key] = strconv.Itoa(v)

		case "request_log_body_retention_days":
			var v int
			if err := json.Unmarshal(raw, &v); err != nil {
				respondError(w, http.StatusBadRequest, "request_log_body_retention_days must be an integer")
				return
			}
			if v < models.MinStreamingLogBodyRetentionDays || v > models.MaxStreamingLogBodyRetentionDays {
				respondError(w, http.StatusBadRequest, fmt.Sprintf("request_log_body_retention_days must be between %d and %d", models.MinStreamingLogBodyRetentionDays, models.MaxStreamingLogBodyRetentionDays))
				return
			}
			config[key] = strconv.Itoa(v)

		case "response_log_body_retention_days":
			var v int
			if err := json.Unmarshal(raw, &v); err != nil {
				respondError(w, http.StatusBadRequest, "response_log_body_retention_days must be an integer")
				return
			}
			if v < models.MinStreamingLogBodyRetentionDays || v > models.MaxStreamingLogBodyRetentionDays {
				respondError(w, http.StatusBadRequest, fmt.Sprintf("response_log_body_retention_days must be between %d and %d", models.MinStreamingLogBodyRetentionDays, models.MaxStreamingLogBodyRetentionDays))
				return
			}
			config[key] = strconv.Itoa(v)

		default:
			respondError(w, http.StatusBadRequest, fmt.Sprintf("unknown config key: %s", key))
			return
		}
	}

	for k, v := range config {
		if err := h.store.SetConfig(r.Context(), k, v); err != nil {
			respondError(w, http.StatusInternalServerError, "failed to save configuration")
			return
		}
	}

	purgeCtx, purgeCancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer purgeCancel()

	if _, had := config["streaming_log_body_retention_days"]; had {
		days, err := strconv.Atoi(config["streaming_log_body_retention_days"])
		if err != nil {
			respondError(w, http.StatusInternalServerError, "invalid streaming_log_body_retention_days after save")
			return
		}
		n, err := logretention.PurgeStreamingChunkBodies(purgeCtx, h.store, days)
		if err != nil {
			slog.Default().Warn("log retention: streaming chunk purge after config update failed", "error", err)
		} else if n > 0 {
			slog.Default().Info("log retention: streaming chunk purge after config update", "rows", n, "retention_days", days)
		}
	}
	if _, had := config["request_log_body_retention_days"]; had {
		days, err := strconv.Atoi(config["request_log_body_retention_days"])
		if err != nil {
			respondError(w, http.StatusInternalServerError, "invalid request_log_body_retention_days after save")
			return
		}
		n, err := logretention.PurgeRequestLogRequestBodies(purgeCtx, h.store, days)
		if err != nil {
			slog.Default().Warn("log retention: request body purge after config update failed", "error", err)
		} else if n > 0 {
			slog.Default().Info("log retention: request body purge after config update", "rows", n, "retention_days", days)
		}
	}
	if _, had := config["response_log_body_retention_days"]; had {
		days, err := strconv.Atoi(config["response_log_body_retention_days"])
		if err != nil {
			respondError(w, http.StatusInternalServerError, "invalid response_log_body_retention_days after save")
			return
		}
		n, err := logretention.PurgeRequestLogResponseBodies(purgeCtx, h.store, days)
		if err != nil {
			slog.Default().Warn("log retention: response body purge after config update failed", "error", err)
		} else if n > 0 {
			slog.Default().Info("log retention: response body purge after config update", "rows", n, "retention_days", days)
		}
	}

	// Reload and return full config
	h.HandleGetConfig(w, r)
}

func (h *ConfigHandler) HandleConfigDefinition(w http.ResponseWriter, r *http.Request) {
	definition := map[string]struct {
		Type        string `json:"type"`
		Default     any    `json:"default"`
		Min         *int   `json:"min,omitempty"`
		Max         *int   `json:"max,omitempty"`
		Description string `json:"description"`
	}{
		"request_body_max_bytes": {
			Type:        "integer",
			Default:     models.DefaultRequestBodyMaxBytes,
			Min:         intPtr(0),
			Max:         intPtr(models.MaxBodyMaxBytes),
			Description: "Cap on how much of each request body is stored on the request log (what you see under Logs). Longer bodies are truncated at this size. 0 = store the full body (subject to memory while handling the request).",
		},
		"response_body_max_bytes": {
			Type:        "integer",
			Default:     models.DefaultResponseBodyMaxBytes,
			Min:         intPtr(0),
			Max:         intPtr(models.MaxBodyMaxBytes),
			Description: "Cap on stored response text: non-streaming JSON body, or reconstructed assistant text from a stream. Truncates when over the limit. 0 = no cap on stored length.",
		},
		"track_streaming": {
			Type:        "boolean",
			Default:     models.DefaultTrackStreaming,
			Description: "When on, each streamed response buffers raw SSE data: lines (after saving, under Streaming chunks in Logs). The client still gets the full stream; this only affects what is persisted.",
		},
		"streaming_buffer_size": {
			Type:        "integer",
			Default:     models.DefaultStreamingBufferSize,
			Min:         intPtr(models.MinStreamingBufferSize),
			Max:         intPtr(models.MaxStreamingBufferSize),
			Description: "Rolling cap on total bytes of raw SSE line text kept per streamed request (not a fixed number of chunks). When a new line would exceed this size, the oldest lines are dropped first. Larger values retain more of long streams but use more database space.",
		},
		"streaming_log_body_retention_days": {
			Type:        "integer",
			Default:     models.DefaultStreamingLogBodyRetentionDays,
			Min:         intPtr(models.MinStreamingLogBodyRetentionDays),
			Max:         intPtr(models.MaxStreamingLogBodyRetentionDays),
			Description: "After this many days, clear stored streaming chunk payloads (raw SSE line and text delta) in streaming_logs. Chunk metadata rows remain. Independent of request/response body retention. Runs daily and immediately when you save.",
		},
		"request_log_body_retention_days": {
			Type:        "integer",
			Default:     models.DefaultRequestLogBodyRetentionDays,
			Min:         intPtr(models.MinStreamingLogBodyRetentionDays),
			Max:         intPtr(models.MaxStreamingLogBodyRetentionDays),
			Description: "After this many days, clear the stored request_body field on each request log row (Logs UI). Log metadata and other columns are kept. Independent of streaming chunks and response body retention. Runs daily and immediately when you save.",
		},
		"response_log_body_retention_days": {
			Type:        "integer",
			Default:     models.DefaultResponseLogBodyRetentionDays,
			Min:         intPtr(models.MinStreamingLogBodyRetentionDays),
			Max:         intPtr(models.MaxStreamingLogBodyRetentionDays),
			Description: "After this many days, clear the stored response_body field on each request log row (JSON or reconstructed stream text). Independent of streaming chunks and request body retention. Runs daily and immediately when you save.",
		},
	}

	respondJSON(w, http.StatusOK, definition)
}

func intPtr(v int) *int {
	return &v
}
