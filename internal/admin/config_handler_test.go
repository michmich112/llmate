package admin

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"context"
	"testing"

	"github.com/llmate/gateway/internal/db"
	"github.com/llmate/gateway/internal/models"
	"github.com/llmate/gateway/internal/stats"
)


func testAdminHandlerReal(store db.Store, cfg HandlerConfig) *Handler {
	qw := NewQueryWorker(store, 4)
	qw.Start(context.Background())
	return NewHandler(store, cfg, stats.NewAccumulator(), qw)
}

func TestHandleUpdateConfig_StreamingRetentionDays(t *testing.T) {
	store, err := db.NewSQLiteStore(":memory:")
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	h := testAdminHandlerReal(store, HandlerConfig{})

	t.Run("reject zero", func(t *testing.T) {
		body := []byte(`{"streaming_log_body_retention_days":0}`)
		req := httptest.NewRequest(http.MethodPut, "/config", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := serve(h, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status %d, body %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("reject above max", func(t *testing.T) {
		body := []byte(`{"streaming_log_body_retention_days":10000}`)
		req := httptest.NewRequest(http.MethodPut, "/config", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := serve(h, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status %d", rec.Code)
		}
	})

	t.Run("accept minimum", func(t *testing.T) {
		body := []byte(`{"streaming_log_body_retention_days":1}`)
		req := httptest.NewRequest(http.MethodPut, "/config", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := serve(h, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status %d, body %s", rec.Code, rec.Body.String())
		}
		var got models.Configuration
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		if got.StreamingLogBodyRetentionDays != 1 {
			t.Fatalf("got %d", got.StreamingLogBodyRetentionDays)
		}
	})

	t.Run("reject invalid request_log_body_retention_days", func(t *testing.T) {
		body := []byte(`{"request_log_body_retention_days":0}`)
		req := httptest.NewRequest(http.MethodPut, "/config", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := serve(h, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status %d", rec.Code)
		}
	})
}

func TestHandleGetConfig_DefaultRetentionDays(t *testing.T) {
	store, err := db.NewSQLiteStore(":memory:")
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	h := testAdminHandlerReal(store, HandlerConfig{})
	req := httptest.NewRequest(http.MethodGet, "/config", nil)
	rec := serve(h, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	var got models.Configuration
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.StreamingLogBodyRetentionDays != models.DefaultStreamingLogBodyRetentionDays {
		t.Fatalf("default retention: got %d want %d", got.StreamingLogBodyRetentionDays, models.DefaultStreamingLogBodyRetentionDays)
	}
	if got.RequestLogBodyRetentionDays != models.DefaultRequestLogBodyRetentionDays {
		t.Fatalf("default request body retention: got %d want %d", got.RequestLogBodyRetentionDays, models.DefaultRequestLogBodyRetentionDays)
	}
	if got.ResponseLogBodyRetentionDays != models.DefaultResponseLogBodyRetentionDays {
		t.Fatalf("default response body retention: got %d want %d", got.ResponseLogBodyRetentionDays, models.DefaultResponseLogBodyRetentionDays)
	}
	if got.HTTPIdleConnTimeoutSeconds != models.DefaultHTTPIdleConnTimeoutSeconds {
		t.Fatalf("default http idle: got %d want %d", got.HTTPIdleConnTimeoutSeconds, models.DefaultHTTPIdleConnTimeoutSeconds)
	}
}

func TestHandleUpdateConfig_HTTPIdleConnTimeoutHook(t *testing.T) {
	store, err := db.NewSQLiteStore(":memory:")
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	var mu sync.Mutex
	var gotHook int
	var hookSeen bool
	h := testAdminHandlerReal(store, HandlerConfig{
		OnHTTPIdleConnTimeoutSaved: func(sec int) {
			mu.Lock()
			gotHook = sec
			hookSeen = true
			mu.Unlock()
		},
	})

	body := []byte(`{"http_idle_conn_timeout_seconds":120}`)
	req := httptest.NewRequest(http.MethodPut, "/config", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := serve(h, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d, body %s", rec.Code, rec.Body.String())
	}
	mu.Lock()
	defer mu.Unlock()
	if !hookSeen || gotHook != 120 {
		t.Fatalf("hook: seen=%v got=%d", hookSeen, gotHook)
	}
}

func TestHandleUpdateConfig_HTTPIdleConnTimeoutValidation(t *testing.T) {
	store, err := db.NewSQLiteStore(":memory:")
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	h := testAdminHandlerReal(store, HandlerConfig{})
	body := []byte(`{"http_idle_conn_timeout_seconds":5}`)
	req := httptest.NewRequest(http.MethodPut, "/config", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := serve(h, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}
