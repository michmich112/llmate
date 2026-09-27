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
  let openMenuId = $state<string | null>(null);
  let editing = $state<APIKey | null>(null);
  let editName = $state('');
  let editRPM = $state('');
  let editTPM = $state('');
  let editActive = $state(true);
  let editSaving = $state(false);
  let editDialog = $state<HTMLDialogElement | null>(null);

  function limitError(rpmRaw: string, tpmRaw: string): string | null {
    const ok = (value: string) => value === '' || /^[1-9]\d*$/.test(value);
    if (!ok(rpmRaw.trim()) || !ok(tpmRaw.trim())) {
      return 'RPM and TPM must be whole numbers greater than 0, or left blank';
    }
    return null;
  }

  function limitValue(raw: string): number | undefined {
    const value = raw.trim();
    if (!value) return undefined;
    return Number(value);
  }

  function nameTaken(name: string, excludeId?: string): boolean {
    const want = name.trim().toLowerCase();
    return keys.some((key) => key.id !== excludeId && key.name.trim().toLowerCase() === want);
  }

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
    const limitsMessage = limitError(rateLimitRPM, rateLimitTPM);
    if (limitsMessage) {
      error = limitsMessage;
      return;
    }
    if (nameTaken(trimmed)) {
      error = `an API key named "${trimmed}" already exists`;
      return;
    }
    saving = true;
    error = null;
    try {
      const input: APIKeyCreateInput = { name: trimmed };
      const rpm = limitValue(rateLimitRPM);
      const tpm = limitValue(rateLimitTPM);
      if (rpm !== undefined) input.rate_limit_rpm = rpm;
      if (tpm !== undefined) input.rate_limit_tpm = tpm;
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

  function openEdit(key: APIKey) {
    editing = key;
    editName = key.name;
    editRPM = key.rate_limit_rpm != null ? String(key.rate_limit_rpm) : '';
    editTPM = key.rate_limit_tpm != null ? String(key.rate_limit_tpm) : '';
    editActive = key.is_active;
    openMenuId = null;
    editDialog?.showModal();
  }

  function closeEdit() {
    editDialog?.close();
    editing = null;
  }

  async function handleSaveEdit() {
    if (!editing) return;
    const trimmed = editName.trim();
    if (!trimmed) {
      error = 'Name is required';
      return;
    }
    const limitsMessage = limitError(editRPM, editTPM);
    if (limitsMessage) {
      error = limitsMessage;
      return;
    }
    if (nameTaken(trimmed, editing.id)) {
      error = `an API key named "${trimmed}" already exists`;
      return;
    }
    editSaving = true;
    error = null;
    try {
      const rpm = limitValue(editRPM);
      const tpm = limitValue(editTPM);
      const updated = await api.updateAPIKey(editing.id, {
        name: trimmed,
        is_active: editActive,
        ...(rpm !== undefined ? { rate_limit_rpm: rpm } : {}),
        ...(tpm !== undefined ? { rate_limit_tpm: tpm } : {})
      });
      keys = keys.map((k) => (k.id === updated.id ? updated : k));
      closeEdit();
    } catch (e) {
      error = e instanceof Error ? e.message : 'Failed to update API key';
    } finally {
      editSaving = false;
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
            <Input id="rpm" bind:value={rateLimitRPM} placeholder="Optional" type="text" inputmode="numeric" />
          </div>
          <div class="space-y-2">
            <label for="tpm" class="text-sm font-medium leading-none">Rate limit (TPM)</label>
            <Input id="tpm" bind:value={rateLimitTPM} placeholder="Optional" type="text" inputmode="numeric" />
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
                <td class="px-4 py-3 text-muted-foreground">{key.is_active ? 'Active' : 'Inactive'}</td>
                <td class="relative px-4 py-3">
                  <Button
                    variant="ghost"
                    size="icon"
                    aria-label={`Actions for ${key.name}`}
                    aria-haspopup="menu"
                    aria-expanded={openMenuId === key.id}
                    onclick={() => (openMenuId = openMenuId === key.id ? null : key.id)}
                  >
                    ⋮
                  </Button>
                  {#if openMenuId === key.id}
                    <div
                      role="menu"
                      class="absolute right-4 z-20 mt-1 w-36 rounded-md border bg-background p-1 shadow-md"
                    >
                      <button
                        type="button"
                        role="menuitem"
                        class="flex w-full rounded-sm px-3 py-2 text-left text-sm hover:bg-accent"
                        onclick={() => openEdit(key)}
                      >
                        Update
                      </button>
                      <button
                        type="button"
                        role="menuitem"
                        class="flex w-full rounded-sm px-3 py-2 text-left text-sm text-destructive hover:bg-accent"
                        onclick={() => {
                          openMenuId = null;
                          handleDelete(key.id);
                        }}
                      >
                        Delete
                      </button>
                    </div>
                  {/if}
                </td>
              </tr>
            {/each}
          </tbody>
        </table>
      {/if}
    </CardContent>
  </Card>
</div>

<dialog
  id="edit-key-dialog"
  bind:this={editDialog}
  class="w-full max-w-md rounded-lg border border-border bg-background p-6 shadow-lg backdrop:bg-black/50"
  aria-labelledby="edit-key-title"
>
  <h3 id="edit-key-title" class="text-lg font-semibold">Update API key</h3>
  <div class="mt-4 space-y-4">
    <div class="space-y-2">
      <label for="edit-key-name" class="text-sm font-medium leading-none">Name</label>
      <Input id="edit-key-name" bind:value={editName} />
    </div>
    <div class="grid gap-4 sm:grid-cols-2">
      <div class="space-y-2">
        <label for="edit-rpm" class="text-sm font-medium leading-none">Rate limit (RPM)</label>
        <Input id="edit-rpm" bind:value={editRPM} placeholder="Optional" type="text" inputmode="numeric" />
      </div>
      <div class="space-y-2">
        <label for="edit-tpm" class="text-sm font-medium leading-none">Rate limit (TPM)</label>
        <Input id="edit-tpm" bind:value={editTPM} placeholder="Optional" type="text" inputmode="numeric" />
      </div>
    </div>
    <div class="flex items-center justify-between">
      <label for="edit-active" class="text-sm font-medium leading-none">Active</label>
      <Switch id="edit-active" checked={editActive} onCheckedChange={(v: boolean) => (editActive = v)} />
    </div>
  </div>
  <div class="mt-6 flex justify-end gap-2">
    <Button variant="outline" type="button" onclick={closeEdit}>Cancel</Button>
    <Button type="button" onclick={handleSaveEdit} disabled={editSaving}>
      {editSaving ? 'Saving…' : 'Save'}
    </Button>
  </div>
</dialog>
