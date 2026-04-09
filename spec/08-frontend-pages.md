# Spec 08: Frontend Pages (Phase 1H)

## Goal

Implement all dashboard page components: overview with stats, provider management (list + detail), onboarding wizard, model alias management, and request logs viewer. These pages consume the **`api`** singleton from Phase 1G (`$lib/api/client`) and shared UI primitives (shadcn-svelte, `StatusBadge`, Tailwind).

**This document is the only file the implementing agent must read** for Phase 1H. Types, API shapes, UX, and Svelte 5 patterns are fully specified below.

---

## Files to Create

| # | Path | Purpose |
|---|------|---------|
| 1 | `frontend/src/routes/(dashboard)/+page.svelte` | Dashboard overview |
| 2 | `frontend/src/routes/(dashboard)/providers/+page.svelte` | Provider list |
| 3 | `frontend/src/routes/(dashboard)/providers/new/+page.svelte` | Onboarding wizard |
| 4 | `frontend/src/routes/(dashboard)/providers/[id]/+page.svelte` | Provider detail / edit |
| 5 | `frontend/src/routes/(dashboard)/models/+page.svelte` | Model alias management |
| 6 | `frontend/src/routes/(dashboard)/logs/+page.svelte` | Request logs viewer |

Do **not** create or modify files outside this list unless a build error forces a minimal fix (prefer fixing the page file first).

---

## Dependencies (Phase 1G — assumed present)

- **`$lib/api/client`** — `export const api` — singleton with all typed methods below.
- **`$lib/types/index`** — re-export or define types matching the **TypeScript Types** section exactly (field names use `snake_case` for JSON alignment).
- **`$lib/components/StatusBadge.svelte`** — displays provider health; use for provider list and detail (pass whatever props that component defines — typically healthy vs unhealthy).
- **shadcn-svelte** — `Button`, `Card`, `Input`, `Table`, `TableHeader`, `TableBody`, `TableRow`, `TableHead`, `TableCell`, `Dialog`, `Badge`, `Select`, `Label`, `Checkbox`, `Skeleton` (or spinner pattern), etc.
- **`$lib/utils.ts`** — `cn()` for class merging where needed.

---

## TypeScript Types (full definitions — implement pages against these)

All timestamps from the API are **ISO 8601 strings** unless noted otherwise.

```typescript
/** JSON keys use snake_case */

export interface Provider {
  id: string;
  name: string;
  base_url: string;
  api_key?: string;
  is_healthy: boolean;
  health_checked_at?: string;
  created_at: string;
  updated_at: string;
}

export interface ProviderEndpoint {
  id: string;
  provider_id: string;
  path: string;
  method: string;
  is_supported: boolean;
  is_enabled: boolean;
  created_at: string;
}

export interface ProviderModel {
  id: string;
  provider_id: string;
  model_id: string;
  created_at: string;
}

export interface ModelAlias {
  id: string;
  alias: string;
  provider_id: string;
  model_id: string;
  weight: number;
  priority: number;
  is_enabled: boolean;
  created_at: string;
  updated_at: string;
}

export interface RequestLog {
  id: string;
  timestamp: string;
  client_ip: string;
  method: string;
  path: string;
  requested_model?: string;
  resolved_model?: string;
  provider_id?: string;
  provider_name?: string;
  status_code: number;
  is_streamed: boolean;
  ttft_ms?: number;
  total_time_ms: number;
  prompt_tokens?: number;
  completion_tokens?: number;
  total_tokens?: number;
  cached_tokens?: number;
  error_message?: string;
  created_at: string;
}

export interface LogFilter {
  model?: string;
  provider_id?: string;
  since?: string;
  until?: string;
  limit?: number;
  offset?: number;
}

export interface DashboardStats {
  total_requests: number;
  avg_latency_ms: number;
  error_rate: number;
  by_model: ModelStats[];
  by_provider: ProviderStats[];
}

export interface ModelStats {
  model: string;
  request_count: number;
  avg_latency_ms: number;
  error_count: number;
  total_tokens: number;
}

export interface ProviderStats {
  provider_id: string;
  provider_name: string;
  request_count: number;
  avg_latency_ms: number;
  error_count: number;
}

/** Result of POST /admin/providers/{id}/discover */
export interface DiscoveryResult {
  models: string[];
  endpoints: DiscoveredEndpoint[];
}

export interface DiscoveredEndpoint {
  path: string;
  method: string;
  /** `true` = supported, `false` = not supported, `null` = unknown (probe inconclusive) */
  is_supported: boolean | null;
}

/** Body for POST /admin/providers/{id}/confirm */
export interface ConfirmProviderBody {
  endpoints: ConfirmEndpointInput[];
  models: string[];
}

export interface ConfirmEndpointInput {
  path: string;
  method: string;
  is_supported: boolean;
  is_enabled: boolean;
}

/** Response shape from GET /admin/providers/{id} and from confirm (see API section) */
export interface ProviderDetailResponse {
  provider: Provider;
  endpoints: ProviderEndpoint[];
  models: ProviderModel[];
}
```

