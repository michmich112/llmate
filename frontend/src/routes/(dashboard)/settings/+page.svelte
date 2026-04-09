<script lang="ts">
  import { onMount } from 'svelte';
  import { Card, CardContent } from '$lib/components/ui/card';
  import { Button } from '$lib/components/ui/button';
  import { Input } from '$lib/components/ui/input';
  import { Label } from '$lib/components/ui/label';
  import { Switch } from '$lib/components/ui/switch';
  import { api } from '$lib/api/client';
  import type { Configuration, ConfigDefinition } from '$lib/types';

  function bytesToKB(bytes: number): number {
    return Math.floor(bytes / 1024);
  }

  function kbToBytes(kb: number): number {
    return Math.floor(kb * 1024);
  }

  function formatBytes(bytes: number): string {
    if (bytes === 0) return 'Unlimited';
    const units = ['bytes', 'KB', 'MB', 'GB'];
    let value = bytes;
    let unitIndex = 0;
    while (value >= 1024 && unitIndex < units.length - 1) {
      value /= 1024;
      unitIndex++;
    }
    return `${value.toFixed(value < 10 && unitIndex > 0 ? 1 : 0)} ${units[unitIndex]}`;
  }

  type LoggingFormConfig = Pick<
    Configuration,
    'request_body_max_bytes' | 'response_body_max_bytes' | 'track_streaming' | 'streaming_buffer_size'
  >;

  const defaultLogging: LoggingFormConfig = {
    request_body_max_bytes: 51200,
    response_body_max_bytes: 51200,
    track_streaming: false,
    streaming_buffer_size: 10240
  };

  const defaultRetentionDays = 30;

  let formState = $state<LoggingFormConfig>({ ...defaultLogging });
  let baseline = $state<LoggingFormConfig>({ ...defaultLogging });
  let retentionStreamingDraft = $state(defaultRetentionDays);
  let retentionStreamingBaseline = $state(defaultRetentionDays);
  let retentionRequestDraft = $state(defaultRetentionDays);
  let retentionRequestBaseline = $state(defaultRetentionDays);
  let retentionResponseDraft = $state(defaultRetentionDays);
  let retentionResponseBaseline = $state(defaultRetentionDays);

  let configDef = $state<ConfigDefinition | null>(null);
  let isLoading = $state(true);
  let saveStatus: 'idle' | 'saving' | 'success' | 'error' = $state('idle');
  let errorMsg = $state('');

  let retentionDialog = $state<HTMLDialogElement | null>(null);
  let retentionConfirmChecked = $state(false);
  let retentionApplyStatus: 'idle' | 'saving' | 'success' | 'error' = $state('idle');
  let retentionErrorMsg = $state('');

  const bodyMaxLimit = $derived(configDef?.request_body_max_bytes.max ?? 1073741824);
  const bufferMinKB = $derived(
    configDef?.streaming_buffer_size.min != null
      ? Math.max(1, Math.ceil(configDef.streaming_buffer_size.min / 1024))
      : 1
  );
  const bufferMaxKB = $derived(
    configDef?.streaming_buffer_size.max != null
      ? Math.floor(configDef.streaming_buffer_size.max / 1024)
      : 1024
  );

  const retentionMin = $derived(configDef?.streaming_log_body_retention_days.min ?? 1);
  const retentionMax = $derived(configDef?.streaming_log_body_retention_days.max ?? 3650);

  const retentionDirty = $derived(
    retentionStreamingDraft !== retentionStreamingBaseline ||
      retentionRequestDraft !== retentionRequestBaseline ||
      retentionResponseDraft !== retentionResponseBaseline
  );
  const bufferKB = $derived(bytesToKB(formState.streaming_buffer_size));

  function handleChange<K extends keyof LoggingFormConfig>(key: K, value: LoggingFormConfig[K]) {
    formState[key] = value;
    saveStatus = 'idle';
  }

  onMount(async () => {
    try {
      const [config, def] = await Promise.all([
        api.getConfig(),
        api.getConfigDefinition().catch(() => null)
      ]);
      formState = {
        request_body_max_bytes: config.request_body_max_bytes,
        response_body_max_bytes: config.response_body_max_bytes,
        track_streaming: config.track_streaming,
        streaming_buffer_size: config.streaming_buffer_size
      };
      baseline = { ...formState };
      retentionStreamingDraft = config.streaming_log_body_retention_days;
      retentionStreamingBaseline = config.streaming_log_body_retention_days;
      retentionRequestDraft = config.request_log_body_retention_days;
      retentionRequestBaseline = config.request_log_body_retention_days;
      retentionResponseDraft = config.response_log_body_retention_days;
      retentionResponseBaseline = config.response_log_body_retention_days;
      configDef = def;
    } catch (err) {
      errorMsg = 'Failed to load configuration';
      console.error(err);
    } finally {
      isLoading = false;
    }
  });

  function isDirty(): boolean {
    return JSON.stringify(formState) !== JSON.stringify(baseline);
  }

  async function handleSubmit() {
    saveStatus = 'saving';
    errorMsg = '';

    try {
      const updated = await api.updateConfig({ ...formState });
      formState = {
        request_body_max_bytes: updated.request_body_max_bytes,
        response_body_max_bytes: updated.response_body_max_bytes,
        track_streaming: updated.track_streaming,
        streaming_buffer_size: updated.streaming_buffer_size
      };
      baseline = { ...formState };
      retentionStreamingDraft = updated.streaming_log_body_retention_days;
      retentionStreamingBaseline = updated.streaming_log_body_retention_days;
      retentionRequestDraft = updated.request_log_body_retention_days;
      retentionRequestBaseline = updated.request_log_body_retention_days;
      retentionResponseDraft = updated.response_log_body_retention_days;
      retentionResponseBaseline = updated.response_log_body_retention_days;
      saveStatus = 'success';
    } catch (err: unknown) {
      saveStatus = 'error';
      errorMsg = err instanceof Error ? err.message : 'Failed to save configuration';
    }
  }

  async function handleReset() {
    formState = { ...defaultLogging };
    retentionStreamingDraft = defaultRetentionDays;
    retentionRequestDraft = defaultRetentionDays;
    retentionResponseDraft = defaultRetentionDays;
    saveStatus = 'idle';
  }

  function handleBufferKBChange(e: Event) {
    const val = parseInt((e.target as HTMLInputElement).value, 10) || bufferMinKB;
    const clamped = Math.max(bufferMinKB, Math.min(bufferMaxKB, val));
    formState.streaming_buffer_size = kbToBytes(clamped);
  }

  function openRetentionDialog() {
    retentionConfirmChecked = false;
    retentionErrorMsg = '';
    retentionApplyStatus = 'idle';
    retentionDialog?.showModal();
  }

  function closeRetentionDialog() {
    retentionDialog?.close();
  }

  function clampRetentionDays(label: string, raw: unknown): number | null {
    const days = Math.floor(Number(raw));
    if (!Number.isFinite(days) || days < retentionMin || days > retentionMax) {
      retentionErrorMsg = `${label}: use ${retentionMin}–${retentionMax} days.`;
      return null;
    }
    return days;
  }

  async function applyRetentionFromDialog() {
    retentionApplyStatus = 'saving';
    retentionErrorMsg = '';
    const patch: Partial<Configuration> = {};

    if (Math.floor(Number(retentionStreamingDraft)) !== retentionStreamingBaseline) {
      const d = clampRetentionDays('Streaming chunk bodies', retentionStreamingDraft);
      if (d === null) {
        retentionApplyStatus = 'error';
        return;
      }
      patch.streaming_log_body_retention_days = d;
    }
    if (Math.floor(Number(retentionRequestDraft)) !== retentionRequestBaseline) {
      const d = clampRetentionDays('Request bodies', retentionRequestDraft);
      if (d === null) {
        retentionApplyStatus = 'error';
        return;
      }
      patch.request_log_body_retention_days = d;
    }
    if (Math.floor(Number(retentionResponseDraft)) !== retentionResponseBaseline) {
      const d = clampRetentionDays('Response bodies', retentionResponseDraft);
      if (d === null) {
        retentionApplyStatus = 'error';
        return;
      }
      patch.response_log_body_retention_days = d;
    }

    if (Object.keys(patch).length === 0) {
      retentionApplyStatus = 'idle';
      closeRetentionDialog();
      return;
    }

    try {
      const updated = await api.updateConfig(patch);
      if (patch.streaming_log_body_retention_days != null) {
        retentionStreamingBaseline = updated.streaming_log_body_retention_days;
        retentionStreamingDraft = updated.streaming_log_body_retention_days;
      }
      if (patch.request_log_body_retention_days != null) {
        retentionRequestBaseline = updated.request_log_body_retention_days;
        retentionRequestDraft = updated.request_log_body_retention_days;
      }
      if (patch.response_log_body_retention_days != null) {
        retentionResponseBaseline = updated.response_log_body_retention_days;
        retentionResponseDraft = updated.response_log_body_retention_days;
      }
      retentionApplyStatus = 'success';
      closeRetentionDialog();
    } catch (err: unknown) {
      retentionApplyStatus = 'error';
      retentionErrorMsg = err instanceof Error ? err.message : 'Failed to apply retention';
    }
  }
