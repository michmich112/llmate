import { Given } from '@cucumber/cucumber';
import { expect } from '@playwright/test';

// The admin API is mounted at /admin (see cmd/gateway/main.go).
const API = 'http://127.0.0.1:8099/admin';

Given('I create a provider named {string} with base URL {string}', async function (name, url) {
  const res = await this.page.request.fetch(`${API}/providers`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${this.authKey}` },
    data: { name, base_url: url },
  });
  expect(res.status()).toBe(201);
  const body = await res.json();
  expect(body.provider.id).toBeTruthy();
  this.providerId = body.provider.id;
});

Given('I register model {string} on that provider', async function (model) {
  const res = await this.page.request.fetch(`${API}/providers/${this.providerId}/models`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${this.authKey}` },
    data: { model_id: model },
  });
  expect(res.status()).toBe(201);
  const body = await res.json();
  const rec = body.models.find((m) => m.model_id === model);
  expect(rec).toBeTruthy();
  // The cost-rate PUT needs the provider_models record id, not the model_id string.
  this.modelRecordId = rec.id;
});

Given('I set cost rates input {float} output {float} cache-read {float} on that model', async function (input, output, cache) {
  const res = await this.page.request.fetch(`${API}/providers/${this.providerId}/models/${this.modelRecordId}`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${this.authKey}` },
    data: {
      cost_per_million_input: input,
      cost_per_million_output: output,
      cost_per_million_cache_read: cache,
    },
  });
  expect(res.status()).toBe(200);
});

// Mirrors pricing.spec.ts: seed a request_logs row directly so the lifetime
// cost card shows a non-zero dollar amount.
Given('I seed request logs with {int} prompt {int} completion {int} cached tokens', async function (prompt, completion, cached) {
  const { execSync } = await import('node:child_process');
  const crypto = await import('node:crypto');
  const DB = '/tmp/llmate-e2e.db';
  const cols = execSync(`sqlite3 "${DB}" "PRAGMA table_info(request_logs);"`)
    .toString().trim().split('\n').map((l) => l.split('|')[1]).filter(Boolean);
  const known = {
    id: crypto.randomUUID(),
    timestamp: new Date().toISOString(),
    created_at: new Date().toISOString(),
    client_ip: '',
    method: 'POST',
    path: '/v1/chat/completions',
    requested_model: 'llama3',
    resolved_model: 'llama3',
    provider_id: this.providerId,
    provider_name: 'Pricing E2E',
    status_code: 200,
    is_streamed: 0,
    prompt_tokens: prompt,
    completion_tokens: completion,
    total_tokens: prompt + completion + cached,
    cached_tokens: cached,
    total_time_ms: 0,
    estimated_cost_usd: 0,
  };
  const colsToUse = cols.filter((c) => c in known);
  const values = colsToUse.map((c) => {
    const v = known[c];
    return typeof v === 'string' ? `'${String(v).replace(/'/g, "''")}'` : v;
  }).join(',');
  execSync(`sqlite3 "${DB}" "INSERT INTO request_logs (${colsToUse.join(',')}) VALUES (${values});"`);
});
