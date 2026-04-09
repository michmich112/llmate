# Spec 06: Health Monitor + HTTP Middleware (Phase 1F)

This spec is **self-contained**: all types, interfaces, and behavioral contracts needed to implement Phase 1F are defined below. Implement exactly these signatures and behaviors unless a follow-up spec supersedes this document.

---

## Goal

Implement:

1. A **background goroutine** that periodically health-checks all registered providers, updates persisted health in the store, and feeds outcomes into the **circuit breaker** (Phase 1C).
2. **Cross-cutting HTTP middleware** for request logging and panic recovery.

---

## Files to Create

| File | Purpose |
|------|---------|
| `internal/health/checker.go` | Background health check goroutine |
| `internal/health/checker_test.go` | Health checker tests |
| `internal/middleware/logging.go` | HTTP request logging middleware |
| `internal/middleware/recovery.go` | Panic recovery middleware |

---

## Module path

Use the project module path consistently with other specs:

```text
github.com/llmate/gateway
```

---

## Part A: Health Checker (`internal/health/checker.go`)

### Package

```go
package health
```

### Dependencies

```go
import (
    "context"
    "net/http"
    "log/slog"
    "sync"
    "time"

    "github.com/llmate/gateway/internal/db"
    "github.com/llmate/gateway/internal/models"
)
```

### CircuitBreakerReporter

The health checker feeds probe results into the circuit breaker without importing its concrete type. Define this **minimal** interface in `checker.go`:

```go
type CircuitBreakerReporter interface {
    ReportSuccess(providerID string)
    ReportFailure(providerID string)
}
```

- `ReportSuccess` is called when the probe succeeds (HTTP 2xx from `GET {BaseURL}/v1/models`).
- `ReportFailure` is called when the probe fails (network error, timeout, or non-2xx status).

### Store contract (methods used)

The checker uses only these `db.Store` methods (signatures as implemented in the SQLite store / Phase 1B):

```go
ListProviders(ctx context.Context) ([]models.Provider, error)
UpdateProviderHealth(ctx context.Context, id string, healthy bool) error
```

### Provider type (reference)

`models.Provider` matches this shape (for readers of this spec; do not redefine in `health`):

```go
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
```

### Checker struct

```go
type Checker struct {
    store    db.Store
    breaker  CircuitBreakerReporter
    client   *http.Client
    interval time.Duration
    logger   *slog.Logger
}
```

- `client`: used for outbound health probes. Callers may pass a shared `http.DefaultClient` or a custom client (timeouts, TLS, etc.). Per-request timeout is still applied in `checkProvider` (see below).
- `interval`: period between tick-driven runs after the initial run.
- `logger`: must not be nil in production; tests may pass `slog.New(slog.DiscardHandler)` or a test handler.

### Constructor

```go
func NewChecker(store db.Store, breaker CircuitBreakerReporter, client *http.Client, interval time.Duration, logger *slog.Logger) *Checker
```

- If `client` is nil, use `http.DefaultClient` (document this behavior in code with a one-line comment if needed).
- Store `interval` as given; `Start` validates usage (see below).

### Start

```go
func (c *Checker) Start(ctx context.Context)
```

Behavior:

1. If `ctx` is already cancelled, return immediately without starting a goroutine (optional but recommended).
2. If `c.interval <= 0`, treat as invalid: log a warning once and return without starting (or use a safe default—**prefer**: log and return to avoid tight loops).
3. Spawn a **single** background goroutine that:
   - Runs `c.checkAll(ctx)` **immediately** once (first health pass on startup).
   - Then uses `time.NewTicker(c.interval)` to call `c.checkAll(ctx)` on each tick.
   - On `ctx.Done()`, stops the ticker, drains if needed, and returns from the goroutine.

**Concurrency:** Only one `checkAll` should run at a time. If a tick fires while `checkAll` is still running, skip the overlapping tick or wait—**simplest acceptable approach**: run checks sequentially per tick; inside `checkAll`, providers are checked concurrently (see `checkAll`).

**Logging:** At `Info` when the checker goroutine starts and stops (optional); errors from `checkAll` should be logged at `Warn` or `Error` with context.

