# Spec 04: Admin API Handlers (Phase 1D)

## Goal

Implement the admin API HTTP handlers for managing providers, model aliases, querying request logs, and dashboard statistics. These handlers are called by the Svelte dashboard and are protected by the ACCESS_KEY auth middleware (implemented by Phase 1E). This phase implements **handlers only**; wiring under `/admin` with auth is Phase 2.

## Files to Create

1. `internal/admin/handler.go` — all admin API handlers, `respondJSON` / `respondError` / `parseDurationParam`, and `Routes()`
2. `internal/admin/stats.go` — `HandleGetStats` (imports shared helpers from `handler.go` in the same package)
3. `internal/admin/handler_test.go` — table-driven tests with a mock `db.Store`

## Package

`package admin`

## Module Path

Use the same module path as the rest of the gateway (see Spec 00), e.g. `github.com/llmate/gateway`.

## Dependencies

- `internal/db` — `Store` interface
- `internal/models` — domain types
- `github.com/go-chi/chi/v5` — URL parameters and sub-router
- `github.com/google/uuid` — new record IDs
- Standard library: `context`, `database/sql` (for `errors.Is` with `sql.ErrNoRows` if the store uses it), `encoding/json`, `errors`, `fmt`, `net/http`, `net/http/httptest`, `strconv`, `strings`, `time`

---

## Domain Types (inline reference — full definitions)

These types live in `internal/models/`; the implementing agent must **use** these packages and **not** redefine them. Definitions are repeated here so this spec stands alone.

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

### `internal/models/alias.go`

```go
package models

import "time"

type ModelAlias struct {
    ID         string    `json:"id"`
    Alias      string    `json:"alias"`
    ProviderID string    `json:"provider_id"`
    ModelID    string    `json:"model_id"`
    Weight     int       `json:"weight"`
    Priority   int       `json:"priority"`
    IsEnabled  bool      `json:"is_enabled"`
    CreatedAt  time.Time `json:"created_at"`
    UpdatedAt  time.Time `json:"updated_at"`
}
```

### `internal/models/request_log.go`

```go
package models

import "time"

type RequestLog struct {
    ID               string    `json:"id"`
    Timestamp        time.Time `json:"timestamp"`
    ClientIP         string    `json:"client_ip"`
    Method           string    `json:"method"`
    Path             string    `json:"path"`
    RequestedModel   string    `json:"requested_model,omitempty"`
    ResolvedModel    string    `json:"resolved_model,omitempty"`
    ProviderID       string    `json:"provider_id,omitempty"`
    ProviderName     string    `json:"provider_name,omitempty"`
    StatusCode       int       `json:"status_code"`
    IsStreamed       bool      `json:"is_streamed"`
    TTFTMs           *int      `json:"ttft_ms,omitempty"`
    TotalTimeMs      int       `json:"total_time_ms"`
    PromptTokens     *int      `json:"prompt_tokens,omitempty"`
    CompletionTokens *int      `json:"completion_tokens,omitempty"`
    TotalTokens      *int      `json:"total_tokens,omitempty"`
    CachedTokens     *int      `json:"cached_tokens,omitempty"`
    ErrorMessage     string    `json:"error_message,omitempty"`
    CreatedAt        time.Time `json:"created_at"`
}

type LogFilter struct {
    Model      string     `json:"model,omitempty"`
    ProviderID string     `json:"provider_id,omitempty"`
    Since      *time.Time `json:"since,omitempty"`
    Until      *time.Time `json:"until,omitempty"`
    Limit      int        `json:"limit"`
    Offset     int        `json:"offset"`
}
```

### `internal/models/stats.go`

