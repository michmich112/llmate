package proxy

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/llmate/gateway/internal/db"
	"github.com/llmate/gateway/internal/models"
)

func TestRequireAPIKeyFollowsSettingNotKeyPresence(t *testing.T) {
	ctx := context.Background()
	store, err := db.NewSQLiteStore(":memory:")
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	const rawKey = "sk-test-require-setting"
	now := time.Now().UTC()
	if err := store.CreateAPIKey(ctx, &models.APIKey{
		ID:        "key-1",
		KeyHash:   HashKey(rawKey),
		Name:      "present",
		IsActive:  true,
		CreatedAt: now,
	}); err != nil {
		t.Fatal(err)
	}

	snap := NewConfigSnapshot(store)
	if err := snap.Reload(ctx); err != nil {
		t.Fatal(err)
	}
	h := NewHandler(nil, nil, nil, snap, nil, store)
	handler := h.RequireAPIKey(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	call := func(token string) (int, string) {
		t.Helper()
		req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		rec := httptest.NewRecorder()
		handler(rec, req)
		var body struct {
			Error string `json:"error"`
		}
		_ = json.Unmarshal(rec.Body.Bytes(), &body)
		return rec.Code, body.Error
	}

	setRequire := func(v string) {
		t.Helper()
		if err := store.SetConfig(ctx, "require_api_keys", v); err != nil {
			t.Fatal(err)
		}
		if err := snap.Reload(ctx); err != nil {
			t.Fatal(err)
		}
	}

	// Keys exist, requirement off: a request without a key is accepted.
	if code, errMsg := call(""); code != http.StatusOK {
		t.Fatalf("open with keys present: status %d error %q", code, errMsg)
	}

	// Turning the setting on rejects a missing key even though keys exist.
	setRequire("true")
	if code, errMsg := call(""); code != http.StatusUnauthorized || errMsg != "missing API key" {
		t.Fatalf("required with keys present: status %d error %q", code, errMsg)
	}

	// Deleting every key does not turn the requirement off.
	if err := store.DeleteAPIKey(ctx, "key-1"); err != nil {
		t.Fatal(err)
	}
	if code, errMsg := call(""); code != http.StatusUnauthorized || errMsg != "missing API key" {
		t.Fatalf("required with no keys: status %d error %q", code, errMsg)
	}

	// Turning the setting off accepts a missing key even with no keys configured.
	setRequire("false")
	if code, errMsg := call(""); code != http.StatusOK {
		t.Fatalf("open with no keys: status %d error %q", code, errMsg)
	}

	// A presented key is still checked when the requirement is off.
	if err := store.CreateAPIKey(ctx, &models.APIKey{
		ID:        "key-2",
		KeyHash:   HashKey(rawKey),
		Name:      "present",
		IsActive:  true,
		CreatedAt: now,
	}); err != nil {
		t.Fatal(err)
	}
	if code, errMsg := call("sk-not-a-key"); code != http.StatusUnauthorized || errMsg != "invalid API key" {
		t.Fatalf("invalid key while open: status %d error %q", code, errMsg)
	}
	if code, errMsg := call(rawKey); code != http.StatusOK {
		t.Fatalf("valid key while open: status %d error %q", code, errMsg)
	}

	// And when the requirement is on.
	setRequire("true")
	if code, errMsg := call(rawKey); code != http.StatusOK {
		t.Fatalf("valid key while required: status %d error %q", code, errMsg)
	}
	if code, errMsg := call(""); code != http.StatusUnauthorized || errMsg != "missing API key" {
		t.Fatalf("missing key while required: status %d error %q", code, errMsg)
	}

	time.Sleep(50 * time.Millisecond)
}
