<script lang="ts">
  import { api } from '$lib/api/client';
  import type { ModelAlias, Provider, ProviderModel } from '$lib/types';
  import { Button } from '$lib/components/ui/button';
  import { Input } from '$lib/components/ui/input';
  import { Card, CardHeader, CardTitle, CardContent } from '$lib/components/ui/card';

  let aliases = $state<ModelAlias[]>([]);
  let providers = $state<Provider[]>([]);
  let loading = $state(true);
  let error = $state<string | null>(null);

  // Dialog state
  let showDialog = $state(false);
  let editingAlias = $state<ModelAlias | null>(null);
  let dialogLoading = $state(false);
  let dialogError = $state<string | null>(null);

  // Dialog form fields
  let formAlias = $state('');
  let formProviderId = $state('');
  let formModelId = $state('');
  let formWeight = $state(1);
  let formPriority = $state(0);
  let formEnabled = $state(true);

  // Provider models for selected provider
  let providerModels = $state<ProviderModel[]>([]);
  let modelsLoading = $state(false);

  // Delete confirm
  let showDeleteDialog = $state(false);
  let deletingAlias = $state<ModelAlias | null>(null);
  let deleteLoading = $state(false);
  let deleteError = $state<string | null>(null);

  $effect(() => {
    loadData();
  });

  async function loadData() {
    loading = true;
    error = null;
    try {
      const [a, p] = await Promise.all([api.listAliases(), api.listProviders()]);
      aliases = a;
      providers = p;
    } catch (e) {
      error = e instanceof Error ? e.message : 'Failed to load data';
    } finally {
      loading = false;
    }
  }

  function providerName(providerId: string): string {
    return providers.find((p) => p.id === providerId)?.name ?? providerId;
  }

  function openAddDialog() {
    editingAlias = null;
    formAlias = '';
    formProviderId = providers[0]?.id ?? '';
    formModelId = '';
    formWeight = 1;
    formPriority = 0;
    formEnabled = true;
    providerModels = [];
    dialogError = null;
    showDialog = true;
    if (formProviderId) loadProviderModels(formProviderId);
  }

  function openEditDialog(alias: ModelAlias) {
    editingAlias = alias;
    formAlias = alias.alias;
    formProviderId = alias.provider_id;
    formModelId = alias.model_id;
    formWeight = alias.weight;
    formPriority = alias.priority;
    formEnabled = alias.is_enabled;
    dialogError = null;
    showDialog = true;
    loadProviderModels(alias.provider_id);
  }

  async function loadProviderModels(pid: string) {
    if (!pid) {
      providerModels = [];
      return;
    }
    modelsLoading = true;
    try {
      const data = await api.getProvider(pid);
      providerModels = data.models;
    } catch {
      providerModels = [];
    } finally {
      modelsLoading = false;
    }
  }

  function handleProviderChange(e: Event) {
    const pid = (e.target as HTMLSelectElement).value;
    formProviderId = pid;
    formModelId = '';
    loadProviderModels(pid);
  }

  async function handleDialogSubmit() {
    if (!formAlias.trim()) {
      dialogError = 'Alias name is required';
      return;
    }
    if (!formProviderId) {
      dialogError = 'Provider is required';
      return;
    }
    if (!formModelId) {
      dialogError = 'Model is required';
      return;
    }
    dialogLoading = true;
    dialogError = null;
    try {
      if (editingAlias) {
        const updated = await api.updateAlias(editingAlias.id, {
          alias: formAlias.trim(),
          provider_id: formProviderId,
          model_id: formModelId,
          weight: formWeight,
          priority: formPriority,
          is_enabled: formEnabled
        });
        aliases = aliases.map((a) => (a.id === updated.id ? updated : a));
      } else {
        const created = await api.createAlias({
          alias: formAlias.trim(),
          provider_id: formProviderId,
          model_id: formModelId,
          weight: formWeight,
          priority: formPriority,
          is_enabled: formEnabled
        });
        aliases = [...aliases, created];
      }
      showDialog = false;
    } catch (e) {
      dialogError = e instanceof Error ? e.message : 'Failed to save alias';
    } finally {
      dialogLoading = false;
    }
  }

  function confirmDelete(alias: ModelAlias) {
    deletingAlias = alias;
    deleteError = null;
    showDeleteDialog = true;
  }

  async function handleDelete() {
    if (!deletingAlias) return;
    deleteLoading = true;
    deleteError = null;
    try {
      await api.deleteAlias(deletingAlias.id);
      aliases = aliases.filter((a) => a.id !== deletingAlias!.id);
      showDeleteDialog = false;
      deletingAlias = null;
    } catch (e) {
      deleteError = e instanceof Error ? e.message : 'Failed to delete alias';
    } finally {
      deleteLoading = false;
    }
  }
</script>

