import { test, expect } from '@playwright/test';
import { execSync } from 'node:child_process';
import { randomUUID } from 'node:crypto';
import { api, shot } from './helpers';

// The gateway runs against this SQLite file (see start-gateway.sh).
const DB = '/tmp/llmate-e2e.db';

/**
 * Seed a request_logs row directly (no live upstream needed) so the stats
 * cost computation has token counts to price against. Builds the INSERT
 * from the actual table columns so the test is robust to schema renames.
 */
function seedRequestLog(pid: string, model: string, prompt: number, completion: number, cached: number): void {
  const cols = execSync(`sqlite3 "${DB}" "PRAGMA table_info(request_logs);"`, { encoding: 'utf8' })
    .trim()
    .split('\n')
    .map((l) => l.split('|')[1])
    .filter(Boolean);

  const known: Record<string, string | number> = {
    id: randomUUID(),
    timestamp: new Date().toISOString(),
    client_ip: '',
    method: 'POST',
    path: '/v1/chat/completions',
    requested_model: 'pricing-alias',
    resolved_model: model,
    provider_id: pid,
    provider_name: 'Pricing E2E',
    status_code: 200,
    is_streamed: 0,
    prompt_tokens: prompt,
    completion_tokens: completion,
    total_tokens: prompt + completion + cached,
    cached_tokens: cached,
    total_time_ms: 0,
    created_at: new Date().toISOString(),
    estimated_cost_usd: 0,
  };
  const present = cols.filter((c) => c in known);
  const sql = `PRAGMA busy_timeout=5000; INSERT INTO request_logs (${present.join(',')})
    VALUES (${present.map((c) => `'${known[c]}'`).join(',')});`;
  execSync(`sqlite3 "${DB}" "${sql}"`, { stdio: 'ignore' });
}

test('provider cost rates flow into lifetime cost and dashboard', async ({ context, page, } , testInfo) => {
  // Login.
  await page.addInitScript(() => localStorage.setItem('access_key', 'e2e-key'));
  await page.goto('/');
  await page.waitForURL('**/');

  // Create a provider.
  const prov = await api(context.request, '/admin/providers', 'POST', {
    name: 'Pricing E2E',
    // Distinct base_url: providers.base_url is UNIQUE and smoke.spec already
    // uses http://127.0.0.1:9000/v1 against the same shared e2e database.
    base_url: 'http://127.0.0.1:9005/v1',
  });
  expect(prov.provider.id).toBeTruthy();
  const pid: string = prov.provider.id;

  // Register a model with cost rates.
  const models = await api(context.request, `/admin/providers/${pid}/models`, 'POST', {
    model_id: 'llama3',
    max_context: 8192,
  });
  const model = models.models.find((m: any) => m.model_id === 'llama3');
  expect(model).toBeTruthy();
  await api(context.request, `/admin/providers/${pid}/models/${model.id}`, 'PUT', {
    cost_per_million_input: 2.0,
    cost_per_million_output: 3.0,
    cost_per_million_cache_read: 0.5,
  });

  // Seed one request_log so cost is computable (1000 prompt, 500 completion, 200 cached tokens).
  seedRequestLog(pid, 'llama3', 1000, 500, 200);

  // Lifetime cost API must report non-zero total cost.
  const cost = await api(context.request, '/admin/stats/lifetime', 'GET');
  expect(cost.total_cost_usd).toBeGreaterThan(0);

  // Dashboard must render the lifetime cost (recomputed from tokens*rates),
  // not $0.0000. Switch to Lifetime time mode so the card uses the lifetime
  // total rather than the 24h usage buckets (which sum per-point costs).
  await page.reload();
  await page.getByRole('button', { name: 'Lifetime' }).click();
  await expect(page.getByText('Est. Total Cost')).toBeVisible();
  const costCard = page.locator('p.text-3xl.font-bold.tabular-nums');
  await expect(costCard).toContainText('$');
  await expect(costCard).not.toContainText('$0.0000');
  await shot(page, testInfo, 'pricing-dashboard');
});
