<script lang="ts">
  import { onMount } from 'svelte';
  import { api } from '$lib/api/client';
  import { Button } from '$lib/components/ui/button';
  import { Card, CardContent, CardHeader, CardTitle } from '$lib/components/ui/card';
  import { Input } from '$lib/components/ui/input';
  import { Switch } from '$lib/components/ui/switch';
  import type { APIKey, APIKeyCreateInput } from '$lib/types';

  let keys = $state<APIKey[]>([]);
  let loading = $state(true);
  let error = $state<string | null>(null);
  let saving = $state(false);

  let name = $state('');
  let rateLimitRPM = $state('');
  let rateLimitTPM = $state('');
  let showKey = $state<string | null>(null);
  let revealedId = $state<string | null>(null);
  let requireAPIKeys = $state(false);
  let configReady = $state(false);
  let requireSaving = $state(false);

  onMount(async () => {
    loading = true;
    error = null;
    const keysPromise = api.listAPIKeys();
    const configPromise = api.getConfig();
    try {
      keys = await keysPromise;
    } catch (e) {
      error = e instanceof Error ? e.message : 'Failed to load API keys';
    } finally {
      loading = false;
    }
    try {
      const config = await configPromise;
      requireAPIKeys = config.require_api_keys;
      configReady = true;
    } catch (e) {
      error = e instanceof Error ? e.message : 'Failed to load API key settings';
    }
  });

  async function handleRequireAPIKeys(next: boolean) {
    const previous = requireAPIKeys;
    requireAPIKeys = next;
    requireSaving = true;
    error = null;
    try {
      const updated = await api.updateConfig({ require_api_keys: next });
      requireAPIKeys = updated.require_api_keys;
    } catch (e) {
      requireAPIKeys = previous;
      error = e instanceof Error ? e.message : 'Failed to update API key requirement';
    } finally {
      requireSaving = false;
    }
  }

  function formatDate(dateStr?: string): string {
    if (!dateStr) return '—';
    return new Date(dateStr).toLocaleString();
  }

  async function handleCreate() {
    const trimmed = name.trim();
    if (!trimmed) {
      error = 'Name is required';
      return;
    }
    saving = true;
    error = null;
    try {
      const input: APIKeyCreateInput = { name: trimmed };
      const rpm = parseInt(rateLimitRPM, 10);
      const tpm = parseInt(rateLimitTPM, 10);
      if (!Number.isNaN(rpm) && rpm > 0) input.rate_limit_rpm = rpm;
      if (!Number.isNaN(tpm) && tpm > 0) input.rate_limit_tpm = tpm;
      const created = await api.createAPIKey(input);
      keys = [created.api_key, ...keys];
      showKey = created.key;
      revealedId = created.api_key.id;
      name = '';
      rateLimitRPM = '';
      rateLimitTPM = '';
    } catch (e) {
      error = e instanceof Error ? e.message : 'Failed to create API key';
    } finally {
      saving = false;
    }
  }

  async function handleToggle(key: APIKey, isActive: boolean) {
    error = null;
    try {
      const updated = await api.updateAPIKey(key.id, {
        name: key.name,
        is_active: isActive,
        ...(key.rate_limit_rpm != null ? { rate_limit_rpm: key.rate_limit_rpm } : {}),
        ...(key.rate_limit_tpm != null ? { rate_limit_tpm: key.rate_limit_tpm } : {})
      });
      keys = keys.map((k) => (k.id === key.id ? updated : k));
    } catch (e) {
      error = e instanceof Error ? e.message : 'Failed to update API key';
    }
  }

  async function handleDelete(id: string) {
    error = null;
    try {
      await api.deleteAPIKey(id);
      keys = keys.filter((k) => k.id !== id);
      if (revealedId === id) {
        showKey = null;
        revealedId = null;
      }
    } catch (e) {
      error = e instanceof Error ? e.message : 'Failed to delete API key';
    }
  }
</script>

