# Spec 03: Smart Router + Circuit Breaker (Phase 1C)

## Goal

Implement the smart routing engine that resolves model names (including aliases), selects the best healthy provider using weighted priority-based selection, and manages per-provider circuit breakers for automatic failure detection and recovery.

## Files to Create

1. `internal/proxy/router.go` -- Smart router implementation
2. `internal/proxy/circuit.go` -- Circuit breaker state machine
3. `internal/proxy/router_test.go` -- Router tests
4. `internal/proxy/circuit_test.go` -- Circuit breaker tests

## Package

`package proxy`

Imports (typical): `context`, `errors`, `fmt`, `math/rand`, `strings`, `sync`, `time`, plus `github.com/llmate/gateway/internal/db`, `github.com/llmate/gateway/internal/models`.

---

## Domain Types (inline reference)

These types are defined in `internal/models/` (Spec 00). Reproduced here so this spec is self-contained.

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

**Note:** `ResolveAlias` returns only **enabled** rows (per Store contract). The router must still ignore any candidate whose provider or endpoint is unusable after filtering.

---

## Store Methods Used (read-only)

The router’s **minimum** hot-path surface is these `Store` methods:

```go
ResolveAlias(ctx context.Context, alias string) ([]models.ModelAlias, error)
GetHealthyProvidersForModel(ctx context.Context, modelID string) ([]models.Provider, error)
GetEnabledEndpoint(ctx context.Context, providerID string, path string) (*models.ProviderEndpoint, error)
```

When aliases exist, the router must load each candidate’s `models.Provider` by id. Use:

```go
GetProvider(ctx context.Context, id string) (*models.Provider, error)
```

(as defined in Spec 00). Do not duplicate SQL outside `Store`.

---

## Router Interface (this is what the proxy handlers call)

```go
type Router interface {
    Route(ctx context.Context, modelID string, endpointPath string) (*RouteResult, error)
    ReportSuccess(providerID string)
    ReportFailure(providerID string)
}

type RouteResult struct {
    Provider  models.Provider
    ModelID   string
    TargetURL string
}
```

- **`modelID`**: The model name from the incoming request (may be an alias or a raw model id).
- **`endpointPath`**: The proxy path to resolve against the store, e.g. `/v1/chat/completions` (must match `ProviderEndpoint.Path` semantics used by `GetEnabledEndpoint`).

---

## SmartRouter struct

```go
type SmartRouter struct {
    store    db.Store
    breakers map[string]*CircuitBreaker // keyed by provider ID
    mu       sync.RWMutex
}

func NewSmartRouter(store db.Store) *SmartRouter
```

- Initialize `breakers` to a non-nil empty map.
- `store` must not be nil; `NewSmartRouter` may panic or return error per project convention (prefer validating and returning error from `Route` if store is nil, or document panic in `NewSmartRouter`).

---

## Routing Algorithm (`Route` method)

1. **Resolve alias**: Call `store.ResolveAlias(ctx, modelID)`.
   - If **aliases found**: use alias entries as candidates (each has `provider_id`, `model_id`, `weight`, `priority`). For each row, load `models.Provider` via `Store.GetProvider(ctx, provider_id)` (Spec 00). If lookup fails or provider is missing, drop that row. Resolved model id for the row is `model_id`.
   - If **no aliases**: treat `modelID` as a direct model id. Call `store.GetHealthyProvidersForModel(ctx, modelID)`. Each returned provider is a candidate with **weight = 1**, **priority = 0**, and resolved model id = `modelID`.

2. **Filter by circuit breaker**: For each candidate, check `getBreaker(provider.ID).Allow()`. Keep candidates where the breaker allows requests (state **Closed** or **HalfOpen**, or **Open** when `Allow` transitions to **HalfOpen** after cooldown). Remove candidates whose breaker does not allow (e.g. **Open** before cooldown).

3. **Filter by endpoint**: For each remaining candidate, call `store.GetEnabledEndpoint(ctx, providerID, endpointPath)`. Remove candidates where the endpoint is `nil`.

4. **Select provider**:
   - **a.** Group candidates by **priority** (highest numeric priority first).
   - **b.** Take only the **highest-priority** group.
   - **c.** Within that group, **weighted random** selection:
     - Sum all **positive** weights in the group; skip candidates with `weight <= 0`. If none remain, return `ErrNoAvailableProvider`.
     - Let `totalWeight` be the sum of weights.
     - Generate a uniform random value in `[0, totalWeight)` and select the candidate whose **cumulative weight** range contains that value (standard weighted pick).

5. **Build result**: Return `RouteResult` with:
   - `Provider`: selected provider
   - `ModelID`: resolved model id for that candidate
   - `TargetURL`: join `Provider.BaseURL` with `endpointPath` (normalize slashes so there is no `//` in the middle except as part of scheme; typical pattern: `strings.TrimRight(base, "/") + "/" + strings.TrimLeft(path, "/")` or equivalent)

6. **Error**: If no candidates remain after filtering, or selection cannot proceed, return `ErrNoAvailableProvider`.

### Concurrency

- `Route` must be safe for concurrent calls from many goroutines.
- Hold **`RLock`** on `mu` while reading/updating the `breakers` map for `getBreaker` / iteration; **`Lock`** when creating a new entry in the map (get-or-create pattern: double-checked locking or mutex around map is acceptable).
- Per-breaker mutations use each `CircuitBreaker`’s own `mu` (see below).

---

## Circuit Breaker

### States

