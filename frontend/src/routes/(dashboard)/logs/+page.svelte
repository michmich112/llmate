<script lang="ts">
  import { onMount } from 'svelte';
  import { api } from '$lib/api/client';
  import type { RequestLog, Provider, StreamingLog, ActiveRequest } from '$lib/types';
  import { Button } from '$lib/components/ui/button';
  import { Input } from '$lib/components/ui/input';
  import { Card, CardHeader, CardTitle, CardContent } from '$lib/components/ui/card';

  let logs = $state<RequestLog[]>([]);
  let total = $state(0);
  let loading = $state(false);
  let error = $state<string | null>(null);
  let providers = $state<Provider[]>([]);
  let activeRequests = $state<ActiveRequest[]>([]);
  let activeLoading = $state(false);
  let activeError = $state<string | null>(null);

  let filter = $state({
    model: '',
    provider_id: '',
    since: '',
    until: '',
    status: '',
    limit: 50,
    offset: 0
  });

  let totalPages = $derived(Math.max(1, Math.ceil(total / filter.limit)));
  let currentPage = $derived(Math.floor(filter.offset / filter.limit) + 1);
  let hasNext = $derived(filter.offset + logs.length < total);
  let hasPrev = $derived(filter.offset > 0);

  // Unify active + completed requests into display rows.
  type Row = {
    key: string;
    kind: 'active' | 'log';
    id: string;
    timestamp: string;
    model: string;
    provider: string;
    isActive: boolean;
    isStreamed: boolean | null;
    latencyMs: number | null;
    ttftMs: number | null | undefined;
    promptTokens: number | null | undefined;
    completionTokens: number | null | undefined;
    cachedTokens: number | null | undefined;
    costUsd: number | undefined;
    errorMessage: string | null | undefined;
    log: RequestLog | null;
    active: ActiveRequest | null;
  };

  function activeToRow(a: ActiveRequest): Row {
    return {
      key: 'active-' + a.id, kind: 'active', id: a.id, timestamp: a.started_at, model: a.model,
      provider: '—', isActive: true, isStreamed: null, latencyMs: null, ttftMs: null,
      promptTokens: null, completionTokens: null, cachedTokens: null, costUsd: undefined,
      errorMessage: null, log: null, active: a
    };
  }

  function logToRow(l: RequestLog): Row {
    return {
      key: 'log-' + l.id, kind: 'log', id: l.id, timestamp: l.timestamp ?? l.created_at,
      model: l.resolved_model ?? l.requested_model ?? '—',
      provider: l.provider_name ?? l.provider_id ?? '—',
      isActive: false, isStreamed: l.is_streamed, latencyMs: l.total_time_ms, ttftMs: l.ttft_ms,
      promptTokens: l.prompt_tokens, completionTokens: l.completion_tokens, cachedTokens: l.cached_tokens,
      costUsd: l.estimated_cost_usd, errorMessage: l.error_message, log: l, active: null
    };
  }

  let activeRows = $derived(activeRequests.map(activeToRow));
  let logRows = $derived(logs.map(logToRow));
  let rows = $derived(filter.status === 'active' ? activeRows : activeRows.concat(logRows));
  let visibleCount = $derived(filter.status === 'active' ? activeRows.length : activeRows.length + logs.length);

  // Refresh handler
  function handleRefresh() {
    fetchLogs();
    fetchActive();
  }

  // Row detail dialog (works for both active and completed requests)
  let detailRow = $state<Row | null>(null);
  let detailLoading = $state(false);
  let detailError = $state<string | null>(null);
  let showDetail = $state(false);
  let streamingLogs = $state<StreamingLog[]>([]);
  let streamingLogsLoading = $state(false);
  let streamingLogsError = $state<string | null>(null);

  const streamingHasPurgedBodies = $derived(streamingLogs.some((c) => c.body_purged));

  function defaultSince(): string {
    const d = new Date(Date.now() - 24 * 60 * 60 * 1000);
    return d.toISOString().slice(0, 16);
  }

  function defaultUntil(): string {
    return new Date().toISOString().slice(0, 16);
  }

  onMount(() => {
    filter.since = defaultSince();
    filter.until = defaultUntil();
    fetchLogs();
    fetchActive();
    api.listProviders().then((p) => (providers = p)).catch(() => {});
  });

  async function fetchActive() {
    activeLoading = true;
    activeError = null;
    try {
      const res = await api.getActive();
      activeRequests = res.requests;
    } catch (e) {
      activeError = e instanceof Error ? e.message : 'Failed to load active requests';
    } finally {
      activeLoading = false;
    }
  }

  async function fetchLogs() {
    loading = true;
    error = null;
    try {
      const params: Record<string, string | number> = {
        limit: filter.limit,
        offset: filter.offset
      };
      if (filter.model.trim()) params.model = filter.model.trim();
      if (filter.provider_id) params.provider_id = filter.provider_id;
      if (filter.since) params.since = new Date(filter.since).toISOString();
      if (filter.until) params.until = new Date(filter.until).toISOString();
      if (filter.status) params.status = filter.status;

      const result = await api.queryLogs(params);
      logs = result.logs;
      total = result.total;
    } catch (e) {
      error = e instanceof Error ? e.message : 'Failed to load logs';
    } finally {
      loading = false;
    }
  }

  function handleSearch() {
    filter.offset = 0;
    fetchLogs();
  }

  function handlePrev() {
    filter.offset = Math.max(0, filter.offset - filter.limit);
    fetchLogs();
  }

  function handleNext() {
    filter.offset = filter.offset + filter.limit;
    fetchLogs();
  }

  async function openDetail(row: Row) {
    showDetail = true;
    detailRow = row;
    detailLoading = false;
    detailError = null;
    streamingLogs = [];
    streamingLogsLoading = false;
    streamingLogsError = null;
    if (row.kind === 'log' && row.log) {
      detailLoading = true;
      try {
        const result = await api.getLog(row.log.id);
        detailRow = logToRow(result.log);
        if (result.log.is_streamed) {
          streamingLogsLoading = true;
          try {
            streamingLogs = await api.getStreamingLogs(result.log.id);
          } catch (e) {
            streamingLogsError = e instanceof Error ? e.message : 'Failed to load streaming chunks';
          } finally {
            streamingLogsLoading = false;
          }
        }
      } catch (e) {
        detailError = e instanceof Error ? e.message : 'Failed to load log detail';
      } finally {
        detailLoading = false;
      }
    }
  }

  function closeDetail() {
    showDetail = false;
    detailRow = null;
    detailError = null;
    streamingLogs = [];
    streamingLogsLoading = false;
    streamingLogsError = null;
  }

  function statusClass(code: number): string {
    if (code >= 200 && code < 300) return 'text-green-600 font-medium';
    if (code >= 400 && code < 500) return 'text-yellow-600 font-medium';
    if (code >= 500) return 'text-red-600 font-medium';
    return 'text-muted-foreground';
  }

  function formatTs(ts: string): string {
    return new Date(ts).toLocaleString();
  }

  function formatCost(cost?: number): string {
    if (cost == null) return '—';
    if (cost === 0) return '$0.0000';
    if (cost < 0.0001) return '<$0.0001';
    return '$' + cost.toFixed(4);
  }

  /** Detail total: breakdown total when API sent it, else stored estimate */
  function detailTotalCostUSD(log: RequestLog): number | undefined {
    return log.cost_breakdown?.total_usd ?? log.estimated_cost_usd;
  }

  function prettyJSON(s: string): string {
    try {
      return JSON.stringify(JSON.parse(s), null, 2);
    } catch {
      return s;
    }
  }
