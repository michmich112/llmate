# LLMate -- Project Context

**LLMate** (*LLM available to everyone*) is a self-hosted LLM gateway.

## What Is This?

LLMate sits between your applications and your locally-hosted LLM backends (vLLM, llama.cpp, Ollama, etc.), providing:

1. **OpenAI-compatible API** -- applications and SDKs that speak to OpenAI can point at LLMate instead, with zero code changes.
2. **Smart routing** -- requests are routed to healthy backends using weighted, priority-based selection with automatic circuit-breaker failover.
3. **Admin dashboard** -- a Svelte 5 web UI for onboarding providers, configuring model aliases, and viewing request analytics.
4. **Request analytics** -- every proxied request is logged (metadata only, not payload data) with timing, token usage, and routing info.

## Architecture

```
┌─────────────────────────────────────────────────────────────────────┐
│                         LLMate Gateway                              │
│                                                                     │
│  ┌──────────────┐    ┌──────────────┐    ┌───────────────────────┐ │
│  │  Proxy        │───▶│ Smart Router  │───▶│ Circuit Breaker      │ │
│  │  Handlers     │    │              │    │ (per-provider)        │ │
│  │  /v1/*        │    │ - aliases    │    │                       │ │
│  └──────┬───────┘    │ - weights    │    │ Closed→Open→HalfOpen  │ │
│         │            │ - priority   │    └───────────┬───────────┘ │
│         │            │ - failover   │                │             │
│         ▼            └──────────────┘                ▼             │
│  ┌──────────────┐                          ┌─────────────────┐    │
│  │ Metrics       │                          │  LLM Backends   │    │
│  │ Tracker       │                          │  (vLLM, Ollama, │    │
│  │ (async write) │                          │   llama.cpp)    │    │
│  └──────┬───────┘                          └─────────────────┘    │
│         │                                          ▲               │
│         ▼                                          │               │
│  ┌──────────────┐    ┌──────────────┐    ┌────────┴──────────┐   │
│  │  SQLite       │◀───│ Admin API    │    │ Health Monitor    │   │
│  │  Store        │    │ /admin/*     │    │ (background)      │   │
│  └──────────────┘    └──────┬───────┘    └───────────────────┘   │
│                             │                                      │
│                      ┌──────┴───────┐                             │
│                      │ Auth          │                             │
│                      │ Middleware    │                             │
│                      │ (ACCESS_KEY)  │                             │
│                      └──────┬───────┘                             │
│                             │                                      │
│                      ┌──────┴───────┐                             │
│                      │ Svelte 5     │                             │
│                      │ Dashboard    │                             │
│                      │ (embedded)   │                             │
│                      └──────────────┘                             │
└─────────────────────────────────────────────────────────────────────┘
```

## Key Design Decisions

### Single Binary Deployment
The Go backend embeds the compiled Svelte frontend as static files using `embed.FS`. This produces a single binary that can be deployed anywhere. Docker images are also supported.

### SQLite for Storage
SQLite is chosen because:
- Zero external dependencies (no database server to run)
- Single file, easy to back up
- More than sufficient for single-instance gateway workloads
- The `Store` interface abstracts the database, so migration to Postgres is possible later

### Contract-First Development
All domain types, the Store interface, and the admin API contract are defined before any implementation begins. This allows parallel agent development with zero inter-agent dependencies.

### No Auth on Proxy Routes
The proxy API (`/v1/*`) has no authentication. The gateway is designed to run on a trusted network. Only the admin API (`/admin/*`) and dashboard require the `ACCESS_KEY`.

### Circuit Breaker Over Simple Health Checks
Instead of a binary healthy/unhealthy flag, each provider has a circuit breaker with three states (Closed, Open, Half-Open). This provides automatic recovery and prevents cascading failures when a backend becomes flaky.

### Async Request Logging
Request metrics are written to SQLite asynchronously via a buffered channel. This prevents database writes from adding latency to proxied requests.

## Data Model

### Core Entities

- **Provider**: A backend LLM service (e.g., a vLLM instance at `http://gpu-server:8000`). Has a `base_url`, optional `api_key`, and health state.
- **ProviderEndpoint**: A specific API capability on a provider (e.g., POST `/v1/chat/completions`). Discovered during onboarding, can be enabled/disabled by the user.
- **ProviderModel**: A model available on a provider (e.g., `llama-3.1-70b`). Discovered from the provider's `/v1/models` endpoint.
- **ModelAlias**: Maps a virtual model name to a real (provider, model) pair. Multiple aliases with the same name but different providers enable load balancing. Has `weight` and `priority` for smart routing.
- **RequestLog**: Metadata about each proxied request -- timing, tokens, routing decisions, errors. No request/response payloads are stored.

### Relationships

```
Provider 1──N ProviderEndpoint
Provider 1──N ProviderModel
Provider 1──N ModelAlias
ModelAlias N──1 Provider (with model_id referencing a ProviderModel.model_id)
RequestLog N──1 Provider
```

