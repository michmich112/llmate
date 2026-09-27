import { When, Then } from '@cucumber/cucumber';
import { expect } from '@playwright/test';

const ROOT = 'http://127.0.0.1:8099';

Then('I land on the usage dashboard', async function () {
  await expect(this.page.locator('[data-testid="usage-dashboard"]')).toBeVisible();
});

Then('I see the {string} table', async function (heading) {
  await expect(this.page.getByText(heading)).toBeVisible();
});

Then('I do not see the sidebar nav items {string}', async function (labels) {
  for (const label of labels.split(',').map((s) => s.trim())) {
    await expect(this.page.locator('nav a', { hasText: label })).toHaveCount(0);
  }
});

When('I visit the admin-only route {string}', async function (route) {
  await this.page.goto(ROOT + route);
  await this.page.waitForLoadState('networkidle');
});

Then('I am redirected to the dashboard', async function () {
  await expect.poll(() => new URL(this.page.url()).pathname, { timeout: 10000 }).toBe('/');
});

Then('I do not see provider information', async function () {
  await expect(this.page.getByText('Requests by Model')).toBeVisible();
  await expect(this.page.locator('[data-testid="requests-by-provider"]')).toHaveCount(0);
  await expect(this.page.getByText('healthy provider')).toHaveCount(0);
  await expect(this.page.getByText('active right now')).toHaveCount(0);
});

When('I request my dashboard stats with the {string} API key', async function (name) {
  const rec = this.createdKeys.find((k) => k.name === name);
  expect(rec).toBeTruthy();
  const res = await this.page.request.fetch(`${ROOT}/admin/me/stats?since=24h`, {
    headers: { Authorization: `Bearer ${rec.key}` },
  });
  expect(res.status()).toBe(200);
  this.myStats = await res.json();
});

Then('the dashboard stats omit provider fields', async function () {
  expect(this.myStats).toBeTruthy();
  expect(this.myStats.by_provider).toBeUndefined();
  expect(this.myStats.active_requests).toBeUndefined();
  expect(Array.isArray(this.myStats.by_model)).toBe(true);
});

Then('the dashboard stats omit the API key breakdown', async function () {
  expect(this.myStats).toBeTruthy();
  expect(this.myStats.by_api_key).toBeUndefined();
});

Then('I do not see the {string} table', async function (heading) {
  await expect(this.page.getByText(heading, { exact: true })).toHaveCount(0);
});

When('I open the dashboard', async function () {
  await this.page.goto(`${ROOT}/`);
  await this.page.waitForLoadState('networkidle');
});

When('I select the {string} dashboard range', async function (label) {
  await this.page.getByRole('button', { name: label, exact: true }).click();
  await this.page.waitForLoadState('networkidle');
});

Then('the API key breakdown includes {string} with {int} requests', async function (name, count) {
  const card = this.page.locator('[data-testid="requests-by-api-key"]');
  const row = card.locator('tr').filter({ hasText: name });
  await expect(row).toBeVisible();
  await expect(row.getByText(String(count), { exact: true })).toBeVisible();
});

Then('I see provider information', async function () {
  await expect(this.page.locator('[data-testid="requests-by-provider"]')).toBeVisible();
});

When('I call the admin API endpoints {string} with the {string} API key', async function (endpoints, name) {
  const rec = this.createdKeys.find((k) => k.name === name);
  expect(rec).toBeTruthy();
  this.adminResponses = [];
  for (const ep of endpoints.split(',').map((s) => s.trim())) {
    const res = await this.page.request.fetch(ROOT + ep, {
      method: 'GET',
      headers: { Authorization: `Bearer ${rec.key}` },
    });
    this.adminResponses.push(res);
  }
});

Then('each admin endpoint returns {int} with {string}', async function (status, errMsg) {
  expect(this.adminResponses).toBeTruthy();
  for (const res of this.adminResponses) {
    expect(res.status()).toBe(status);
    const json = await res.json();
    expect(json.error).toContain(errMsg);
  }
});

When('I request my usage timeseries with the {string} API key', async function (name) {
  const rec = this.createdKeys.find((k) => k.name === name);
  expect(rec).toBeTruthy();
  const until = new Date();
  const since = new Date(until.getTime() - 48 * 60 * 60 * 1000);
  const res = await this.page.request.fetch(
    `${ROOT}/admin/me/usage/timeseries?since=${encodeURIComponent(since.toISOString())}&until=${encodeURIComponent(until.toISOString())}&granularity=hour`,
    { headers: { Authorization: `Bearer ${rec.key}` } }
  );
  expect(res.status()).toBe(200);
  this.myTimeSeries = await res.json();
});

Then('the timeseries contains {int} requests for the {string} API key', async function (count, name) {
  expect(this.myTimeSeries).toBeTruthy();
  let total = 0;
  for (const p of this.myTimeSeries.points) {
    total += p.requests ?? 0;
  }
  expect(total).toBeGreaterThanOrEqual(count);
});

Then('I see the Requests metric shows {int}', async function (expected) {
  const loc = this.page.locator('[data-testid="usage-metric-requests"]');
  await loc.waitFor({ state: 'visible', timeout: 10000 });
  const text = (await loc.innerText()).replace(/[^0-9]/g, '');
  expect(Number(text)).toBe(expected);
});

Then('the requests chart is rendered', async function () {
  const canvas = this.page.locator('[data-testid="usage-chart"]');
  await canvas.waitFor({ state: 'visible', timeout: 10000 });
  let drawn = false;
  for (let i = 0; i < 20; i++) {
    drawn = await canvas.evaluate((el) => {
      const ctx = el.getContext('2d');
      if (!ctx) return false;
      const data = ctx.getImageData(0, 0, el.width, el.height).data;
      let nonBlank = 0;
      for (let j = 0; j < data.length; j += 4) {
        if (data[j + 3] > 0) nonBlank++;
      }
      return nonBlank > 100;
    });
    if (drawn) break;
    await this.page.waitForTimeout(250);
  }
  expect(drawn).toBe(true);
});

When('I query the admin usage endpoint', async function () {
  const res = await this.page.request.fetch(`${ROOT}/admin/usage`, {
    method: 'GET',
    headers: { Authorization: `Bearer ${this.authKey}` },
  });
  expect(res.status()).toBe(200);
  this.adminUsage = await res.json();
});

Then('the admin usage endpoint includes the {string} API key with {int} requests', async function (name, requests) {
  expect(this.adminUsage).toBeTruthy();
  const rec = this.adminUsage.usage.find((u) => u.api_key_name === name);
  expect(rec).toBeTruthy();
  expect(rec.request_count).toBe(requests);
});
