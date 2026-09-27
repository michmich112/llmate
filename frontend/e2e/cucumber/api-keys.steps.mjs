import { After, Given, When, Then } from '@cucumber/cucumber';
import { expect } from '@playwright/test';

const ROOT = 'http://127.0.0.1:8099';
const ADMIN = `${ROOT}/admin`;
const GW = `${ROOT}/v1`;

async function adminHeaders(ctx) {
  return { Authorization: `Bearer ${ctx.authKey}` };
}

async function removeExistingKeyNamed(ctx, name) {
  const want = name.trim().toLowerCase();
  const keys = await listAdminKeys(ctx);
  const matches = keys.filter((k) => (k.name || '').trim().toLowerCase() === want);
  if (matches.length === 0) return;
  const { execSync } = await import('node:child_process');
  const DB = '/tmp/llmate-e2e.db';
  for (const key of matches) {
    const id = String(key.id).replace(/'/g, "''");
    execSync(`sqlite3 "${DB}" "DELETE FROM request_logs WHERE api_key_id = '${id}';"`);
    await deleteAdminKey(ctx, key.id);
  }
}

async function createAdminKey(ctx, name, opts = {}) {
  await removeExistingKeyNamed(ctx, name);
  const body = { name };
  if (opts.rpm !== undefined) body.rate_limit_rpm = opts.rpm;
  if (opts.tpm !== undefined) body.rate_limit_tpm = opts.tpm;
  const res = await ctx.page.request.fetch(`${ADMIN}/keys`, {
    method: 'POST',
    headers: await adminHeaders(ctx),
    data: body,
  });
  expect(res.status()).toBe(201);
  const json = await res.json();
  expect(json.api_key).toBeTruthy();
  expect(json.key).toBeTruthy();
  const rec = { id: json.api_key.id, name: json.api_key.name, key: json.key, active: json.api_key.is_active !== false };
  ctx.createdKeys = ctx.createdKeys || [];
  ctx.createdKeys.push(rec);
  ctx.lastKey = rec;
  return rec;
}

async function listAdminKeys(ctx) {
  const res = await ctx.page.request.fetch(`${ADMIN}/keys`, {
    method: 'GET',
    headers: await adminHeaders(ctx),
  });
  expect(res.status()).toBe(200);
  const json = await res.json();
  return json.keys || [];
}

async function updateAdminKey(ctx, id, patch) {
  const res = await ctx.page.request.fetch(`${ADMIN}/keys/${id}`, {
    method: 'PUT',
    headers: await adminHeaders(ctx),
    data: patch,
  });
  expect(res.status()).toBe(200);
  return res.json();
}

async function deleteAdminKey(ctx, id) {
  const res = await ctx.page.request.fetch(`${ADMIN}/keys/${id}`, {
    method: 'DELETE',
    headers: await adminHeaders(ctx),
  });
  expect(res.status()).toBe(204);
}

async function chatCompletion(ctx, { key, noAuth } = {}) {
  const headers = {};
  if (key !== undefined) headers['Authorization'] = `Bearer ${key}`;
  const res = await ctx.page.request.fetch(`${GW}/chat/completions`, {
    method: 'POST',
    headers,
    data: { model: 'test-model', messages: [{ role: 'user', content: 'hello' }] },
  });
  return res;
}

function findKey(keys, name) {
  return keys.find((k) => k.name === name);
}

async function setRequireAPIKeys(ctx, value) {
  ctx.requireAPIKeysTouched = true;
  const res = await ctx.page.request.fetch(`${ADMIN}/config`, {
    method: 'PUT',
    headers: { ...(await adminHeaders(ctx)), 'Content-Type': 'application/json' },
    data: { require_api_keys: value },
  });
  expect(res.status()).toBe(200);
  const json = await res.json();
  expect(json.require_api_keys).toBe(value);
}

async function getRequireAPIKeys(ctx) {
  const res = await ctx.page.request.fetch(`${ADMIN}/config`, {
    method: 'GET',
    headers: await adminHeaders(ctx),
  });
  expect(res.status()).toBe(200);
  const json = await res.json();
  return json.require_api_keys === true;
}

function requireSwitch(page) {
  return page.locator('label[role="switch"]:has(#require-api-keys)');
}

// Keep later scenarios on the default (keys not required) if one of these turns the setting on.
After(async function () {
  if (!this.requireAPIKeysTouched) return;
  await fetch(`${ADMIN}/config`, {
    method: 'PUT',
    headers: { Authorization: `Bearer ${this.authKey}`, 'Content-Type': 'application/json' },
    body: JSON.stringify({ require_api_keys: false }),
  }).catch(() => {});
});

// ---------------------------------------------------------------------------
// Scenario 1: admin manages API keys from the dashboard
// ---------------------------------------------------------------------------

Given('I log in to the dashboard as an admin', async function () {
  await this.page.goto(`${ROOT}/login`);
  await this.page.locator('#access-key').fill(this.authKey);
  await this.page.getByRole('button', { name: 'Sign in' }).click();
  await this.page.waitForURL('**/');
});

When('I open the API keys page', async function () {
  await this.page.goto(`${ROOT}/keys`);
  await this.page.waitForLoadState('networkidle');
});

When('I create an API key named {string} with RPM {int} and TPM {int}', async function (name, rpm, tpm) {
  await this.page.locator('#key-name').fill(name);
  await this.page.locator('#rpm').fill(String(rpm));
  await this.page.locator('#tpm').fill(String(tpm));
  await this.page.getByRole('button', { name: 'Create key' }).click();
  this.createdKeys = this.createdKeys || [];
  this.lastKey = { name };
});

Then('I see the raw API key shown once', async function () {
  await expect(this.page.getByText(/Key created/)).toBeVisible();
  const raw = await this.page.locator('code').first().innerText();
  expect(raw.length).toBeGreaterThan(20);
  this.lastKey.key = raw;
});

Then('the {string} API key appears in the keys table with RPM {int} and TPM {int}', async function (name, rpm, tpm) {
  const row = this.page.locator('tr').filter({ hasText: name });
  await expect(row).toBeVisible();
  await expect(row.getByText(String(rpm), { exact: true })).toBeVisible();
  await expect(row.getByText(String(tpm), { exact: true })).toBeVisible();
});

When('I deactivate the {string} API key', async function (name) {
  const row = this.page.locator('tr').filter({ hasText: name });
  await row.getByRole('button', { name: `Actions for ${name}` }).click();
  await this.page.getByRole('menuitem', { name: 'Update' }).click();
  const dialog = this.page.locator('#edit-key-dialog');
  await expect(dialog).toBeVisible();
  const active = dialog.locator('#edit-active');
  if (await active.isChecked()) {
    await dialog.locator('label[role="switch"]:has(#edit-active)').click();
  }
  await dialog.getByRole('button', { name: 'Save' }).click();
  await expect(dialog).toBeHidden();
});

Then('the {string} API key is inactive', async function (name) {
  const keys = await listAdminKeys(this);
  const rec = findKey(keys, name);
  expect(rec).toBeTruthy();
  expect(rec.is_active).toBe(false);
});

When('I delete the {string} API key', async function (name) {
  const row = this.page.locator('tr').filter({ hasText: name });
  await row.getByRole('button', { name: `Actions for ${name}` }).click();
  await this.page.getByRole('menuitem', { name: 'Delete' }).click();
});

When('I update the {string} API key to RPM {int} and TPM {int}', async function (name, rpm, tpm) {
  const row = this.page.locator('tr').filter({ hasText: name });
  await row.getByRole('button', { name: `Actions for ${name}` }).click();
  await this.page.getByRole('menuitem', { name: 'Update' }).click();
  const dialog = this.page.locator('#edit-key-dialog');
  await expect(dialog).toBeVisible();
  await dialog.locator('#edit-rpm').fill(String(rpm));
  await dialog.locator('#edit-tpm').fill(String(tpm));
  await dialog.getByRole('button', { name: 'Save' }).click();
  await expect(dialog).toBeHidden();
});

When('I try to create an API key named {string} with RPM {string} and TPM {string}', async function (name, rpm, tpm) {
  await this.page.locator('#key-name').fill(name);
  await this.page.locator('#rpm').fill(String(rpm));
  await this.page.locator('#tpm').fill(String(tpm));
  await this.page.getByRole('button', { name: 'Create key' }).click();
});

Then('the keys table lists {string} once', async function (name) {
  await expect(this.page.locator('tbody tr').filter({ hasText: name })).toHaveCount(1);
});

Then('no API key named {string} appears in the keys table', async function (name) {
  await expect(this.page.locator('tr').filter({ hasText: name })).toHaveCount(0);
  const keys = await listAdminKeys(this);
  expect(findKey(keys, name)).toBeFalsy();
});

// ---------------------------------------------------------------------------
// Scenario 2: gateway rejects missing / invalid API keys
// ---------------------------------------------------------------------------

Given('I create an active API key via the admin API named {string}', async function (name) {
  await createAdminKey(this, name);
});

When('I send a chat completion request with no API key', async function () {
  this.lastGw = await chatCompletion(this, { noAuth: true });
});

When('I send a chat completion request with an invalid API key', async function () {
  this.lastGw = await chatCompletion(this, { key: 'sk-invalid-key' });
});

When('I create a second API key named {string} and deactivate it', async function (name) {
  await createAdminKey(this, name);
  const rec = this.createdKeys[this.createdKeys.length - 1];
  await updateAdminKey(this, rec.id, { name: rec.name, is_active: false });
});

When('I send a chat completion request with the deactivated {string} API key', async function (name) {
  const rec = this.createdKeys.find((k) => k.name === name);
  expect(rec).toBeTruthy();
  this.lastGw = await chatCompletion(this, { key: rec.key });
});

When('I send a chat completion request with the valid {string} API key', async function (name) {
  const rec = this.createdKeys.find((k) => k.name === name);
  expect(rec).toBeTruthy();
  this.lastGw = await chatCompletion(this, { key: rec.key });
});

Then('the gateway returns {int} with {string}', async function (status, errMsg) {
  expect(this.lastGw.status()).toBe(status);
  const json = await this.lastGw.json();
  expect(json.error).toContain(errMsg);
});

Then('the gateway grants the request', async function () {
  expect(this.lastGw.status()).not.toBe(401);
  const json = await this.lastGw.json();
  expect(json.error).toContain('no available provider');
});

Given('the admin requires API keys', async function () {
  await setRequireAPIKeys(this, true);
});

Given('the admin does not require API keys', async function () {
  await setRequireAPIKeys(this, false);
});

When('I delete every API key via the admin API', async function () {
  const keys = await listAdminKeys(this);
  for (const key of keys) {
    await deleteAdminKey(this, key.id);
  }
  const remaining = await listAdminKeys(this);
  expect(remaining).toHaveLength(0);
});

async function setRequireFromDashboard(ctx, want) {
  ctx.requireAPIKeysTouched = true;
  const input = ctx.page.locator('#require-api-keys');
  await expect(input).toBeEnabled();
  const toggle = requireSwitch(ctx.page);
  await expect(toggle).toBeVisible();
  const checked = (await toggle.getAttribute('aria-checked')) === 'true';
  if (checked === want) return;
  const pending = ctx.page.waitForResponse(
    (res) => res.url().includes('/admin/config') && res.request().method() === 'PUT'
  );
  await toggle.click();
  const res = await pending;
  expect(res.status()).toBe(200);
  await expect(toggle).toHaveAttribute('aria-checked', want ? 'true' : 'false');
}

When('I turn on requiring API keys', async function () {
  await setRequireFromDashboard(this, true);
});

When('I turn off requiring API keys', async function () {
  await setRequireFromDashboard(this, false);
});

Then('API keys are required for gateway requests', async function () {
  expect(await getRequireAPIKeys(this)).toBe(true);
  await expect(requireSwitch(this.page)).toHaveAttribute('aria-checked', 'true');
});

Then('API keys are not required for gateway requests', async function () {
  expect(await getRequireAPIKeys(this)).toBe(false);
  await expect(requireSwitch(this.page)).toHaveAttribute('aria-checked', 'false');
});

// ---------------------------------------------------------------------------
// Scenario 3: RPM rate limits
// ---------------------------------------------------------------------------

Given('I create an API key via the admin API with RPM {int}', async function (rpm) {
  await createAdminKey(this, `rate-key-${Date.now()}`, { rpm });
});

When('I send {int} chat completion requests with that API key', async function (count) {
  this.gwResponses = [];
  for (let i = 0; i < count; i++) {
    this.gwResponses.push(await chatCompletion(this, { key: this.lastKey.key }));
  }
});

Then('the gateway rate-limits the {int}rd request with {int} {string}', async function (n, status, errMsg) {
  const res = this.gwResponses[n - 1];
  expect(res.status()).toBe(status);
  const json = await res.json();
  expect(json.error).toContain(errMsg);
  // earlier requests passed auth (not 401)
  for (let i = 0; i < n - 1; i++) {
    expect(this.gwResponses[i].status()).not.toBe(401);
  }
});

// ---------------------------------------------------------------------------
// Scenario 4: dashboard access using an API key
// ---------------------------------------------------------------------------

Given('I create an active API key via the admin API named {string} for dashboard access', async function (name) {
  await createAdminKey(this, name);
});

When('I log in to the dashboard using the {string} API key', async function (name) {
  const rec = this.createdKeys.find((k) => k.name === name);
  expect(rec).toBeTruthy();
  await this.page.goto(`${ROOT}/login`);
  await this.page.locator('#access-key').fill(rec.key);
  await this.page.getByRole('button', { name: 'Sign in' }).click();
  // The dashboard is a SvelteKit SPA: wait for the async login/route/data to
  // settle so subsequent steps and screenshots see the rendered dashboard.
  await this.page.waitForLoadState('networkidle').catch(() => {});
  await this.page.waitForTimeout(250);
});

Then('the dashboard is accessible with the {string} API key', async function (name) {
  const rec = this.createdKeys.find((k) => k.name === name);
  const res = await this.page.request.fetch(`${ADMIN}/auth`, {
    method: 'POST',
    headers: { Authorization: `Bearer ${rec.key}` },
  });
  expect(res.status()).toBe(200);
  const json = await res.json();
  expect(json.valid).toBe(true);
});
