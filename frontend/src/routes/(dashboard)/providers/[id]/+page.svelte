<script lang="ts">
  import { goto } from '$app/navigation';
  import { page } from '$app/state';
  import { api } from '$lib/api/client';
  import type { Provider, ProviderEndpoint, ProviderModel, DiscoveryResult, ConfirmEndpointInput } from '$lib/types';
  import { Button } from '$lib/components/ui/button';
  import { Input } from '$lib/components/ui/input';
  import { Card, CardHeader, CardTitle, CardContent } from '$lib/components/ui/card';
  import StatusBadge from '$lib/components/StatusBadge.svelte';

  let providerId = $derived(page.params.id ?? '');

  let provider = $state<Provider | null>(null);
  let endpoints = $state<ProviderEndpoint[]>([]);
  let models = $state<ProviderModel[]>([]);
  let loading = $state(true);
  let error = $state<string | null>(null);

  // Edit form state
  let editName = $state('');
  let editBaseUrl = $state('');
  let editApiKey = $state('');
  let saveLoading = $state(false);
  let saveError = $state<string | null>(null);
  let saveSuccess = $state(false);

  // Delete dialog
  let showDeleteDialog = $state(false);
  let deleteLoading = $state(false);
  let deleteError = $state<string | null>(null);

  // Re-discover dialog
  let showDiscoverDialog = $state(false);
  let discoverLoading = $state(false);
  let discoverError = $state<string | null>(null);
  let discoverResult = $state<DiscoveryResult | null>(null);
  let discoverSelectedModels = $state<string[]>([]);
  let discoverEndpointEnabled = $state<Record<string, boolean>>({});
  let applyLoading = $state(false);
  let applyError = $state<string | null>(null);

  // Endpoint toggle loading: track by endpoint id
  let endpointTogglingIds = $state<Set<string>>(new Set());

  // Model cost editing: draft values keyed by model record id
  type CostDraft = {
    cost_per_million_input: string;
    cost_per_million_output: string;
    cost_per_million_cache_read: string;
    cost_per_million_cache_write: string;
  };
  let costDrafts = $state<Record<string, CostDraft>>({});
  let costSavingIds = $state<Set<string>>(new Set());
  let costErrors = $state<Record<string, string>>({});

  function initCostDraft(model: ProviderModel) {
    costDrafts[model.id] = {
      cost_per_million_input: model.cost_per_million_input?.toString() ?? '',
      cost_per_million_output: model.cost_per_million_output?.toString() ?? '',
      cost_per_million_cache_read: model.cost_per_million_cache_read?.toString() ?? '',
      cost_per_million_cache_write: model.cost_per_million_cache_write?.toString() ?? ''
    };
  }

  function parseOptionalFloat(s: string): number | undefined {
    const trimmed = s.trim();
    if (trimmed === '') return undefined;
    const n = parseFloat(trimmed);
    return isNaN(n) ? undefined : n;
  }

  async function handleSaveCosts(model: ProviderModel) {
    if (!provider) return;
    const draft = costDrafts[model.id];
    if (!draft) return;

    const newSet = new Set(costSavingIds);
    newSet.add(model.id);
    costSavingIds = newSet;
    const errs = { ...costErrors };
    delete errs[model.id];
    costErrors = errs;

    try {
      const result = await api.updateProviderModel(provider.id, model.id, {
        cost_per_million_input: parseOptionalFloat(draft.cost_per_million_input),
        cost_per_million_output: parseOptionalFloat(draft.cost_per_million_output),
        cost_per_million_cache_read: parseOptionalFloat(draft.cost_per_million_cache_read),
        cost_per_million_cache_write: parseOptionalFloat(draft.cost_per_million_cache_write)
      });
      models = result.models;
      // Re-init drafts with returned values to normalize display
      const updated = result.models.find((m) => m.id === model.id);
      if (updated) initCostDraft(updated);
    } catch (e) {
      costErrors = {
        ...costErrors,
        [model.id]: e instanceof Error ? e.message : 'Failed to save costs'
      };
    } finally {
      const s = new Set(costSavingIds);
      s.delete(model.id);
      costSavingIds = s;
    }
  }

  $effect(() => {
    if (providerId) {
      loadProvider();
    }
  });

  async function loadProvider() {
    loading = true;
    error = null;
    try {
      const data = await api.getProvider(providerId);
      provider = data.provider;
      endpoints = data.endpoints;
      models = data.models;
      editName = data.provider.name;
      editBaseUrl = data.provider.base_url;
      editApiKey = '';
      data.models.forEach(initCostDraft);
    } catch (e) {
      error = e instanceof Error ? e.message : 'Failed to load provider';
    } finally {
      loading = false;
    }
  }

  async function handleSave() {
    if (!provider) return;
    saveLoading = true;
    saveError = null;
    saveSuccess = false;
    try {
      const updateData: Partial<Provider> = {
        name: editName.trim(),
        base_url: editBaseUrl.trim()
      };
      if (editApiKey.trim()) {
        updateData.api_key = editApiKey.trim();
      }
      provider = await api.updateProvider(provider.id, updateData);
      saveSuccess = true;
    } catch (e) {
      saveError = e instanceof Error ? e.message : 'Failed to save provider';
    } finally {
      saveLoading = false;
    }
  }

  async function handleDelete() {
    if (!provider) return;
    deleteLoading = true;
    deleteError = null;
    try {
      await api.deleteProvider(provider.id);
      goto('/providers');
    } catch (e) {
      deleteError = e instanceof Error ? e.message : 'Failed to delete provider';
      deleteLoading = false;
    }
  }

  async function handleToggleEndpoint(endpoint: ProviderEndpoint, enabled: boolean) {
    if (!provider) return;
    const newSet = new Set(endpointTogglingIds);
    newSet.add(endpoint.id);
    endpointTogglingIds = newSet;
    try {
      const updated = await api.updateEndpoint(provider.id, endpoint.id, { is_enabled: enabled });
      endpoints = endpoints.map((ep) => (ep.id === updated.id ? updated : ep));
    } catch (e) {
      // revert on failure — error surfaced implicitly; could add toast
    } finally {
      const s = new Set(endpointTogglingIds);
      s.delete(endpoint.id);
      endpointTogglingIds = s;
    }
  }

  async function handleRediscover() {
    if (!provider) return;
    showDiscoverDialog = true;
    discoverLoading = true;
    discoverError = null;
    discoverResult = null;
    applyError = null;
    try {
      const result = await api.discoverProvider(provider.id);
      discoverResult = result;
      discoverSelectedModels = [...result.models];
      discoverEndpointEnabled = Object.fromEntries(
        result.endpoints.map((ep) => [ep.path + ':' + ep.method, ep.is_supported === true])
      );
    } catch (e) {
      discoverError = e instanceof Error ? e.message : 'Discovery failed';
    } finally {
      discoverLoading = false;
    }
  }

  async function handleApplyDiscovery() {
    if (!provider || !discoverResult) return;
    applyLoading = true;
    applyError = null;
    try {
      const endpointInputs: ConfirmEndpointInput[] = discoverResult.endpoints.map((ep) => ({
        path: ep.path,
        method: ep.method,
        is_supported: ep.is_supported === true,
        is_enabled: discoverEndpointEnabled[ep.path + ':' + ep.method] ?? false
      }));
      await api.confirmProvider(provider.id, {
        models: discoverSelectedModels,
        endpoints: endpointInputs
      });
      showDiscoverDialog = false;
      await loadProvider();
    } catch (e) {
      applyError = e instanceof Error ? e.message : 'Failed to apply discovery results';
    } finally {
      applyLoading = false;
    }
  }

  function statusFromHealthy(isHealthy: boolean): 'healthy' | 'unhealthy' {
    return isHealthy ? 'healthy' : 'unhealthy';
  }

  function formatDate(dateStr?: string): string {
    if (!dateStr) return '—';
    return new Date(dateStr).toLocaleString();
  }