<div class="space-y-6">
  <div>
    <h1 class="text-2xl font-bold tracking-tight">API Keys</h1>
    <p class="text-muted-foreground mt-1">Create and manage API keys for proxy access.</p>
  </div>

  <Card>
    <CardContent class="pt-6">
      <div class="flex items-center justify-between gap-4">
        <div class="space-y-1">
          <label for="require-api-keys" class="text-sm font-medium leading-none">Require API keys</label>
          <p class="text-sm text-muted-foreground">
            When on, every gateway request must include a valid API key. When off, requests without a key are
            accepted. Creating or deleting keys does not change this. A key that is sent is still checked.
          </p>
        </div>
        <Switch
          id="require-api-keys"
          checked={requireAPIKeys}
          disabled={!configReady || requireSaving}
          onCheckedChange={handleRequireAPIKeys}
        />
      </div>
    </CardContent>
  </Card>

  {#if error}
    <div class="rounded-md border border-destructive/50 bg-destructive/10 px-4 py-3 text-sm text-destructive">
      {error}
    </div>
  {/if}

  <Card>
    <CardHeader>
      <CardTitle>Create API key</CardTitle>
    </CardHeader>
    <CardContent>
      <div class="space-y-4">
        <div class="space-y-2">
          <label for="key-name" class="text-sm font-medium leading-none">Name</label>
          <Input id="key-name" bind:value={name} placeholder="e.g. dev-key" />
        </div>
        <div class="grid gap-4 sm:grid-cols-2">
          <div class="space-y-2">
            <label for="rpm" class="text-sm font-medium leading-none">Rate limit (RPM)</label>
            <Input id="rpm" bind:value={rateLimitRPM} placeholder="Optional" type="number" />
          </div>
          <div class="space-y-2">
            <label for="tpm" class="text-sm font-medium leading-none">Rate limit (TPM)</label>
            <Input id="tpm" bind:value={rateLimitTPM} placeholder="Optional" type="number" />
          </div>
        </div>
        <Button onclick={handleCreate} disabled={saving || loading}>
          {saving ? 'Creating…' : 'Create key'}
        </Button>
      </div>
    </CardContent>
  </Card>

  {#if showKey}
    <Card class="border-green-500/30">
      <CardContent class="pt-6">
        <p class="text-sm font-semibold text-green-600">
          Key created. The raw key is shown once and cannot be recovered later.
        </p>
        <code
          class="mt-2 block break-all rounded bg-muted px-3 py-2 font-mono text-sm"
        >{showKey}</code>
        <Button class="mt-3" variant="outline" onclick={() => (showKey = null)}>Dismiss</Button>
      </CardContent>
    </Card>
  {/if}

  <Card>
    <CardHeader>
      <CardTitle>Keys</CardTitle>
    </CardHeader>
    <CardContent>
      {#if loading}
        <div class="space-y-2">
          {#each [1, 2, 3] as _, i (i)}
            <div class="h-12 animate-pulse rounded-md bg-muted"></div>
          {/each}
        </div>
      {:else if keys.length === 0}
        <p class="text-muted-foreground">No API keys yet.</p>
      {:else}
        <table class="w-full text-sm">
          <thead>
            <tr class="border-b bg-muted/50 text-left text-xs font-medium uppercase tracking-wide text-muted-foreground">
              <th class="px-4 py-3">Name</th>
              <th class="px-4 py-3">Created</th>
              <th class="px-4 py-3">RPM</th>
              <th class="px-4 py-3">TPM</th>
              <th class="px-4 py-3">Active</th>
              <th class="px-4 py-3">Actions</th>
            </tr>
          </thead>
          <tbody>
            {#each keys as key (key.id)}
              <tr class="border-b last:border-0">
                <td class="px-4 py-3 font-medium">{key.name}</td>
                <td class="px-4 py-3 text-muted-foreground">{formatDate(key.created_at)}</td>
                <td class="px-4 py-3 text-muted-foreground">{key.rate_limit_rpm ?? '—'}</td>
                <td class="px-4 py-3 text-muted-foreground">{key.rate_limit_tpm ?? '—'}</td>
                <td class="px-4 py-3">
                  <Switch
                    checked={key.is_active}
                    onCheckedChange={(v: boolean) => handleToggle(key, v)}
                  />
                </td>
                <td class="px-4 py-3">
                  <Button
                    variant="destructive"
                    size="sm"
                    onclick={() => handleDelete(key.id)}
                  >
                    Delete
                  </Button>
                </td>
              </tr>
            {/each}
          </tbody>
        </table>
      {/if}
    </CardContent>
  </Card>
</div>
