<script lang="ts">
  import type { Snippet } from 'svelte';
  import Sidebar from '$lib/components/Sidebar.svelte';

  let { children }: { children: Snippet } = $props();
</script>

<div class="flex h-screen overflow-hidden">
  <aside class="w-64 shrink-0 border-r border-border bg-card">
    <Sidebar />
  </aside>
  <main class="flex-1 overflow-auto bg-background p-6">
    {@render children()}
  </main>
</div>