```go
package models

type DashboardStats struct {
    TotalRequests int             `json:"total_requests"`
    AvgLatencyMs  float64         `json:"avg_latency_ms"`
    ErrorRate     float64         `json:"error_rate"`
    ByModel       []ModelStats    `json:"by_model"`
    ByProvider    []ProviderStats `json:"by_provider"`
}

type ModelStats struct {
    Model        string  `json:"model"`
    RequestCount int     `json:"request_count"`
    AvgLatencyMs float64 `json:"avg_latency_ms"`
    ErrorCount   int     `json:"error_count"`
    TotalTokens  int     `json:"total_tokens"`
}

type ProviderStats struct {
    ProviderID   string  `json:"provider_id"`
    ProviderName string  `json:"provider_name"`
    RequestCount int     `json:"request_count"`
    AvgLatencyMs float64 `json:"avg_latency_ms"`
    ErrorCount   int     `json:"error_count"`
}
```

---

## Store Interface (inline reference — full)

The implementing agent must use the real `db.Store` from `internal/db/store.go`. Full interface:

```go
package db

import (
    "context"
    "time"

    "github.com/llmate/gateway/internal/models"
)

type Store interface {
    // --- Providers ---

    // CreateProvider inserts a new provider. The caller must set ID and timestamps.
    CreateProvider(ctx context.Context, p *models.Provider) error

    // GetProvider returns a provider by ID. Returns error if not found.
    GetProvider(ctx context.Context, id string) (*models.Provider, error)

    // ListProviders returns all providers ordered by created_at desc.
    ListProviders(ctx context.Context) ([]models.Provider, error)

    // UpdateProvider updates name, base_url, api_key, updated_at. Identified by p.ID.
    UpdateProvider(ctx context.Context, p *models.Provider) error

    // DeleteProvider removes a provider and cascades to endpoints, models, and aliases.
    DeleteProvider(ctx context.Context, id string) error

    // --- Provider Endpoints ---

    // UpsertProviderEndpoints replaces all endpoints for a provider with the given set.
    UpsertProviderEndpoints(ctx context.Context, providerID string, eps []models.ProviderEndpoint) error

    // ListProviderEndpoints returns all endpoints for a provider.
    ListProviderEndpoints(ctx context.Context, providerID string) ([]models.ProviderEndpoint, error)

    // UpdateProviderEndpoint updates is_enabled for a single endpoint. Identified by ep.ID.
    UpdateProviderEndpoint(ctx context.Context, ep *models.ProviderEndpoint) error

    // --- Provider Models ---

    // SyncProviderModels replaces all models for a provider with the given model IDs.
    // Generates new UUIDs for new models, removes models not in the list.
    SyncProviderModels(ctx context.Context, providerID string, modelIDs []string) error

    // ListProviderModels returns all models for a provider.
    ListProviderModels(ctx context.Context, providerID string) ([]models.ProviderModel, error)

    // ListAllModels returns all models across all providers.
    ListAllModels(ctx context.Context) ([]models.ProviderModel, error)

    // --- Model Aliases ---

    // CreateAlias inserts a new alias. The caller must set ID and timestamps.
    CreateAlias(ctx context.Context, a *models.ModelAlias) error

    // ListAliases returns all aliases ordered by alias name, then priority desc.
    ListAliases(ctx context.Context) ([]models.ModelAlias, error)

    // UpdateAlias updates weight, priority, is_enabled, updated_at. Identified by a.ID.
    UpdateAlias(ctx context.Context, a *models.ModelAlias) error

    // DeleteAlias removes an alias by ID.
    DeleteAlias(ctx context.Context, id string) error

    // ResolveAlias returns all enabled alias entries for a given alias name,
    // ordered by priority desc. Used by the smart router.
    ResolveAlias(ctx context.Context, alias string) ([]models.ModelAlias, error)

    // --- Routing (hot path, read-only) ---

    // GetHealthyProvidersForModel returns all healthy providers that have
    // the given model_id in their provider_models table.
    GetHealthyProvidersForModel(ctx context.Context, modelID string) ([]models.Provider, error)

    // GetEnabledEndpoint returns the endpoint for a provider+path if it exists
    // and is both supported and enabled. Returns nil (not error) if not found.
    GetEnabledEndpoint(ctx context.Context, providerID string, path string) (*models.ProviderEndpoint, error)

    // --- Request Logs ---

    // InsertRequestLog inserts a request log entry.
    InsertRequestLog(ctx context.Context, log *models.RequestLog) error

    // QueryRequestLogs returns filtered request logs and total count.
    // Results are ordered by timestamp desc.
    QueryRequestLogs(ctx context.Context, filter models.LogFilter) ([]models.RequestLog, int, error)

    // --- Stats ---

    // GetDashboardStats returns aggregated statistics since the given time.
    GetDashboardStats(ctx context.Context, since time.Time) (*models.DashboardStats, error)

    // --- Health ---

    // UpdateProviderHealth updates is_healthy and health_checked_at for a provider.
    UpdateProviderHealth(ctx context.Context, id string, healthy bool) error

    // --- Lifecycle ---

    // Close closes the database connection.
    Close() error
}
```

