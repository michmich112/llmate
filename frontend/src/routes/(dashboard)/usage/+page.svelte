<script lang="ts">
  import { api } from '$lib/api/client';
  import { Card, CardContent, CardHeader, CardTitle } from '$lib/components/ui/card';
  import type { APIKeyUsage, ModelStats } from '$lib/types';

  let usage = $state<APIKeyUsage | null>(null);
  let keyName = $state('');
  let byModel = $state<ModelStats[]>([]);
  let error = $state<string | null>(null);
  let loading = $state(true);
  let loaded = $state(false);

  async function loadUsage() {
    loading = true;
    error = null;
    try {
      const key = api.getAccessKey();
      if (!key) {
        error = 'No access key stored';
        return;
      }
      const res = await api.getMyUsage(key);
      usage = res.usage;
      keyName = res.api_key;
      byModel = res.by_model;
      loaded = true;
    } catch (e) {
      error = e instanceof Error ? e.message : 'Failed to load usage';
    } finally {
      loading = false;
    }
  }

  $effect(() => {
    void loadUsage();
  });
</script>

<div class="usage-dashboard space-y-6" data-testid="usage-dashboard">
  <div>
    <h1 class="text-2xl font-bold tracking-tight">My Usage</h1>
    <p class="mt-1 text-muted-foreground">
      {keyName ? `Usage for ${keyName}` : 'Personal usage across models.'}
    </p>
  </div>

  {#if error}
    <div class="rounded-md border border-destructive/50 bg-destructive/10 px-4 py-3 text-sm text-destructive">
      {error}
    </div>
  {/if}

  {#if loading}
    <div class="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
      {#each [1, 2, 3, 4] as n (n)}
        <div class="h-28 animate-pulse rounded-lg bg-muted"></div>
      {/each}
    </div>
  {:else if loaded}
    {#if usage && usage.request_count > 0}
      <div class="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
        <Card>
          <CardHeader>
            <CardTitle class="text-sm font-medium text-muted-foreground">Requests</CardTitle>
          </CardHeader>
          <CardContent>
            <p class="text-3xl font-bold">{usage.request_count.toLocaleString()}</p>
          </CardContent>
        </Card>
        <Card>
          <CardHeader>
            <CardTitle class="text-sm font-medium text-muted-foreground">Total Tokens</CardTitle>
          </CardHeader>
          <CardContent>
            <p class="text-3xl font-bold">{usage.total_tokens.toLocaleString()}</p>
          </CardContent>
        </Card>
        <Card>
          <CardHeader>
            <CardTitle class="text-sm font-medium text-muted-foreground">Estimated Cost (USD)</CardTitle>
          </CardHeader>
          <CardContent>
            <p class="text-3xl font-bold tabular-nums">{usage.total_cost_usd.toFixed(4)}</p>
          </CardContent>
        </Card>
        <Card>
          <CardHeader>
            <CardTitle class="text-sm font-medium text-muted-foreground">Last Used</CardTitle>
          </CardHeader>
          <CardContent>
            <p class="text-3xl font-bold">
              {usage.last_used_at ? new Date(usage.last_used_at).toLocaleString() : '—'}
            </p>
          </CardContent>
        </Card>
      </div>
    {:else}
      <div class="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
        <Card>
          <CardHeader>
            <CardTitle class="text-sm font-medium text-muted-foreground">Requests</CardTitle>
          </CardHeader>
          <CardContent>
            <p class="text-3xl font-bold">0</p>
          </CardContent>
        </Card>
        <Card>
          <CardHeader>
            <CardTitle class="text-sm font-medium text-muted-foreground">Total Tokens</CardTitle>
          </CardHeader>
          <CardContent>
            <p class="text-3xl font-bold">0</p>
          </CardContent>
        </Card>
        <Card>
          <CardHeader>
            <CardTitle class="text-sm font-medium text-muted-foreground">Estimated Cost (USD)</CardTitle>
          </CardHeader>
          <CardContent>
            <p class="text-3xl font-bold tabular-nums">0.0000</p>
          </CardContent>
        </Card>
        <Card>
          <CardHeader>
            <CardTitle class="text-sm font-medium text-muted-foreground">Last Used</CardTitle>
          </CardHeader>
          <CardContent>
            <p class="text-3xl font-bold">—</p>
          </CardContent>
        </Card>
      </div>
    {/if}

    <Card>
      <CardHeader>
        <CardTitle>Requests by Model</CardTitle>
      </CardHeader>
      <CardContent class="p-0">
        {#if byModel.length === 0}
          <p class="px-6 py-4 text-sm text-muted-foreground">No data yet.</p>
        {:else}
          <div class="overflow-x-auto">
            <table class="w-full text-sm">
              <thead>
                <tr class="border-b bg-muted/50 text-left text-xs font-medium uppercase tracking-wide text-muted-foreground">
                  <th class="px-4 py-3">Resolved Model</th>
                  <th class="px-4 py-3 text-right">Requests</th>
                  <th class="px-4 py-3 text-right">Avg Latency</th>
                  <th class="px-4 py-3 text-right">Errors</th>
                  <th class="px-4 py-3 text-right">Tokens</th>
                </tr>
              </thead>
              <tbody>
                {#each byModel as row (row.model)}
                  <tr class="border-b last:border-0 hover:bg-muted/30">
                    <td class="max-w-[160px] px-4 py-3 font-medium" title={row.model}>
                      <div class="truncate">{row.model}</div>
                    </td>
                    <td class="px-4 py-3 text-right">{row.request_count.toLocaleString()}</td>
                    <td class="px-4 py-3 text-right">{row.avg_latency_ms.toFixed(0)}ms</td>
                    <td class="px-4 py-3 text-right">{row.error_count}</td>
                    <td class="px-4 py-3 text-right">{row.total_tokens.toLocaleString()}</td>
                  </tr>
                {/each}
              </tbody>
            </table>
          </div>
        {/if}
      </CardContent>
    </Card>
  {/if}
</div>
