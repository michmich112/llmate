<script lang="ts">
  import { onMount } from 'svelte';
  import { api } from '$lib/api/client';
  import type { DashboardStats, Provider, TimeSeriesPoint } from '$lib/types';
  import { Card, CardHeader, CardTitle, CardContent } from '$lib/components/ui/card';
  import { Button } from '$lib/components/ui/button';
  import { Chart, LineController, BarController, LineElement, BarElement, PointElement, LinearScale, TimeScale, CategoryScale, Tooltip, Legend, Filler } from 'chart.js';

  Chart.register(LineController, BarController, LineElement, BarElement, PointElement, LinearScale, TimeScale, CategoryScale, Tooltip, Legend, Filler);

  let stats = $state<DashboardStats | null>(null);
  let providers = $state<Provider[]>([]);
  let tsPoints = $state<TimeSeriesPoint[]>([]);
  let loading = $state(true);
  let error = $state<string | null>(null);
  let timeRange = $state('24h');

  // Chart metric selector - order: tokens, cost, requests
  type ChartMetric = 'tokens' | 'cost' | 'requests';
  let chartMetric = $state<ChartMetric>('tokens');

  // Chart canvas and instance
  let chartCanvas = $state<HTMLCanvasElement | null>(null);
  let chartInstance: Chart | null = null;
  let breakdownExpanded = $state(false);

  // Granularity follows the explicit rule: ≤48h → hourly, else daily
  function granularityFor(range: string): 'hour' | 'day' {
    if (range === '24h') return 'hour';
    return 'day';
  }

  async function fetchData() {
    loading = true;
    error = null;
    try {
      const granularity = granularityFor(timeRange);
      const [s, p, ts] = await Promise.all([
        api.getStats(timeRange),
        api.listProviders(),
        api.getTimeSeries(timeRange, granularity)
      ]);
      stats = s;
      providers = p;
      tsPoints = ts.points;
    } catch (e) {
      error = e instanceof Error ? e.message : 'Failed to load dashboard';
      stats = null;
      tsPoints = [];
    } finally {
      loading = false;
    }
  }

  function handleRefresh() {
    fetchData();
  }

  $effect(() => {
    timeRange; // track for reactivity
    fetchData();
  });

  // Rebuild chart whenever data or metric changes
  $effect(() => {
    if (!chartCanvas || tsPoints.length === 0) {
      chartInstance?.destroy();
      chartInstance = null;
      return;
    }

    // Access reactive values to establish dependencies
    const points = tsPoints;
    const metric = chartMetric;

    chartInstance?.destroy();

    const labels = points.map((p) => {
      // Hourly: show HH:mm; daily: show MMM D
      const d = new Date(p.bucket.includes('T') ? p.bucket + 'Z' : p.bucket + 'T00:00:00Z');
      if (p.bucket.includes('T')) {
        return d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
      }
      return d.toLocaleDateString([], { month: 'short', day: 'numeric' });
    });

    const getTooltipLabel = (ctx: any) => {
      const v = ctx.parsed.y ?? 0;
      if (metric === 'cost') return `$${v.toFixed(6)}`;
      return v.toLocaleString();
    };

    let datasets: any[];
    let yTickCallback: any;
    let chartType: 'line' | 'bar' = 'line';

    if (metric === 'requests') {
      datasets = [
        {
          label: 'Successful',
          data: points.map((p) => (p.success_count ?? 0)),
          backgroundColor: 'rgba(20, 184, 166, 0.7)',
          borderColor: 'rgb(20, 184, 166)',
          borderWidth: 1,
          stack: 'stack1'
        },
        {
          label: 'Errors',
          data: points.map((p) => (p.error_count ?? 0)),
          backgroundColor: 'rgba(239, 68, 68, 0.7)',
          borderColor: 'rgb(239, 68, 68)',
          borderWidth: 1,
          stack: 'stack1'
        }
      ];
      yTickCallback = (v: number) => v.toLocaleString();
      chartType = 'bar';
    } else if (metric === 'tokens') {
      datasets = [
        {
          label: 'Input Tokens',
          data: points.map((p) => p.input_tokens),
          borderColor: 'rgb(59, 130, 246)',
          backgroundColor: 'rgba(59, 130, 246, 0.1)',
          borderWidth: 2,
          pointRadius: points.length > 48 ? 0 : 3,
          tension: 0.3,
          fill: true
        },
        {
          label: 'Output Tokens',
          data: points.map((p) => p.completion_tokens),
          borderColor: 'rgb(239, 68, 68)',
          backgroundColor: 'rgba(239, 68, 68, 0.1)',
          borderWidth: 2,
          pointRadius: points.length > 48 ? 0 : 3,
          tension: 0.3,
          fill: true
        },
        {
          label: 'Cached Tokens',
          data: points.map((p) => p.cached_tokens),
          borderColor: 'rgb(156, 163, 175)',
          backgroundColor: 'rgba(156, 163, 175, 0.1)',
          borderWidth: 2,
          pointRadius: points.length > 48 ? 0 : 3,
          tension: 0.3,
          fill: true
        }
      ];
      yTickCallback = (v: number) => v.toLocaleString();
    } else {
      // cost
      datasets = [
        {
          label: 'Total Cost',
          data: points.map((p) => p.total_cost_usd),
          borderColor: 'rgb(245, 158, 11)',
          backgroundColor: 'rgba(245, 158, 11, 0.1)',
          borderWidth: 2,
          pointRadius: points.length > 48 ? 0 : 3,
          tension: 0.3,
          fill: true
        }
      ];
      yTickCallback = (v: number) => `$${Number(v).toFixed(4)}`;
    }

    chartInstance = new Chart(chartCanvas, {
      type: chartType === 'bar' ? 'bar' : 'line',
      data: {
        labels,
        datasets
      },
      options: {
        responsive: true,
        maintainAspectRatio: false,
        plugins: {
          legend: { 
            display: true, 
            position: 'bottom',
            labels: {
              usePointStyle: true,
              pointStyle: 'circle'
            }
          },
          tooltip: {
            callbacks: {
              label: getTooltipLabel
            }
          }
        },
        scales: {
          x: { 
            grid: { display: false },
            offset: chartType === 'bar'
          },
          y: {
            beginAtZero: true,
            stacked: chartType === 'bar',
            ticks: { callback: yTickCallback }
          }
        }
      }
    });
  });

  onMount(() => {
    return () => {
      chartInstance?.destroy();
    };
  });

  let activeProviders = $derived(providers.filter((p) => p.is_healthy).length);
  let errorRateDisplay = $derived(
    stats ? (stats.error_rate * 100).toFixed(1) + '%' : '—'
  );

  // Total cost across time series window
  let totalCostDisplay = $derived(
    tsPoints.length > 0
      ? '$' + tsPoints.reduce((s, p) => s + p.total_cost_usd, 0).toFixed(4)
      : '—'
  );