---

## Handler struct and constructor

```go
type Handler struct {
    store db.Store
}

func NewHandler(store db.Store) *Handler
```

`NewHandler` must panic or the caller must ensure `store != nil`; **this spec:** return `&Handler{store: store}` and document that the gateway must not pass nil.

---

## Routes

Expose:

```go
func (h *Handler) Routes() chi.Router
```

Implementation pattern:

- `r := chi.NewRouter()`
- Register only the routes **this phase** owns. Do **not** register `POST /providers/{id}/discover` or `POST /providers/{id}/confirm` here — those are Phase 1E (`onboard.go`).
- Use `chi.URLParam(r, "id")` and `chi.URLParam(r, "eid")` for path segments as listed below.

### Route table (relative paths on the returned router)

| Method | Path | Handler | Notes |
|--------|------|---------|--------|
| POST | `/auth` | `h.HandleAuth` | Returns `{"valid":true}` when reached |
| GET | `/providers` | `h.HandleListProviders` | |
| POST | `/providers` | `h.HandleCreateProvider` | 201 |
| GET | `/providers/{id}` | `h.HandleGetProvider` | |
| PUT | `/providers/{id}` | `h.HandleUpdateProvider` | |
| DELETE | `/providers/{id}` | `h.HandleDeleteProvider` | 204 |
| PUT | `/providers/{id}/endpoints/{eid}` | `h.HandleUpdateEndpoint` | |
| GET | `/aliases` | `h.HandleListAliases` | |
| POST | `/aliases` | `h.HandleCreateAlias` | 201 |
| PUT | `/aliases/{id}` | `h.HandleUpdateAlias` | |
| DELETE | `/aliases/{id}` | `h.HandleDeleteAlias` | 204 |
| GET | `/logs` | `h.HandleQueryLogs` | |
| GET | `/stats` | `h.HandleGetStats` | Implemented in `stats.go` |

Phase 2 mounts this router at `/admin` and wraps it with ACCESS_KEY middleware.

---

## JSON helpers

Place in `handler.go` (same package as `stats.go` so `HandleGetStats` can use them):

```go
func respondJSON(w http.ResponseWriter, status int, data interface{})
func respondError(w http.ResponseWriter, status int, msg string)
func parseDurationParam(s string) (time.Duration, error)
```

### `respondJSON`

- Set `Content-Type: application/json`
- `json.NewEncoder(w).Encode(data)`; on encode error, log (optional) and send `500` with `respondError` if headers not yet sent (stdlib pattern: prefer encoding before `WriteHeader` or use a buffer — **this spec:** encode to a `bytes.Buffer` first, then `w.WriteHeader(status)` and write buffer, so a failed encode returns `500` with a generic error body)

### `respondError`

- Response body: `{"error":"<msg>"}` with `Content-Type: application/json`
- Escape or sanitize is not required for v1; use plain string from handler

### `parseDurationParam`

