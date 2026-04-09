<script lang="ts">
  import { goto } from '$app/navigation';
  import { api } from '$lib/api/client';
  import { Button } from '$lib/components/ui/button';
  import { Input } from '$lib/components/ui/input';
  import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '$lib/components/ui/card';

  let accessKey = $state('');
  let error = $state('');
  let loading = $state(false);

  async function handleLogin(e: Event) {
    e.preventDefault();
    loading = true;
    error = '';
    try {
      const key = accessKey.trim();
      if (!key) {
        error = 'Enter an access key';
        return;
      }
      const ok = await api.validateKey(key);
      if (!ok) {
        error = 'Invalid access key';
        return;
      }
      api.setAccessKey(key);
      await goto('/');
    } finally {
      loading = false;
    }
  }
</script>

<div class="flex min-h-screen items-center justify-center bg-background px-4">
  <Card class="w-full max-w-md">
    <CardHeader class="space-y-1">
      <CardTitle class="text-2xl font-bold">LLMate</CardTitle>
      <CardDescription>LLM available to everyone. Enter your access key to continue.</CardDescription>
    </CardHeader>
    <CardContent>
      <form onsubmit={handleLogin} class="space-y-4">
        <div class="space-y-2">
          <label for="access-key" class="text-sm font-medium leading-none">Access Key</label>
          <Input
            id="access-key"
            type="password"
            placeholder="sk-..."
            bind:value={accessKey}
            disabled={loading}
            autocomplete="current-password"
          />
        </div>
        {#if error}
          <p class="text-sm text-red-600">{error}</p>
        {/if}
        <Button type="submit" class="w-full" disabled={loading}>
          {loading ? 'Signing in…' : 'Sign in'}
        </Button>
      </form>
    </CardContent>
  </Card>
</div>
