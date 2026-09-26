import { When, Then } from '@cucumber/cucumber';
import { expect } from '@playwright/test';

Then('I see the provider {string} in the table', async function (name) {
  await expect(this.page.getByText(name).first()).toBeVisible();
});

const API = 'http://127.0.0.1:8099/admin';

When('I open the edit page for {string}', async function (name) {
  const row = this.page.locator('tr').filter({ hasText: name }).first();
  await row.getByRole('link', { name: 'Edit', exact: true }).click();
  await this.page.waitForURL('**/providers/' + this.providerId);
});

Then('I see the name input on the provider detail page', async function () {
  await expect(this.page.locator('#edit-name')).toBeVisible();
});

When('I rename the provider to {string}', async function (name) {
  await this.page.locator('#edit-name').fill(name);
  await this.page.getByRole('button', { name: 'Save Changes', exact: true }).click();
});

Then('the provider heading shows {string}', async function (name) {
  await expect(this.page.locator('h1')).toContainText(name);
});

Then('the provider name persisted is {string}', async function (name) {
  const res = await this.page.request.fetch(`${API}/providers/${this.providerId}`, {
    method: 'GET',
    headers: { Authorization: `Bearer ${this.authKey}` },
  });
  expect(res.status()).toBe(200);
  const body = await res.json();
  expect(body.provider.name).toBe(name);
});