- Input: duration string for dashboard windows, e.g. `24h`, `7d`, `30d`
- Empty string: return error (caller uses default — see `HandleGetStats`)
- Supported:
  - Go `time.ParseDuration` for standard forms (`24h`, `1h30m`, etc.)
  - **Days:** suffix `d` meaning 24 hours per day: parse leading number with `strconv.Atoi` on the numeric prefix, then `time.Duration(days) * 24 * time.Hour`. Examples: `7d` → 168h, `30d` → 720h
- Reject invalid combinations with a clear error

---

## Per-handler specification

Unless noted, use `context.Background()` or `r.Context()` for store calls (**must** use `r.Context()` for cancellation).

### `HandleAuth` — `POST /auth`

1. **Input:** none (body ignored).
2. **Store:** none.
3. **Response:** `200` with `{"valid": true}`. Middleware already enforced ACCESS_KEY; reaching this handler implies validity.
4. **Errors:** none expected.

---

### `HandleListProviders` — `GET /providers`

1. **Input:** none.
2. **Store:** `ListProviders(ctx)`.
3. **Response:** `200` with `{"providers": [...]}` (JSON array of `models.Provider`). Empty list is `[]`.
4. **Errors:** store failure → `500` + `{"error":"..."}`.

---

### `HandleCreateProvider` — `POST /providers`

1. **Input:** JSON body:
   ```json
   {"name":"string","base_url":"string","api_key":"string"}
   ```
   - `name` and `base_url` are **required** (non-empty after `strings.TrimSpace`).
   - `api_key` is optional (may be omitted or empty string).

2. **Validation:** `400` if JSON invalid, or `name` / `base_url` missing or empty.

3. **Build `models.Provider`:**
   - `ID` = `uuid.NewString()`
   - `Name`, `BaseURL` from body
   - `APIKey` from body
   - `IsHealthy` = `false`
   - `HealthCheckedAt` = `nil`
   - `CreatedAt` = `UpdatedAt` = `time.Now().UTC()` (or local monotonic wall clock — **be consistent** with store; prefer UTC)

4. **Store:** `CreateProvider(ctx, &p)`.

5. **Response:** `201` with `{"provider": <provider>}`. Include full struct as stored.

6. **Errors:** duplicate `base_url` or DB constraint → `500` with message (or `409` if store distinguishes — **this spec:** `500` unless store returns a typed conflict error you map to `409`).

---

### `HandleGetProvider` — `GET /providers/{id}`

1. **Input:** `id` = `chi.URLParam(r, "id")`. `400` if empty.

2. **Store:**
   - `GetProvider(ctx, id)` → if not found (`errors.Is(err, sql.ErrNoRows)` or documented store sentinel), **`404`**
   - `ListProviderEndpoints(ctx, id)`
   - `ListProviderModels(ctx, id)`

3. **Response:** `200` with:
   ```json
   {
     "provider": { ... },
     "endpoints": [ ... ],
     "models": [ ... ]
   }
   ```

4. **Errors:** not found → `404`; other store errors → `500`.

---

### `HandleUpdateProvider` — `PUT /providers/{id}`

1. **Input:** `id` from URL. JSON body:
   ```json
   {"name":"...","base_url":"...","api_key":"..."}
   ```
   All three fields **must** be present in JSON for a strict PUT (**this spec:** require `name` and `base_url` non-empty after trim; `api_key` may be empty string to clear).

2. **Store:** `GetProvider(ctx, id)` — not found → `404`.

3. **Merge:** Copy existing provider, set `Name`, `BaseURL`, `APIKey` from body, set `UpdatedAt = time.Now().UTC()`, preserve `ID`, `IsHealthy`, `HealthCheckedAt`, `CreatedAt`.

4. **Store:** `UpdateProvider(ctx, &merged)`.

5. **Response:** `200` with `{"provider": ...}` (reload from store after update if you want freshness — **optional**; returning merged struct is acceptable if it matches DB).

6. **Errors:** `400` bad JSON/validation; `404`; `500`.

---

### `HandleDeleteProvider` — `DELETE /providers/{id}`

