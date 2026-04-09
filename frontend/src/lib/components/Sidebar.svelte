<script lang="ts">
  import { goto } from '$app/navigation';
  import { page } from '$app/state';
  import { api } from '$lib/api/client';
  import { cn } from '$lib/utils';

  const navItems = [
    { label: 'Dashboard', path: '/' },
    { label: 'Providers', path: '/providers' },
    { label: 'Models', path: '/models' },
    { label: 'Logs', path: '/logs' },
    { label: 'Settings', path: '/settings' }
  ] as const;

  function isActive(path: string): boolean {
    if (path === '/') {
      return page.url.pathname === '/';
    }
    return page.url.pathname.startsWith(path);
  }

  async function handleLogout() {
    api.clearAccessKey();
    await goto('/login');
  }
</script>

<nav class="flex h-full flex-col px-3 py-4">
  <div class="mb-6 px-2">
    <span class="text-lg font-semibold">LLMate</span>
  </div>

  <ul class="flex-1 space-y-1">
    {#each navItems as item}
      <li>
        <a
          href={item.path}
          class={cn(
            'flex items-center rounded-md px-3 py-2 text-sm font-medium transition-colors',
            isActive(item.path)
              ? 'bg-accent text-accent-foreground'
              : 'text-muted-foreground hover:bg-accent hover:text-accent-foreground'
          )}
        >
          {item.label}
        </a>
      </li>
    {/each}
  </ul>

  <div class="mt-auto pt-4 border-t">
    <button
      onclick={handleLogout}
      class="flex w-full items-center rounded-md px-3 py-2 text-sm font-medium text-muted-foreground transition-colors hover:bg-accent hover:text-accent-foreground"
    >
      Log out
    </button>
  </div>
</nav>