- **Closed** (normal): Requests flow through. Track error rate over a sliding time window.
- **Open** (tripped): Requests rejected until cooldown elapses; then transition to **HalfOpen**.
- **HalfOpen** (probing): Allow traffic per `Allow()` rules below; **RecordSuccess** → **Closed**; **RecordFailure** → **Open**.

### `CircuitBreakerState` and struct

```go
type CircuitBreakerState int

const (
    StateClosed CircuitBreakerState = iota
    StateOpen
    StateHalfOpen
)

type CircuitBreaker struct {
    mu              sync.Mutex
    state           CircuitBreakerState
    failures        []time.Time // timestamps of recent failures (sliding window)
    successes       []time.Time // timestamps of recent successes (sliding window)
    lastStateChange time.Time

    // Configuration
    errorThreshold   float64       // e.g. 0.5 = 50%
    windowSize       time.Duration // e.g. 60s
    cooldownPeriod   time.Duration // e.g. 30s
}

func NewCircuitBreaker() *CircuitBreaker
```

**Defaults** (in `NewCircuitBreaker`): `errorThreshold = 0.5`, `windowSize = 60s`, `cooldownPeriod = 30s`.

### Sliding window maintenance

Before using `failures` / `successes` for rate calculation or counting, **prune** entries older than `now - windowSize` (and optionally cap slice growth). Use `time.Now()` in production code; tests may inject time if the implementation exposes a clock interface (optional, not required by this spec).

### Methods

```go
func (cb *CircuitBreaker) Allow() bool
```

- **Closed:** return `true`.
- **Open:** if `time.Since(lastStateChange) >= cooldownPeriod`, set state to **HalfOpen**, update `lastStateChange`, return `true` (first probe). Otherwise return `false`.
- **HalfOpen:** return `true` (probe allowed).

```go
func (cb *CircuitBreaker) RecordSuccess()
```

- Prune old entries from `successes` / `failures` per window.
- Append current time to `successes`.
- If state is **HalfOpen:** transition to **Closed**, clear failure history (and optionally success history per implementation consistency), update `lastStateChange`.

```go
func (cb *CircuitBreaker) RecordFailure()
```

- Prune old entries.
- Append current time to `failures`.
- If state is **HalfOpen:** transition to **Open**, set `lastStateChange` to now.
- If state is **Closed:** compute error rate over the window:
  - Let `f = len(failures)` and `s = len(successes)` after pruning and after recording this failure.
  - Rate = `float64(f) / float64(f+s)` when `(f+s) > 0`. If denominator is 0, do not trip.
  - If rate **>** `errorThreshold`, transition to **Open** and set `lastStateChange`.

```go
func (cb *CircuitBreaker) State() CircuitBreakerState
```

Return current state (lock held briefly).

---

## SmartRouter breaker integration

```go
func (r *SmartRouter) ReportSuccess(providerID string)
```

- `getBreaker(providerID).RecordSuccess()`

```go
func (r *SmartRouter) ReportFailure(providerID string)
```

- `getBreaker(providerID).RecordFailure()`

```go
func (r *SmartRouter) getBreaker(providerID string) *CircuitBreaker
```

- Thread-safe **get-or-create** on `breakers` map: return existing `*CircuitBreaker` or create via `NewCircuitBreaker()`, store, and return. Must use `r.mu` appropriately to avoid races on the map.

---

## Error Types

```go
var ErrNoAvailableProvider = errors.New("no available provider for model")
```

---

## Testing

### `router_test.go`

Use a **mock `Store`** implementing only the three methods the router calls (and `GetProvider` if used for alias resolution). Table-driven tests where helpful.

**Cases:**

1. **Alias resolution:** Given aliases for `modelID`, route returns one of the expected providers and correct resolved `ModelID` / `TargetURL`.
2. **Direct model:** No aliases; `GetHealthyProvidersForModel` returns providers; route picks among them according to rules.
3. **Priority selection:** Candidates with higher `priority` always win over lower priority (deterministic: only one candidate in top priority group, or mock RNG if injected).
4. **Weighted selection:** With fixed seed or mocked `rand`, or statistical: over many iterations, observed frequencies approximate weight ratios within tolerance.
5. **Circuit breaker filtering:** Pre-set a breaker to **Open** (via failures or test hook) for provider A; only B is chosen.
6. **Failover:** Preferred provider excluded (open breaker or no endpoint); next candidate is chosen.
7. **No providers:** Empty lists / all filtered → `ErrNoAvailableProvider` with `errors.Is(err, ErrNoAvailableProvider)`.
8. **Endpoint filtering:** Provider has no enabled endpoint for path → skipped.

### `circuit_test.go`

**Cases:**

1. **Closed** allows all `Allow()` calls (subject to sliding window logic not tripping).
2. **Failures** beyond threshold in window → **Open**.
3. **Open** rejects `Allow()` until cooldown.
4. After **cooldown**, `Allow()` transitions to **HalfOpen** and permits probe.
5. **HalfOpen** + `RecordSuccess` → **Closed**.
6. **HalfOpen** + `RecordFailure` → **Open**.
7. **Sliding window:** old failures/successes fall outside window and no longer affect rate.
8. **Concurrent** `Allow` / `RecordSuccess` / `RecordFailure` from multiple goroutines without data races (use `-race` in CI).

---

## Done Criteria

- [ ] `SmartRouter` implements `Router` interface
- [ ] Circuit breaker state machine with Closed / Open / HalfOpen
- [ ] Weighted priority-based provider selection
- [ ] Thread-safe breaker map access and per-breaker locking
- [ ] All tests pass (`go test ./internal/proxy/...`)
- [ ] `go build ./internal/proxy/...` succeeds