### checkAll

```go
func (c *Checker) checkAll(ctx context.Context)
```

1. Call `providers, err := c.store.ListProviders(ctx)`. On error, log and return (do not panic).
2. For each provider in `providers`, launch `checkProvider` in its own goroutine; use `sync.WaitGroup` to wait for all to finish.
3. Log a summary at `Debug` (optional): count of providers checked.

**Note:** If `ctx` is cancelled mid-flight, individual `checkProvider` calls should respect context cancellation on the outbound HTTP request (see `checkProvider`).

### checkProvider

```go
func (c *Checker) checkProvider(ctx context.Context, p models.Provider)
```

1. Build request URL: **string concatenation** of `p.BaseURL` and `"/v1/models"` (normalize `BaseURL` if it has a trailing slash—avoid double slashes: trim trailing `/` from `BaseURL` before appending `"/v1/models"`, or use `strings.TrimSuffix` + join).
2. Create `GET` request with `http.NewRequestWithContext` using a **child context** with **10 second** timeout:  
   `context.WithTimeout(ctx, 10*time.Second)`.
3. If `p.APIKey != ""`, set header: `Authorization: Bearer <p.APIKey>`.
4. Execute with `c.client.Do(req)`.
5. **Success path (2xx):**
   - Ensure response body is fully consumed and closed (`io.Copy(io.Discard, resp.Body)` then `resp.Body.Close()`) to allow connection reuse.
   - Call `c.store.UpdateProviderHealth(ctx, p.ID, true)` — use the same `ctx` as passed in, or the timeout ctx if store honors cancellation (either is acceptable; prefer the timeout ctx for the HTTP portion and parent `ctx` for store if shutdown should abort updates quickly).
   - Call `c.breaker.ReportSuccess(p.ID)`.
   - Log at **Debug**: message `"provider healthy"`, include `slog.String("name", p.Name)` and `slog.String("provider_id", p.ID)`.
6. **Failure path** (network error, timeout, or status not in 2xx):
   - On non-nil `resp`, read/discard body and close it.
   - Call `c.store.UpdateProviderHealth(ctx, p.ID, false)`.
   - Call `c.breaker.ReportFailure(p.ID)`.
   - Log at **Warn**: message `"provider unhealthy"`, include `slog.String("name", p.Name)`, error detail (`slog.String("error", err.Error())` or status code if HTTP error without Go error).

**Idempotency:** Multiple failures/successes in a row are fine; store and breaker must reflect latest probe.

---

## Part B: Tests (`internal/health/checker_test.go`)

### Approach

- Use `net/http/httptest` **NewServer** for mock backends that return 200, 500, etc.
- Implement a **mock `db.Store`** (or minimal stub with only `ListProviders` / `UpdateProviderHealth`) recording calls.
- Implement a **mock `CircuitBreakerReporter`** recording `ReportSuccess` / `ReportFailure` per `providerID`.
- Pass a real `*http.Client` pointing at `httptest` server URLs by setting each mock provider’s `BaseURL` to the server URL.

### Required test cases

1. **Healthy provider:** mock server returns `200` for `GET /v1/models` → `UpdateProviderHealth(id, true)` called once; `ReportSuccess(id)` once; no failure calls.
2. **Unhealthy provider (HTTP 500):** mock returns `500` → `UpdateProviderHealth(id, false)`; `ReportFailure(id)`; no success.
3. **Unreachable:** `BaseURL` to `http://127.0.0.1:1` (or unused port) → connection refused → unhealthy + `ReportFailure`.
4. **Concurrent providers:** two `httptest` servers (or paths); `ListProviders` returns both; assert both health updates and breaker calls after `checkAll` completes (WaitGroup in implementation ensures completion).
5. **Context cancellation:** start `Checker.Start` with a cancellable context; cancel after first `checkAll` or use short interval; assert goroutine exits without deadlock (e.g. use `sync.WaitGroup` in test or channel signal when goroutine returns—**simplest**: run `checkAll` directly with cancelled ctx and expect no hang / skipped work).
6. **API key:** provider with non-empty `APIKey`; handler verifies `Authorization: Bearer <token>` header on `GET /v1/models`.

