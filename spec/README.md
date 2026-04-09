# Implementation Specs

This directory contains the implementation specifications for the LLMate (*LLM available to everyone*) LLM gateway. Each spec is a self-contained document that an agent can use to implement its assigned subsystem without reading any other spec.

## Execution Order

### Phase 0: Contracts (must complete before Phase 1)

| Spec | Description | Est. Lines | Files |
|------|-------------|-----------|-------|
| [00-contracts.md](00-contracts.md) | Go module, domain types, Store interface, DB migrations, config, TS types | ~400 | `go.mod`, `internal/models/*`, `internal/db/store.go`, `internal/db/migrations/*`, `internal/config/config.go`, `frontend/src/lib/types/index.ts`, `context/openai-proxy-endpoints.md` |

### Phase 1: Implementation (all run in parallel after Phase 0)

| Spec | Agent | Description | Est. Lines | Files |
|------|-------|-------------|-----------|-------|
| [01-sqlite-store.md](01-sqlite-store.md) | 1A | SQLite Store implementation | ~500-600 | `internal/db/sqlite.go`, `internal/db/sqlite_test.go` |
| [02-proxy-handlers.md](02-proxy-handlers.md) | 1B | OpenAI-compat proxy handlers + SSE streaming | ~500-600 | `internal/proxy/handler.go`, `internal/proxy/streaming.go`, `internal/proxy/handler_test.go` |
| [03-router-circuit-breaker.md](03-router-circuit-breaker.md) | 1C | Smart routing + circuit breaker | ~400-500 | `internal/proxy/router.go`, `internal/proxy/circuit.go`, `internal/proxy/router_test.go`, `internal/proxy/circuit_test.go` |
| [04-admin-api.md](04-admin-api.md) | 1D | Admin CRUD handlers + stats | ~400-500 | `internal/admin/handler.go`, `internal/admin/stats.go`, `internal/admin/handler_test.go` |
| [05-auth-onboarding.md](05-auth-onboarding.md) | 1E | ACCESS_KEY middleware + provider discovery | ~300-400 | `internal/auth/middleware.go`, `internal/auth/middleware_test.go`, `internal/admin/onboard.go`, `internal/admin/onboard_test.go` |
| [06-health-monitor.md](06-health-monitor.md) | 1F | Background health checker + HTTP middleware | ~300-350 | `internal/health/checker.go`, `internal/health/checker_test.go`, `internal/middleware/logging.go`, `internal/middleware/recovery.go` |
| [07-frontend-core.md](07-frontend-core.md) | 1G | SvelteKit setup, layouts, login, API client | ~700-900 | `frontend/` scaffold, layouts, login page, `src/lib/api/client.ts`, `src/lib/components/*` |
| [08-frontend-pages.md](08-frontend-pages.md) | 1H | Dashboard pages (overview, providers, models, logs) | ~800-1000 | `frontend/src/routes/(dashboard)/*` |

### Phase 2: Integration (after all Phase 1 agents complete)

| Spec | Description | Est. Lines | Files |
|------|-------------|-----------|-------|
| [09-integration.md](09-integration.md) | Wire everything in main.go, Makefile, Dockerfile | ~300-400 | `cmd/gateway/main.go`, `Makefile`, `Dockerfile`, `.env.example` |

## Context Budget Notes (Qwen3-Coder-Next, 250k tokens)

Each spec is designed to fit within the effective working capacity of a 250k-token model:

- **Spec input**: ~40-50k tokens (types, interfaces, signatures, behavior descriptions)
- **Code reads**: ~30-40k tokens (reading files from disk during implementation)
- **Code output**: ~60-80k tokens (writing files)
- **Reasoning**: ~50-60k tokens (model's internal chain of thought)
- **Overhead**: ~20-30k tokens (tool calls, linter output, shell output)

### Spec Design Principles

1. **Self-contained**: Every spec includes ALL types, interfaces, and function signatures the agent needs. The agent never needs to read other specs, `Context.md`, or `AGENTS.md` to do its work.
2. **Exact file paths**: No ambiguity about where code goes.
3. **Function-level signatures**: Every exported function is pre-defined.
4. **Behavioral descriptions**: Clear "what this function does" without pseudocode.
5. **Test outlines**: Key test cases listed.
6. **Done criteria**: Concrete checklist for self-verification.
7. **No external references**: All needed information is inline.

## Dependency Graph

```
Phase 0 (00-contracts)
    │
    ├── Phase 1A (01-sqlite-store)
    ├── Phase 1B (02-proxy-handlers)
    ├── Phase 1C (03-router-circuit-breaker)
    ├── Phase 1D (04-admin-api)
    ├── Phase 1E (05-auth-onboarding)
    ├── Phase 1F (06-health-monitor)
    ├── Phase 1G (07-frontend-core)
    └── Phase 1H (08-frontend-pages)
            │
            └── Phase 2 (09-integration)
```

No Phase 1 agent depends on any other Phase 1 agent. They all depend only on Phase 0 outputs.
