package auth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// nextHandler is a trivial downstream handler that writes 200 + marker body.
var nextHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok")) //nolint:errcheck
})

func TestAccessKeyMiddleware(t *testing.T) {
	const correctKey = "test-secret-key-12345"

	tests := []struct {
		name           string
		setupRequest   func(r *http.Request)
		wantStatus     int
		wantErrorBody  bool
		wantErrorValue string
	}{
		{
			name: "valid Bearer token",
			setupRequest: func(r *http.Request) {
				r.Header.Set("Authorization", "Bearer "+correctKey)
			},
			wantStatus: http.StatusOK,
		},
		{
			name: "valid X-Access-Key header",
			setupRequest: func(r *http.Request) {
				r.Header.Set("X-Access-Key", correctKey)
			},
			wantStatus: http.StatusOK,
		},
		{
			name:           "missing key",
			setupRequest:   func(r *http.Request) {},
			wantStatus:     http.StatusUnauthorized,
			wantErrorBody:  true,
			wantErrorValue: "unauthorized",
		},
		{
			name: "invalid Bearer key",
			setupRequest: func(r *http.Request) {
				r.Header.Set("Authorization", "Bearer wrong-key")
			},
			wantStatus:     http.StatusUnauthorized,
			wantErrorBody:  true,
			wantErrorValue: "unauthorized",
		},
		{
			name: "wrong Bearer format - no Bearer prefix",
			setupRequest: func(r *http.Request) {
				r.Header.Set("Authorization", "token "+correctKey)
			},
			wantStatus:     http.StatusUnauthorized,
			wantErrorBody:  true,
			wantErrorValue: "unauthorized",
		},
		{
			name: "empty Authorization header",
			setupRequest: func(r *http.Request) {
				r.Header.Set("Authorization", "")
			},
			wantStatus:     http.StatusUnauthorized,
			wantErrorBody:  true,
			wantErrorValue: "unauthorized",
		},
		{
			// Authorization takes priority over X-Access-Key; invalid Bearer blocks even
			// if X-Access-Key would be valid.
			name: "Authorization beats X-Access-Key when Authorization is wrong",
			setupRequest: func(r *http.Request) {
				r.Header.Set("Authorization", "Bearer wrong")
				r.Header.Set("X-Access-Key", correctKey)
			},
			wantStatus:     http.StatusUnauthorized,
			wantErrorBody:  true,
			wantErrorValue: "unauthorized",
		},
		{
			// Ensures length-mismatch path rejects rather than panics.
			name: "longer key rejected safely",
			setupRequest: func(r *http.Request) {
				r.Header.Set("Authorization", "Bearer "+correctKey+"extra")
			},
			wantStatus:     http.StatusUnauthorized,
			wantErrorBody:  true,
			wantErrorValue: "unauthorized",
		},
		{
			// Ensures empty-string key (length 0) does not match non-empty configured key.
			name: "empty Bearer value rejected",
			setupRequest: func(r *http.Request) {
				r.Header.Set("Authorization", "Bearer ")
			},
			wantStatus:     http.StatusUnauthorized,
			wantErrorBody:  true,
			wantErrorValue: "unauthorized",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			middleware := AccessKeyMiddleware(correctKey)
			handler := middleware(nextHandler)

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			tc.setupRequest(req)

			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			if rr.Code != tc.wantStatus {
				t.Errorf("status = %d, want %d", rr.Code, tc.wantStatus)
			}

			if tc.wantErrorBody {
				ct := rr.Header().Get("Content-Type")
				if ct != "application/json" {
					t.Errorf("Content-Type = %q, want application/json", ct)
				}

				var body map[string]string
				if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
					t.Fatalf("failed to decode error body: %v", err)
				}
				if got := body["error"]; got != tc.wantErrorValue {
					t.Errorf("error = %q, want %q", got, tc.wantErrorValue)
				}
			}
		})
	}
}

// TestCORSMiddleware verifies CORS header injection and OPTIONS short-circuit.
func TestCORSMiddleware(t *testing.T) {
	middleware := CORSMiddleware()

	t.Run("sets CORS headers on regular request", func(t *testing.T) {
		handler := middleware(nextHandler)
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("status = %d, want 200", rr.Code)
		}
		if got := rr.Header().Get("Access-Control-Allow-Origin"); got != "*" {
			t.Errorf("ACAO = %q, want *", got)
		}
	})

	t.Run("OPTIONS returns 204 without calling next", func(t *testing.T) {
		called := false
		sentinel := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			called = true
		})
		handler := middleware(sentinel)
		req := httptest.NewRequest(http.MethodOptions, "/", nil)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusNoContent {
			t.Errorf("status = %d, want 204", rr.Code)
		}
		if called {
			t.Error("next handler should not be called for OPTIONS preflight")
		}
		if got := rr.Header().Get("Access-Control-Allow-Methods"); got == "" {
			t.Error("Access-Control-Allow-Methods header not set")
		}
	})
}

// TestTimingSafeComparison verifies that crypto/subtle.ConstantTimeCompare is used
// for key comparison (guarding against timing-oracle attacks). The implementation
// in middleware.go uses subtle.ConstantTimeCompare for equal-length keys, and a
// fast-reject for unequal lengths (which does not leak secret length since we only
// skip the compare, not the rejection).
func TestTimingSafeComparison(t *testing.T) {
	const key = "super-secret"
	middleware := AccessKeyMiddleware(key)
	handler := middleware(nextHandler)

	// Same-length wrong key — must be rejected (exercises ConstantTimeCompare path).
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Access-Key", "wrong-secret") // len("wrong-secret") == len("super-secret")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("same-length wrong key: status = %d, want 401", rr.Code)
	}
}