1. **Input:** `id` from URL; `400` if empty.

2. **Store:** `DeleteProvider(ctx, id)`. If store returns not found, **`404`**.

3. **Response:** `204 No Content` with empty body on success.

4. **Errors:** `404`, `500`.

---

### `HandleUpdateEndpoint` — `PUT /providers/{id}/endpoints/{eid}`

1. **Input:** `providerID` = `chi.URLParam(r, "id")`, `endpointID` = `chi.URLParam(r, "eid")`. `400` if either empty.

2. **Body:** JSON `{"is_enabled": true|false}` — **only** field allowed for update; `400` if missing or wrong type.

3. **Store:**
   - Load endpoints via `ListProviderEndpoints(ctx, providerID)` and find `eid` belonging to that provider. If not found → **`404`**.
   - Set `IsEnabled` from body; leave all other fields unchanged.
   - `UpdateProviderEndpoint(ctx, &ep)`.

4. **Response:** `200` with `{"endpoint": <updated ProviderEndpoint>}`.

5. **Errors:** `400`, `404`, `500`.

---

### `HandleListAliases` — `GET /aliases`

1. **Store:** `ListAliases(ctx)`.

2. **Response:** `200` with `{"aliases": [...]}`.

3. **Errors:** `500`.

---

### `HandleCreateAlias` — `POST /aliases`

1. **Body:**
   ```json
   {"alias":"gpt-4","provider_id":"<uuid>","model_id":"...","weight":1,"priority":0}
   ```
   - Required: `alias`, `provider_id`, `model_id` (non-empty after trim).
   - Default `weight` = `1` if omitted or zero and you treat zero as default (**this spec:** if `weight` absent, use `1`; if `priority` absent, use `0`).
   - `is_enabled` defaults to **`true`** for new aliases.

2. **Build:** `ID = uuid.NewString()`, `CreatedAt`/`UpdatedAt` = now, set fields.

3. **Store:** `CreateAlias(ctx, &a)`.

4. **Response:** `201` with `{"alias": ...}`.

5. **Errors:** `400` validation; FK/DB errors → `500` (or `400` if provider missing — **this spec:** `500` unless you verify provider exists with `GetProvider` first and return `400` for unknown provider — **prefer** optional pre-check: `GetProvider` for `provider_id`, not found → `400` `"unknown provider"`).

---

### `HandleUpdateAlias` — `PUT /aliases/{id}`

1. **Input:** `id` = `chi.URLParam(r, "id")`.

2. **Body:** Partial JSON. Updatable fields (aligned with `Store.UpdateAlias`): **`weight`**, **`priority`**, **`is_enabled`**. Optionally allow **`alias`**, **`provider_id`**, **`model_id`** if the SQLite implementation supports them; **this spec:** only send fields that `UpdateAlias` persists — per Store comment, **`weight`**, **`priority`**, **`is_enabled`** only. Ignore unknown keys or reject — **prefer** merge:
   - `Get` existing alias: load via `ListAliases` and find by id, or add no new store method — **this spec:** use `ListAliases(ctx)` and find `id` (**O(n)** acceptable for v1) or document that implementer may add `GetAlias` — **to avoid new Store methods**, use `ListAliases` filter in memory. If not found → **`404`**.

3. **Merge:** For each provided JSON field, overwrite the corresponding field on the model; set `UpdatedAt = now`.

4. **Store:** `UpdateAlias(ctx, &merged)`.

5. **Response:** `200` with `{"alias": ...}`.

6. **Errors:** `400`, `404`, `500`.

**Note:** If `ListAliases` is inefficient, the implementing agent may add `GetAlias` to Store in a separate change — **out of scope for 1D**; use `ListAliases` + linear search.

---

### `HandleDeleteAlias` — `DELETE /aliases/{id}`

1. **Input:** `id` from URL.

2. **Store:** `DeleteAlias(ctx, id)`. Not found → **`404`** if store signals it; if `DeleteAlias` is idempotent, **this spec:** still prefer `404` when no row deleted if the store returns `sql.ErrNoRows`.

