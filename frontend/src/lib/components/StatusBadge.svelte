<script lang="ts">
  interface Props {
    status: 'healthy' | 'unhealthy' | 'unknown';
  }

  let { status }: Props = $props();

  const styles = {
    healthy: 'bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-300',
    unhealthy: 'bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-300',
    unknown: 'bg-gray-100 text-gray-600 dark:bg-gray-800 dark:text-gray-400'
  } as const;

  const labels = {
    healthy: 'Healthy',
    unhealthy: 'Unhealthy',
    unknown: 'Unknown'
  } as const;
</script>

<span class="inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-medium {styles[status]}">
  {labels[status]}
</span>
