package admin

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/llmate/gateway/internal/db"
	"github.com/llmate/gateway/internal/models"
)

func TestHandleUpdateConfig_StreamingRetentionDays(t *testing.T) {
	store, err := db.NewSQLiteStore(":memory:")
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	h := NewHandler(store)

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

	h := NewHandler(store)
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
}