3. **Response:** `204` empty.

4. **Errors:** `404`, `500`.

---

### `HandleQueryLogs` — `GET /logs`

1. **Query parameters:**
   - `model` → `LogFilter.Model`
   - `provider_id` → `LogFilter.ProviderID`
   - `since` → RFC3339 datetime; parse with `time.Parse(time.RFC3339, s)`; invalid → `400`
   - `until` → same; invalid → `400`
   - `limit` → integer, **default 50**, **max 1000** (if `> 1000`, clamp to 1000; if `< 1` after parse, `400` or clamp to 1 — **this spec:** default 50, clamp max 1000, if `limit` present and `< 1` → `400`)
   - `offset` → integer, **default 0**; if `< 0` → `400`

2. **Store:** `QueryRequestLogs(ctx, filter)` with `Limit`/`Offset` set.

3. **Response:** `200` with:
   ```json
   {"logs":[...],"total":N}
   ```
   where `total` is the second return value from `QueryRequestLogs`.

4. **Errors:** parse errors → `400`; store → `500`.

---

### `HandleGetStats` — `GET /stats` (in `stats.go`)

1. **Query:** `since` — duration string (`24h`, `7d`, `30d`, etc.). **Default** if empty: **`24h`** (parse via `parseDurationParam("24h")`).

2. **Compute:** `t0 := time.Now().Add(-d)` (use monotonic-safe wall time).

3. **Store:** `GetDashboardStats(ctx, t0)`.

4. **Response:** `200` with the `*models.DashboardStats` JSON (dereference pointer; if nil, return empty stats object with zero values — **prefer** store never returns nil without error).

5. **Errors:** invalid `since` → `400`; store → `500`.

---

## Testing (`handler_test.go`)

- Use `httptest.NewRecorder` and `httptest.NewRequest` with context.
- Implement a **mock** `db.Store` (struct with function fields or manual stub methods) satisfying `Store` — return canned data for tests; unused methods can `panic("unexpected")` or return zero values.
- Mount handler: `h := NewHandler(mock); srv := httptest.NewServer(h.Routes())` **or** call `h.Routes().ServeHTTP(rec, req)` via `chi` — **simplest:** `rec := httptest.NewRecorder(); h.Routes().ServeHTTP(rec, req)`.

### Required test cases (names illustrative)

| Test | Behavior |
|------|----------|
| Create provider — valid | `POST /providers` with name+base_url → **201**, body has `provider.id` non-empty UUID |
| Create provider — missing name | **400** |
| Get provider — ok | Mock returns provider + endpoints + models → **200**, keys `provider`, `endpoints`, `models` |
| Get provider — not found | **404** |
| Delete provider | **204**, empty body |
| Create alias — valid | **201** |
| List aliases | **200**, `aliases` array matches mock |
| Query logs — filters | Query string sets model/provider_id; mock asserts filter fields |
| Query logs — pagination | No limit → default 50; total returned |
| Stats | `GET /stats?since=24h` → **200**, JSON matches mock `DashboardStats` |

Use `encoding/json` to decode responses in assertions.

---

## Done Criteria

- [ ] All handler functions implemented
- [ ] `Routes()` returns a configured `chi.Router` with the routes above (excluding discover/confirm)
- [ ] JSON helpers: `respondJSON`, `respondError`, `parseDurationParam`
- [ ] Input validation on create/update endpoints
- [ ] HTTP status codes: **201** create, **204** delete, **400** / **404** / **500** as specified
- [ ] `go test ./internal/admin/...` passes
- [ ] `go build ./internal/admin/...` succeeds

---

## Notes for Phase 2 integration

- Mount `NewHandler(store).Routes()` under `/admin` with ACCESS_KEY middleware.
- Do not duplicate `POST /providers/{id}/discover` or `POST /providers/{id}/confirm` in this package; Phase 1E adds them on the same mount prefix.
