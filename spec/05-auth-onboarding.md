# Spec 05: Auth + Onboarding (Phase 1E)

## Goal

Implement (1) the **ACCESS_KEY** authentication middleware that protects admin API routes, and (2) the **provider onboarding/discovery** logic that probes backend LLM servers for supported models and endpoints.

This spec is **fully self-contained**: an implementing agent must not read other spec files or `Context.md`. All types, interfaces, behavior, and signatures needed are below.

## Module Path

Use the same module path as the rest of the gateway (see Spec 00), e.g. `github.com/llmate/gateway`.

## Files to Create

1. `internal/auth/middleware.go` — ACCESS_KEY middleware + optional CORS helper
2. `internal/auth/middleware_test.go` — middleware tests
3. `internal/admin/onboard.go` — provider discovery/onboarding handlers
4. `internal/admin/onboard_test.go` — onboarding tests

## Dependencies

### Auth package (`internal/auth`)

- Standard library: `crypto/subtle`, `net/http`, `strings`

### Admin onboarding package (`internal/admin`)

- `internal/db` — `Store` interface
- `internal/models` — domain types (use existing packages; definitions repeated below for self-containment)
- `github.com/google/uuid` — UUIDs for new `ProviderEndpoint` rows
- `github.com/go-chi/chi/v5` — URL parameter `id` (handlers receive `chi.URLParam(r, "id")` when wired; see **Route registration**)
- Standard library: `context`, `encoding/json`, `errors`, `fmt`, `io`, `net/http`, `net/http/httptest`, `net/url`, `strings`, `time`

### Same-package helpers (admin)

`onboard.go` lives in `package admin` alongside `handler.go` (Spec 04). **Reuse** the existing `respondJSON` and `respondError` helpers from `handler.go` for JSON responses and consistent `{"error":"..."}` bodies. If those helpers are unexported, duplicate minimal private helpers in `onboard.go` only if necessary to compile — prefer calling shared package-level functions from `handler.go`.

---

## Domain Types (inline reference — full definitions)

These types live in `internal/models/`; the implementing agent must **use** `models` and **not** redefine them. Definitions are repeated here so this spec stands alone.

### `internal/models/provider.go`

```go
package models

import "time"

type Provider struct {
    ID              string     `json:"id"`
    Name            string     `json:"name"`
    BaseURL         string     `json:"base_url"`
    APIKey          string     `json:"api_key,omitempty"`
    IsHealthy       bool       `json:"is_healthy"`
    HealthCheckedAt *time.Time `json:"health_checked_at,omitempty"`
    CreatedAt       time.Time  `json:"created_at"`
    UpdatedAt       time.Time  `json:"updated_at"`
}

type ProviderEndpoint struct {
    ID          string    `json:"id"`
    ProviderID  string    `json:"provider_id"`
    Path        string    `json:"path"`
    Method      string    `json:"method"`
    IsSupported bool      `json:"is_supported"`
    IsEnabled   bool      `json:"is_enabled"`
    CreatedAt   time.Time `json:"created_at"`
}

type ProviderModel struct {
    ID         string    `json:"id"`
    ProviderID string    `json:"provider_id"`
    ModelID    string    `json:"model_id"`
    CreatedAt  time.Time `json:"created_at"`
}
```

---

## Store Methods Used (subset of `db.Store`)

The implementing agent only needs these methods from `internal/db`:

```go
// From internal/db/store.go — signatures as implemented in Phase 0 / 1A

GetProvider(ctx context.Context, id string) (*models.Provider, error)

// Replaces all endpoints for the provider with the given slice.
UpsertProviderEndpoints(ctx context.Context, providerID string, eps []models.ProviderEndpoint) error

// Replaces all models for the provider with the given model IDs (store generates UUIDs per model row).
SyncProviderModels(ctx context.Context, providerID string, modelIDs []string) error

// Used after upsert/sync to build the confirm response.
ListProviderEndpoints(ctx context.Context, providerID string) ([]models.ProviderEndpoint, error)

ListProviderModels(ctx context.Context, providerID string) ([]models.ProviderModel, error)
```

**Errors:** If `GetProvider` indicates not found (e.g. wrapped `sql.ErrNoRows` or store-specific sentinel), respond **`404 Not Found`** with `{"error":"provider not found"}` (or a similarly clear message).

---

# Part A — Auth Middleware (`internal/auth/middleware.go`)

## Package

`package auth`

## `AccessKeyMiddleware`