Use table-driven tests where it reduces duplication.

---

## Part C: Logging Middleware (`internal/middleware/logging.go`)

### Package

```go
package middleware
```

### Imports (typical)

```go
import (
    "log/slog"
    "net"
    "net/http"
    "strings"
    "time"
)
```

### statusRecorder

```go
type statusRecorder struct {
    http.ResponseWriter
    statusCode int
}

func (sr *statusRecorder) WriteHeader(code int) {
    sr.statusCode = code
    sr.ResponseWriter.WriteHeader(code)
}
```

- Default `statusCode` should be treated as **200** if `WriteHeader` is never called (middleware should initialize to `http.StatusOK` when wrapping, or before logging if still 0).
- If the wrapped `ResponseWriter` implements `http.Flusher`, `http.Hijacker`, etc., **optional**: embed and forward—minimum requirement is correct status logging for normal handlers.

### Logging

```go
func Logging(logger *slog.Logger) func(http.Handler) http.Handler
```

For each request:

1. Record `start := time.Now()`.
2. Wrap `w` with `statusRecorder`; default `statusCode` to `http.StatusOK`.
3. Call `next.ServeHTTP` on the wrapper.
4. Compute `durationMs` as `time.Since(start).Milliseconds()` (integer).
5. Resolve **client IP**:
   - If `X-Forwarded-For` is present, use the **first** comma-separated IP (trim space).
   - Else parse `r.RemoteAddr` with `net.SplitHostPort`; on error use raw `RemoteAddr`.
6. Log one line at **Info**:

```go
slog.Info("request",
    "method", r.Method,
    "path", r.URL.Path,
    "status", statusCode,
    "duration_ms", durationMs,
    "client_ip", clientIP,
)
```

Use the provided `logger` argument (not the global default), unless `logger` is nil—if nil, **no-op** or use `slog.Default()` (pick one and document in a short comment).

---

## Part D: Recovery Middleware (`internal/middleware/recovery.go`)

### Package

```go
package middleware
```

### Imports (typical)

```go
import (
    "encoding/json"
    "log/slog"
    "net/http"
    "runtime/debug"
)
```

### Recovery

```go
func Recovery(logger *slog.Logger) func(http.Handler) http.Handler
```

For each request:

1. `defer` a function that calls `recover()`.
2. If `recover()` returns non-nil `v`:
   - Log at **Error** with panic value and **stack**: e.g. `slog.Error("panic", "panic", v, "stack", string(debug.Stack()))` or equivalent.
   - Write status **500**.
   - Write JSON body: `{"error":"internal server error"}` with `Content-Type: application/json`.
3. **Do not** re-panic.

If response was already partially written, logging still occurs; avoid double-write if possible (standard pattern: only write 500 if `recover` catches before headers sent—acceptable v1 behavior: always attempt JSON 500).

---

## Integration notes (non-normative)

- Wire `Logging` and `Recovery` in `cmd/gateway/main.go` **outside** Chi routes so all requests are covered; order typically **Recovery** outermost, then **Logging**, then router (or Logging outer—either works; document chosen order: **Recovery first** so panics are logged and converted to 500).
- Start `Checker.Start` with the same `context.Context` used for server shutdown.

---

## Done criteria

- [ ] Health checker runs as a background goroutine with configurable interval.
- [ ] Health checks probe `GET /v1/models` on each provider (with optional `Authorization: Bearer` when API key set).
- [ ] Results update the store (`UpdateProviderHealth`) and circuit breaker (`ReportSuccess` / `ReportFailure`).
- [ ] Graceful shutdown via context cancellation (ticker stopped, goroutine exits).
- [ ] Logging middleware records method, path, status, duration (ms), client IP (`X-Forwarded-For` or `RemoteAddr`).
- [ ] Recovery middleware catches panics, logs stack, returns 500 with JSON error body, does not re-panic.
- [ ] `go test ./internal/health/...` passes.
- [ ] `go build ./internal/health/...` succeeds.
- [ ] `go build ./internal/middleware/...` succeeds.