</script>

<div class="space-y-6">
  <div class="flex items-center justify-between">
    <h1 class="text-2xl font-bold tracking-tight">Request Logs</h1>
    <Button variant="outline" size="sm" onclick={handleRefresh} disabled={loading}>
      {#if loading}
        <svg class="h-4 w-4 animate-spin" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <path d="M21 12a9 9 0 1 1-6.219-8.56" />
        </svg>
      {/if}
      Refresh
    </Button>
  </div>

  <!-- Filter bar -->
  <Card>
    <CardContent class="pt-4">
      <div class="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-6">
        <div class="space-y-1">
          <label for="filter-model" class="text-xs font-medium text-muted-foreground">Model</label>
          <Input id="filter-model" bind:value={filter.model} placeholder="Filter by model..." />
        </div>

        <div class="space-y-1">
          <label for="filter-provider" class="text-xs font-medium text-muted-foreground">Provider</label>
          <select
            id="filter-provider"
            bind:value={filter.provider_id}
            class="flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2"
          >
            <option value="">All Providers</option>
            {#each providers as p}
              <option value={p.id}>{p.name}</option>
            {/each}
          </select>
        </div>

        <div class="space-y-1">
          <label for="filter-status" class="text-xs font-medium text-muted-foreground">Status</label>
          <select
            id="filter-status"
            bind:value={filter.status}
            class="flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2"
          >
            <option value="">All Statuses</option>
            <option value="success">Success (2xx)</option>
            <option value="error">All Errors (4xx + 5xx)</option>
            <option value="4xx">Client Error (4xx)</option>
            <option value="5xx">Server Error (5xx)</option>
          </select>
        </div>

        <div class="space-y-1">
          <label for="filter-since" class="text-xs font-medium text-muted-foreground">Since</label>
          <input
            id="filter-since"
            type="datetime-local"
            bind:value={filter.since}
            class="flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2"
          />
        </div>

        <div class="space-y-1">
          <label for="filter-until" class="text-xs font-medium text-muted-foreground">Until</label>
          <input
            id="filter-until"
            type="datetime-local"
            bind:value={filter.until}
            class="flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2"
          />
        </div>

        <div class="flex items-end">
          <Button class="w-full" onclick={handleSearch}>Search</Button>
        </div>
      </div>
    </CardContent>
  </Card>

  {#if error}
    <div class="rounded-md border border-destructive/50 bg-destructive/10 px-4 py-3 text-sm text-destructive">
      {error}
    </div>
  {/if}

  {#if !loading}
    <p class="text-sm text-muted-foreground">
      Showing {filter.offset + 1}–{Math.min(filter.offset + logs.length, total)} of {total} requests
    </p>
  {/if}

  <!-- Combined requests table -->
  <Card>
    <CardHeader>
      <CardTitle>
        Requests
        {#if filter.status === 'active' && activeRequests.length > 0}
          <span class="ml-1 text-xs font-normal text-muted-foreground">
            ({activeRequests.length} in-flight)
          </span>
        {/if}
      </CardTitle>
    </CardHeader>
    <CardContent class="p-0">
      {#if filter.status === 'active'}
        {#if activeLoading}
          <p class="px-6 py-12 text-center text-sm text-muted-foreground">Loading…</p>
        {:else if activeError}
          <div class="rounded-md border border-destructive/50 bg-destructive/10 px-3 py-2 text-sm text-destructive">
            {activeError}
          </div>
        {:else if activeRequests.length === 0}
          <p class="px-6 py-12 text-center text-sm text-muted-foreground">No requests currently in flight.</p>
        {/if}
      {:else if loading}
        <p class="px-6 py-12 text-center text-sm text-muted-foreground">Loading…</p>
      {:else if rows.length === 0}
        <p class="px-6 py-12 text-center text-sm text-muted-foreground">No requests found for the selected filters.</p>
      {/if}

      {#if rows.length > 0}
        <div class="overflow-x-auto">
          <table class="w-full text-sm">
            <thead>
              <tr class="border-b bg-muted/50 text-left text-xs font-medium uppercase tracking-wide text-muted-foreground">
                <th class="whitespace-nowrap px-4 py-3">Request ID</th>
                <th class="whitespace-nowrap px-4 py-3">Timestamp</th>
                <th class="px-4 py-3">Model</th>
                <th class="px-4 py-3">Provider</th>
                <th class="px-4 py-3">Status</th>
                <th class="px-4 py-3">Stream</th>
                <th class="px-4 py-3 text-right">Latency</th>
                <th class="px-4 py-3 text-right">TTFT</th>
                <th class="px-4 py-3 text-right">In</th>
                <th class="px-4 py-3 text-right">Out</th>
                <th class="px-4 py-3 text-right">Cached</th>
                <th class="px-4 py-3 text-right">Cost</th>
                <th class="px-4 py-3">Error</th>
              </tr>
            </thead>
            <tbody>
              {#each rows as row (row.key)}
                <tr
                  class="border-b last:border-0 cursor-pointer hover:bg-muted/30"
                  onclick={() => openDetail(row)}
                >
                  <td class="whitespace-nowrap px-4 py-3">
  <span class="font-mono text-xs underline decoration-primary underline-offset-4 text-primary">{row.id.slice(0, 8)}</span>
</td>
                  <td class="whitespace-nowrap px-4 py-3 text-xs">{formatTs(row.timestamp)}</td>
                  <td class="max-w-[160px] px-4 py-3">
                    <span class="block truncate font-mono text-xs">{row.model}</span>
                  </td>
                  <td class="max-w-[120px] px-4 py-3">
                    <span class="block truncate text-xs">{row.provider}</span>
                  </td>
                  <td class="px-4 py-3">
                    {#if row.isActive}
                      <span
                        class="inline-flex items-center rounded bg-yellow-500/15 px-2 py-0.5 text-xs font-medium text-yellow-700 dark:text-yellow-500"
                      >
                        Active
                      </span>
                    {:else if row.log?.status_code != null}
                      <span class={statusClass(row.log.status_code)}>{row.log.status_code}</span>
                    {:else}
                      <span class="text-xs text-muted-foreground">—</span>
                    {/if}
                  </td>
                  <td class="px-4 py-3 text-xs">
                    {#if row.isActive}
                      —
                    {:else}
                      {row.log?.is_streamed ? 'Yes' : 'No'}
                    {/if}
                  </td>
                  <td class="px-4 py-3 text-right text-xs">
                    {#if row.isActive}
                      —
                    {:else if row.log?.total_time_ms != null}
                      {row.log.total_time_ms}ms
                    {:else}
                      —
                    {/if}
                  </td>
                  <td class="px-4 py-3 text-right text-xs">
                    {#if row.isActive}
                      —
                    {:else if row.log?.ttft_ms != null}
                      {row.log.ttft_ms}ms
                    {:else}
                      —
                    {/if}
                  </td>
                  <td class="px-4 py-3 text-right text-xs">
                    {#if row.isActive}
                      —
                    {:else}
                      {row.log?.prompt_tokens != null ? row.log.prompt_tokens.toLocaleString() : '—'}
                    {/if}
                  </td>
                  <td class="px-4 py-3 text-right text-xs">
                    {#if row.isActive}
                      —
                    {:else}
                      {row.log?.completion_tokens != null ? row.log.completion_tokens.toLocaleString() : '—'}
                    {/if}
                  </td>
                  <td class="px-4 py-3 text-right text-xs">
                    {#if row.isActive}
                      —
                    {:else}
                      {row.log?.cached_tokens != null && row.log.cached_tokens > 0 ? row.log.cached_tokens.toLocaleString() : '—'}
                    {/if}
                  </td>
                  <td class="px-4 py-3 text-right text-xs">
                    {#if row.isActive}
                      —
                    {:else}
                      {formatCost(row.log?.estimated_cost_usd)}
                    {/if}
                  </td>
                  <td class="max-w-[180px] px-4 py-3">
                    {#if row.isActive}
                      <span class="text-xs text-muted-foreground">In flight</span>
                    {:else if row.log?.error_message}
                      <span class="block truncate text-xs text-red-500">{row.log.error_message}</span>
                    {:else}
                      <span class="text-xs text-muted-foreground">—</span>
                    {/if}
                  </td>
                </tr>
              {/each}
            </tbody>
          </table>
        </div>
      {/if}
    </CardContent>
  </Card>
  <!-- Pagination -->
  {#if total > filter.limit}
    <div class="flex items-center justify-between">
      <Button variant="outline" type="button" disabled={!hasPrev} onclick={handlePrev}>
        ← Previous
      </Button>
      <span class="text-sm text-muted-foreground">
        Page {currentPage} of {totalPages}
      </span>
      <Button variant="outline" type="button" disabled={!hasNext} onclick={handleNext}>
        Next →
      </Button>
    </div>
  {/if}
</div>

<!-- Request Detail Dialog -->
{#if showDetail}
  <div
    role="presentation"
    class="fixed inset-0 z-50 flex items-start justify-center overflow-y-auto bg-black/50 py-8"
    onclick={closeDetail}
    onkeydown={(e) => { if (e.key === 'Escape') closeDetail(); }}
  >
    <div
      role="dialog"
      aria-modal="true"
      tabindex="-1"
      class="mx-4 w-full max-w-3xl rounded-lg bg-popover shadow-xl"
      onclick={(e) => e.stopPropagation()}
      onkeydown={(e) => e.stopPropagation()}
    >
      <!-- Header -->
      <div class="flex items-center justify-between border-b px-6 py-4">
        <h2 class="font-semibold">Request Detail</h2>
        <button
          type="button"
          class="rounded-md p-1 text-muted-foreground hover:bg-accent"
          onclick={closeDetail}
          aria-label="Close"
        >
          ✕
        </button>
      </div>

      <div class="space-y-4 p-6">
        {#if detailLoading}
          <div class="flex items-center gap-3 py-8 justify-center text-muted-foreground">
            <div class="h-5 w-5 animate-spin rounded-full border-2 border-primary border-t-transparent"></div>
            Loading...
          </div>
        {:else if detailError}
          <div class="rounded-md border border-destructive/50 bg-destructive/10 px-4 py-3 text-sm text-destructive">
            {detailError}
          </div>
        {:else if detailRow && detailRow.kind === 'active' && detailRow.active}
          <!-- Active request metadata -->
          <div class="grid grid-cols-2 gap-x-6 gap-y-2 text-sm sm:grid-cols-3">
            <div>
              <span class="text-xs text-muted-foreground">Request ID</span>
              <p class="font-mono text-xs">{detailRow.active.id}</p>
            </div>
            <div>
              <span class="text-xs text-muted-foreground">Started</span>
              <p class="text-xs">{formatTs(detailRow.active.started_at)}</p>
            </div>
            <div>
              <span class="text-xs text-muted-foreground">Endpoint</span>
              <p class="font-mono text-xs truncate" title={detailRow.active.endpoint}>{detailRow.active.endpoint}</p>
            </div>
            <div>
              <span class="text-xs text-muted-foreground">Model</span>
              <p class="font-mono text-xs truncate" title={detailRow.active.model}>{detailRow.active.model}</p>
            </div>
            {#if detailRow.active.remote}
              <div>
                <span class="text-xs text-muted-foreground">Remote</span>
                <p class="text-xs">{detailRow.active.remote}</p>
              </div>
            {/if}
          </div>
          <div class="rounded-md border border-primary/30 bg-primary/10 px-4 py-3 text-xs text-muted-foreground">
            This request is still in progress. Request and response bodies are captured once it finishes and
            appears in the request log.
          </div>
        {:else if detailRow && detailRow.log}
          <!-- Metadata grid -->
          <div class="grid grid-cols-2 gap-x-6 gap-y-2 text-sm sm:grid-cols-3">
            <div>
              <span class="text-xs text-muted-foreground">Request ID</span>
              <p class="font-mono text-xs">{detailRow.log.id}</p>
            </div>
            <div>
              <span class="text-xs text-muted-foreground">Timestamp</span>
              <p class="text-xs">{formatTs(detailRow.log.timestamp)}</p>
            </div>
            <div>
              <span class="text-xs text-muted-foreground">Status</span>
              <p class={statusClass(detailRow.log.status_code) + ' text-xs'}>{detailRow.log.status_code}</p>
            </div>
            <div>
              <span class="text-xs text-muted-foreground">Model (Requested)</span>
              <p class="font-mono text-xs truncate" title={detailRow.log.requested_model}>{detailRow.log.requested_model ?? '—'}</p>
            </div>
            <div>
              <span class="text-xs text-muted-foreground">Model (Resolved)</span>
              <p class="font-mono text-xs truncate" title={detailRow.log.resolved_model}>{detailRow.log.resolved_model ?? '—'}</p>
            </div>
            <div>
              <span class="text-xs text-muted-foreground">Provider</span>
              <p class="text-xs">{detailRow.log.provider_name ?? detailRow.log.provider_id ?? '—'}</p>
            </div>
            <div>
              <span class="text-xs text-muted-foreground">Latency</span>
              <p class="text-xs">{detailRow.log.total_time_ms}ms{detailRow.log.ttft_ms != null ? ` (TTFT ${detailRow.log.ttft_ms}ms)` : ''}</p>
            </div>
            <div>
              <span class="text-xs text-muted-foreground">Tokens</span>
              <p class="text-xs tabular-nums">
                {detailRow.log.prompt_tokens != null ? detailRow.log.prompt_tokens.toLocaleString() : '—'} in
                / {detailRow.log.completion_tokens != null ? detailRow.log.completion_tokens.toLocaleString() : '—'} out
                {detailRow.log.cached_tokens && detailRow.log.cached_tokens > 0 ? `(${detailRow.log.cached_tokens.toLocaleString()} cached)` : ''}
              </p>
            </div>
            <div>
              <span class="text-xs text-muted-foreground">Est. Cost</span>
              <p class="text-xs tabular-nums">{formatCost(detailTotalCostUSD(detailRow.log))}</p>
            </div>

            <!-- Token breakdown table -->
            <div class="col-span-2 sm:col-span-3">
              <details class="group">
                <summary class="cursor-pointer list-none text-sm font-medium hover:text-primary">
                  <span class="inline-block mr-1 transition-transform group-open:rotate-90">›</span>
                  Token & Cost Breakdown
                </summary>
                <div class="mt-3 border-t pt-3">
                  <table class="w-full text-sm">
                    <tbody>
                      <tr class="border-b">
                        <td class="py-2 pr-4 font-medium text-muted-foreground">Total tokens</td>
                        <td class="py-2 pl-4 text-right tabular-nums">
                          {detailRow.log.total_tokens != null ? detailRow.log.total_tokens.toLocaleString() : '—'}
                        </td>
                        <td class="py-2 pr-4 font-medium text-muted-foreground text-right">Total cost</td>
                        <td class="py-2 pl-4 text-right tabular-nums">
                          {formatCost(detailTotalCostUSD(detailRow.log))}
                        </td>
                      </tr>
                      <tr class="border-b">
                        <td class="py-2 pr-4 font-medium text-muted-foreground">Input tokens</td>
                        <td class="py-2 pl-4 text-right tabular-nums">
                          {detailRow.log.prompt_tokens != null ? detailRow.log.prompt_tokens.toLocaleString() : '—'}
                        </td>
                        <td class="py-2 pr-4 font-medium text-muted-foreground text-right">Input cost</td>
                        <td class="py-2 pl-4 text-right tabular-nums">
                          {formatCost(detailRow.log.cost_breakdown?.input_usd)}
                        </td>
                      </tr>
                      <tr class="border-b">
                        <td class="py-2 pr-4 font-medium text-muted-foreground">Output tokens</td>
                        <td class="py-2 pl-4 text-right tabular-nums">
                          {detailRow.log.completion_tokens != null ? detailRow.log.completion_tokens.toLocaleString() : '—'}
                        </td>
                        <td class="py-2 pr-4 font-medium text-muted-foreground text-right">Output cost</td>
                        <td class="py-2 pl-4 text-right tabular-nums">
                          {formatCost(detailRow.log.cost_breakdown?.output_usd)}
                        </td>
                      </tr>
                      <tr>
                        <td class="py-2 pr-4 font-medium text-muted-foreground">Cached tokens</td>
                        <td class="py-2 pl-4 text-right tabular-nums">
                          {detailRow.log.cached_tokens != null ? detailRow.log.cached_tokens.toLocaleString() : '—'}
                        </td>
                        <td class="py-2 pr-4 font-medium text-muted-foreground text-right">Cached cost</td>
                        <td class="py-2 pl-4 text-right tabular-nums">
                          {formatCost(detailRow.log.cost_breakdown?.cached_read_usd)}
                        </td>
                      </tr>
                    </tbody>
                  </table>
                </div>
              </details>
            </div>
          </div>

          {#if detailRow.log.error_message}
            <div class="rounded-md border border-destructive/50 bg-destructive/10 px-4 py-3 text-xs text-destructive">
              <span class="font-medium">Error:</span> {detailRow.log.error_message}
            </div>
          {/if}

          <!-- Request body -->
          <div class="space-y-1">
            <h3 class="text-sm font-medium">Request Body</h3>
            {#if detailRow.log.request_body}
              <pre class="max-h-64 overflow-auto rounded-md bg-muted px-4 py-3 text-xs leading-relaxed">{prettyJSON(detailRow.log.request_body)}</pre>
            {:else}
              <p class="text-xs text-muted-foreground">Not captured.</p>
            {/if}
          </div>

          <!-- Response body -->
          <div class="space-y-1">
            <h3 class="text-sm font-medium">
              Response Body
              {#if detailRow.log.is_streamed && detailRow.log.response_body}
                <span class="ml-1 text-xs font-normal text-muted-foreground">(reconstructed from stream)</span>
              {:else if detailRow.log.is_streamed}
                <span class="ml-1 text-xs font-normal text-muted-foreground">(streaming)</span>
              {/if}
            </h3>
            {#if detailRow.log.response_body}
              <pre class="max-h-64 overflow-auto rounded-md bg-muted px-4 py-3 text-xs leading-relaxed">{prettyJSON(detailRow.log.response_body)}</pre>
            {:else}
              <p class="text-xs text-muted-foreground">
                {detailRow.log.is_streamed
                  ? 'No text was reconstructed from this stream (empty deltas, non-OpenAI shape, or client disconnect). Adjust response body limits in Settings if you expect large streamed text.'
                  : 'Not captured.'}
              </p>
            {/if}
          </div>

          <!-- Streaming chunks section (visible only for streamed requests) -->
          {#if detailRow.log.is_streamed}
            <div class="space-y-1">
              <details class="group">
                <summary class="cursor-pointer list-none text-sm font-medium hover:text-primary">
                  <span class="inline-block mr-1 transition-transform group-open:rotate-90">›</span>
                  Streaming chunks
                </summary>
                <div class="mt-3 space-y-3 border-t pt-3">
                  {#if streamingLogsLoading}
                    <div class="flex items-center gap-2 text-xs text-muted-foreground">
                      <span
                        class="inline-block h-4 w-4 animate-spin rounded-full border-2 border-primary border-t-transparent"
                      ></span>
                      Loading chunks…
                    </div>
                  {:else if streamingLogsError}
                    <p class="text-xs text-destructive">{streamingLogsError}</p>
                  {:else if streamingLogs.length === 0}
                    <p class="text-xs text-muted-foreground">
                      No SSE lines stored for this request. Turn on &ldquo;Track streaming&rdquo; in Settings to record a
                      rolling buffer of raw stream lines.
                    </p>
                  {:else}
                    <p class="text-xs text-muted-foreground">
                      {streamingLogs.length} SSE line{streamingLogs.length === 1 ? '' : 's'}. Each row shows the text
                      delta (if any), the running assistant text after that chunk, and the raw line on demand.
                    </p>
                    {#if streamingHasPurgedBodies}
                      <p
                        class="rounded-md border border-amber-500/40 bg-amber-500/10 px-3 py-2 text-xs text-amber-900 dark:text-amber-100"
                      >
                        One or more chunks no longer have stored bodies; they were removed by the streaming body
                        retention setting. Metadata (index, timestamps) is still shown.
                      </p>
                    {/if}
                    <div class="max-h-[28rem] space-y-2 overflow-y-auto rounded-md border bg-muted/30 p-2">
                      {#each streamingLogs as chunk (chunk.id)}
                        <div class="rounded border bg-background px-3 py-2 text-xs leading-snug">
                          <div class="mb-2 flex flex-wrap items-center gap-2 text-[10px] font-medium uppercase tracking-wide text-muted-foreground">
                            <span>Chunk #{chunk.chunk_index}</span>
                            {#if chunk.body_purged}
                              <span class="text-amber-600">bodies cleared</span>
                            {/if}
                            {#if chunk.is_truncated}
                              <span class="text-amber-600">prefix dropped</span>
                            {/if}
                          </div>
                          {#if chunk.body_purged}
                            <p class="mb-2 text-[11px] text-muted-foreground">
                              Raw SSE line and text delta were removed by streaming body retention (Settings). Chunk
                              metadata is kept for auditing.
                            </p>
                          {:else}
                            {#if chunk.content_delta}
                              <div class="mb-2">
                                <div class="mb-0.5 text-[10px] font-medium text-muted-foreground">Delta</div>
                                <pre
                                  class="max-h-24 overflow-auto whitespace-pre-wrap break-words rounded bg-muted/60 px-2 py-1 font-mono text-[11px]"
                                >{chunk.content_delta}</pre>
                              </div>
                            {/if}
                            {#if chunk.cumulative_body}
                              <div class="mb-2">
                                <div class="mb-0.5 text-[10px] font-medium text-muted-foreground">
                                  Total text after this chunk
                                </div>
                                <pre
                                  class="max-h-32 overflow-auto whitespace-pre-wrap break-words rounded border border-primary/20 bg-primary/5 px-2 py-1 font-mono text-[11px]"
                                >{chunk.cumulative_body}</pre>
                              </div>
                            {/if}
                            {#if !chunk.content_delta && !chunk.cumulative_body}
                              <p class="mb-2 text-[11px] text-muted-foreground">No assistant text delta in this line.</p>
                            {/if}
                            <details class="group text-[11px]">
                              <summary
                                class="cursor-pointer list-none font-medium text-muted-foreground hover:text-foreground"
                              >
                                <span class="mr-1 inline-block transition-transform group-open:rotate-90">›</span>
                                Raw SSE line
                              </summary>
                              <pre
                                class="mt-1 max-h-40 overflow-auto whitespace-pre-wrap break-all rounded bg-muted/40 px-2 py-1 font-mono"
                              >{chunk.data}</pre>
                            </details>
                          {/if}
                        </div>
                      {/each}
                    </div>
                  {/if}
                </div>
              </details>
            </div>
          {/if}
        {/if}
      </div>
    </div>
  </div>
{/if}
