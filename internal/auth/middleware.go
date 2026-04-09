package auth

import (
	"crypto/subtle"
	"net/http"
	"strings"
)

// AccessKeyMiddleware returns a chi-compatible middleware that validates
// the ACCESS_KEY from either Authorization: Bearer <key> or X-Access-Key header.
func AccessKeyMiddleware(accessKey string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			candidateKey := extractAccessKey(r)

			if !isValidKey(candidateKey, accessKey) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				w.Write([]byte(`{"error":"unauthorized"}`))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// extractAccessKey extracts the access key from the request, in order:
// 1. Authorization: Bearer <key> header
// 2. X-Access-Key header
// Returns empty string if neither yields a candidate.
func extractAccessKey(r *http.Request) string {
	// Try Authorization header first
	authHeader := r.Header.Get("Authorization")
	if authHeader != "" {
		// Must be "Bearer <key>" with a single space
		if strings.HasPrefix(authHeader, "Bearer ") {
			return strings.TrimSpace(authHeader[7:])
		}
	}

	// Fall back to X-Access-Key header
	return strings.TrimSpace(r.Header.Get("X-Access-Key"))
}

// isValidKey compares the candidate key to the configured accessKey using
// timing-safe comparison to prevent timing attacks.
func isValidKey(candidateKey, accessKey string) bool {
	// If lengths differ, they cannot match - do not call ConstantTimeCompare
	if len(candidateKey) != len(accessKey) {
		return false
	}

	// Use timing-safe comparison
	return subtle.ConstantTimeCompare([]byte(candidateKey), []byte(accessKey)) == 1
}

// CORSMiddleware sets CORS headers for development.
func CORSMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Access-Key")

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
