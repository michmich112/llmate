package proxy

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"
	"sync"
	"time"
)

// apiKeyContextKey carries the authenticated API key on the request context so
// handlers can stamp it onto request logs.
type apiKeyContextKey struct{}

// keyAuth carries the authenticated API key identity for the current request.
type keyAuth struct {
	ID   string
	Name string
}

// apiKeyFromContext returns the authenticated API key identity, or nil.
func apiKeyFromContext(ctx context.Context) *keyAuth {
	if v, ok := ctx.Value(apiKeyContextKey{}).(*keyAuth); ok {
		return v
	}
	return nil
}

// KeyLimiter provides fixed-window rate limits per API key.
// RPM is enforced as a hard limit (request count per 60s).
// TPM is enforced as a soft limit using a rough prompt-token estimate per 60s.
type KeyLimiter struct {
	mu       sync.Mutex
	entries  map[string]*keyLimiterEntry
}

type keyLimiterEntry struct {
	rpmStart   time.Time
	rpmCount   int
	tpmStart   time.Time
	tpmTokens  int
}

func NewKeyLimiter() *KeyLimiter {
	return &KeyLimiter{entries: make(map[string]*keyLimiterEntry)}
}

// allow reports whether the request passes rate limits, and the estimated tokens
// consumed. It is a no-op (allowed) when no key is provided.
func (l *KeyLimiter) allow(keyID string, rpmLimit, tpmLimit *int, promptTokens int) (bool, string) {
	if rpmLimit == nil && tpmLimit == nil {
		return true, ""
	}
	l.mu.Lock()
	defer l.mu.Unlock()

	entry, ok := l.entries[keyID]
	if !ok {
		entry = &keyLimiterEntry{}
		l.entries[keyID] = entry
	}

	now := time.Now()

	if rpmLimit != nil {
		if now.Sub(entry.rpmStart) >= 60*time.Second {
			entry.rpmStart = now
			entry.rpmCount = 0
		}
		entry.rpmCount++
		if entry.rpmCount > *rpmLimit {
			return false, "rpm limit exceeded"
		}
	}

	if tpmLimit != nil {
		if now.Sub(entry.tpmStart) >= 60*time.Second {
			entry.tpmStart = now
			entry.tpmTokens = 0
		}
		entry.tpmTokens += promptTokens
		if entry.tpmTokens > *tpmLimit {
			return false, "tpm limit exceeded"
		}
	}

	return true, ""
}

// estimatePromptTokens returns a rough token estimate for a request.
// Uses Content-Length when available (~4 chars/token), else falls back to 0.
func estimatePromptTokens(r *http.Request) int {
	if r.ContentLength > 0 {
		return int(r.ContentLength / 4)
	}
	return 0
}

// RequireAPIKey wraps a proxy handler with API key authentication and rate limiting.
// When require_api_keys config is disabled (default) and no API keys exist in the
// store, auth is skipped (open gateway) so existing deployments remain unchanged
// until keys are created.
func (h *Handler) RequireAPIKey(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		requireKeys := getConfigBool(h.config.Get(), "require_api_keys", false)
		if !requireKeys {
			keysExist, err := h.storeKeysExist()
			if err != nil {
				respondError(w, http.StatusInternalServerError, "internal error")
				return
			}
			if !keysExist {
				// No API keys configured — treat requests as unauthenticated (open gateway).
				r = r.WithContext(context.WithValue(r.Context(), apiKeyContextKey{}, (*keyAuth)(nil)))
				next(w, r)
				return
			}
		}

		token := BearerToken(r)
		if token == "" {
			respondError(w, http.StatusUnauthorized, "missing API key")
			return
		}

		key, err := h.store.GetAPIKeyByHash(r.Context(), HashKey(token))
		if err != nil {
			respondError(w, http.StatusInternalServerError, "internal error")
			return
		}
		if key == nil || !key.IsActive {
			respondError(w, http.StatusUnauthorized, "invalid API key")
			return
		}

		// Rate limiting (hard RPM, soft TPM).
		promptTokens := estimatePromptTokens(r)
		ok, msg := h.limiter.allow(key.ID, key.RateLimitRPM, key.RateLimitTPM, promptTokens)
		if !ok {
			respondError(w, http.StatusTooManyRequests, msg)
			return
		}

		// Touch last used timestamp asynchronously.
		go func(id string) {
			_ = h.store.TouchAPIKeyLastUsed(context.Background(), id, time.Now().UTC())
		}(key.ID)

		r = r.WithContext(context.WithValue(r.Context(), apiKeyContextKey{}, &keyAuth{ID: key.ID, Name: key.Name}))
		next(w, r)
	}
}

// storeKeysExist reports whether any API keys exist.
func (h *Handler) storeKeysExist() (bool, error) {
	keys, err := h.store.ListAPIKeys(context.Background())
	if err != nil {
		return false, err
	}
	return len(keys) > 0, nil
}

// BearerToken extracts the token from "Authorization: Bearer <token>".
func BearerToken(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if auth == "" {
		return ""
	}
	if !strings.HasPrefix(strings.ToLower(auth), "bearer ") {
		return ""
	}
	return strings.TrimSpace(auth[7:])
}

// HashKey returns the sha256 hex of a raw API key.
func HashKey(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