---

## API Client Methods (inline reference)

The **`api`** singleton implements (at minimum) the following. Method names and return types **must** be used as written so pages compile against `$lib/api/client`.

```typescript
interface ApiClient {
  // --- Auth (for context; login page uses these) ---
  setAccessKey(key: string): void;
  clearAccessKey(): void;
  isAuthenticated(): boolean;
  validateKey(key: string): Promise<boolean>;

  // --- Providers ---
  listProviders(): Promise<Provider[]>;
  createProvider(data: {
    name: string;
    base_url: string;
    api_key?: string;
  }): Promise<Provider>;

  /** Returns { provider, endpoints, models } */
  getProvider(id: string): Promise<ProviderDetailResponse>;

  updateProvider(id: string, data: Partial<Provider>): Promise<Provider>;
  deleteProvider(id: string): Promise<void>;

  discoverProvider(id: string): Promise<DiscoveryResult>;

  /**
   * Persists selected models and endpoint matrix after discovery.
   * Returns the same shape as getProvider for convenience.
   */
  confirmProvider(id: string, data: ConfirmProviderBody): Promise<ProviderDetailResponse>;

  updateEndpoint(
    providerId: string,
    endpointId: string,
    data: { is_enabled: boolean }
  ): Promise<ProviderEndpoint>;

  // --- Aliases ---
  listAliases(): Promise<ModelAlias[]>;
  createAlias(
    data: Omit<ModelAlias, 'id' | 'created_at' | 'updated_at'>
  ): Promise<ModelAlias>;
  updateAlias(id: string, data: Partial<ModelAlias>): Promise<ModelAlias>;
  deleteAlias(id: string): Promise<void>;

  // --- Logs & stats ---
  queryLogs(filter?: Partial<LogFilter>): Promise<{ logs: RequestLog[]; total: number }>;

  /**
   * `since` is a duration string: `24h`, `7d`, `30d` (query param — see Phase 1G implementation).
   * Response is a bare `DashboardStats` JSON object (not wrapped).
   */
  getStats(since?: string): Promise<DashboardStats>;
}

/** Singleton: `export const api: ApiClient` */
```

**Error handling:** All async methods may **throw** `Error` with a message. Pages must **`try/catch`** (or equivalent) and surface a short user-visible error (e.g. `Card` with destructive text, or a `Banner` pattern using `Alert` if available).

---

## Shared UX Rules (all pages)

1. **Loading:** Show a centered spinner or skeleton rows while `loading === true`.
2. **Errors:** Show non-dismiss-blocking message until next successful fetch or user action.
3. **Layout width:** Max width ~1280px (`max-w-7xl mx-auto px-4 py-6` or similar), desktop-first.
4. **Navigation:** Use SvelteKit **`goto`** from `$app/navigation` for programmatic redirects after success.
5. **Route params:** Use **`import { page } from '$app/state'`** (SvelteKit 2 + runes). Example: `const providerId = $derived(page.params.id ?? '')` for dynamic `[id]` routes. Do **not** use legacy `$page` store subscription patterns.
6. **Tables:** Use shadcn `Table` components; truncate long text with `title` tooltip or `truncate max-w-*`.
7. **Numbers:** Format counts and percentages for display (e.g. error rate as `(stats.error_rate * 100).toFixed(1) + '%'` if API returns 0–1; if API returns 0–100 already, display accordingly — **inspect `getStats` response in dev** and document in code comment if ambiguous).

---

## Page 1: Dashboard Overview — `(dashboard)/+page.svelte`

### Layout

1. **Page title:** “Dashboard” (or “Overview”).
2. **Top row:** Four **stat cards** (use `Card` + `CardHeader` + `CardTitle` + `CardContent`):
   - Total requests — `stats.total_requests`
   - Avg latency — `stats.avg_latency_ms` (suffix `ms`)
   - Error rate — from `stats.error_rate` (format per Shared UX)
   - Active providers — **computed**: `providers.filter((p) => p.is_healthy).length`