<div class="space-y-6">
  <div class="flex items-center justify-between">
    <h1 class="text-2xl font-bold tracking-tight">Model Aliases</h1>
    <Button onclick={openAddDialog}>Add Alias</Button>
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
  {:else if aliases.length === 0}
    <Card>
      <CardContent class="py-12 text-center">
        <p class="mb-4 text-muted-foreground">No model aliases configured yet.</p>
        <Button onclick={openAddDialog}>Add Your First Alias</Button>
      </CardContent>
    </Card>
  {:else}
    <Card>
      <CardContent class="p-0">
        <table class="w-full text-sm">
          <thead>
            <tr class="border-b bg-muted/50 text-left text-xs font-medium uppercase tracking-wide text-muted-foreground">
              <th class="px-4 py-3">Alias</th>
              <th class="px-4 py-3">Provider</th>
              <th class="px-4 py-3">Model ID</th>
              <th class="px-4 py-3 text-right">Weight</th>
              <th class="px-4 py-3 text-right">Priority</th>
              <th class="px-4 py-3">Enabled</th>
              <th class="px-4 py-3">Actions</th>
            </tr>
          </thead>
          <tbody>
            {#each aliases as alias}
              <tr class="border-b last:border-0 hover:bg-muted/30">
                <td class="px-4 py-3 font-medium">{alias.alias}</td>
                <td class="px-4 py-3 text-muted-foreground">{providerName(alias.provider_id)}</td>
                <td class="px-4 py-3 font-mono text-xs">{alias.model_id}</td>
                <td class="px-4 py-3 text-right">{alias.weight}</td>
                <td class="px-4 py-3 text-right">{alias.priority}</td>
                <td class="px-4 py-3">
                  {#if alias.is_enabled}
                    <span class="inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-300">Yes</span>
                    {:else}
                    <span class="inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium bg-gray-100 text-gray-600 dark:bg-gray-800 dark:text-gray-400">No</span>
                  {/if}
                </td>
                <td class="px-4 py-3">
                  <div class="flex gap-2">
                    <button
                      type="button"
                      class="text-primary hover:underline text-xs"
                      onclick={() => openEditDialog(alias)}
                    >
                      Edit
                    </button>
                    <button
                      type="button"
                      class="text-destructive hover:underline text-xs"
                      onclick={() => confirmDelete(alias)}
                    >
                      Delete
                    </button>
                  </div>
                </td>
              </tr>
            {/each}
          </tbody>
        </table>
      </CardContent>
    </Card>
  {/if}
</div>

<!-- Add/Edit Dialog -->
{#if showDialog}
  <div
    role="presentation"
    class="fixed inset-0 z-50 flex items-center justify-center bg-black/50"
    onclick={() => (showDialog = false)}
    onkeydown={(e) => { if (e.key === 'Escape') showDialog = false; }}
  >
    <div
      role="dialog"
      aria-modal="true"
      tabindex="-1"
      class="mx-4 w-full max-w-md rounded-lg bg-popover p-6 shadow-xl"
      onclick={(e) => e.stopPropagation()}
      onkeydown={(e) => e.stopPropagation()}
    >
      <h2 class="mb-4 text-lg font-bold">{editingAlias ? 'Edit Alias' : 'Add Alias'}</h2>

      <div class="space-y-4">
        {#if dialogError}
          <div class="rounded-md border border-destructive/50 bg-destructive/10 px-4 py-3 text-sm text-destructive">
            {dialogError}
          </div>
        {/if}

        <div class="space-y-2">
          <label for="form-alias" class="text-sm font-medium">Alias Name <span class="text-destructive">*</span></label>
          <Input id="form-alias" bind:value={formAlias} placeholder="gpt-4" />
        </div>

        <div class="space-y-2">
          <label for="form-provider" class="text-sm font-medium">Provider <span class="text-destructive">*</span></label>
          <select
            id="form-provider"
            value={formProviderId}
            onchange={handleProviderChange}
            class="flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2"
          >
            <option value="">Select provider...</option>
            {#each providers as p}
              <option value={p.id}>{p.name}</option>
            {/each}
          </select>
        </div>

        <div class="space-y-2">
          <label for="form-model" class="text-sm font-medium">Model ID <span class="text-destructive">*</span></label>
          <select
            id="form-model"
            bind:value={formModelId}
            disabled={!formProviderId || modelsLoading}
            class="flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:opacity-50"
          >
            <option value="">{modelsLoading ? 'Loading...' : 'Select model...'}</option>
            {#each providerModels as m}
              <option value={m.model_id}>{m.model_id}</option>
            {/each}
          </select>
        </div>

        <div class="grid grid-cols-2 gap-4">
          <div class="space-y-2">
            <label for="form-weight" class="text-sm font-medium">Weight</label>
            <Input id="form-weight" type="number" bind:value={formWeight} min="0" step="1" />
          </div>
          <div class="space-y-2">
            <label for="form-priority" class="text-sm font-medium">Priority</label>
            <Input id="form-priority" type="number" bind:value={formPriority} step="1" />
          </div>
        </div>

        <label class="flex cursor-pointer items-center gap-2 text-sm">
          <input
            type="checkbox"
            bind:checked={formEnabled}
            class="h-4 w-4 rounded border-gray-300"
          />
          <span>Enabled</span>
        </label>
      </div>

      <div class="mt-6 flex justify-end gap-2">
        <Button variant="outline" type="button" onclick={() => (showDialog = false)}>Cancel</Button>
        <Button onclick={handleDialogSubmit} disabled={dialogLoading}>
          {dialogLoading ? 'Saving...' : editingAlias ? 'Save Changes' : 'Create Alias'}
        </Button>
      </div>
    </div>
  </div>
{/if}

<!-- Delete Confirm Dialog -->
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
      <h2 class="mb-2 text-lg font-bold">Delete Alias</h2>
      <p class="mb-4 text-sm text-muted-foreground">
        Are you sure you want to delete alias <strong>{deletingAlias?.alias}</strong>?
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