```go
func AccessKeyMiddleware(accessKey string) func(http.Handler) http.Handler
```

Returns a **chi-compatible** middleware: `func(http.Handler) http.Handler`.

### Behavior

1. **Extract credential** from the incoming request, **in order**:
   - **`Authorization` header:** If present and non-empty after trim, it must match **`Bearer <key>`** (single space after `Bearer`). Case-sensitive prefix `Bearer `. Extract `<key>` as the candidate access key.
   - Else **`X-Access-Key` header:** If present, use its value (trim surrounding whitespace) as the candidate access key.
   - If neither yields a candidate key, treat as **missing**.

2. **Compare** the candidate key to the configured `accessKey` using **`crypto/subtle.ConstantTimeCompare`** on equal-length byte slices:
   - If lengths differ, the keys do not match (do **not** call `ConstantTimeCompare` with different lengths; compare safely, e.g. pad to same length in a way that does not leak the configured key, or use a fixed-length scheme — the standard pattern is: if `len(a) != len(b)` then reject; else `ConstantTimeCompare([]byte(a), []byte(b)) == 1`).

3. **If match:** call `next.ServeHTTP(w, r)`.

4. **If missing or no match:** respond with **`401 Unauthorized`**, body **`{"error":"unauthorized"}`**, `Content-Type: application/json`.

### Notes

- Do not log the access key.
- Empty `Authorization` header → missing key → 401.
- `Authorization: not-bearer-token` (wrong scheme) → 401.

## `CORSMiddleware` (optional helper)

```go
func CORSMiddleware() func(http.Handler) http.Handler
```

Development-oriented CORS:

- On **every** request, set:
  - `Access-Control-Allow-Origin: *`
  - `Access-Control-Allow-Methods: GET, POST, PUT, DELETE, OPTIONS`
  - `Access-Control-Allow-Headers: Authorization, Content-Type, X-Access-Key`
- If **`OPTIONS`** (preflight): respond with **`204 No Content`** after setting headers; **do not** call `next` for OPTIONS (typical pattern).

---

## Testing (`internal/auth/middleware_test.go`)

Use `net/http/httptest` with a trivial `next` handler that writes **200** and a marker body.

Table-driven cases:

| Case | Setup | Expected |
|------|--------|----------|
| Valid Bearer | `Authorization: Bearer <correct-key>` | 200 |
| Valid `X-Access-Key` | `X-Access-Key: <correct-key>` | 200 |
| Missing key | No auth headers | 401, `{"error":"unauthorized"}` |
| Invalid Bearer key | `Authorization: Bearer wrong` | 401 |
| Wrong Bearer format | `Authorization: token` (no `Bearer ` prefix) | 401 |
| Empty Authorization | `Authorization:` empty | 401 |
| **Timing-safe path** | Assert `crypto/subtle.ConstantTimeCompare` is used for comparison (e.g. via code review comment in test file that greps for `ConstantTimeCompare`, or a small test that ensures both length-mismatch and wrong-key paths hit the compare helper — do not weaken security for testing). |

Parse response body JSON for the error case to assert the exact `error` string.

---

# Part B — Provider Onboarding (`internal/admin/onboard.go`)

## Package

`package admin`

## `OnboardHandler`

```go
type OnboardHandler struct {
    store  db.Store
    client *http.Client
}

func NewOnboardHandler(store db.Store, client *http.Client) *OnboardHandler
```

- **`client`:** Use for all outbound probes. The handler must not assume a global `http.DefaultClient` unless `nil` is documented — if `client == nil`, set a default in `NewOnboardHandler` such as `&http.Client{Timeout: 30 * time.Second}` for the outer operations; **per-probe** timeouts are specified below.

## Route registration (Phase 2 integration)

These routes are registered on the **admin** router (chi) by integration work; this spec only defines methods to attach:

| Method | Path | Handler |
|--------|------|---------|
| `POST` | `/providers/{id}/discover` | `h.HandleDiscover` |
| `POST` | `/providers/{id}/confirm` | `h.HandleConfirm` |

**URL parameter:** `id` is the provider ID — use `chi.URLParam(r, "id")` inside handlers.

---

## URL building and provider API key

- **Normalize `BaseURL`:** trim trailing `/`. When joining paths, ensure a single `/` between base and path (e.g. `baseURL + "/v1/models"` or use `url.JoinPath` / `path.Join` semantics appropriate for URLs).
- **Outbound auth to provider:** If `provider.APIKey` is non-empty, set `Authorization: Bearer <APIKey>` on requests to the provider backend. Do not send an `Authorization` header if `APIKey` is empty.

