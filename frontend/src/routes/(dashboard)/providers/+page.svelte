<script lang="ts">
  import { goto } from '$app/navigation';
  import { api } from '$lib/api/client';
  import type { Provider } from '$lib/types';
  import { Button } from '$lib/components/ui/button';
  import { Card, CardHeader, CardTitle, CardContent } from '$lib/components/ui/card';
  import StatusBadge from '$lib/components/StatusBadge.svelte';

  let providers = $state<Provider[]>([]);
  let loading = $state(true);
  let error = $state<string | null>(null);

  $effect(() => {
    loadProviders();
  });

  async function loadProviders() {
    loading = true;
    error = null;
    try {
      providers = await api.listProviders();
    } catch (e) {
      error = e instanceof Error ? e.message : 'Failed to load providers';
    } finally {
      loading = false;
    }
  }

  function formatDate(dateStr?: string): string {
    if (!dateStr) return '—';
    return new Date(dateStr).toLocaleString();
  }

  function statusFromHealthy(isHealthy: boolean): 'healthy' | 'unhealthy' {
    return isHealthy ? 'healthy' : 'unhealthy';
  }
</script>

<div class="space-y-6">
  <div class="flex items-center justify-between">
    <h1 class="text-2xl font-bold tracking-tight">Providers</h1>
    <a href="/providers/new">
      <Button>Add Provider</Button>
    </a>
  </div>

  {#if error}
    <div class="rounded-md border border-destructive/50 bg-destructive/10 px-4 py-3 text-sm text-destructive">
      {error}
    </div>
  {/if}

  {#if loading}
    <div class="space-y-2">
      {#each [1, 2, 3] as _}
        <div class="h-12 animate-pulse rounded-md bg-muted"></div>
      {/each}
    </div>
  {:else if providers.length === 0}
    <Card>
      <CardContent class="py-12 text-center">
        <p class="mb-4 text-muted-foreground">No providers configured yet.</p>
        <a href="/providers/new">
          <Button>Add Your First Provider</Button>
        </a>
      </CardContent>
    </Card>
  {:else}
    <Card>
      <CardContent class="p-0">
        <table class="w-full text-sm">
          <thead>
            <tr class="border-b bg-muted/50 text-left text-xs font-medium uppercase tracking-wide text-muted-foreground">
              <th class="px-4 py-3">Name</th>
              <th class="px-4 py-3">Base URL</th>
              <th class="px-4 py-3">Status</th>
              <th class="px-4 py-3">Models</th>
              <th class="px-4 py-3">Last Check</th>
              <th class="px-4 py-3">Actions</th>
            </tr>
          </thead>
          <tbody>
            {#each providers as provider}
              <tr
                class="cursor-pointer border-b last:border-0 hover:bg-muted/30"
                onclick={() => goto('/providers/' + provider.id)}
              >
                <td class="px-4 py-3 font-medium">{provider.name}</td>
                <td class="max-w-[200px] px-4 py-3">
                  <span class="block truncate text-muted-foreground" title={provider.base_url}>
                    {provider.base_url}
                  </span>
                </td>
                <td class="px-4 py-3">
                  <StatusBadge status={statusFromHealthy(provider.is_healthy)} />
                </td>
                <td class="px-4 py-3">
                  {#if provider.models && provider.models.length > 0}
                    <div class="flex flex-wrap gap-1">
                      {#each provider.models.slice(0, 3) as model}
                        <span class="inline-block rounded bg-muted px-1.5 py-0.5 text-xs font-mono text-muted-foreground">
                          {model}
                        </span>
                      {/each}
                      {#if provider.models.length > 3}
                        <span class="inline-block rounded bg-muted px-1.5 py-0.5 text-xs text-muted-foreground">
                          +{provider.models.length - 3} more
                        </span>
                      {/if}
                    </div>
                  {:else}
                    <span class="text-muted-foreground">—</span>
                  {/if}
                </td>
                <td class="px-4 py-3 text-muted-foreground">{formatDate(provider.health_checked_at)}</td>
                <td class="px-4 py-3">
                  <a
                    href="/providers/{provider.id}"
                    class="text-primary hover:underline"
                    onclick={(e) => e.stopPropagation()}
                  >
                    View
                  </a>
                </td>
              </tr>
            {/each}
          </tbody>
        </table>
      </CardContent>
    </Card>
  {/if}
</div>
