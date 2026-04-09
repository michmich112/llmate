# Spec 07: Frontend Core (Phase 1G)

## Goal

Set up the SvelteKit project with **Svelte 5** (runes: `$state`, `$derived`, `$effect`), **shadcn-svelte**, **Tailwind CSS v4**, and **TypeScript**. Implement the core shell: root layout, dashboard layout with sidebar navigation, login page, typed API client, shared domain types, and shared UI components.

This spec is **fully self-contained**. The implementing agent must **not** read other repository files (`Context.md`, other specs, or existing code). Everything required to implement and verify Phase 1G is in this document.

---

## Non-goals (Phase 1G)

- No data-heavy dashboard pages beyond a minimal placeholder for the dashboard home (if needed for `npm run build`).
- No charts, tables, or provider CRUD screens (later phases).

---

## Tech Stack (pinned by this spec)

| Layer | Choice |
|--------|--------|
| Framework | SvelteKit (latest 2.x compatible with Svelte 5) |
| UI | Svelte 5 with **runes only** — do **not** use Svelte 4 patterns (`$:` reactive blocks, legacy `export let`, store auto-subscription `$store` in components) |
| Styling | Tailwind CSS **v4** via `@tailwindcss/vite` |
| Components | **shadcn-svelte** for primitives (Button, Input, Card, etc.) |
| Language | TypeScript |
| Output | **SPA**: `@sveltejs/adapter-static` with `fallback: 'index.html'`, no prerendered pages |

---

## Files to Create

| # | Path |
|---|------|
| 1 | `frontend/package.json` |
| 2 | `frontend/svelte.config.js` |
| 3 | `frontend/vite.config.ts` |
| 4 | `frontend/tailwind.config.ts` |
| 5 | `frontend/tsconfig.json` |
| 6 | `frontend/src/app.html` |
| 7 | `frontend/src/app.css` |
| 8 | `frontend/src/routes/+layout.svelte` |
| 9 | `frontend/src/routes/+layout.ts` |
| 10 | `frontend/src/routes/login/+page.svelte` |
| 11 | `frontend/src/routes/(dashboard)/+layout.svelte` |
| 12 | `frontend/src/routes/(dashboard)/+layout.ts` |
| 13 | `frontend/src/lib/api/client.ts` |
| 14 | `frontend/src/lib/types/index.ts` |
| 15 | `frontend/src/lib/components/Sidebar.svelte` |
| 16 | `frontend/src/lib/components/StatusBadge.svelte` |

**Additional files required for a green build (not optional if the checklist fails):**

| Path | Purpose |
|------|---------|
| `frontend/src/routes/(dashboard)/+page.svelte` | Minimal dashboard home so `/` resolves under `(dashboard)` and `npm run build` succeeds |
| `frontend/src/lib/utils.ts` | `cn()` helper for shadcn class merging (see shadcn-svelte section) |
| `frontend/components.json` | shadcn-svelte configuration (if using CLI-generated components) |

---

## Backend contract (admin HTTP API) — inline reference

The gateway serves an **admin JSON API** mounted under the **`/admin`** prefix (ACCESS_KEY auth on the server). The dev Vite proxy forwards browser requests to the Go server.

### Auth

- Clients send **`Authorization: Bearer <ACCESS_KEY>`** (preferred) or **`X-Access-Key`** (server may support both; the frontend uses Bearer only in this phase).
- **`POST /admin/auth`** — body ignored. If the key is valid, response **`200`** with JSON `{"valid": true}`. If invalid/missing, **`401`** with `{"error":"..."}`.

### JSON error shape

Failed requests use **`{"error": "<message>"}`** with `Content-Type: application/json` unless noted.

### Routes and response envelopes

All paths below are **relative to `/admin`** (i.e. fetch `'/admin/providers'`, not `'/providers'`).

