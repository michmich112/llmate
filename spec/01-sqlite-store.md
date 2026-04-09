# Spec 01: SQLite Store (Phase 1A)

## Goal

Implement the `Store` interface from `internal/db/store.go` using SQLite. This is the sole database access layer. Uses `modernc.org/sqlite` (pure Go, no CGo).

## Files to Create

1. `internal/db/sqlite.go` -- SQLite implementation of Store
2. `internal/db/sqlite_test.go` -- Integration tests against in-memory SQLite

## Package

`package db`

Imports for implementation (not exhaustive; add stdlib as needed):

```go
import (
    "context"
    "database/sql"
    "errors"
    "fmt"
    "time"

    "github.com/google/uuid"
    _ "modernc.org/sqlite"

    "github.com/llmate/gateway/internal/models"
)
```

---

## Domain Types (inline reference -- these exist in `internal/models/`)

Include the FULL type definitions:

```go
// internal/models/provider.go
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

// internal/models/alias.go
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

// internal/models/request_log.go
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

// internal/models/stats.go
type DashboardStats struct {
    TotalRequests  int              `json:"total_requests"`
    AvgLatencyMs   float64          `json:"avg_latency_ms"`
    ErrorRate      float64          `json:"error_rate"`
    ByModel        []ModelStats     `json:"by_model"`
    ByProvider     []ProviderStats  `json:"by_provider"`
}

type ModelStats struct {
    Model         string  `json:"model"`
    RequestCount  int     `json:"request_count"`
    AvgLatencyMs  float64 `json:"avg_latency_ms"`
    ErrorCount    int     `json:"error_count"`
    TotalTokens   int     `json:"total_tokens"`
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

## Store Interface (inline reference -- defined in `internal/db/store.go`)

Include the FULL Store interface:

```go
type Store interface {
    CreateProvider(ctx context.Context, p *models.Provider) error
    GetProvider(ctx context.Context, id string) (*models.Provider, error)
    ListProviders(ctx context.Context) ([]models.Provider, error)
    UpdateProvider(ctx context.Context, p *models.Provider) error
    DeleteProvider(ctx context.Context, id string) error
    UpsertProviderEndpoints(ctx context.Context, providerID string, eps []models.ProviderEndpoint) error
    ListProviderEndpoints(ctx context.Context, providerID string) ([]models.ProviderEndpoint, error)
    UpdateProviderEndpoint(ctx context.Context, ep *models.ProviderEndpoint) error
    SyncProviderModels(ctx context.Context, providerID string, modelIDs []string) error
    ListProviderModels(ctx context.Context, providerID string) ([]models.ProviderModel, error)
    ListAllModels(ctx context.Context) ([]models.ProviderModel, error)
    CreateAlias(ctx context.Context, a *models.ModelAlias) error
    ListAliases(ctx context.Context) ([]models.ModelAlias, error)
    UpdateAlias(ctx context.Context, a *models.ModelAlias) error
    DeleteAlias(ctx context.Context, id string) error
    ResolveAlias(ctx context.Context, alias string) ([]models.ModelAlias, error)
    GetHealthyProvidersForModel(ctx context.Context, modelID string) ([]models.Provider, error)
    GetEnabledEndpoint(ctx context.Context, providerID string, path string) (*models.ProviderEndpoint, error)
    InsertRequestLog(ctx context.Context, log *models.RequestLog) error
    QueryRequestLogs(ctx context.Context, filter models.LogFilter) ([]models.RequestLog, int, error)
    GetDashboardStats(ctx context.Context, since time.Time) (*models.DashboardStats, error)
    UpdateProviderHealth(ctx context.Context, id string, healthy bool) error
    Close() error
}
```

---

## Migration SQL (inline reference)

Full `internal/db/migrations/0001_initial.up.sql` (providers, provider_endpoints, provider_models, model_aliases, request_logs with columns, FKs, indexes):

```sql
CREATE TABLE IF NOT EXISTS providers (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    base_url TEXT NOT NULL UNIQUE,
    api_key TEXT,
    is_healthy BOOLEAN NOT NULL DEFAULT 0,
    health_checked_at DATETIME,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS provider_endpoints (
    id TEXT PRIMARY KEY,
    provider_id TEXT NOT NULL,
    path TEXT NOT NULL,
    method TEXT NOT NULL,
    is_supported BOOLEAN NOT NULL,
    is_enabled BOOLEAN NOT NULL DEFAULT 1,
    created_at DATETIME NOT NULL,
    FOREIGN KEY (provider_id) REFERENCES providers(id) ON DELETE CASCADE,
    UNIQUE(provider_id, path, method)
);

CREATE TABLE IF NOT EXISTS provider_models (
    id TEXT PRIMARY KEY,
    provider_id TEXT NOT NULL,
    model_id TEXT NOT NULL,
    created_at DATETIME NOT NULL,
    FOREIGN KEY (provider_id) REFERENCES providers(id) ON DELETE CASCADE,
    UNIQUE(provider_id, model_id)
);

CREATE TABLE IF NOT EXISTS model_aliases (
    id TEXT PRIMARY KEY,
    alias TEXT NOT NULL,
    provider_id TEXT NOT NULL,
    model_id TEXT NOT NULL,
    weight INTEGER NOT NULL DEFAULT 1,
    priority INTEGER NOT NULL DEFAULT 0,
    is_enabled BOOLEAN NOT NULL DEFAULT 1,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL,
    FOREIGN KEY (provider_id) REFERENCES providers(id) ON DELETE CASCADE,
    UNIQUE(alias, provider_id, model_id)
);

CREATE TABLE IF NOT EXISTS request_logs (
    id TEXT PRIMARY KEY,
    timestamp DATETIME NOT NULL,
    client_ip TEXT NOT NULL,
    method TEXT NOT NULL,
    path TEXT NOT NULL,
    requested_model TEXT,
    resolved_model TEXT,
    provider_id TEXT,
    provider_name TEXT,
    status_code INTEGER NOT NULL,
    is_streamed BOOLEAN NOT NULL DEFAULT 0,
    ttft_ms INTEGER,
    total_time_ms INTEGER NOT NULL,
    prompt_tokens INTEGER,
    completion_tokens INTEGER,
    total_tokens INTEGER,
    cached_tokens INTEGER,
    error_message TEXT,
    created_at DATETIME NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_request_logs_timestamp ON request_logs(timestamp);
CREATE INDEX IF NOT EXISTS idx_request_logs_requested_model ON request_logs(requested_model);
CREATE INDEX IF NOT EXISTS idx_request_logs_provider_id ON request_logs(provider_id);
```

---

## Implementation Details

### SQLiteStore struct

```go
type SQLiteStore struct {
    db *sql.DB
}
```

### Constructor

```go
func NewSQLiteStore(dbPath string) (*SQLiteStore, error)
```

- Open with driver `sqlite` (modernc): append query params `?_pragma=foreign_keys(1)&_pragma=journal_mode(wal)` to the DSN (or `&` if the path already has `?`). Adjust if the driver requires uppercase `WAL`—follow `modernc.org/sqlite` connection string docs.
- Set `SetMaxOpenConns(1)` for SQLite file databases to avoid locking issues; for `:memory:` this is optional but harmless.
- Run migrations: either `embed` the contents of `0001_initial.up.sql` into the binary and execute as a single script, or read the file from `internal/db/migrations/` at runtime. All `CREATE TABLE` / `CREATE INDEX` statements must run successfully on first open.
- Return `(*SQLiteStore, nil)` on success; wrap errors with `fmt.Errorf("...: %w", err)`.

### Scanning helpers (recommended)

- **Nullable INTEGER → `*int`**: scan into `sql.NullInt64`, then if `Valid` set pointer to `int(v.Int64)` else `nil`.
- **Nullable DATETIME → `*time.Time`**: scan into `sql.NullTime` or nullable string and parse; for `health_checked_at` use pointer.
- **BOOLEAN**: SQLite stores as integer 0/1; `Scan` into `bool` works with the driver.
- **DATETIME strings**: ensure round-trip with RFC3339 or SQLite-compatible format consistent with writes.

---

### Method-by-method behavior

#### `Close() error`

- Call `s.db.Close()`. Return the error wrapped if non-nil.

#### `CreateProvider(ctx, p *models.Provider) error`

1. **SQL**: `INSERT INTO providers (id, name, base_url, api_key, is_healthy, health_checked_at, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`
2. **Params**: `p.ID`, `p.Name`, `p.BaseURL`, `p.APIKey` (empty string if omitted), `p.IsHealthy`, nullable `p.HealthCheckedAt`, `p.CreatedAt`, `p.UpdatedAt`.
3. **Errors**: wrap with `fmt.Errorf("create provider: %w", err)`. On unique violation on `base_url`, surface the wrapped SQLite error.

#### `GetProvider(ctx, id string) (*models.Provider, error)`

1. **SQL**: `SELECT id, name, base_url, api_key, is_healthy, health_checked_at, created_at, updated_at FROM providers WHERE id = ?`
2. **Scan**: into `Provider`; nullable `health_checked_at` → `*time.Time`.
3. **Not found**: if `errors.Is(err, sql.ErrNoRows)`, return `nil, fmt.Errorf("get provider: %w", err)` (or a dedicated sentinel; tests should assert not-found). Prefer wrapping `sql.ErrNoRows` so callers can use `errors.Is`.
4. **Success**: `&Provider{...}, nil`.

#### `ListProviders(ctx) ([]models.Provider, error)`

1. **SQL**: `SELECT ... FROM providers ORDER BY created_at DESC`
2. **Scan**: loop rows; append each `Provider`.
3. **Errors**: wrap query/scan errors.

#### `UpdateProvider(ctx, p *models.Provider) error`

1. **SQL**: `UPDATE providers SET name = ?, base_url = ?, api_key = ?, updated_at = ? WHERE id = ?`
2. **Params**: `p.Name`, `p.BaseURL`, `p.APIKey`, `p.UpdatedAt`, `p.ID`.
3. **Not found**: `res.RowsAffected() == 0` → return an error such as `fmt.Errorf("update provider: no rows for id %s", p.ID)` (not `sql.ErrNoRows`, but explicit).
4. **Unique** `base_url` conflict: wrap driver error.

#### `DeleteProvider(ctx, id string) error`

1. **SQL**: `DELETE FROM providers WHERE id = ?` with arg `id`.
2. **Cascade**: FK `ON DELETE CASCADE` removes `provider_endpoints`, `provider_models`, `model_aliases` automatically.
3. **Transaction**: Wrap in `BeginTx` / `Commit` so the done-criteria checklist applies; the delete itself is still a single statement—transaction documents multi-step intent and keeps the API consistent with other mutating paths.
4. **Optional**: treat `RowsAffected == 0` as not-found error for consistency with update.

#### `UpsertProviderEndpoints(ctx, providerID string, eps []models.ProviderEndpoint) error`

1. **Transaction**: `BeginTx`; `defer rollback on panic/error`.
2. **SQL (step A)**: `DELETE FROM provider_endpoints WHERE provider_id = ?`
3. **SQL (step B)** for each endpoint: `INSERT INTO provider_endpoints (id, provider_id, path, method, is_supported, is_enabled, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`
   - Use `ep.ID`, `providerID` (enforce `ep.ProviderID == providerID` or ignore struct field and use arg), `ep.Path`, `ep.Method`, `ep.IsSupported`, `ep.IsEnabled`, `ep.CreatedAt`.
4. **Empty `eps`**: still run DELETE; result is zero endpoints.
5. **Commit**; wrap errors `fmt.Errorf("upsert provider endpoints: %w", err)`.

#### `ListProviderEndpoints(ctx, providerID string) ([]models.ProviderEndpoint, error)`

1. **SQL**: `SELECT id, provider_id, path, method, is_supported, is_enabled, created_at FROM provider_endpoints WHERE provider_id = ? ORDER BY path ASC, method ASC`
2. **Scan**: slice of `ProviderEndpoint`.

#### `UpdateProviderEndpoint(ctx, ep *models.ProviderEndpoint) error`

1. **SQL**: `UPDATE provider_endpoints SET is_enabled = ? WHERE id = ?`
2. **Params**: `ep.IsEnabled`, `ep.ID`.
3. **RowsAffected == 0**: not-found error.

#### `SyncProviderModels(ctx, providerID string, modelIDs []string) error`

1. **Transaction**.
2. **SQL**: `DELETE FROM provider_models WHERE provider_id = ?`
3. For each `modelID`: `INSERT INTO provider_models (id, provider_id, model_id, created_at) VALUES (?, ?, ?, ?)` with `id = uuid.NewString()` from `github.com/google/uuid`, `created_at = time.Now().UTC()` (or `time.Now()` consistently).
4. **Commit**; wrap errors.

#### `ListProviderModels(ctx, providerID string) ([]models.ProviderModel, error)`

1. **SQL**: `SELECT id, provider_id, model_id, created_at FROM provider_models WHERE provider_id = ? ORDER BY model_id ASC`

#### `ListAllModels(ctx) ([]models.ProviderModel, error)`

1. **SQL**: `SELECT id, provider_id, model_id, created_at FROM provider_models ORDER BY provider_id ASC, model_id ASC`

#### `CreateAlias(ctx, a *models.ModelAlias) error`

1. **SQL**: `INSERT INTO model_aliases (id, alias, provider_id, model_id, weight, priority, is_enabled, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`

#### `ListAliases(ctx) ([]models.ModelAlias, error)`

1. **SQL**: `SELECT id, alias, provider_id, model_id, weight, priority, is_enabled, created_at, updated_at FROM model_aliases ORDER BY alias ASC, priority DESC`

#### `UpdateAlias(ctx, a *models.ModelAlias) error`

1. **SQL**: `UPDATE model_aliases SET weight = ?, priority = ?, is_enabled = ?, updated_at = ? WHERE id = ?`

#### `DeleteAlias(ctx, id string) error`

1. **SQL**: `DELETE FROM model_aliases WHERE id = ?`

#### `ResolveAlias(ctx, alias string) ([]models.ModelAlias, error)`

1. **SQL**: `SELECT id, alias, provider_id, model_id, weight, priority, is_enabled, created_at, updated_at FROM model_aliases WHERE alias = ? AND is_enabled = 1 ORDER BY priority DESC`
2. **Empty result**: return `nil, nil` or empty slice and nil error (prefer `[]models.ModelAlias{}`).

#### `GetHealthyProvidersForModel(ctx, modelID string) ([]models.Provider, error)`

1. **SQL** (no duplicate providers if multiple model rows; use `DISTINCT`):

```sql
SELECT DISTINCT p.id, p.name, p.base_url, p.api_key, p.is_healthy, p.health_checked_at, p.created_at, p.updated_at
FROM providers p
INNER JOIN provider_models m ON m.provider_id = p.id
WHERE m.model_id = ? AND p.is_healthy = 1
ORDER BY p.name ASC
```

2. **Scan**: `[]models.Provider`.

#### `GetEnabledEndpoint(ctx, providerID, path string) (*models.ProviderEndpoint, error)`

1. **SQL**: `SELECT id, provider_id, path, method, is_supported, is_enabled, created_at FROM provider_endpoints WHERE provider_id = ? AND path = ? AND is_supported = 1 AND is_enabled = 1`
2. **Not found**: `sql.ErrNoRows` → return `nil, nil` (no error).
3. **Multiple rows**: unique `(provider_id, path, method)` allows same path with different methods; spec filters by `path` only. If multiple rows match, use `LIMIT 1` and deterministic `ORDER BY method ASC`, or return error if more than one—prefer `LIMIT 1` with `ORDER BY method` for stability.

#### `InsertRequestLog(ctx, log *models.RequestLog) error`

1. **SQL**: `INSERT INTO request_logs (id, timestamp, client_ip, method, path, requested_model, resolved_model, provider_id, provider_name, status_code, is_streamed, ttft_ms, total_time_ms, prompt_tokens, completion_tokens, total_tokens, cached_tokens, error_message, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
2. **Nullable columns**: bind `NULL` for empty optional strings / unset pointers as appropriate.

#### `QueryRequestLogs(ctx, filter models.LogFilter) ([]models.RequestLog, int, error)`

1. **Count query**: `SELECT COUNT(*) FROM request_logs` + dynamic `WHERE`:
   - If `filter.Model != ""`: `AND requested_model = ?`
   - If `filter.ProviderID != ""`: `AND provider_id = ?`
   - If `filter.Since != nil`: `AND timestamp >= ?`
   - If `filter.Until != nil`: `AND timestamp <= ?`
2. **Data query**: same `WHERE`, then `ORDER BY timestamp DESC`, then `LIMIT ? OFFSET ?` with `filter.Limit` and `filter.Offset`. If `Limit == 0`, use a sensible default (e.g. `100`) or document no limit—prefer default cap to avoid unbounded reads.
3. **Returns**: `(logs, totalCount, err)`.

#### `GetDashboardStats(ctx, since time.Time) (*models.DashboardStats, error)`

All aggregates restricted to `timestamp >= ?` (bind `since`).

1. **Totals** (single row):
   - `total_requests`: `SELECT COUNT(*) FROM request_logs WHERE timestamp >= ?`
   - `avg_latency_ms`: `SELECT AVG(CAST(total_time_ms AS REAL)) FROM request_logs WHERE timestamp >= ?` — if no rows, use `0`.
   - `error_rate`: `SELECT CAST(SUM(CASE WHEN status_code >= 400 THEN 1 ELSE 0 END) AS REAL) / COUNT(*) FROM request_logs WHERE timestamp >= ?` — if `COUNT(*) == 0`, use `0` for rate.

2. **By model**:

```sql
SELECT
  COALESCE(requested_model, '') AS model,
  COUNT(*) AS request_count,
  AVG(CAST(total_time_ms AS REAL)) AS avg_latency_ms,
  SUM(CASE WHEN status_code >= 400 THEN 1 ELSE 0 END) AS error_count,
  COALESCE(SUM(COALESCE(total_tokens, 0)), 0) AS total_tokens
FROM request_logs
WHERE timestamp >= ?
GROUP BY requested_model
ORDER BY request_count DESC
```

Map `NULL` requested_model to empty string or `"unknown"` consistently in tests.

3. **By provider** (join providers for name):

```sql
SELECT
  COALESCE(r.provider_id, '') AS provider_id,
  COALESCE(p.name, r.provider_name, '') AS provider_name,
  COUNT(*) AS request_count,
  AVG(CAST(r.total_time_ms AS REAL)) AS avg_latency_ms,
  SUM(CASE WHEN r.status_code >= 400 THEN 1 ELSE 0 END) AS error_count
FROM request_logs r
LEFT JOIN providers p ON p.id = r.provider_id
WHERE r.timestamp >= ?
GROUP BY r.provider_id
ORDER BY request_count DESC
```

Handle rows with no `provider_id` (empty group) as needed.

4. **Return**: `&models.DashboardStats{...}`; ensure slices are non-nil (`[]` not `nil`) if empty.

#### `UpdateProviderHealth(ctx, id string, healthy bool) error`

1. **SQL**: `UPDATE providers SET is_healthy = ?, health_checked_at = ? WHERE id = ?`
2. **Params**: `healthy`, `time.Now().UTC()` (or consistent timezone), `id`.
3. **RowsAffected == 0**: not-found error.

---

## Migrations wiring

- Prefer `//go:embed migrations/*.sql` in a small `migrate.go` or inside `sqlite.go` to embed `0001_initial.up.sql`.
- Execute the full script in one `Exec` or split on `;` carefully (SQLite accepts multiple statements). If splitting, ignore empty statements.
- Do not run `down` migration on normal startup.

---

## Testing

**File**: `internal/db/sqlite_test.go`

- Use in-memory DSN: `"file::memory:?cache=shared"` or plain `":memory:"` per modernc docs; ensure `NewSQLiteStore` works with `:memory:` for tests (may require passing `":memory:"` as `dbPath`).
- Construct store via `NewSQLiteStore` or a test helper that opens memory DB and runs migrations.

**Table-driven / subtests** covering:

| Area | Cases |
|------|--------|
| Provider CRUD | create → get → list → update → delete |
| Cascade delete | provider with endpoints, models, aliases → delete provider → verify related tables empty for that id |
| Upsert endpoints | insert two; upsert with new set of one; list reflects replacement |
| Sync models | sync `[a,b]` then `[b,c]`; IDs change; models match |
| Aliases | create, list order (alias asc, priority desc), update, delete |
| ResolveAlias | multiple rows same alias, different priority; only `is_enabled=1`; order `priority DESC` |
| GetHealthyProvidersForModel | healthy + model present vs unhealthy or missing model |
| GetEnabledEndpoint | supported+enabled returns row; `is_enabled=0` or `is_supported=0` → nil |
| Request logs | insert; query with model/provider/since/until; count matches |
| Dashboard stats | seeded logs → totals and groupings non-empty where expected |
| Not-found | `GetProvider` missing id; optional `UpdateProvider` missing id |

- Use `t.Parallel()` only if each test uses its own isolated DB handle (avoid shared `:memory:` races).

---

## Done Criteria

- [ ] `internal/db/sqlite.go` implements all Store interface methods
- [ ] `internal/db/sqlite_test.go` has tests for all methods
- [ ] `go build ./internal/db/...` succeeds
- [ ] `go test ./internal/db/...` passes
- [ ] All queries use parameterized placeholders (no string interpolation)
- [ ] Transactions used for multi-step operations (`UpsertProviderEndpoints`, `SyncProviderModels`, `DeleteProvider` cascade)