</script>

<div class="space-y-6">
  <div class="flex items-center justify-between">
    <div class="flex items-center gap-4">
      <a href="/providers" class="text-sm text-muted-foreground hover:text-foreground">← Providers</a>
      <h1 class="text-2xl font-bold tracking-tight">{provider?.name ?? 'Provider'}</h1>
    </div>
    {#if provider}
      <div class="flex gap-2">
        <Button variant="outline" type="button" onclick={handleRediscover}>Re-discover</Button>
        <Button variant="destructive" type="button" onclick={() => (showDeleteDialog = true)}>Delete</Button>
      </div>
    {/if}
  </div>

  {#if error}
    <div class="rounded-md border border-destructive/50 bg-destructive/10 px-4 py-3 text-sm text-destructive">
      {error}
    </div>
  {/if}

  {#if loading}
    <div class="space-y-4">
      {#each [1, 2, 3] as _}
        <div class="h-32 animate-pulse rounded-lg bg-muted"></div>
      {/each}
    </div>
  {:else if provider}
    <!-- Info card -->
    <Card>
      <CardHeader>
        <CardTitle>Overview</CardTitle>
      </CardHeader>
      <CardContent class="grid grid-cols-2 gap-4 text-sm sm:grid-cols-4">
        <div>
          <p class="text-muted-foreground">Status</p>
          <StatusBadge status={statusFromHealthy(provider.is_healthy)} />
        </div>
        <div>
          <p class="text-muted-foreground">Base URL</p>
          <p class="truncate font-mono" title={provider.base_url}>{provider.base_url}</p>
        </div>
        <div>
          <p class="text-muted-foreground">Last Check</p>
          <p>{formatDate(provider.health_checked_at)}</p>
        </div>
        <div>
          <p class="text-muted-foreground">Created</p>
          <p>{formatDate(provider.created_at)}</p>
        </div>
      </CardContent>
    </Card>

    <!-- Edit form -->
    <Card>
      <CardHeader>
        <CardTitle>Edit Provider</CardTitle>
      </CardHeader>
      <CardContent class="space-y-4">
        {#if saveError}
          <div class="rounded-md border border-destructive/50 bg-destructive/10 px-4 py-3 text-sm text-destructive">
            {saveError}
          </div>
        {/if}
        {#if saveSuccess}
          <div class="rounded-md border border-green-500/50 bg-green-50 px-4 py-3 text-sm text-green-700 dark:bg-green-950 dark:text-green-300">
            Provider updated successfully.
          </div>
        {/if}

        <div class="space-y-2">
          <label for="edit-name" class="text-sm font-medium">Name</label>
          <Input id="edit-name" bind:value={editName} />
        </div>

        <div class="space-y-2">
          <label for="edit-url" class="text-sm font-medium">Base URL</label>
          <Input id="edit-url" bind:value={editBaseUrl} />
        </div>

        <div class="space-y-2">
          <label for="edit-key" class="text-sm font-medium">
            API Key
            <span class="ml-1 text-xs font-normal text-muted-foreground">(leave blank to keep existing)</span>
          </label>
          <Input id="edit-key" type="password" bind:value={editApiKey} placeholder="Enter new key to update" />
        </div>

        <div class="flex justify-end">
          <Button onclick={handleSave} disabled={saveLoading}>
            {saveLoading ? 'Saving...' : 'Save Changes'}
          </Button>
        </div>
      </CardContent>
    </Card>

    <!-- Endpoints table -->
    <Card>
      <CardHeader>
        <CardTitle>Endpoints ({endpoints.length})</CardTitle>
      </CardHeader>
      <CardContent class="p-0">
        {#if endpoints.length === 0}
          <p class="px-6 py-4 text-sm text-muted-foreground">No endpoints configured.</p>
        {:else}
          <table class="w-full text-sm">
            <thead>
              <tr class="border-b bg-muted/50 text-left text-xs font-medium uppercase tracking-wide text-muted-foreground">
                <th class="px-4 py-3">Path</th>
                <th class="px-4 py-3">Method</th>
                <th class="px-4 py-3">Supported</th>
                <th class="px-4 py-3">Enabled</th>
              </tr>
            </thead>
            <tbody>
              {#each endpoints as endpoint}
                <tr class="border-b last:border-0">
                  <td class="px-4 py-3 font-mono text-xs">{endpoint.path}</td>
                  <td class="px-4 py-3 font-mono text-xs uppercase">{endpoint.method}</td>
                  <td class="px-4 py-3">
                    {#if endpoint.is_supported}
                      <span class="inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-300">Yes</span>
                    {:else}
                      <span class="inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium bg-gray-100 text-gray-600 dark:bg-gray-800 dark:text-gray-400">No</span>
                    {/if}
                  </td>
                  <td class="px-4 py-3">
                    <input
                      type="checkbox"
                      checked={endpoint.is_enabled}
                      disabled={endpointTogglingIds.has(endpoint.id)}
                      onchange={(e) =>
                        handleToggleEndpoint(endpoint, (e.target as HTMLInputElement).checked)}
                      class="h-4 w-4 rounded border-gray-300 disabled:opacity-50"
                    />
                  </td>
                </tr>
              {/each}
            </tbody>
          </table>
        {/if}
      </CardContent>
    </Card>

    <!-- Model Pricing -->
    <Card>
      <CardHeader>
        <CardTitle>Model Pricing ({models.length})</CardTitle>
      </CardHeader>
      <CardContent class="p-0">
        {#if models.length === 0}
          <p class="px-6 py-4 text-sm text-muted-foreground">No models configured. Use Re-discover to add models.</p>
        {:else}
          <div class="overflow-x-auto">
            <table class="w-full text-sm">
              <thead>
                <tr class="border-b bg-muted/50 text-left text-xs font-medium uppercase tracking-wide text-muted-foreground">
                  <th class="px-4 py-3">Model ID</th>
                  <th class="px-4 py-3 text-right">Input ($/M)</th>
                  <th class="px-4 py-3 text-right">Output ($/M)</th>
                  <th class="px-4 py-3 text-right">Cache Read ($/M)</th>
                  <th class="px-4 py-3 text-right">Cache Write ($/M)</th>
                  <th class="px-4 py-3"></th>
                </tr>
              </thead>
              <tbody>
                {#each models as model (model.id)}
                  {@const draft = costDrafts[model.id]}
                  <tr class="border-b last:border-0">
                    <td class="px-4 py-2 font-mono text-xs">{model.model_id}</td>
                    {#if draft}
                      <td class="px-2 py-2">
                        <Input
                          class="h-8 w-24 text-right text-xs"
                          placeholder="e.g. 1.50"
                          bind:value={draft.cost_per_million_input}
                        />
                      </td>
                      <td class="px-2 py-2">
                        <Input
                          class="h-8 w-24 text-right text-xs"
                          placeholder="e.g. 2.00"
                          bind:value={draft.cost_per_million_output}
                        />
                      </td>
                      <td class="px-2 py-2">
                        <Input
                          class="h-8 w-24 text-right text-xs"
                          placeholder="e.g. 0.15"
                          bind:value={draft.cost_per_million_cache_read}
                        />
                      </td>
                      <td class="px-2 py-2">
                        <Input
                          class="h-8 w-24 text-right text-xs"
                          placeholder="e.g. 3.75"
                          bind:value={draft.cost_per_million_cache_write}
                        />
                      </td>
                      <td class="px-4 py-2">
                        {#if costErrors[model.id]}
                          <span class="block text-xs text-destructive mb-1">{costErrors[model.id]}</span>
                        {/if}
                        <Button
                          size="sm"
                          variant="outline"
                          disabled={costSavingIds.has(model.id)}
                          onclick={() => handleSaveCosts(model)}
                        >
                          {costSavingIds.has(model.id) ? 'Saving…' : 'Save'}
                        </Button>
                      </td>
                    {:else}
                      <td colspan="5" class="px-4 py-2 text-xs text-muted-foreground">—</td>
                    {/if}
                  </tr>
                {/each}
              </tbody>
            </table>
          </div>
          <p class="px-4 py-2 text-xs text-muted-foreground border-t">
            Leave fields blank to omit a cost component. Costs are used to compute estimated spend per request.
          </p>
        {/if}
      </CardContent>
    </Card>
  {/if}
</div>

<!-- Delete Confirmation Dialog -->
{#if showDeleteDialog}
  <div
    role="presentation"
    class="fixed inset-0 z-50 flex items-center justify-center bg-black/50"
    onclick={() => (showDeleteDialog = false)}
    onkeydown={(e) => { if (e.key === 'Escape') showDeleteDialog = false; }}
  >
    <div
      role="dialog"
      aria-modal="true"
      tabindex="-1"
      class="mx-4 w-full max-w-md rounded-lg bg-popover p-6 shadow-xl"
      onclick={(e) => e.stopPropagation()}
      onkeydown={(e) => e.stopPropagation()}
    >
      <h2 class="mb-2 text-lg font-bold">Delete Provider</h2>
      <p class="mb-4 text-sm text-muted-foreground">
        Are you sure you want to delete <strong>{provider?.name}</strong>? This action cannot be undone.
      </p>
      {#if deleteError}
        <div class="mb-4 rounded-md border border-destructive/50 bg-destructive/10 px-4 py-3 text-sm text-destructive">
          {deleteError}
        </div>
      {/if}
      <div class="flex justify-end gap-2">
        <Button variant="outline" type="button" onclick={() => (showDeleteDialog = false)}>Cancel</Button>
        <Button variant="destructive" type="button" onclick={handleDelete} disabled={deleteLoading}>
          {deleteLoading ? 'Deleting...' : 'Delete'}
        </Button>
      </div>
    </div>
  </div>
{/if}

<!-- Re-discover Dialog -->
{#if showDiscoverDialog}
  <div
    role="presentation"
    class="fixed inset-0 z-50 flex items-center justify-center bg-black/50"
    onclick={() => (showDiscoverDialog = false)}
    onkeydown={(e) => { if (e.key === 'Escape') showDiscoverDialog = false; }}
  >
    <div
      role="dialog"
      aria-modal="true"
      tabindex="-1"
      class="mx-4 w-full max-w-lg rounded-lg bg-popover p-6 shadow-xl"
      onclick={(e) => e.stopPropagation()}
      onkeydown={(e) => e.stopPropagation()}
    >
      <h2 class="mb-4 text-lg font-bold">Discovery Results</h2>

      {#if discoverLoading}
        <div class="flex items-center gap-3 py-6 text-muted-foreground">
          <div class="h-5 w-5 animate-spin rounded-full border-2 border-primary border-t-transparent"></div>
          Discovering...
        </div>
      {:else if discoverError}
        <div class="rounded-md border border-destructive/50 bg-destructive/10 px-4 py-3 text-sm text-destructive">
          {discoverError}
        </div>
      {:else if discoverResult}
        <div class="space-y-4 text-sm">
          <div>
            <h3 class="mb-2 font-medium">Models ({discoverResult.models.length})</h3>
            {#if discoverResult.models.length === 0}
              <p class="text-muted-foreground">No models found.</p>
            {:else}
              <ul class="space-y-1">
                {#each discoverResult.models as m}
                  <li class="flex items-center gap-2">
                    <input
                      type="checkbox"
                      id="model-{m}"
                      checked={discoverSelectedModels.includes(m)}
                      onchange={(e) => {
                        if ((e.target as HTMLInputElement).checked) {
                          discoverSelectedModels = [...discoverSelectedModels, m];
                        } else {
                          discoverSelectedModels = discoverSelectedModels.filter((x) => x !== m);
                        }
                      }}
                      class="h-4 w-4 rounded border-gray-300"
                    />
                    <label for="model-{m}" class="font-mono text-xs cursor-pointer">{m}</label>
                  </li>
                {/each}
              </ul>
            {/if}
          </div>
          <div>
            <h3 class="mb-2 font-medium">Endpoints ({discoverResult.endpoints.length})</h3>
            {#if discoverResult.endpoints.length === 0}
              <p class="text-muted-foreground">No endpoints found.</p>
            {:else}
              <table class="w-full text-xs">
                <thead>
                  <tr class="border-b text-left text-muted-foreground">
                    <th class="pb-1 pr-4">Path</th>
                    <th class="pb-1 pr-4">Method</th>
                    <th class="pb-1 pr-4">Status</th>
                    <th class="pb-1">Enable</th>
                  </tr>
                </thead>
                <tbody>
                  {#each discoverResult.endpoints as ep}
                    <tr class="border-b last:border-0">
                      <td class="py-1 pr-4 font-mono">{ep.path}</td>
                      <td class="py-1 pr-4 uppercase">{ep.method}</td>
                      <td class="py-1 pr-4">
                        {#if ep.is_supported === true}
                          <span class="inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-300">Supported</span>
                        {:else if ep.is_supported === false}
                          <span class="inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium bg-red-100 text-red-700 dark:bg-red-900 dark:text-red-300">Not Supported</span>
                        {:else}
                          <span class="inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium bg-yellow-100 text-yellow-700 dark:bg-yellow-900 dark:text-yellow-300">Unknown</span>
                        {/if}
                      </td>
                      <td class="py-1">
                        <input
                          type="checkbox"
                          checked={discoverEndpointEnabled[ep.path + ':' + ep.method] ?? false}
                          onchange={(e) => {
                            discoverEndpointEnabled = {
                              ...discoverEndpointEnabled,
                              [ep.path + ':' + ep.method]: (e.target as HTMLInputElement).checked
                            };
                          }}
                          class="h-4 w-4 rounded border-gray-300"
                        />
                      </td>
                    </tr>
                  {/each}
                </tbody>
              </table>
            {/if}
          </div>
        </div>
      {/if}

      {#if applyError}
        <div class="mt-4 rounded-md border border-destructive/50 bg-destructive/10 px-4 py-3 text-sm text-destructive">
          {applyError}
        </div>
      {/if}

      <div class="mt-4 flex justify-end gap-2">
        <Button variant="outline" type="button" onclick={() => (showDiscoverDialog = false)}>Close</Button>
        {#if discoverResult}
          <Button type="button" onclick={handleApplyDiscovery} disabled={applyLoading}>
            {#if applyLoading}
              <span class="flex items-center gap-2">
                <span class="h-4 w-4 animate-spin rounded-full border-2 border-white border-t-transparent"></span>
                Applying...
              </span>
            {:else}
              Apply
            {/if}
          </Button>
        {/if}
      </div>
    </div>
  </div>
{/if}