## API Surface

### Proxy API (no auth, OpenAI-compatible)

| Method | Path | Notes |
|--------|------|-------|
| POST | `/v1/chat/completions` | Streaming + non-streaming |
| POST | `/v1/completions` | Legacy completions |
| POST | `/v1/embeddings` | |
| POST | `/v1/images/generations` | |
| POST | `/v1/audio/speech` | Binary response |
| POST | `/v1/audio/transcriptions` | Multipart upload |
| GET | `/v1/models` | Aggregated from all providers + aliases |
| GET | `/v1/models/{model}` | |

### Admin API (ACCESS_KEY required)

| Method | Path | Notes |
|--------|------|-------|
| POST | `/admin/auth` | Validate key for dashboard login |
| GET/POST | `/admin/providers` | List / create providers |
| GET/PUT/DELETE | `/admin/providers/:id` | Provider CRUD |
| POST | `/admin/providers/:id/discover` | Probe provider capabilities |
| POST | `/admin/providers/:id/confirm` | Save discovery results |
| PUT | `/admin/providers/:id/endpoints/:eid` | Toggle endpoint |
| GET/POST | `/admin/aliases` | List / create model aliases |
| PUT/DELETE | `/admin/aliases/:id` | Alias CRUD |
| GET | `/admin/logs` | Query request logs |
| GET | `/admin/stats` | Dashboard statistics |

## Technology Stack

| Component | Technology |
|-----------|-----------|
| Backend language | Go 1.22+ |
| HTTP router | chi v5 |
| Database | libSQL/Turso (default) or SQLite, via `modernc.org/sqlite` / Turso's `libsql` client. Legacy SQLite data is migrated into a new/empty Turso DB on first boot. |
| Frontend framework | SvelteKit + Svelte 5 |
| UI components | shadcn-svelte |
| Styling | Tailwind CSS |
| Frontend adapter | `@sveltejs/adapter-static` (SPA mode) |
| Deployment | Single binary (embedded frontend) or Docker |

## Configuration

All configuration is via environment variables:

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `ACCESS_KEY` | Yes | -- | Key for admin dashboard and admin API access |
| `PORT` | No | `8080` | HTTP listen port |
| `DB_PATH` | No | `./llmate.db` | Path to SQLite database file |
| `HEALTH_INTERVAL` | No | `30s` | Interval between health checks |
| `LOG_LEVEL` | No | `info` | Log level (debug, info, warn, error) |
| `MAX_BODY_SIZE` | No | `10485760` | Max request body size in bytes (10MB) |

## Directory Structure

```
llmate/
├── cmd/gateway/main.go              # Entry point
├── internal/
│   ├── config/config.go             # Environment config
│   ├── models/                      # Domain types (no logic)
│   ├── db/                          # Store interface + SQLite impl
│   │   └── migrations/              # SQL migration files
│   ├── proxy/                       # OpenAI-compat proxy + routing
│   ├── admin/                       # Admin API handlers
│   ├── auth/                        # ACCESS_KEY middleware
│   ├── health/                      # Background health checker
│   └── middleware/                   # Logging, recovery
├── frontend/                        # SvelteKit app
│   ├── src/
│   │   ├── routes/                  # SvelteKit routes
│   │   └── lib/                     # Shared code
│   └── build/                       # Compiled output (embedded by Go)
├── spec/                            # Implementation specs
├── context/                         # Reference material
├── Context.md                       # This file
└── AGENTS.md                        # Agent coding rules
```

## Request Flow

1. Client sends request to `/v1/chat/completions` with `{"model": "gpt-4", ...}`
2. Proxy handler parses body, extracts `model` field
3. Smart router resolves `gpt-4`:
   a. Check model_aliases for `gpt-4` -> finds aliases pointing to (provider_A, llama-3.1-70b) and (provider_B, llama-3.1-70b)
   b. If no alias, look up providers that have a model named `gpt-4` directly
4. Filter to providers that are healthy (circuit breaker in Closed or Half-Open state) and have `/v1/chat/completions` enabled
5. Select provider: highest priority first, then weighted random within same priority tier
6. Forward request to selected provider's `base_url + /v1/chat/completions`
7. If streaming (`stream: true`): pipe SSE chunks to client, capture TTFT on first chunk
8. If non-streaming: read full response, forward to client
9. Parse usage from response (prompt_tokens, completion_tokens, etc.)
10. Write request log asynchronously to SQLite
11. On provider failure: record error in circuit breaker, failover to next candidate, retry from step 6

## Onboarding Flow

1. User provides provider name and base URL in dashboard
2. Gateway calls `GET {base_url}/v1/models` to discover available models
3. Gateway probes supported endpoints with minimal test requests
4. Results shown in dashboard: discovered models and endpoint support matrix
5. User reviews, toggles endpoints, confirms
6. Provider, models, and endpoints saved to database
7. Health monitor begins periodic checks on the new provider