</script>

<div class="space-y-6">
  <div class="flex items-center justify-between">
    <h1 class="text-2xl font-bold tracking-tight">Dashboard</h1>
    <div class="flex items-center gap-2">
      {#each ['24h', '7d', '30d'] as range}
        <button
          type="button"
          onclick={() => (timeRange = range)}
          class="rounded-md px-3 py-1.5 text-sm font-medium transition-colors {timeRange === range
            ? 'bg-primary text-primary-foreground'
            : 'border border-input bg-background hover:bg-accent'}"
        >
          {range}
        </button>
      {/each}
      <div class="h-8 w-px bg-border"></div>
      <Button variant="outline" size="sm" onclick={handleRefresh} disabled={loading}>
        <svg class="h-4 w-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
          <path d="M3 12a9 9 0 0 1 9-9 9.75 9.75 0 0 1 6.74 2.74L21 8" />
          <path d="M21 3v5h-5" />
          <path d="M21 12a9 9 0 0 1-9 9 9.75 9.75 0 0 1-6.74-2.74L3 16" />
          <path d="M8 16H3v5" />
        </svg>
        Refresh
      </Button>
    </div>
  </div>

  {#if error}
    <div class="rounded-md border border-destructive/50 bg-destructive/10 px-4 py-3 text-sm text-destructive">
      {error}
    </div>
  {/if}

  {#if loading}
    <div class="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
      {#each [1, 2, 3, 4] as _}
        <div class="h-28 animate-pulse rounded-lg bg-muted"></div>
      {/each}
    </div>
  {:else if stats}
    <!-- Stat cards -->
    <div class="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
      <Card>
        <CardHeader>
          <CardTitle class="text-sm font-medium text-muted-foreground">Total Requests</CardTitle>
        </CardHeader>
        <CardContent>
          <p class="text-3xl font-bold">{stats.total_requests.toLocaleString()}</p>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle class="text-sm font-medium text-muted-foreground">Avg Latency</CardTitle>
        </CardHeader>
        <CardContent>
          <p class="text-3xl font-bold">{stats.avg_latency_ms.toFixed(0)}<span class="ml-1 text-lg font-normal text-muted-foreground">ms</span></p>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle class="text-sm font-medium text-muted-foreground">Error Rate</CardTitle>
        </CardHeader>
        <CardContent>
          <p class="text-3xl font-bold">{errorRateDisplay}</p>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle class="text-sm font-medium text-muted-foreground">Est. Total Cost</CardTitle>
        </CardHeader>
        <CardContent>
          <p class="text-3xl font-bold tabular-nums">{totalCostDisplay}</p>
          <p class="mt-1 text-xs text-muted-foreground">{activeProviders} healthy provider{activeProviders !== 1 ? 's' : ''}</p>
        </CardContent>
      </Card>
    </div>

    <!-- Time series chart -->
    <Card>
      <CardHeader>
        <div class="flex items-center justify-between">
          <CardTitle>Usage Over Time</CardTitle>
          <div class="flex gap-1">
            {#each [['tokens', 'Tokens'], ['cost', 'Cost'], ['requests', 'Requests']] as [key, label]}
              <button
                type="button"
                onclick={() => (chartMetric = key as ChartMetric)}
                class="rounded px-2.5 py-1 text-xs font-medium transition-colors {chartMetric === key
                  ? 'bg-primary text-primary-foreground'
                  : 'border border-input bg-background hover:bg-accent'}"
              >
                {label}
              </button>
            {/each}
          </div>
        </div>
      </CardHeader>
      <CardContent class="pt-4">
        {#if tsPoints.length === 0}
          <p class="py-8 text-center text-sm text-muted-foreground">No data for the selected window.</p>
        {:else}
          <div class="h-56">
            <canvas bind:this={chartCanvas}></canvas>
          </div>
        {/if}
        
        <!-- Breakdown Section (expandable) -->
        {#if breakdownExpanded && tsPoints.length > 0}
          <div class="mt-4 border-t pt-4">
            <!-- Tokens Row -->
            <div class="grid grid-cols-2 gap-4 sm:grid-cols-4">
              <div class="border-b border-r border-muted p-3">
                <p class="text-xs text-muted-foreground">Total Tokens</p>
                <p class="text-xl font-bold">
                  {tsPoints.reduce((s, p) => s + p.total_tokens, 0).toLocaleString()}
                </p>
              </div>
              <div class="border-b border-r border-muted p-3">
                <p class="text-xs text-muted-foreground">Input Tokens</p>
                <p class="text-xl font-bold">
                  {tsPoints.reduce((s, p) => s + p.input_tokens, 0).toLocaleString()}
                </p>
              </div>
              <div class="border-b border-r border-muted p-3">
                <p class="text-xs text-muted-foreground">Output Tokens</p>
                <p class="text-xl font-bold">
                  {tsPoints.reduce((s, p) => s + p.completion_tokens, 0).toLocaleString()}
                </p>
              </div>
              <div class="border-b p-3">
                <p class="text-xs text-muted-foreground">Cached Tokens</p>
                <p class="text-xl font-bold">
                  {tsPoints.reduce((s, p) => s + p.cached_tokens, 0).toLocaleString()}
                </p>
              </div>
            </div>
            <!-- Costs Row -->
            <div class="grid grid-cols-2 gap-4 sm:grid-cols-4 mt-2">
              <div class="border-b border-r border-muted p-3">
                <p class="text-xs text-muted-foreground">Total Cost</p>
                <p class="text-xl font-bold">
                  ${tsPoints.reduce((s, p) => s + p.total_cost_usd, 0).toFixed(4)}
                </p>
              </div>
              <div class="border-b border-r border-muted p-3">
                <p class="text-xs text-muted-foreground">Input Cost</p>
                <p class="text-xl font-bold">
                  ${tsPoints.reduce((s, p) => s + p.input_cost_usd, 0).toFixed(4)}
                </p>
              </div>
              <div class="border-b border-r border-muted p-3">
                <p class="text-xs text-muted-foreground">Output Cost</p>
                <p class="text-xl font-bold">
                  ${tsPoints.reduce((s, p) => s + p.output_cost_usd, 0).toFixed(4)}
                </p>
              </div>
              <div class="border-b p-3">
                <p class="text-xs text-muted-foreground">Cached Cost</p>
                <p class="text-xl font-bold">
                  ${tsPoints.reduce((s, p) => s + p.cached_cost_usd, 0).toFixed(4)}
                </p>
              </div>
            </div>
          </div>
        {/if}
        
        <!-- Expand/Collapse Toggle -->
        {#if tsPoints.length > 0}
          <div class="mt-3 text-center">
            <button
              type="button"
              onclick={() => (breakdownExpanded = !breakdownExpanded)}
              class="text-primary hover:underline"
            >
              <svg class="h-4 w-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                {#if breakdownExpanded}
                  <path d="m18 15-6-6-6 6" />
                {:else}
                  <path d="m6 9 6 6 6-6" />
                {/if}
              </svg>
            </button>
          </div>
        {/if}
      </CardContent>
    </Card>

    <!-- Tables side by side -->
    <div class="grid grid-cols-1 gap-6 lg:grid-cols-2">
      <Card>
        <CardHeader>
          <CardTitle>Requests by Model</CardTitle>
        </CardHeader>
        <CardContent class="p-0">
          {#if stats.by_model.length === 0}
            <p class="px-6 py-4 text-sm text-muted-foreground">No data yet.</p>
          {:else}
            <div class="overflow-x-auto">
              <table class="w-full text-sm">
                <thead>
                  <tr class="border-b bg-muted/50 text-left text-xs font-medium uppercase tracking-wide text-muted-foreground">
                    <th class="px-4 py-3">Model</th>
                    <th class="px-4 py-3 text-right">Requests</th>
                    <th class="px-4 py-3 text-right">Avg Latency</th>
                    <th class="px-4 py-3 text-right">Errors</th>
                    <th class="px-4 py-3 text-right">Tokens</th>
                  </tr>
                </thead>
                <tbody>
                  {#each stats.by_model as row}
                    <tr class="border-b last:border-0 hover:bg-muted/30">
                      <td class="max-w-[160px] px-4 py-3 font-medium" title={row.model}>
                        <div class="truncate">{row.model}</div>
                      </td>
                      <td class="px-4 py-3 text-right">{row.request_count.toLocaleString()}</td>
                      <td class="px-4 py-3 text-right">{row.avg_latency_ms.toFixed(0)}ms</td>
                      <td class="px-4 py-3 text-right">{row.error_count}</td>
                      <td class="px-4 py-3 text-right">{row.total_tokens.toLocaleString()}</td>
                    </tr>
                  {/each}
                </tbody>
              </table>
            </div>
          {/if}
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Requests by Provider</CardTitle>
        </CardHeader>
        <CardContent class="p-0">
          {#if stats.by_provider.length === 0}
            <p class="px-6 py-4 text-sm text-muted-foreground">No data yet.</p>
          {:else}
            <table class="w-full text-sm">
              <thead>
                <tr class="border-b bg-muted/50 text-left text-xs font-medium uppercase tracking-wide text-muted-foreground">
                  <th class="px-4 py-3">Provider</th>
                  <th class="px-4 py-3 text-right">Requests</th>
                  <th class="px-4 py-3 text-right">Avg Latency</th>
                  <th class="px-4 py-3 text-right">Errors</th>
                </tr>
              </thead>
              <tbody>
                {#each stats.by_provider as row}
                  <tr class="border-b last:border-0 hover:bg-muted/30">
                    <td class="max-w-[180px] truncate px-4 py-3 font-medium" title={row.provider_name}>{row.provider_name}</td>
                    <td class="px-4 py-3 text-right">{row.request_count.toLocaleString()}</td>
                    <td class="px-4 py-3 text-right">{row.avg_latency_ms.toFixed(0)}ms</td>
                    <td class="px-4 py-3 text-right">{row.error_count}</td>
                  </tr>
                {/each}
              </tbody>
            </table>
          {/if}
        </CardContent>
      </Card>
    </div>
  {:else if !loading && !error}
    <p class="text-sm text-muted-foreground">No data available.</p>
  {/if}
</div>
