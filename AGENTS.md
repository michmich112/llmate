# AI Agent Coding Rules

Rules and conventions for AI agents working on the **LLMate** (*LLM available to everyone*) codebase. Follow these strictly when adding, modifying, or reviewing code.

## Project Overview

LLMate is an LLM gateway that exposes an OpenAI-compatible API, routes requests to self-hosted LLM backends with smart routing and circuit breaking, and provides a Svelte 5 dashboard for configuration and analytics. See `Context.md` for full architecture details.

## General Principles

### DRY -- Don't Repeat Yourself
- Extract shared logic into functions, utilities, or middleware.
- Never copy-paste code between files. If two places need the same logic, create a shared module.
- Database queries that are used in multiple handlers belong in the Store interface, not duplicated inline.

### KIS -- Keep It Simple
- Prefer the simplest solution that works correctly.
- Avoid premature abstraction. Add abstractions only when there are at least two concrete use cases.
- Don't add configuration options for things that don't need to be configurable yet.
- Prefer standard library solutions over third-party packages when the difference is trivial.

### Explicit Over Implicit
- Name things clearly. A function called `GetHealthyProvidersForModel` is better than `GetProviders`.
- Don't rely on side effects. If a function modifies state, its name and signature should make that obvious.
- Error messages should include context about what was being attempted.

## Go Backend Rules

### Project Structure
- Entry point: `cmd/gateway/main.go`.
- All internal packages live under `internal/`. No `pkg/` directory -- nothing is intended for external consumption.
- Package responsibilities are strictly separated:
  - `internal/models/` -- domain types only, zero business logic
  - `internal/db/` -- Store interface and SQLite implementation
  - `internal/proxy/` -- OpenAI-compatible proxy handlers, streaming, routing, circuit breaker
  - `internal/admin/` -- admin API handlers, onboarding, stats
  - `internal/auth/` -- ACCESS_KEY middleware
  - `internal/health/` -- background health checker
  - `internal/config/` -- configuration from environment
  - `internal/middleware/` -- cross-cutting HTTP middleware (logging, recovery)

### Error Handling
- Always handle errors explicitly. Never use `_` to discard errors unless the function truly cannot fail in context (and add a comment explaining why).
- Wrap errors with context using `fmt.Errorf("doing X: %w", err)`.
- Return errors to callers; don't log-and-continue unless the operation is truly non-critical.
- HTTP handlers return appropriate status codes with JSON error bodies: `{"error": "message"}`.

### Database
- Use parameterized queries. Never interpolate user input into SQL strings.
- All schema changes go through migration files in `internal/db/migrations/`.
- All database access goes through the `Store` interface defined in `internal/db/store.go`.
- The Store interface is the single source of truth for data access patterns.

### HTTP Handlers
- Keep handlers thin: validate input, call a service/store method, format the response.
- Use middleware for cross-cutting concerns (auth, logging, recovery, CORS).
- All API responses are JSON (except binary endpoints like audio/speech).
- Use `chi` router for all HTTP routing.
- Consistent JSON response helpers: `respondJSON(w, status, data)` and `respondError(w, status, msg)`.

### Concurrency
- The health checker runs as a background goroutine. Use `context.Context` for graceful shutdown.
- Circuit breaker state is in-memory, protected by `sync.RWMutex`.
- Request log writes are async via a buffered channel to avoid blocking the proxy hot path.
- Don't spawn goroutines in HTTP handlers unless necessary (streaming proxy is the exception).

### Testing
- Write table-driven tests for business logic.
- Use `httptest.NewRecorder` for handler tests.
- Mock the Store interface in handler tests; test real SQL in integration tests against in-memory SQLite.
- Test files live next to the code they test: `foo.go` -> `foo_test.go`.

### Dependencies
- HTTP router: `github.com/go-chi/chi/v5`
- SQLite: `github.com/mattn/go-sqlite3` (CGo) or `modernc.org/sqlite` (pure Go)
- UUID: `github.com/google/uuid`
- No ORM. Write SQL directly with `database/sql`.
- Minimize external dependencies. Prefer stdlib where possible.

## SvelteKit Frontend Rules

### Svelte 5
- This project uses Svelte 5 with runes (`$state`, `$derived`, `$effect`).
- Do NOT use Svelte 4 patterns (reactive `$:` declarations, stores with `$` prefix auto-subscription).
- Use `$state` for component-local reactive state, `$derived` for computed values.

### File Organization
- Route structure: `login/` for auth, `(dashboard)/` group for all authenticated pages.
- Shared components go in `src/lib/components/`.
- API client functions go in `src/lib/api/`.
- Type definitions go in `src/lib/types/`.

### Component Rules
- Keep components focused. If a component exceeds ~150 lines, split it.
- Props use the `$props()` rune. Destructure with defaults where appropriate.
- Use shadcn-svelte components for all UI primitives (buttons, inputs, tables, cards, dialogs).
- Don't install additional UI libraries without explicit approval.

### Data Fetching
- Client-side fetches use typed functions from `src/lib/api/client.ts`.
- The API client stores the ACCESS_KEY in localStorage and sends it as `Authorization: Bearer <key>`.
- Always handle loading and error states in the UI.

### Styling
- Use Tailwind CSS classes via shadcn-svelte's conventions.
- Don't write custom CSS unless Tailwind genuinely can't express it.
- Dark mode support is not required for v1 but don't make choices that prevent it later.

### Frontend Build
- The frontend is built as a static SPA using `@sveltejs/adapter-static`.
- The Go binary embeds the compiled frontend from `frontend/build/` using `embed.FS`.
- During development, run the SvelteKit dev server separately and proxy API calls.

## Git & Commit Conventions

- Commit messages follow conventional commits: `feat:`, `fix:`, `refactor:`, `docs:`, `chore:`.
- One logical change per commit. Don't mix unrelated changes.
- Never commit secrets, `.env` files, or database files.

## Code Comments

- Don't add comments that restate the code. `// increment counter` above `counter++` is noise.
- Do add comments explaining *why* a non-obvious decision was made, trade-offs, or constraints.
- TODO comments must include context: `// TODO(#issue): description` or `// TODO: description of what and why`.

## File Naming

- Go: `snake_case.go` for files, `CamelCase` for exported names, `camelCase` for unexported.
- Svelte: `PascalCase.svelte` for components, `+page.svelte` / `+layout.svelte` for routes.
- TypeScript: `camelCase.ts` for modules, `PascalCase.ts` for types/interfaces files.
- SQL migrations: `NNNN_description.up.sql` / `NNNN_description.down.sql`.

## Agent-Specific Guidelines

### Spec-Driven Development
- Every agent has a spec file in `/spec`. Read your spec FIRST before writing any code.
- Specs are self-contained: all types, interfaces, and function signatures you need are inline.
- Do NOT read other agents' spec files or modify files outside your spec's file list.
- Follow the function signatures in your spec exactly. Other agents depend on these contracts.

### Verification
- Run `go build ./...` after writing Go code to check for compilation errors.
- Run `go test ./...` for your package after writing tests.
- Run `npm run check` in `frontend/` after writing Svelte code.

### When Unsure
- Read the relevant spec file in `/spec` before making changes to a subsystem.
- Check `Context.md` for overall project context.
- If a design decision isn't covered by spec or context, ask rather than guess.
- Prefer reversible decisions over irreversible ones.