---

## `HandleDiscover`

`func (h *OnboardHandler) HandleDiscover(w http.ResponseWriter, r *http.Request)`

**Does not persist anything** — returns discovery results for the UI.

### Steps

1. **Provider ID** from URL. Reject non-`POST` with **405** if desired (router may already constrain method).

2. **`store.GetProvider(ctx, id)`** — on not found, **404** JSON error.

3. **Discover models — `discoverModels`**

   - `GET {baseURL}/v1/models` with provider API key header as above.
   - **Success response shape** (OpenAI-compatible):

     ```json
     {
       "data": [
         { "id": "model-name", "object": "model", "...": "..." }
       ]
     }
     ```

   - Parse JSON, extract each **`data[i].id`** into a `[]string` of model IDs (preserve order from JSON).
   - If the HTTP request **fails** (network error, TLS error, timeout), or non-success status, or JSON cannot be parsed, or **`data` is missing/empty** — return an error response: **`502 Bad Gateway`** or **`503 Service Unavailable`** with `{"error":"..."}` describing that the provider is unreachable or invalid (choose one status and use consistently).

4. **First model for probes:** `firstModel := models[0]`. If `len(models) == 0`, return **502** (or **400**) with a clear error — no model to probe.

5. **Probe endpoints** — for each row below, either skip without HTTP (audio/image) or call **`probeEndpoint`** with the given body:

   | Path | Method | Probe body (JSON) | Special |
   |------|--------|-------------------|--------|
   | `/v1/chat/completions` | `POST` | `{"model":"<firstModel>","messages":[{"role":"user","content":"hi"}],"max_tokens":1}` | — |
   | `/v1/completions` | `POST` | `{"model":"<firstModel>","prompt":"hi","max_tokens":1}` | — |
   | `/v1/embeddings` | `POST` | `{"model":"<firstModel>","input":"test"}` | — |
   | `/v1/images/generations` | `POST` | — | **Skip HTTP** — `is_supported: null` |
   | `/v1/audio/speech` | `POST` | — | **Skip HTTP** — `is_supported: null` |
   | `/v1/audio/transcriptions` | `POST` | — | **Skip HTTP** — `is_supported: null` |

   Replace `<firstModel>` with the actual first model string.

6. **Probe rules** (`probeEndpoint` — see below):

   - Use a **10-second timeout per probe** via `context.WithTimeout` on the request context.
   - **2xx** → `is_supported: true`
   - **404** → `is_supported: false`
   - **Timeout, connection error, 5xx, other 4xx (except 404)** → `is_supported: null` (unknown)
   - Skipped rows → `is_supported: null`

7. **Response JSON (200):**

```json
{
  "models": ["llama-3.1-70b", "llama-3.1-8b"],
  "endpoints": [
    {"path": "/v1/chat/completions", "method": "POST", "is_supported": true},
    {"path": "/v1/completions", "method": "POST", "is_supported": false},
    {"path": "/v1/embeddings", "method": "POST", "is_supported": true},
    {"path": "/v1/images/generations", "method": "POST", "is_supported": null},
    {"path": "/v1/audio/speech", "method": "POST", "is_supported": null},
    {"path": "/v1/audio/transcriptions", "method": "POST", "is_supported": null}
  ]
}
```

Use Go types with `*bool` for `is_supported` so JSON **`null`** encodes correctly (`omitempty` must **not** omit `false`; use pointers: `nil` → null, `ptr(true)` → true, `ptr(false)` → false).

---

## `HandleConfirm`

`func (h *OnboardHandler) HandleConfirm(w http.ResponseWriter, r *http.Request)`

### Request body (JSON)

```json
{
  "endpoints": [
    {
      "path": "/v1/chat/completions",
      "method": "POST",
      "is_supported": true,
      "is_enabled": true
    },
    {
      "path": "/v1/embeddings",
      "method": "POST",
      "is_supported": true,
      "is_enabled": true
    }
  ],
  "models": ["llama-3.1-70b", "llama-3.1-8b"]
}
```

- **`endpoints`:** Only entries the user confirms; map each to a `models.ProviderEndpoint` with **new UUID** from `uuid.NewString()` for `ID`, `ProviderID` from URL, `Path`, `Method`, `IsSupported`, `IsEnabled`, **`CreatedAt: time.Now().UTC()`** (or store convention from Spec 04).
- **`models`:** List of model ID strings to sync.

