<script lang="ts">
  import { cn } from '$lib/utils';

  interface Props {
    class?: string;
    /** Pixel size for width and height. Default 28. */
    size?: number;
    /** Show wordmark next to the mark. */
    withWordmark?: boolean;
  }

  let { class: className = '', size = 28, withWordmark = false }: Props = $props();
</script>

<span class={cn('inline-flex items-center gap-2', className)}>
  <img
    src="/llmate-mark.png"
    width={size}
    height={size}
    alt={withWordmark ? '' : 'LLMate'}
    class="shrink-0 rounded-[22%]"
    decoding="async"
  />
  {#if withWordmark}
    <span class="text-lg font-semibold tracking-tight">LLMate</span>
  {/if}
</span>