3. **Middle:** Two tables **side by side** on large screens (`grid grid-cols-1 lg:grid-cols-2 gap-6`):
   - **Requests by model** — rows from `stats.by_model`: columns Model, Requests, Avg latency, Errors, Tokens (use fields from `ModelStats`).
   - **Requests by provider** — rows from `stats.by_provider`: columns Provider, Requests, Avg latency, Errors (use `ProviderStats`).
4. **Bottom:** **Time range** control: segmented buttons or `Select` with options **`24h`**, **`7d`**, **`30d`**. Changing the value re-fetches stats.

### Behavior

- On load and whenever **`timeRange`** changes, call:
  - `api.getStats(timeRange)`
  - `api.listProviders()` (for active provider count only; still fetch in parallel with stats).
- **`$state`:** `stats: DashboardStats | null`, `providers: Provider[]`, `loading: boolean`, `error: string | null`, `timeRange: string` (default `'24h'`).
- **`$effect`:** Must **track** `timeRange`. Read `timeRange` inside the effect before calling fetch, e.g.:

```svelte
<script lang="ts">
  import { api } from '$lib/api/client';
  import type { DashboardStats, Provider } from '$lib/types';

  let stats = $state<DashboardStats | null>(null);
  let providers = $state<Provider[]>([]);
  let loading = $state(true);
  let error = $state<string | null>(null);
  let timeRange = $state('24h');

  async function fetchData() {
    loading = true;
    error = null;
    try {
      const [s, p] = await Promise.all([api.getStats(timeRange), api.listProviders()]);
      stats = s;
      providers = p;
    } catch (e) {
      error = e instanceof Error ? e.message : 'Failed to load dashboard';
      stats = null;
    } finally {
      loading = false;
    }
  }

  $effect(() => {
    timeRange; // subscribe to changes
    fetchData();
  });
</script>
```

- If `stats` is null and not loading, show error empty state.
- **Do not** use `$:` reactive statements.

---

## Page 2: Provider List — `(dashboard)/providers/+page.svelte`

### Layout

- **Header row:** Title “Providers” + primary **Button** “Add Provider” linking to **`/providers/new`** (`<a href="/providers/new">` with `Button` as child, or `goto` on click).
- **Table** columns:
  | Column | Source |
  |--------|--------|
  | Name | `provider.name` |
  | Base URL | `provider.base_url` (truncate) |
  | Status | `<StatusBadge ... />` from `provider.is_healthy` |
  | Models count | The `Provider` type has **no** `model_count` field. **Do not** N+1 `getProvider` per row. Display **`—`** for v1. |
  | Last Check | `provider.health_checked_at` formatted (relative or short date) |
  | Actions | Link “View” or make row clickable |

### Behavior

- **`$effect` on mount:** `api.listProviders()` — set loading/error/list state.
- **Row navigation:** Entire row click or “View” navigates to **`/providers/{id}`** using `<a href="/providers/{id}">` or `goto('/providers/' + id)`.
- **Empty state:** Message + button to add provider when list is empty.

---

## Page 3: Onboarding Wizard — `(dashboard)/providers/new/+page.svelte`

Multi-step flow: **Info → Discovery → Confirm**.

### Step 1 — Provider info

- Fields: **Name** (required), **Base URL** (required), **API Key** (optional, password `Input`).
- **Next:** Validate non-empty name and base URL (basic URL shape optional). On success:
  - `api.createProvider({ name, base_url, api_key })`
  - Store returned `Provider` in `$state`, then `step = 2` and trigger discovery.

### Step 2 — Discovery

- When entering step 2 with a `provider` set:
  - Call **`api.discoverProvider(provider.id)`** once (guard with a flag if `$effect` runs twice).
  - Show loading spinner until resolved; on failure show error + retry button.
- **Models:** `discovery.models` — render checklist (`Checkbox`), **all checked by default**. Track selected model IDs in `$state<string[]>`.
- **Endpoints matrix:** Table columns:
  - Path | Method | Status | Enabled (toggle)
- **Status cell mapping** from `DiscoveredEndpoint.is_supported`:
  - `true` → label **Supported** (green `Badge`)
  - `false` → **Not supported** (muted/red)
  - `null` → **Unknown** (warning/yellow)