| Method | Path | Success body (JSON) |
|--------|------|---------------------|
| `POST` | `/auth` | `{"valid": true}` |
| `GET` | `/providers` | `{"providers": Provider[]}` |
| `POST` | `/providers` | `{"provider": Provider}` (201) |
| `GET` | `/providers/{id}` | `{"provider": Provider, "endpoints": ProviderEndpoint[], "models": ProviderModel[]}` |
| `PUT` | `/providers/{id}` | `{"provider": Provider}` |
| `DELETE` | `/providers/{id}` | empty (204) |
| `POST` | `/providers/{id}/discover` | `DiscoveryResult` (see types) |
| `POST` | `/providers/{id}/confirm` | `{"provider": Provider, "endpoints": ProviderEndpoint[], "models": ProviderModel[]}` |
| `PUT` | `/providers/{id}/endpoints/{eid}` | `{"endpoint": ProviderEndpoint}` |
| `GET` | `/aliases` | `{"aliases": ModelAlias[]}` |
| `POST` | `/aliases` | `{"alias": ModelAlias}` (201) |
| `PUT` | `/aliases/{id}` | `{"alias": ModelAlias}` |
| `DELETE` | `/aliases/{id}` | empty (204) |
| `GET` | `/logs` | `{"logs": RequestLog[], "total": number}` |
| `GET` | `/stats?since=...` | `DashboardStats` (top-level fields, not wrapped) |

### Query parameters — `GET /admin/logs`

| Param | Maps to |
|-------|---------|
| `model` | filter by model |
| `provider_id` | filter by provider |
| `since` | RFC3339 string |
| `until` | RFC3339 string |
| `limit` | number (server default 50, max 1000) |
| `offset` | number (default 0) |

### Query parameters — `GET /admin/stats`

- `since` — duration string, e.g. `24h`, `7d`, `30d`. Server default when omitted: `24h`.

### `POST /admin/providers/{id}/confirm` body

```json
{
  "endpoints": [
    {
      "path": "/v1/chat/completions",
      "method": "POST",
      "is_supported": true,
      "is_enabled": true
    }
  ],
  "models": ["model-id-1", "model-id-2"]
}
```

---

## Project setup commands (reference)

From repository root:

```bash
cd frontend
npm install
npx shadcn-svelte@latest init
npx shadcn-svelte@latest add button input card
npm run check
npm run build
```

Adjust CLI prompts to match **Tailwind v4**, **TypeScript**, and **Svelte 5** as offered by the installed shadcn-svelte version. If the CLI differs, create **`components.json`** and component files manually so that imports like `$lib/components/ui/button/button.svelte` resolve — the login and sidebar specs below assume **Button**, **Input**, and **Card** exist under the shadcn convention your init produces.

---

## File: `frontend/package.json`

Must include at minimum:

- **SvelteKit / Svelte:** `svelte` (^5), `@sveltejs/kit`, `@sveltejs/adapter-static`
- **Build:** `vite`, `typescript`, `@types/node`
- **Tailwind v4:** `tailwindcss`, `@tailwindcss/vite`
- **shadcn-svelte peers:** `bits-ui`, `clsx`, `tailwind-merge` (and `tailwind-variants` if your shadcn init adds it)
- **shadcn-svelte** package (CLI may add `shadcn-svelte` as devDependency — follow current docs)

**Scripts:**

```json
{
  "scripts": {
    "dev": "vite dev",
    "build": "vite build",
    "preview": "vite preview",
    "check": "svelte-kit sync && svelte-check --tsconfig ./tsconfig.json"
  }
}
```

Use compatible version ranges so `npm install` resolves on the day of implementation.

---

## File: `frontend/svelte.config.js`

Requirements:

- Preprocess: `vitePreprocess()` from `@sveltejs/vite-plugin-svelte`
- **`adapter-static`** with:
  - `pages: 'build'`
  - `assets: 'build'`
  - `fallback: 'index.html'` (SPA)
- **`kit.prerender.entries: []`** so no pages are prerendered (SPA mode)

Example structure (adjust imports to match installed package versions):

```js
import adapter from '@sveltejs/adapter-static';
import { vitePreprocess } from '@sveltejs/vite-plugin-svelte';

/** @type {import('@sveltejs/kit').Config} */
const config = {
  preprocess: vitePreprocess(),
  kit: {
    adapter: adapter({
      pages: 'build',
      assets: 'build',
      fallback: 'index.html',
      strict: true
    }),
    prerender: {
      entries: []
    }
  }
};

export default config;
```