### Steps

1. Parse JSON; invalid body → **400** with `{"error":"..."}`.

2. `GetProvider` — **404** if not found.

3. Build `[]models.ProviderEndpoint` from request (UUIDs + timestamps).

4. `UpsertProviderEndpoints(ctx, providerID, endpoints)`.

5. `SyncProviderModels(ctx, providerID, modelIDs)`.

6. Reload presentation data: `ListProviderEndpoints` + `ListProviderModels` for the provider.

7. **Response (200):** Return the **provider** plus **endpoints** and **models** in one JSON object, for example:

```json
{
  "provider": { "...": "..." },
  "endpoints": [ "...ProviderEndpoint..." ],
  "models": [ "...ProviderModel..." ]
}
```

Field names **`provider`**, **`endpoints`**, **`models`**. Omit empty slices if the project convention prefers; otherwise return empty arrays.

---

## Helper functions

```go
func (h *OnboardHandler) discoverModels(ctx context.Context, baseURL string, apiKey string) ([]string, error)
```

- Performs `GET` to `/v1/models` relative to `baseURL`.
- Returns ordered model IDs from `data[].id`.
- Returns a **non-nil error** if the provider cannot be reached or the response is unusable.

```go
func (h *OnboardHandler) probeEndpoint(ctx context.Context, baseURL string, apiKey string, path string, probeBody []byte) (supported *bool, err error)
```

- **`POST`** to `baseURL` + `path`, `Content-Type: application/json`, body `probeBody`.
- Apply **10s timeout** on the request context.
- Return values:
  - **`supported` non-nil, `err == nil`:** `true` for 2xx, `false` for 404.
  - **`supported == nil`, `err == nil`:** unknown (5xx, other status, timeout — classify per rules above).
  - **`err != nil`:** transport/timeout errors — treat as unknown (`nil` pointer) at caller, or return `err` for `discover` to fail — **spec:** treat probe **failure** as **unknown** (`is_supported: null`), **not** aborting the whole discover (only model list failure aborts). So `probeEndpoint` may return `(nil, err)` and caller maps to `nil` supported.

Clarification: **Only** `discoverModels` failure fails the entire `HandleDiscover`. Individual probe failures → unknown for that endpoint.

---

## Testing (`internal/admin/onboard_test.go`)

Use **`httptest.NewServer`** to mock the backend LLM:

- **Discover — models list:** Server returns `200` and `{"data":[{"id":"m1"},{"id":"m2"}]}`. Assert response `models` equals `["m1","m2"]` (or expected order).

- **Discover — chat completions 2xx:** Mock returns `200` for `POST /v1/chat/completions`. Assert that endpoint has `is_supported: true`.

- **Discover — completions 404:** Mock returns `404` for `POST /v1/completions`. Assert `is_supported: false`.

- **Discover — unreachable:** Server closed or connection refused / models `GET` returns 500. Assert `HandleDiscover` returns an error status (502/503) and JSON error.

- **Confirm — persists:** Use a **mock `db.Store`** (in-memory or stub) implementing the needed methods; call `HandleConfirm`; assert `UpsertProviderEndpoints` and `SyncProviderModels` received expected values (or read back via mock).

- **Probe timeout:** Mock delays response > 10s on one endpoint; assert that endpoint **`is_supported` is JSON `null`**.

Use `chi` router or direct `OnboardHandler` method calls with `httptest.NewRecorder` as appropriate.

---

## Done Criteria

- [ ] Auth middleware validates **ACCESS_KEY** with **timing-safe** comparison (`crypto/subtle.ConstantTimeCompare`).
- [ ] Auth middleware checks **`Authorization: Bearer`** and **`X-Access-Key`** in the specified order.
- [ ] CORS middleware sets headers and returns **204** for **OPTIONS** preflight.
- [ ] **`HandleDiscover`** fetches models, probes POST endpoints per table, skips image/audio with `null`, returns JSON as specified.
- [ ] **`HandleConfirm`** upserts endpoints and syncs models, returns provider + lists.
- [ ] All tests in `middleware_test.go` and `onboard_test.go` pass.
- [ ] `go build ./internal/auth/...` succeeds.
- [ ] `go build ./internal/admin/...` succeeds.
- [ ] `go test ./internal/auth/... ./internal/admin/...` succeeds.
