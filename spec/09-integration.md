# Spec 09: Integration (Phase 2)

This spec is **self-contained**: all types, interfaces, function signatures, and wiring contracts needed to implement Phase 2 are defined below. Implement exactly these signatures and behaviors unless a follow-up document supersedes this one.

---

## Goal

Wire all components together in `main.go`, set up the chi HTTP router with all route groups, embed the compiled Svelte frontend, implement graceful shutdown, and create build tooling (`Makefile`, `Dockerfile`, `.env.example`, `.gitignore` updates).

---

## Module path

```text
github.com/llmate/gateway
```

---

## Files to Create or Update

| # | Path | Purpose |
|---|------|---------|
| 1 | `cmd/gateway/main.go` | Application entry point: config, logger, DB, router, metrics, HTTP client, handlers, health checker, chi routes, server lifecycle |
| 2 | `cmd/gateway/frontend.go` | `embed.FS` for static assets + `frontendHandler()` SPA file server |
| 3 | `Makefile` | `dev`, `build`, `test`, `docker`, `clean`, and related targets |
| 4 | `Dockerfile` | Multi-stage: frontend build → Go build → minimal runtime image |
| 5 | `.env.example` | Documented environment variables (copy to `.env` locally; never commit secrets) |
| 6 | `.gitignore` | Append entries listed in [Gitignore updates](#gitignore-updates) (merge with existing; do not remove unrelated rules) |

**Optional (if `main.go` becomes unwieldy):** extract `MetricsCollector` into `cmd/gateway/metrics.go` in the same package — only if needed for clarity; this spec treats `MetricsCollector` as part of the gateway command package.

---

## Package structure (imports and constructors)

These packages **must** be imported and wired in `main.go` as follows.

| Import path | Constructor / function | Returns / notes |
|-------------|------------------------|-----------------|
| `github.com/llmate/gateway/internal/config` | `config.Load()` | `(*Config, error)` |
| `github.com/llmate/gateway/internal/db` | `db.NewSQLiteStore(path string)` | `(*SQLiteStore, error)` — implements `db.Store` |
| `github.com/llmate/gateway/internal/proxy` | `proxy.NewSmartRouter(store db.Store)` | `*SmartRouter` — implements `proxy.Router` and `health.CircuitBreakerReporter` (see [Health checker wiring](#health-checker-wiring)) |
| `github.com/llmate/gateway/internal/proxy` | `proxy.NewHandler(router, metrics, store, client)` | `*proxy.Handler` |
| `github.com/llmate/gateway/internal/admin` | `admin.NewHandler(store db.Store)` | `*admin.Handler` |
| `github.com/llmate/gateway/internal/admin` | `admin.NewOnboardHandler(store db.Store, client *http.Client)` | `*admin.OnboardHandler` |
| `github.com/llmate/gateway/internal/auth` | `auth.AccessKeyMiddleware(key string)` | `func(http.Handler) http.Handler` |
| `github.com/llmate/gateway/internal/auth` | `auth.CORSMiddleware()` | `func(http.Handler) http.Handler` |
| `github.com/llmate/gateway/internal/health` | `health.NewChecker(store, breaker, client, interval, logger)` | `*health.Checker` — see signature below |
| `github.com/llmate/gateway/internal/middleware` | `middleware.Logging(logger *slog.Logger)` | `func(http.Handler) http.Handler` |
| `github.com/llmate/gateway/internal/middleware` | `middleware.Recovery(logger *slog.Logger)` | `func(http.Handler) http.Handler` |

### `health.NewChecker` (exact signature — Spec 06)

```go
func NewChecker(
    store db.Store,
    breaker health.CircuitBreakerReporter,
    client *http.Client,
    interval time.Duration,
    logger *slog.Logger,
) *Checker
```

### `health.CircuitBreakerReporter` (inline — must match Spec 06)

```go
type CircuitBreakerReporter interface {
    ReportSuccess(providerID string)
    ReportFailure(providerID string)
}
```

**Health checker wiring:** Pass the `*SmartRouter` returned by `proxy.NewSmartRouter(store)` as `breaker`. It implements `CircuitBreakerReporter` (Spec 03: `(*SmartRouter).ReportSuccess` / `ReportFailure`).

### `proxy.MetricsCollector` interface (inline — must match Spec 02)

The proxy `Handler` expects this interface; your concrete `*MetricsCollector` must implement **only** `Record`:

```go
type MetricsCollector interface {
    Record(log *models.RequestLog)
}
```

Lifecycle methods `Start` and `Close` are **not** part of the interface; they are called from `main` for the async worker.

### `db.Store` subset for metrics worker (inline)

The metrics collector must persist logs via:

```go
InsertRequestLog(ctx context.Context, log *models.RequestLog) error
```

(Signature as in `internal/db/store.go` / Spec 00.)

---

## Config struct (inline reference)

Environment loading is implemented in `internal/config` (Spec 00). The integration layer must use the loaded struct as follows:

```go
type Config struct {
    AccessKey      string
    Port           string
    DBPath         string
    HealthInterval time.Duration
    LogLevel       string
    MaxBodySize    int64
}
```

- **`ACCESS_KEY`:** required for admin routes; empty must fail fast at startup with a clear error (or document explicit dev exception — **prefer** require non-empty in production; `make dev` may set `ACCESS_KEY=dev-key` in the Makefile only).
- **`PORT`:** listen port without leading `:` (e.g. `"8080"`); server address is `":" + config.Port`.
- **`DB_PATH`:** path to SQLite file.
- **`HEALTH_INTERVAL`:** parsed as duration (e.g. `30s`).
- **`LOG_LEVEL`:** one of `debug`, `info`, `warn`, `error` (map to `slog` level; default `info` if unset/invalid).
- **`MAX_BODY_SIZE`:** maximum bytes read from request bodies for applicable handlers; enforce via `http.MaxBytesReader` wrapper in middleware or per-route wrapper **before** handlers run (global middleware on the chi router is acceptable).

---

## `cmd/gateway/main.go`

### Imports (expected)

```go
import (
    "context"
    "embed"
    "log/slog"
    "net/http"
    "os"
    "os/signal"
    "syscall"
    "time"

    "github.com/go-chi/chi/v5"

    "github.com/llmate/gateway/internal/admin"
    "github.com/llmate/gateway/internal/auth"
    "github.com/llmate/gateway/internal/config"
    "github.com/llmate/gateway/internal/db"
    "github.com/llmate/gateway/internal/health"
    "github.com/llmate/gateway/internal/middleware"
    "github.com/llmate/gateway/internal/models"
    "github.com/llmate/gateway/internal/proxy"
)
```

(Adjust if package names differ slightly; keep module path `github.com/llmate/gateway`.)

### Initialization sequence

Execute **in order**:

1. **Load config:** `cfg, err := config.Load()` — on error, log and `os.Exit(1)`.
2. **Logger:** build `*slog.Logger` from `cfg.LogLevel` (text handler to stderr is fine).
3. **Database:** `store, err := db.NewSQLiteStore(cfg.DBPath)` — on error, exit.
4. **Smart router:** `smartRouter := proxy.NewSmartRouter(store)`.
5. **Metrics collector:** `metricsCollector := NewMetricsCollector(store, bufferSize)` with a fixed `bufferSize` (e.g. `1024`); call `metricsCollector.Start(ctx)` after `ctx` for shutdown is created (see below) — **or** start after HTTP server is up; **must** run worker goroutine before accepting traffic that records metrics. Simplest: create root `ctx, cancel := signal.NotifyContext(...)`, derive a child if needed, call `metricsCollector.Start(ctx)` early so `Record` never blocks forever.
6. **HTTP client:** `httpClient := &http.Client{ Timeout: ..., Transport: ... }` with reasonable defaults, e.g.:
   - Overall request timeout: 5–15 minutes for long LLM calls (or no global `Timeout` on client if per-request timeouts are applied in proxy — follow proxy spec; **minimum**: non-zero dial / TLS timeouts via `Transport`).
   - `MaxIdleConns`, `MaxIdleConnsPerHost`, `IdleConnTimeout` set to avoid connection leaks.
7. **Handlers:**
   - `proxyHandler := proxy.NewHandler(smartRouter, metricsCollector, store, httpClient)`
   - `adminHandler := admin.NewHandler(store)`
   - `onboardHandler := admin.NewOnboardHandler(store, httpClient)`
8. **Health checker:** `healthChecker := health.NewChecker(store, smartRouter, httpClient, cfg.HealthInterval, logger)` then `go healthChecker.Start(ctx)` **or** `healthChecker.Start(ctx)` as specified in Spec 06 (if `Start` spawns its own goroutine, do not double-wrap). **Do not** pass `nil` client — if health package substitutes `http.DefaultClient` when nil, still pass a configured client from main for production parity.
9. **Chi router:** build `r` as in [Chi router setup](#chi-router-setup); apply `cfg.MaxBodySize` where appropriate.
10. **HTTP server:** `srv := &http.Server{ Addr: ":" + cfg.Port, Handler: r }`.
11. **Signal handling:** use `signal.NotifyContext` for `os.Interrupt` and `syscall.SIGTERM`.
12. **Listen:** run `srv.ListenAndServe()` in a goroutine; main blocks on `<-ctx.Done()`.
13. **Graceful shutdown:** see [Graceful shutdown](#graceful-shutdown).

### Metrics collector (concrete type)

Implement in package `main` (same package as `frontend.go`).

```go
type MetricsCollector struct {
    store db.Store
    ch    chan *models.RequestLog
    done  chan struct{}
}

func NewMetricsCollector(store db.Store, bufferSize int) *MetricsCollector

func (m *MetricsCollector) Record(log *models.RequestLog)
```

- **`Record`:** non-blocking on the hot path: send `log` to `ch`. If channel is full, **drop** with a debug log or increment a counter — **must not** block proxy indefinitely (match Spec 02 intent). Alternative: use `select` with `default` to drop.

```go
func (m *MetricsCollector) Start(ctx context.Context)
```

- Spawn a **single** worker goroutine that:
  - Loops: `select` on `ctx.Done()`, or receive from `ch`.
  - On receive: call `store.InsertRequestLog(ctx, log)` with a **timeout-bound** context if desired (e.g. 5s) to avoid hanging shutdown; on error, log at `Warn`/`Error`.

```go
func (m *MetricsCollector) Close()
```

- Signal shutdown: close `ch` or use `done` to stop worker after draining `ch` (drain all pending logs before returning). **Order:** called during graceful shutdown **before** `srv.Shutdown` or in parallel with a short timeout — **must** flush or best-effort persist remaining items.

**Implements:** `proxy.MetricsCollector` via `Record` only.

---

## Chi router setup

```go
r := chi.NewRouter()

// Global middleware (order matters: recovery outermost, then logging, then CORS)
r.Use(middleware.Recovery(logger))
r.Use(middleware.Logging(logger))
r.Use(auth.CORSMiddleware())

// Optional: wrap with MaxBytesReader middleware using cfg.MaxBodySize if not applied inside handlers
```

### OpenAI-compatible proxy routes (**no** ACCESS_KEY)

Register on `r` directly (top-level):

| Method | Path | Handler |
|--------|------|---------|
| POST | `/v1/chat/completions` | `proxyHandler.HandleChatCompletions` |
| POST | `/v1/completions` | `proxyHandler.HandleCompletions` |
| POST | `/v1/embeddings` | `proxyHandler.HandleEmbeddings` |
| POST | `/v1/images/generations` | `proxyHandler.HandleImageGenerations` |
| POST | `/v1/audio/speech` | `proxyHandler.HandleAudioSpeech` |
| POST | `/v1/audio/transcriptions` | `proxyHandler.HandleAudioTranscriptions` |
| GET | `/v1/models` | `proxyHandler.HandleListModels` |
| GET | `/v1/models/{model}` | `proxyHandler.HandleGetModel` |

### Admin routes (**ACCESS_KEY** required)

```go
r.Route("/admin", func(r chi.Router) {
    r.Use(auth.AccessKeyMiddleware(cfg.AccessKey))

    // Register specific onboarding routes BEFORE Mount so they are not shadowed
    r.Post("/providers/{id}/discover", onboardHandler.HandleDiscover)
    r.Post("/providers/{id}/confirm", onboardHandler.HandleConfirm)

    r.Mount("/", adminHandler.Routes())
})
```

**Note:** If `adminHandler.Routes()` already registers conflicting paths, merge per admin spec — **chi** matches more specific routes first when registered in order; placing `Post` routes before `Mount` avoids the catch-all subrouter swallowing them.

### Frontend (embedded static files + SPA fallback)

```go
r.Handle("/*", frontendHandler())
```

**Important:** Register the catch-all **last** so API routes take precedence.

---

## `cmd/gateway/frontend.go`

### Embed directive

The frontend build output is copied to `cmd/gateway/frontend_dist/` by the Makefile before `go build`. Embed that directory:

```go
//go:embed frontend_dist/*
var frontendFS embed.FS
```

If the repository prefers embedding from `frontend/build` via a relative path without copy step, the Makefile **must** still guarantee embed paths exist at build time; **this spec** standardizes on **`cmd/gateway/frontend_dist/`** populated by `make build`.

### `frontendHandler` signature

```go
func frontendHandler() http.Handler
```

**Behavior:**

1. Use `http.FileServer` with `http.FS` wrapping a **subtree** of `frontendFS` rooted at `frontend_dist` (e.g. `fs.Sub(frontendFS, "frontend_dist")`).
2. For requests where the file does not exist (or path is a “directory” without `index.html`), serve **`index.html`** from the root of the static assets (SPA fallback) so client-side routing works.
3. Typical pattern: wrap `http.FileServer` with a handler that checks `os.IsNotExist` or stat failure and falls back to `index.html` for `GET` (and optionally `HEAD`).
4. Do not serve `index.html` for API prefixes (`/v1`, `/admin`) — those are registered above this route.

---

## Graceful shutdown

```go
ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
defer cancel()

srv := &http.Server{
    Addr:    ":" + cfg.Port,
    Handler: r,
}

go func() {
    logger.Info("server starting", "port", cfg.Port)
    if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
        logger.Error("server error", "error", err)
        os.Exit(1)
    }
}()

<-ctx.Done()
logger.Info("shutting down...")

shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
defer shutdownCancel()

// 1. Stop accepting new metrics: close/drain metrics collector
metricsCollector.Close()

// 2. Cancel health checker context (if healthChecker.Start uses ctx passed from main, canceling root ctx achieves this — ensure shared ctx is the same signal ctx or cancel a dedicated child after signal)
cancel() // if not already done by signal
// If health checker uses a separate context, cancel that explicitly.

// 3. Shutdown HTTP server
if err := srv.Shutdown(shutdownCtx); err != nil {
    logger.Error("server shutdown", "error", err)
}

// 4. Close database
if err := store.Close(); err != nil {
    logger.Error("store close", "error", err)
}
```

**Requirements:**

- Shutdown timeout: **30 seconds** (`shutdownCtx`).
- On `SIGINT` / `SIGTERM`, trigger the sequence above.
- `healthChecker`: pass the **same** `ctx` from `signal.NotifyContext` into `healthChecker.Start(ctx)` so when the signal arrives, `ctx` is cancelled and the checker exits (per Spec 06).

**Ordering nuance:** Start `metricsCollector` with the signal `ctx` so `Start` exits when context is cancelled; call `Close()` during shutdown to flush the channel.

---

## Makefile

Create `Makefile` at repository root with at least:

```makefile
.PHONY: dev dev-frontend dev-backend build test clean docker

# Development
dev: dev-backend dev-frontend

dev-backend:
	ACCESS_KEY=dev-key go run ./cmd/gateway/

dev-frontend:
	cd frontend && npm run dev

# Build
build: build-frontend build-backend

build-frontend:
	cd frontend && npm ci && npm run build

build-backend: build-frontend
	mkdir -p cmd/gateway/frontend_dist
	cp -r frontend/build/* cmd/gateway/frontend_dist/
	go build -o bin/gateway ./cmd/gateway/

# Test
test:
	go test ./...

test-frontend:
	cd frontend && npm run check

# Docker
docker:
	docker build -t llmate .

# Clean
clean:
	rm -rf bin/ cmd/gateway/frontend_dist/ frontend/build/
```

**Notes:**

- `dev` runs backend and frontend; in practice developers may run two terminals — document that `make dev` may run both (if parallel targets are problematic on some shells, splitting is acceptable; **goal** is reproducible commands).
- `build-backend` depends on `build-frontend` so assets exist before `cp`.
- Ensure `go build` runs from module root with `frontend_dist` populated.

---

## Dockerfile

Multi-stage build at repository root:

```dockerfile
# Stage 1: Build frontend
FROM node:20-alpine AS frontend
WORKDIR /app/frontend
COPY frontend/package*.json ./
RUN npm ci
COPY frontend/ .
RUN npm run build

# Stage 2: Build backend
FROM golang:1.22-alpine AS backend
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=frontend /app/frontend/build cmd/gateway/frontend_dist/
RUN CGO_ENABLED=0 go build -o /gateway ./cmd/gateway/

# Stage 3: Runtime
FROM alpine:3.19
RUN apk add --no-cache ca-certificates
COPY --from=backend /gateway /usr/local/bin/gateway
EXPOSE 8080
ENTRYPOINT ["gateway"]
```

**Requirements:**

- `CGO_ENABLED=0` for static binary (matches `modernc.org/sqlite` / no CGo).
- Copy **only** built frontend into `cmd/gateway/frontend_dist/` before `go build` so `//go:embed` matches.
- Default `EXPOSE 8080` — align with `PORT` default in `.env.example`.
- Runtime must set `ACCESS_KEY` and optional vars via orchestration (`docker run -e`).

---

## `.env.example`

```env
# Required
ACCESS_KEY=your-secret-key-here

# Optional
PORT=8080
DB_PATH=./llmate.db
HEALTH_INTERVAL=30s
LOG_LEVEL=info
MAX_BODY_SIZE=10485760
```

Document in comments (in file or README — **this spec** requires the variables above to appear in `.env.example`):

| Variable | Purpose |
|----------|---------|
| `ACCESS_KEY` | Bearer token for `/admin/*` routes |
| `PORT` | HTTP listen port |
| `DB_PATH` | SQLite database file path |
| `HEALTH_INTERVAL` | Period between health check ticks |
| `LOG_LEVEL` | `slog` level |
| `MAX_BODY_SIZE` | Max request body size (bytes) for protected handlers / global limit |

---

## `.gitignore` updates

**Append** (merge with existing `.gitignore`; do not delete unrelated entries):

```gitignore
# Build artifacts
bin/
cmd/gateway/frontend_dist/
frontend/build/
frontend/node_modules/
frontend/.svelte-kit/

# Database
*.db
*.db-journal
*.db-wal
*.db-shm

# Environment
.env

# IDE
.cursor/mcp.json
```

---

## Done criteria

- [ ] `main.go` initializes all components in the [Initialization sequence](#initialization-sequence) order (adjusting only where a strict order is impossible — document in code if so).
- [ ] Chi router registers all proxy routes **without** auth and admin routes **with** `AccessKeyMiddleware`.
- [ ] Onboarding routes `/admin/providers/{id}/discover` and `/admin/providers/{id}/confirm` are reachable (registered before `Mount`).
- [ ] `MetricsCollector` buffers via channel and flushes with `InsertRequestLog` asynchronously; `Record` does not block the proxy hot path indefinitely.
- [ ] Embedded frontend serves static files and falls back to `index.html` for unknown paths (SPA).
- [ ] Graceful shutdown on `SIGINT`/`SIGTERM` with **30s** `Shutdown` timeout; metrics drained/closed; health checker stops via context; `store.Close()` called.
- [ ] `Makefile` provides `dev`, `build`, `test`, `docker`, `clean` (and `test-frontend` / `dev-frontend` / `dev-backend` as specified).
- [ ] `Dockerfile` multi-stage build produces a runnable image with `gateway` binary.
- [ ] `.env.example` lists all configuration variables with sensible defaults documented.
- [ ] `.gitignore` includes build artifacts, DB files, `.env`, and listed IDE entry.
- [ ] `make build` completes successfully end-to-end (frontend install + build, copy to `frontend_dist`, `go build`).

---

## Verification commands

```bash
make build
go test ./...
cd frontend && npm run check
```

(Docker image: `make docker` then `docker run --rm -e ACCESS_KEY=test -p 8080:8080 llmate` — optional manual check.)

---

## Non-goals (Phase 2)

- Changing Phase 1 handler internals or Store schema.
- Kubernetes manifests or CI YAML (unless added in a later spec).