- **Default enabled state when discovery completes:**
  - Supported (`true`) → **enabled** by default.
  - Not supported (`false`) → **disabled** by default (toggle off, consider `disabled` on toggle if you want to prevent enabling unsupported — **spec:** allow toggle but validation on confirm is OK; **prefer:** unsupported defaults off, user can still enable if product allows).
  - Unknown (`null`) → **user choice**, default **off** or **on** — **spec:** default **off** for unknown; user may enable.
- Store endpoint rows as **`ConfirmEndpointInput[]`** built from discovery: `is_supported` must be **boolean** in payload — map **`null` → `false`** for persistence, or use **`is_supported: true`** only when API returned true; **spec:** use **`is_supported: endpoint.is_supported === true`** for the stored struct, and **`is_enabled`** from toggles.

Clarification for **`ConfirmEndpointInput.is_supported`:**

- Set **`is_supported`** to `true` only if discovery returned `true`; **`false`** if discovery returned `false` or `null` (unknown stored as not supported for DB).

### Step 3 — Confirmation

- Show summary: provider name, base URL, count of selected models, count of enabled endpoints.
- **Confirm** calls:

```typescript
await api.confirmProvider(provider.id, {
  models: selectedModelIds,
  endpoints: endpointInputs // ConfirmEndpointInput[]
});
```

- On success, **`goto('/providers/' + provider.id)`**.
- **Back** buttons optional: step 1 ↔ 2 only if no provider created yet; after create, avoid going back to step 1 without discarding (simplest: no back from step 3).

### State sketch

```svelte
<script lang="ts">
  import type { Provider, DiscoveryResult, ConfirmEndpointInput } from '$lib/types';

  let step = $state(1);
  let provider = $state<Provider | null>(null);
  let discovery = $state<DiscoveryResult | null>(null);
  let discoveryLoading = $state(false);
  let discoveryError = $state<string | null>(null);
  let selectedModels = $state<string[]>([]);
  let endpointRows = $state<ConfirmEndpointInput[]>([]);
  // form fields step 1: name, base_url, api_key with $state
</script>
```

---

## Page 4: Provider Detail — `(dashboard)/providers/[id]/+page.svelte`

### Layout

1. **Header:** Provider name + actions: **Re-discover**, **Delete**.
2. **Info card:** Name, base URL, `StatusBadge` for health, last check time.
3. **Edit form:** Name, Base URL, API Key (optional placeholder if empty). **Save** → `api.updateProvider(id, { name, base_url, api_key })` (omit empty api_key if API treats empty as no change — follow Phase 1G client behavior).
4. **Endpoints table:** Path, Method, Supported (yes/no), **Enabled** (`Switch` or checkbox) — on change call `api.updateEndpoint(providerId, endpoint.id, { is_enabled })`. Disable toggling while request in flight for that row optional.
5. **Models section:** List `ProviderModel` as table or list (`model_id`).
6. **Delete:** `Dialog` confirmation (“Type name to confirm” optional; **minimum:** Confirm/Cancel). On confirm → `api.deleteProvider(id)` → `goto('/providers')`.
7. **Re-discover:** Button → `api.discoverProvider(id)` → show results in a **`Dialog`** or inline panel (read-only list of models + endpoint statuses). **Does not** replace persisted data until user goes through confirm on onboarding — for detail page, **spec:** show discovery result in dialog only; optional “Apply” could call confirm with full payload — **not required for v1**; **minimum:** informational dialog after discover.

### Behavior

- **`page.params.id`** — if missing, show error.
- **`$effect`:** when `id` changes, `api.getProvider(id)` → populate local `$state` for provider, endpoints, models.
- **Optimistic UI** optional; on failure show toast-style error.

---

## Page 5: Model Aliases — `(dashboard)/models/+page.svelte`

### Layout

- Title “Model Aliases” + **Add Alias** opens `Dialog`.
- **Table:** Alias Name | Provider (name) | Model ID | Weight | Priority | Enabled | Actions (Edit, Delete).

### Behavior

- Mount: `api.listAliases()` + `api.listProviders()`.
- Join: `provider_id` → `providers.find(p => p.id === alias.provider_id)?.name ?? alias.provider_id`.
- **Add dialog fields:**
  - Alias name — `Input`
  - Provider — `Select` from providers
  - Model ID — `Select` **loaded after** provider selected: `api.getProvider(selectedProviderId)` and list `models[].model_id`
  - Weight — number, default **1**
  - Priority — number, default **0**
  - Enabled — checkbox, default **true**
- **Create:** `api.createAlias({ alias, provider_id, model_id, weight, priority, is_enabled })` — IDs snake_case per types.
- **Edit:** prefill dialog; `api.updateAlias(id, { ... })`.
- **Delete:** confirm dialog then `api.deleteAlias(id)`.
- Reset model dropdown when provider changes.