If `strict: true` conflicts with SPA-only output in your toolchain version, set `strict: false` **only** if required for a successful build, and document the reason in a one-line comment.

---

## File: `frontend/vite.config.ts`

Requirements:

- **Plugins:** `@tailwindcss/vite`, `sveltekit()` from `@sveltejs/kit/vite`
- **Dev server proxy** (object form), all **`http://localhost:8080`**:
  - `'/api'` → gateway
  - `'/admin'` → gateway
  - `'/v1'` → gateway (OpenAI-compatible proxy path for local tools)

Example:

```ts
import tailwindcss from '@tailwindcss/vite';
import { sveltekit } from '@sveltejs/kit/vite';
import { defineConfig } from 'vite';

export default defineConfig({
  plugins: [tailwindcss(), sveltekit()],
  server: {
    proxy: {
      '/api': { target: 'http://localhost:8080', changeOrigin: true },
      '/admin': { target: 'http://localhost:8080', changeOrigin: true },
      '/v1': { target: 'http://localhost:8080', changeOrigin: true }
    }
  }
});
```

---

## File: `frontend/tailwind.config.ts`

Tailwind v4: content paths must include Svelte and TS files under `src`. Example:

```ts
import type { Config } from 'tailwindcss';

export default {
  content: ['./src/**/*.{html,js,svelte,ts}']
} satisfies Config;
```

If your Tailwind v4 setup relies **only** on the Vite plugin and defaults, keep this file minimal but valid.

---

## File: `frontend/tsconfig.json`

Use SvelteKit’s standard strict TS config: `extends` `./.svelte-kit/tsconfig.json` when present after `svelte-kit sync`, **`moduleResolution`**: `"bundler"` or `"node16"` per Kit template, **`verbatimModuleSyntax`**: true if the template sets it.

Include path alias:

```json
{
  "compilerOptions": {
    "paths": {
      "$lib": ["./src/lib"],
      "$lib/*": ["./src/lib/*"]
    }
  }
}
```

(Merge with generated options — do not break `svelte-check`.)

---

## File: `frontend/src/app.html`

Standard SvelteKit shell: `%sveltekit.head%`, `%sveltekit.body%`, viewport meta, title placeholder.

---

## File: `frontend/src/app.css`

Tailwind v4 entry:

```css
@import 'tailwindcss';
```

Add **`@layer base`** rules only if shadcn’s theme CSS variables are required by your init; otherwise keep this file minimal.

---

## File: `frontend/src/lib/utils.ts`

Provide **`cn`** for merging classes (shadcn pattern):

```typescript
import { type ClassValue, clsx } from 'clsx';
import { twMerge } from 'tailwind-merge';

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}
```

---

## File: `frontend/src/lib/types/index.ts`

These mirror the Go/JSON field names (`snake_case` in JSON). **Implement exactly** the interfaces below (add `LogFilter` and confirm/discovery helpers as specified).

```typescript
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

/** Query filter for GET /admin/logs — use ISO strings for since/until */
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

export interface DiscoveryResult {
  models: string[];
  endpoints: DiscoveredEndpoint[];
}

export interface DiscoveredEndpoint {
  path: string;
  method: string;
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

export interface ProviderDetailResponse {
  provider: Provider;
  endpoints: ProviderEndpoint[];
  models: ProviderModel[];
}
```

---

## File: `frontend/src/lib/api/client.ts`

### Responsibilities

- Singleton **`export const api = new ApiClient()`**
- Persist ACCESS_KEY in **`localStorage`** under the key **`access_key`**
- All admin requests use URL prefix **`/admin`** (leading slash, no trailing slash on prefix)
- **`Authorization: Bearer <key>`** on every admin request when a key is set
- **`Content-Type: application/json`** when a body is present
- Parse JSON responses; on **401**: clear stored key and **redirect** the browser to **`/login`** (`window.location.href = '/login'` or equivalent; do not use SvelteKit navigation from a shared singleton unless you know it is client-only and safe)
- Non-2xx: read JSON if possible and throw **`Error`** with a useful message (include `error` field from body when present)

### Methods (signatures)