</script>

<div class="space-y-6">
  <div>
    <h1 class="text-2xl font-bold">Logging Configuration</h1>
    <p class="text-muted-foreground mt-1">
      Configure request/response body size limits and optional SSE chunk logging
    </p>
  </div>

  {#if errorMsg && saveStatus === 'error'}
    <div class="rounded-md border border-destructive/50 bg-destructive/10 px-4 py-3 text-sm text-destructive">
      {errorMsg}
    </div>
  {/if}

  {#if saveStatus === 'success'}
    <div class="rounded-md border border-green-500/50 bg-green-500/10 px-4 py-3 text-sm text-green-600">
      Configuration saved successfully
    </div>
  {/if}

  <Card>
    <CardContent class="pt-6 space-y-6">
      <div class="space-y-2">
        <div class="flex items-center justify-between">
          <Label for="request-body-max">Request body max (bytes)</Label>
          <span class="text-xs font-mono text-muted-foreground">
            {formatBytes(formState.request_body_max_bytes)}
          </span>
        </div>
        <Input
          id="request-body-max"
          type="number"
          min={configDef?.request_body_max_bytes.min ?? 0}
          max={bodyMaxLimit}
          step={1024}
          bind:value={formState.request_body_max_bytes}
        />
        <p class="text-xs text-muted-foreground leading-relaxed">
          {configDef?.request_body_max_bytes.description ??
            'Cap on how much of each request body is stored on the request log. Longer bodies are truncated. 0 = store the full body.'}
        </p>
      </div>

      <div class="space-y-2">
        <div class="flex items-center justify-between">
          <Label for="response-body-max">Response body max (bytes)</Label>
          <span class="text-xs font-mono text-muted-foreground">
            {formatBytes(formState.response_body_max_bytes)}
          </span>
        </div>
        <Input
          id="response-body-max"
          type="number"
          min={configDef?.response_body_max_bytes.min ?? 0}
          max={bodyMaxLimit}
          step={1024}
          bind:value={formState.response_body_max_bytes}
        />
        <p class="text-xs text-muted-foreground leading-relaxed">
          {configDef?.response_body_max_bytes.description ??
            'Cap on stored response text (JSON or reconstructed stream text). Truncates when over the limit. 0 = no cap.'}
        </p>
      </div>

      <div class="space-y-2 flex items-center justify-between">
        <div class="space-y-1">
          <Label for="track-streaming">Track streaming (SSE lines)</Label>
          <p class="text-xs text-muted-foreground leading-relaxed">
            {configDef?.track_streaming.description ??
              'When on, buffers raw SSE data lines per stream and saves them under Streaming chunks in Logs. Does not change what the client receives.'}
          </p>
        </div>
        <Switch
          id="track-streaming"
          name="track_streaming"
          checked={formState.track_streaming}
          disabled={isLoading || saveStatus === 'saving'}
          onCheckedChange={(v: boolean) => handleChange('track_streaming', v)}
        />
      </div>

      <div class="space-y-2">
        <div class="flex items-center justify-between">
          <Label for="streaming-buffer">Streaming buffer (KB)</Label>
          <span class="text-xs font-mono text-muted-foreground">
            {formatBytes(formState.streaming_buffer_size)}
          </span>
        </div>
        <Input
          id="streaming-buffer"
          type="number"
          min={bufferMinKB}
          max={bufferMaxKB}
          step={1}
          disabled={!formState.track_streaming}
          value={bufferKB}
          onchange={handleBufferKBChange}
        />
        <p class="text-xs text-muted-foreground leading-relaxed">
          {configDef?.streaming_buffer_size.description ??
            `Rolling cap on total bytes of raw SSE text per stream (${bufferMinKB}–${bufferMaxKB} KB); oldest lines drop first—not a fixed chunk count.`}
        </p>
      </div>

      <div class="flex items-center gap-3 pt-4 border-t">
        <Button onclick={handleSubmit} disabled={isLoading || !isDirty() || saveStatus === 'saving'}>
          {saveStatus === 'saving' ? 'Saving...' : 'Save configuration'}
        </Button>
        <Button variant="outline" onclick={handleReset}>Reset to defaults</Button>
      </div>
    </CardContent>
  </Card>

  <Card class="border-amber-500/30">
    <CardContent class="pt-6 space-y-6">
      <div>
        <h2 class="text-lg font-semibold">Log body retention</h2>
        <p class="text-sm text-muted-foreground mt-1 leading-relaxed">
          Three independent day counts. The server clears matching stored text older than each window. All three run on
          the same daily schedule and immediately when you confirm below.
        </p>
      </div>

      <div class="space-y-4 max-w-md">
        <div class="space-y-2">
          <Label for="retention-streaming">Streaming chunk bodies (days)</Label>
          <p class="text-xs text-muted-foreground leading-relaxed">
            {configDef?.streaming_log_body_retention_days.description ??
              'Clears raw SSE line and text delta in streaming_logs; metadata rows stay.'}
          </p>
          <Input
            id="retention-streaming"
            type="number"
            min={retentionMin}
            max={retentionMax}
            step={1}
            bind:value={retentionStreamingDraft}
            disabled={isLoading || retentionApplyStatus === 'saving'}
          />
          <p class="text-xs text-muted-foreground">
            Saved: {retentionStreamingBaseline}
            {#if Math.floor(Number(retentionStreamingDraft)) !== retentionStreamingBaseline}
              <span class="text-amber-600 font-medium"> · Unsaved</span>
            {/if}
          </p>
        </div>

        <div class="space-y-2">
          <Label for="retention-request">Request bodies on log rows (days)</Label>
          <p class="text-xs text-muted-foreground leading-relaxed">
            {configDef?.request_log_body_retention_days.description ??
              'Clears request_body on each request log; other columns unchanged.'}
          </p>
          <Input
            id="retention-request"
            type="number"
            min={retentionMin}
            max={retentionMax}
            step={1}
            bind:value={retentionRequestDraft}
            disabled={isLoading || retentionApplyStatus === 'saving'}
          />
          <p class="text-xs text-muted-foreground">
            Saved: {retentionRequestBaseline}
            {#if Math.floor(Number(retentionRequestDraft)) !== retentionRequestBaseline}
              <span class="text-amber-600 font-medium"> · Unsaved</span>
            {/if}
          </p>
        </div>

        <div class="space-y-2">
          <Label for="retention-response">Response bodies on log rows (days)</Label>
          <p class="text-xs text-muted-foreground leading-relaxed">
            {configDef?.response_log_body_retention_days.description ??
              'Clears response_body on each request log (JSON or reconstructed stream text).'}
          </p>
          <Input
            id="retention-response"
            type="number"
            min={retentionMin}
            max={retentionMax}
            step={1}
            bind:value={retentionResponseDraft}
            disabled={isLoading || retentionApplyStatus === 'saving'}
          />
          <p class="text-xs text-muted-foreground">
            Saved: {retentionResponseBaseline}
            {#if Math.floor(Number(retentionResponseDraft)) !== retentionResponseBaseline}
              <span class="text-amber-600 font-medium"> · Unsaved</span>
            {/if}
          </p>
        </div>

        <p class="text-xs text-muted-foreground">Allowed range for each: {retentionMin}–{retentionMax} days.</p>
      </div>

      <Button
        variant="destructive"
        disabled={isLoading || !retentionDirty || retentionApplyStatus === 'saving'}
        onclick={openRetentionDialog}
      >
        Save &amp; apply
      </Button>

      {#if retentionApplyStatus === 'success'}
        <p class="text-sm text-green-600">Retention updated; matching old bodies were cleared on the server.</p>
      {/if}
      {#if retentionApplyStatus === 'error' && retentionErrorMsg}
        <p class="text-sm text-destructive">{retentionErrorMsg}</p>
      {/if}
    </CardContent>
  </Card>
</div>

<dialog
  bind:this={retentionDialog}
  class="max-w-lg rounded-lg border border-border bg-background p-6 shadow-lg backdrop:bg-black/50"
  aria-labelledby="retention-dialog-title"
>
  <h3 id="retention-dialog-title" class="text-lg font-semibold">Apply log body retention?</h3>
  <p class="mt-3 text-sm text-muted-foreground leading-relaxed">
    This will save any changed day counts and <strong class="text-foreground">immediately and permanently clear</strong>
    stored text older than each policy:
  </p>
  <ul class="mt-2 list-disc pl-5 text-sm text-muted-foreground space-y-1">
    {#if Math.floor(Number(retentionStreamingDraft)) !== retentionStreamingBaseline}
      <li>
        Streaming chunks: <strong class="text-foreground">{Math.floor(Number(retentionStreamingDraft))}</strong> days
      </li>
    {/if}
    {#if Math.floor(Number(retentionRequestDraft)) !== retentionRequestBaseline}
      <li>
        Request bodies: <strong class="text-foreground">{Math.floor(Number(retentionRequestDraft))}</strong> days
      </li>
    {/if}
    {#if Math.floor(Number(retentionResponseDraft)) !== retentionResponseBaseline}
      <li>
        Response bodies: <strong class="text-foreground">{Math.floor(Number(retentionResponseDraft))}</strong> days
      </li>
    {/if}
  </ul>
  <p class="mt-2 text-sm text-muted-foreground leading-relaxed">Request log metadata rows stay; only stored body fields are emptied. This cannot be undone.</p>
  <label class="mt-4 flex items-start gap-2 text-sm cursor-pointer">
    <input
      type="checkbox"
      class="mt-1 rounded border-input"
      bind:checked={retentionConfirmChecked}
    />
    <span>I understand that old body content will be removed from the database.</span>
  </label>
  {#if retentionErrorMsg && retentionApplyStatus === 'error'}
    <p class="mt-3 text-sm text-destructive">{retentionErrorMsg}</p>
  {/if}
  <div class="mt-6 flex justify-end gap-2">
    <Button variant="outline" type="button" onclick={closeRetentionDialog}>Cancel</Button>
    <Button
      variant="destructive"
      type="button"
      disabled={!retentionConfirmChecked || retentionApplyStatus === 'saving'}
      onclick={applyRetentionFromDialog}
    >
      {retentionApplyStatus === 'saving' ? 'Applying…' : 'Confirm and apply'}
    </Button>
  </div>
</dialog>