---

## Page 6: Request Logs — `(dashboard)/logs/+page.svelte`

### Layout

- **Filter bar:**
  - Model — text `Input` (filters `requested_model` / `resolved_model` per API)
  - Provider — `Select` populated from `api.listProviders()` (empty option “All”)
  - Since / Until — datetime-local `Input` **or** text ISO; **spec:** use **`Input type="datetime-local"`** and convert to ISO string for API, or plain text ISO. Defaults: **last 24 hours** — set `since` to now−24h and `until` to now on first load.
  - **Search** / **Apply** button applies filters.
- **Table:** Timestamp | Model | Provider | Status | Streamed | Latency | TTFT | Tokens | Error
  - Timestamp: `timestamp` or `created_at` — format with `toLocaleString()` or `Intl.DateTimeFormat`.
  - Model: `resolved_model ?? requested_model ?? '—'`
  - Provider: `provider_name ?? provider_id ?? '—'`
  - Status: HTTP status code with color: **2xx** green, **4xx** yellow, **5xx** red (`class` via small helper).
  - Streamed: yes/no from `is_streamed`
  - Latency: `total_time_ms` + `ms`
  - TTFT: `ttft_ms ?? '—'`
  - Tokens: show `total_tokens ?? '—'`; **optional:** expandable row or `title` tooltip with prompt/completion/cached breakdown.
  - Error: truncate `error_message`, full on tooltip.

### Behavior

- **`$state`:**

```svelte
<script lang="ts">
  import type { RequestLog } from '$lib/types';

  let logs = $state<RequestLog[]>([]);
  let total = $state(0);
  let loading = $state(false);
  let error = $state<string | null>(null);

  let filter = $state({
    model: '',
    provider_id: '',
    since: '',
    until: '',
    limit: 50,
    offset: 0
  });

  let totalPages = $derived(Math.max(1, Math.ceil(total / filter.limit)));
  let currentPage = $derived(Math.floor(filter.offset / filter.limit) + 1);
</script>
```

- **`fetchLogs`:** `api.queryLogs({ ...filter })` — assign `logs`, `total`.
- **`$effect`:** initial load with default `since`/`until` for last 24h; **do not** infinite-loop: either run once on mount via a flag or depend on explicit “Search” only — **spec:** refetch on **Search** click and when **Previous/Next** changes `offset`; initial load in `$effect` once (use `onMount` from `svelte` is acceptable for imperative fetch if clearer).
- **Pagination:** Previous sets `offset = Math.max(0, offset - limit)`, Next sets `offset = offset + limit` if more results (`offset + logs.length < total` or `currentPage < totalPages`).

---

## Svelte 5 Rules (mandatory)

| Do | Don’t |
|----|-------|
| `$state` for mutable UI state | `$:` reactive declarations |
| `$derived` for computed values | Svelte 4 `writable`/`readable` stores with `$` auto-subscription |
| `$effect` for side effects; read dependencies inside the effect | Implicit untracked deps causing stale UI |
| `$props()` in **components** that accept props | `<slot />` without `{@render children?.()}` in **new** layouts (layouts may already exist from Phase 1G — follow project: **`{@render children()}`** if the parent layout uses snippets) |
| Event handlers `onclick=` in Svelte 5 | `on:click` unless project still uses legacy — **prefer `onclick`** for Svelte 5 |

---

## Accessibility & Polish

- **Buttons:** `type="button"` for non-submit actions.
- **Forms:** associate `Label` + `id` on inputs.
- **Dialogs:** trap focus per shadcn defaults.

---

## Verification

From `frontend/`:

```bash
npm run check
```

Must pass with zero errors.

---

## Done Criteria

- [ ] Dashboard overview shows stats with time range selector (`24h` / `7d` / `30d`).
- [ ] Provider list with status badges and navigation to detail.
- [ ] Onboarding wizard: 3 steps (info, discovery with matrix + model selection, confirmation) and redirect after confirm.
- [ ] Provider detail: edit provider, toggle endpoints, delete with confirmation, re-discover feedback.
- [ ] Model aliases: list with provider names, add/edit/delete dialog with dynamic model list.
- [ ] Request logs: filters, table, pagination, status coloring, loading/error states.
- [ ] All pages handle loading and error states.
- [ ] Svelte 5 runes only — no Svelte 4 `$:` or store `$` patterns.
- [ ] `npm run check` passes.
