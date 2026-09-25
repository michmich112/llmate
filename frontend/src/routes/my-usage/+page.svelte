<script lang="ts">
  import { api } from '$lib/api/client';
  import { Button } from '$lib/components/ui/button';
  import { Card, CardContent, CardHeader, CardTitle } from '$lib/components/ui/card';
  import { Input } from '$lib/components/ui/input';
  import type { APIKeyUsage } from '$lib/types';

  let apiKey = $state('');
  let usage = $state<APIKeyUsage | null>(null);
  let keyName = $state('');
  let error = $state<string | null>(null);
  let loading = $state(false);
  let loaded = $state(false);

  async function handleCheck() {
    const trimmed = apiKey.trim();
    if (!trimmed) {
      error = 'Enter your API key';
      return;
    }
    loading = true;
    error = null;
    usage = null;
    keyName = '';
    loaded = false;
    try {
      const res = await api.getMyUsage(trimmed);
      usage = res.usage;
      keyName = res.api_key;
      loaded = true;
    } catch (e) {
      error = e instanceof Error ? e.message : 'Failed to fetch usage';
    } finally {
      loading = false;
    }
  }
</script>

<div class="mx-auto max-w-3xl space-y-6 px-4 py-10">
  <div>
    <h1 class="text-2xl font-bold tracking-tight">My Usage</h1>
    <p class="text-muted-foreground mt-1">
      Enter your API key to see how many tokens and requests it has consumed.
    </p>
  </div>

  <Card>
    <CardHeader>
      <CardTitle>Check usage</CardTitle>
    </CardHeader>
    <CardContent>
      <div class="space-y-4">
        <div class="space-y-2">
          <label for="api-key" class="text-sm font-medium leading-none">API key</label>
          <Input id="api-key" bind:value={apiKey} placeholder="sk-..." />
        </div>
        {#if error}
          <div class="rounded-md border border-destructive/50 bg-destructive/10 px-4 py-3 text-sm text-destructive">
            {error}
          </div>
        {/if}
        <Button onclick={handleCheck} disabled={loading}>
          {loading ? 'Checking…' : 'Check usage'}
        </Button>
      </div>
    </CardContent>
  </Card>

  {#if loaded}
    <Card>
      <CardHeader>
        <CardTitle>Usage for {keyName || 'this key'}</CardTitle>
      </CardHeader>
      <CardContent>
        {#if usage && usage.request_count > 0}
          <div class="grid gap-4 sm:grid-cols-2">
            <div class="space-y-1">
              <p class="text-sm text-muted-foreground">Requests</p>
              <p class="text-lg font-semibold">{usage.request_count}</p>
            </div>
            <div class="space-y-1">
              <p class="text-sm text-muted-foreground">Total tokens</p>
              <p class="text-lg font-semibold">{usage.total_tokens}</p>
            </div>
            <div class="space-y-1">
              <p class="text-sm text-muted-foreground">Estimated cost (USD)</p>
              <p class="text-lg font-semibold">{usage.total_cost_usd.toFixed(4)}</p>
            </div>
            <div class="space-y-1">
              <p class="text-sm text-muted-foreground">Last used</p>
              <p class="text-lg font-semibold">
                {usage.last_used_at ? new Date(usage.last_used_at).toLocaleString() : '—'}
              </p>
            </div>
          </div>
        {:else}
          <p class="text-muted-foreground">No usage recorded for this key yet.</p>
        {/if}
      </CardContent>
    </Card>
  {/if}
</div>
