<script lang="ts">
  import { goto } from '$app/navigation';
  import { api } from '$lib/api/client';
  import type { Provider, DiscoveryResult, ConfirmEndpointInput } from '$lib/types';
  import { Button } from '$lib/components/ui/button';
  import { Input } from '$lib/components/ui/input';
  import { Card, CardHeader, CardTitle, CardContent } from '$lib/components/ui/card';

  let step = $state(1);
  let provider = $state<Provider | null>(null);
  let discovery = $state<DiscoveryResult | null>(null);
  let discoveryLoading = $state(false);
  let discoveryError = $state<string | null>(null);
  let selectedModels = $state<string[]>([]);
  let endpointRows = $state<ConfirmEndpointInput[]>([]);

  // Step 1 form fields
  let name = $state('');
  let baseUrl = $state('');
  let apiKey = $state('');
  let step1Loading = $state(false);
  let step1Error = $state<string | null>(null);

  // Step 3
  let confirmLoading = $state(false);
  let confirmError = $state<string | null>(null);

  // Guard to prevent double-discovery
  let discoveryStarted = $state(false);

  $effect(() => {
    if (step === 2 && provider && !discoveryStarted) {
      discoveryStarted = true;
      runDiscovery();
    }
  });

  async function runDiscovery() {
    discoveryLoading = true;
    discoveryError = null;
    try {
      const result = await api.discoverProvider(provider!.id);
      discovery = result;
      selectedModels = [...result.models];
      endpointRows = result.endpoints.map((ep) => ({
        path: ep.path,
        method: ep.method,
        is_supported: ep.is_supported === true,
        is_enabled: ep.is_supported === true
      }));
    } catch (e) {
      discoveryError = e instanceof Error ? e.message : 'Discovery failed';
    } finally {
      discoveryLoading = false;
    }
  }

  async function handleStep1Submit() {
    if (!name.trim()) {
      step1Error = 'Name is required';
      return;
    }
    if (!baseUrl.trim()) {
      step1Error = 'Base URL is required';
      return;
    }
    step1Loading = true;
    step1Error = null;
    try {
      const data: { name: string; base_url: string; api_key?: string } = {
        name: name.trim(),
        base_url: baseUrl.trim()
      };
      if (apiKey.trim()) data.api_key = apiKey.trim();
      provider = await api.createProvider(data);
      step = 2;
    } catch (e) {
      step1Error = e instanceof Error ? e.message : 'Failed to create provider';
    } finally {
      step1Loading = false;
    }
  }

  async function handleConfirm() {
    confirmLoading = true;
    confirmError = null;
    try {
      await api.confirmProvider(provider!.id, {
        models: selectedModels,
        endpoints: endpointRows
      });
      goto('/providers/' + provider!.id);
    } catch (e) {
      confirmError = e instanceof Error ? e.message : 'Failed to confirm provider';
    } finally {
      confirmLoading = false;
    }
  }

  function toggleModel(modelId: string) {
    if (selectedModels.includes(modelId)) {
      selectedModels = selectedModels.filter((m) => m !== modelId);
    } else {
      selectedModels = [...selectedModels, modelId];
    }
  }

  function toggleEndpoint(index: number, enabled: boolean) {
    endpointRows = endpointRows.map((row, i) =>
      i === index ? { ...row, is_enabled: enabled } : row
    );
  }

  function supportedLabel(ep: { path: string; method: string }): {
    label: string;
    class: string;
  } {
    const row = endpointRows.find((r) => r.path === ep.path && r.method === ep.method);
    if (!row) return { label: 'Unknown', class: 'bg-yellow-100 text-yellow-800 dark:bg-yellow-900 dark:text-yellow-300' };
    const original = discovery?.endpoints.find((e) => e.path === ep.path && e.method === ep.method);
    if (original?.is_supported === true) return { label: 'Supported', class: 'bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-300' };
    if (original?.is_supported === false) return { label: 'Not Supported', class: 'bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-300' };
    return { label: 'Unknown', class: 'bg-yellow-100 text-yellow-800 dark:bg-yellow-900 dark:text-yellow-300' };
  }

  let enabledEndpointCount = $derived(endpointRows.filter((r) => r.is_enabled).length);