```typescript
import type {
  ConfirmProviderBody,
  DashboardStats,
  DiscoveryResult,
  LogFilter,
  ModelAlias,
  Provider,
  ProviderDetailResponse,
  ProviderEndpoint,
  RequestLog
} from '$lib/types';

class ApiClient {
  private accessKey: string | null;

  constructor() {
    if (typeof localStorage !== 'undefined') {
      this.accessKey = localStorage.getItem('access_key');
    } else {
      this.accessKey = null;
    }
  }

  setAccessKey(key: string): void;
  clearAccessKey(): void;
  isAuthenticated(): boolean;

  /** POST /admin/auth — returns true if response is 200 and JSON valid:true */
  async validateKey(key: string): Promise<boolean>;

  async listProviders(): Promise<Provider[]>;
  async createProvider(data: {
    name: string;
    base_url: string;
    api_key?: string;
  }): Promise<Provider>;

  async getProvider(id: string): Promise<ProviderDetailResponse>;

  async updateProvider(id: string, data: Partial<Provider>): Promise<Provider>;

  async deleteProvider(id: string): Promise<void>;

  async discoverProvider(id: string): Promise<DiscoveryResult>;

  async confirmProvider(id: string, data: ConfirmProviderBody): Promise<ProviderDetailResponse>;

  async updateEndpoint(
    providerId: string,
    endpointId: string,
    data: { is_enabled: boolean }
  ): Promise<ProviderEndpoint>;

  async listAliases(): Promise<ModelAlias[]>;

  async createAlias(
    data: Omit<ModelAlias, 'id' | 'created_at' | 'updated_at'>
  ): Promise<ModelAlias>;

  async updateAlias(id: string, data: Partial<ModelAlias>): Promise<ModelAlias>;

  async deleteAlias(id: string): Promise<void>;

  async queryLogs(filter?: Partial<LogFilter>): Promise<{ logs: RequestLog[]; total: number }>;

  /** since: duration string e.g. 24h, 7d — passed as query param */
  async getStats(since?: string): Promise<DashboardStats>;
}

export const api = new ApiClient();
```

### Implementation notes

- **`validateKey(key)`:** temporarily set Authorization to the candidate key (without persisting until success), `POST /admin/auth`. If **200** and body `{ valid: true }`, return **`true`**. Otherwise **`false`**. Do **not** clear an existing stored key on failure of validation of a *candidate* key unless you explicitly overwrite — recommended: use a private `requestWithKey` for this call or pass an override key header.
- **`setAccessKey`:** write `localStorage` and update in-memory field.
- **`clearAccessKey`:** remove from `localStorage` and set field `null`.
- **`isAuthenticated`:** `true` if non-empty key in memory/storage.
- **Unwrap** envelopes: e.g. `listProviders` returns `data.providers` as `Provider[]`.
- **`getStats`:** response is **not** wrapped — parse as `DashboardStats` directly.

---

## File: `frontend/src/routes/+layout.ts`

Disable SSR for the SPA shell:

```typescript
export const ssr = false;
export const prerender = false;
```

---

## File: `frontend/src/routes/+layout.svelte`

Svelte 5 + SvelteKit 2: use **snippet children**, not legacy slots.

```svelte
<script lang="ts">
  import type { Snippet } from 'svelte';

  let { children }: { children: Snippet } = $props();
</script>

{@render children()}
```

If your Kit version types `children` differently, align with the official SvelteKit 2 + Svelte 5 layout template **without** reintroducing `<slot />`.

---

## File: `frontend/src/routes/login/+page.svelte`

### Behavior

- Controlled input bound to **`accessKey`** state (`$state`)
- **Submit** (button or form `onsubmit` with `preventDefault`) calls **`api.validateKey(trimmed)`**
  - If **true**: **`api.setAccessKey(trimmed)`**, then **`goto('/')`** from **`$app/navigation`**
  - If **false**: set **`error`** to a clear message (e.g. “Invalid access key”)
- **`loading`**: disable button / show loading state while awaiting
- Use shadcn **Card**, **Input**, **Button**; centered column layout (flex/min-h-screen)

### Runes (required shape)

