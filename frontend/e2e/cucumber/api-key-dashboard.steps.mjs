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

Then('I am redirected to \\/usage', async function () {
  expect(this.page.url()).toContain('/usage');
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