</script>

<div class="mx-auto max-w-2xl space-y-6">
  <div class="flex items-center gap-4">
    <a href="/providers" class="text-sm text-muted-foreground hover:text-foreground">← Providers</a>
    <h1 class="text-2xl font-bold tracking-tight">Add Provider</h1>
  </div>

  <!-- Step indicators -->
  <div class="flex items-center gap-2">
    {#each [
      { n: 1, label: 'Info' },
      { n: 2, label: 'Discovery' },
      { n: 3, label: 'Confirm' }
    ] as s}
      <div class="flex items-center gap-2">
        <div
          class="flex h-7 w-7 items-center justify-center rounded-full text-xs font-bold {step === s.n
            ? 'bg-primary text-primary-foreground'
            : step > s.n
              ? 'bg-green-500 text-white'
              : 'bg-muted text-muted-foreground'}"
        >
          {s.n}
        </div>
        <span class="text-sm {step === s.n ? 'font-medium' : 'text-muted-foreground'}">{s.label}</span>
        {#if s.n < 3}
          <span class="text-muted-foreground">›</span>
        {/if}
      </div>
    {/each}
  </div>

  <!-- Step 1: Provider Info -->
  {#if step === 1}
    <Card>
      <CardHeader>
        <CardTitle>Provider Information</CardTitle>
      </CardHeader>
      <CardContent class="space-y-4">
        {#if step1Error}
          <div class="rounded-md border border-destructive/50 bg-destructive/10 px-4 py-3 text-sm text-destructive">
            {step1Error}
          </div>
        {/if}

        <div class="space-y-2">
          <label for="provider-name" class="text-sm font-medium">Name <span class="text-destructive">*</span></label>
          <Input id="provider-name" bind:value={name} placeholder="e.g. Local Ollama" />
        </div>

        <div class="space-y-2">
          <label for="provider-url" class="text-sm font-medium">Base URL <span class="text-destructive">*</span></label>
          <Input id="provider-url" bind:value={baseUrl} placeholder="https://api.example.com" />
        </div>

        <div class="space-y-2">
          <label for="provider-key" class="text-sm font-medium">API Key <span class="text-muted-foreground text-xs">(optional)</span></label>
          <Input id="provider-key" type="password" bind:value={apiKey} placeholder="sk-..." />
        </div>

        <div class="flex justify-end pt-2">
          <Button onclick={handleStep1Submit} disabled={step1Loading}>
            {step1Loading ? 'Creating...' : 'Next: Discover →'}
          </Button>
        </div>
      </CardContent>
    </Card>
  {/if}

  <!-- Step 2: Discovery -->
  {#if step === 2}
    <Card>
      <CardHeader>
        <CardTitle>Discovery Results</CardTitle>
      </CardHeader>
      <CardContent class="space-y-6">
        {#if discoveryError}
          <div class="rounded-md border border-destructive/50 bg-destructive/10 px-4 py-3 text-sm text-destructive">
            {discoveryError}
            <button
              type="button"
              class="ml-2 underline"
              onclick={() => {
                discoveryStarted = false;
                discoveryError = null;
                runDiscovery();
              }}
            >
              Retry
            </button>
          </div>
        {:else if discoveryLoading}
          <div class="flex items-center gap-3 py-6 text-muted-foreground">
            <div class="h-5 w-5 animate-spin rounded-full border-2 border-primary border-t-transparent"></div>
            Discovering provider capabilities...
          </div>
        {:else if discovery}
          <!-- Models -->
          <div>
            <h3 class="mb-3 text-sm font-semibold">Available Models ({discovery.models.length})</h3>
            {#if discovery.models.length === 0}
              <p class="text-sm text-muted-foreground">No models discovered.</p>
            {:else}
              <div class="space-y-2 rounded-md border p-3">
                {#each discovery.models as modelId}
                  <label class="flex cursor-pointer items-center gap-2 text-sm">
                    <input
                      type="checkbox"
                      checked={selectedModels.includes(modelId)}
                      onchange={() => toggleModel(modelId)}
                      class="h-4 w-4 rounded border-gray-300"
                    />
                    <span class="font-mono">{modelId}</span>
                  </label>
                {/each}
              </div>
            {/if}
          </div>

          <!-- Endpoints -->
          <div>
            <h3 class="mb-3 text-sm font-semibold">Endpoints ({discovery.endpoints.length})</h3>
            {#if discovery.endpoints.length === 0}
              <p class="text-sm text-muted-foreground">No endpoints discovered.</p>
            {:else}
              <table class="w-full text-sm">
                <thead>
                  <tr class="border-b text-left text-xs font-medium uppercase tracking-wide text-muted-foreground">
                    <th class="py-2 pr-4">Path</th>
                    <th class="py-2 pr-4">Method</th>
                    <th class="py-2 pr-4">Status</th>
                    <th class="py-2">Enabled</th>
                  </tr>
                </thead>
                <tbody>
                  {#each discovery.endpoints as ep, i}
                    {@const badge = supportedLabel(ep)}
                    <tr class="border-b last:border-0">
                      <td class="py-2 pr-4 font-mono text-xs">{ep.path}</td>
                      <td class="py-2 pr-4 font-mono text-xs uppercase">{ep.method}</td>
                      <td class="py-2 pr-4">
                        <span class="inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium {badge.class}">
                          {badge.label}
                        </span>
                      </td>
                      <td class="py-2">
                        <input
                          type="checkbox"
                          checked={endpointRows[i]?.is_enabled ?? false}
                          onchange={(e) => toggleEndpoint(i, (e.target as HTMLInputElement).checked)}
                          class="h-4 w-4 rounded border-gray-300"
                        />
                      </td>
                    </tr>
                  {/each}
                </tbody>
              </table>
            {/if}
          </div>

          <div class="flex justify-end pt-2">
            <Button onclick={() => (step = 3)}>Next: Review →</Button>
          </div>
        {/if}
      </CardContent>
    </Card>
  {/if}

  <!-- Step 3: Confirmation -->
  {#if step === 3}
    <Card>
      <CardHeader>
        <CardTitle>Confirm Setup</CardTitle>
      </CardHeader>
      <CardContent class="space-y-4">
        {#if confirmError}
          <div class="rounded-md border border-destructive/50 bg-destructive/10 px-4 py-3 text-sm text-destructive">
            {confirmError}
          </div>
        {/if}

        <div class="rounded-md border bg-muted/30 p-4 space-y-2 text-sm">
          <div class="flex justify-between">
            <span class="text-muted-foreground">Provider Name</span>
            <span class="font-medium">{provider?.name}</span>
          </div>
          <div class="flex justify-between">
            <span class="text-muted-foreground">Base URL</span>
            <span class="font-medium font-mono truncate max-w-[260px]">{provider?.base_url}</span>
          </div>
          <div class="flex justify-between">
            <span class="text-muted-foreground">Selected Models</span>
            <span class="font-medium">{selectedModels.length}</span>
          </div>
          <div class="flex justify-between">
            <span class="text-muted-foreground">Enabled Endpoints</span>
            <span class="font-medium">{enabledEndpointCount}</span>
          </div>
        </div>

        <div class="flex justify-between pt-2">
          <Button variant="outline" type="button" onclick={() => (step = 2)}>← Back</Button>
          <Button onclick={handleConfirm} disabled={confirmLoading}>
            {confirmLoading ? 'Confirming...' : 'Confirm & Save'}
          </Button>
        </div>
      </CardContent>
    </Card>
  {/if}
</div>