```svelte
<script lang="ts">
  import { goto } from '$app/navigation';
  import { api } from '$lib/api/client';

  let accessKey = $state('');
  let error = $state('');
  let loading = $state(false);

  async function handleLogin(e: Event) {
    e.preventDefault();
    loading = true;
    error = '';
    try {
      const key = accessKey.trim();
      if (!key) {
        error = 'Enter an access key';
        return;
      }
      const ok = await api.validateKey(key);
      if (!ok) {
        error = 'Invalid access key';
        return;
      }
      api.setAccessKey(key);
      await goto('/');
    } finally {
      loading = false;
    }
  }
</script>
```

Import paths for UI components must match your shadcn output (e.g. `$lib/components/ui/button/button.svelte`).

---

## File: `frontend/src/routes/(dashboard)/+layout.ts`

Auth guard — runs on client (SSR off):

```typescript
import { redirect } from '@sveltejs/kit';
import { api } from '$lib/api/client';

export function load() {
  if (!api.isAuthenticated()) {
    redirect(307, '/login');
  }
}
```

---

## File: `frontend/src/routes/(dashboard)/+layout.svelte`

### Layout

- Full viewport flex row
- **Left:** fixed width **~250px**, full height, border-r, background subtle
- **Right:** **`flex-1`**, overflow auto, padding
- Embed **`Sidebar`** at left; **`{@render children()}`** in main

Use Svelte 5 `$props()` / snippet children consistent with root layout.

---

## File: `frontend/src/lib/components/Sidebar.svelte`

### Navigation

| Label | Path |
|-------|------|
| Dashboard | `/` |
| Providers | `/providers` |
| Models | `/models` |
| Logs | `/logs` |

### Active link

- Use **`page.url.pathname`** from **`$app/state`** (`import { page } from '$app/state'`) for SvelteKit 2 + Svelte 5
- Highlight the nav item whose path matches (exact **`/`** for dashboard; use **`startsWith`** for section roots if needed, but **`/`** must not always match every route — treat `/` as active only when pathname is **`/`**)

### Logout

- Button at bottom: **`api.clearAccessKey()`**, then **`goto('/login')`**

### Styling

- Vertical stack with gap; use shadcn **Button** variant `ghost` or `outline` for links (or `<a>` styled as buttons)

---

## File: `frontend/src/lib/components/StatusBadge.svelte`

### Props

```typescript
status: 'healthy' | 'unhealthy' | 'unknown';
```

### Appearance

| status | Style |
|--------|--------|
| `healthy` | Green background/text (e.g. Tailwind `bg-green-100 text-green-800` or dark-friendly pair) |
| `unhealthy` | Red |
| `unknown` | Gray |

Small rounded pill; text: **Healthy**, **Unhealthy**, **Unknown**.

Use `$props()` with TypeScript.

---

## File: `frontend/src/routes/(dashboard)/+page.svelte` (recommended)

Minimal placeholder so navigation to **`/`** works:

- Heading: “Dashboard”
- Short subtitle or empty state
- Optional: `StatusBadge` demo with three states (development-only visual check)

---

## Verification

From `frontend/`:

```bash
npm run check
npm run build
```

- **`npm run check`:** zero errors from `svelte-check`/TypeScript
- **`npm run build`:** produces static output under `build/` per adapter config

---

## Done Criteria

- [ ] SvelteKit project scaffolded with dependencies above
- [ ] **`@sveltejs/adapter-static`** with **`fallback: 'index.html'`**, **`prerender.entries: []`**
- [ ] Vite dev proxy: **`/api`**, **`/admin`**, **`/v1`** → **`http://localhost:8080`**
- [ ] TypeScript types in **`src/lib/types/index.ts`** match this spec (including **`LogFilter`**, **`ConfirmProviderBody`**)
- [ ] **`ApiClient`** implements all methods with correct **`/admin/...`** paths and JSON unwrapping
- [ ] Login page validates key, stores key, redirects to **`/`**
- [ ] Dashboard layout shows **Sidebar**; unauthenticated users redirected to **`/login`**
- [ ] **Svelte 5 runes only** in new components — no Svelte 4 reactivity syntax
- [ ] **`npm run build`** succeeds
- [ ] **`npm run check`** passes

---

## Notes for later phases

- Add route files for **`/providers`**, **`/models`**, **`/logs`** under **`(dashboard)`** when those UIs are implemented.
- Keep **`api`** as the single integration point for admin HTTP calls.
