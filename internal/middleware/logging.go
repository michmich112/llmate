package middleware

import (
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"
)

type statusRecorder struct {
	http.ResponseWriter
	statusCode int
}

func (sr *statusRecorder) WriteHeader(code int) {
	sr.statusCode = code
	sr.ResponseWriter.WriteHeader(code)
}

// Flush delegates to the underlying ResponseWriter if it implements http.Flusher.
// Without this, SSE streaming fails because the logging middleware wraps the writer
// and the proxy's flusher type-assertion would otherwise return false.
func (sr *statusRecorder) Flush() {
	if f, ok := sr.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// Unwrap returns the underlying ResponseWriter so middleware introspection works
// correctly through the chain.
func (sr *statusRecorder) Unwrap() http.ResponseWriter {
	return sr.ResponseWriter
}

// Logging returns an HTTP middleware that logs each request's method, path, status code,
// duration in milliseconds, and client IP address.
// If logger is nil, slog.Default() is used.
func Logging(logger *slog.Logger) func(http.Handler) http.Handler {
	if logger == nil {
		logger = slog.Default()
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			rec := &statusRecorder{ResponseWriter: w, statusCode: http.StatusOK}
			next.ServeHTTP(rec, r)

			durationMs := time.Since(start).Milliseconds()

			// Prefer X-Forwarded-For (first entry) over RemoteAddr.
			clientIP := r.Header.Get("X-Forwarded-For")
			if clientIP != "" {
				clientIP = strings.TrimSpace(strings.SplitN(clientIP, ",", 2)[0])
			} else {
				host, _, err := net.SplitHostPort(r.RemoteAddr)
				if err != nil {
					host = r.RemoteAddr
				}
				clientIP = host
			}

			logger.Info("request",
				"request_id", GetRequestID(r.Context()),
				"method", r.Method,
				"path", r.URL.Path,
				"status", rec.statusCode,
				"duration_ms", durationMs,
				"client_ip", clientIP,
			)
		})
	}
}
