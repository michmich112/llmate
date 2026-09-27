import { When, Then } from '@cucumber/cucumber';
import { expect } from '@playwright/test';

When('I open the alias dialog', async function () {
  await this.page.getByRole('button', { name: 'Add Alias', exact: true }).click();
});

When('I fill the alias input with {string}', async function (alias) {
  await this.page.locator('#form-alias').fill(alias);
});

When('I select the provider {string}', async function (name) {
  await this.page.locator('#form-provider').selectOption({ label: name });
});

When('I wait for model {string} options', async function (model) {
  await this.page.locator(`#form-model option[value="${model}"]`).waitFor({ state: 'attached' });
});

When('I select model {string}', async function (model) {
  await this.page.locator('#form-model').selectOption(model);
});

Then('I see the alias {string} in the table', async function (alias) {
  await expect(this.page.locator('tr').filter({ hasText: alias }).first()).toBeVisible();
});

const API = 'http://127.0.0.1:8099/admin';

Then('I see the {string} button', async function (label) {
  await expect(this.page.getByRole('button', { name: label, exact: true })).toBeVisible();
});

When('I edit the alias {string} to name {string} and model {string}', async function (oldAlias, newAlias, model) {
  const row = this.page.locator('tr').filter({ hasText: oldAlias }).first();
  await row.getByRole('button', { name: 'Edit', exact: true }).click();
  await this.page.locator('#form-alias').fill(newAlias);
  await this.page.locator('#form-model').selectOption({ label: model });
  await this.page.getByRole('button', { name: 'Save Changes', exact: true }).click();
});

Then('the alias {string} points to model {string}', async function (alias, model) {
  const row = this.page.locator('tr').filter({ hasText: alias }).first();
  await expect(row).toBeVisible();
  await expect(row).toContainText(model);
});

Then('the alias {string} is persisted with model {string}', async function (alias, model) {
  const res = await this.page.request.fetch(`${API}/aliases`, {
    method: 'GET',
    headers: { Authorization: `Bearer ${this.authKey}` },
  });
  expect(res.status()).toBe(200);
  const body = await res.json();
  const found = body.aliases.find((a) => a.alias === alias);
  expect(found).toBeTruthy();
  expect(found.model_id).toBe(model);
});

When('I delete the alias {string}', async function (alias) {
  const row = this.page.locator('tr').filter({ hasText: alias }).first();
  await row.getByRole('button', { name: 'Delete', exact: true }).click();
  await this.page.getByRole('dialog').getByRole('button', { name: 'Delete', exact: true }).click();
});

Then('no alias named {string} exists', async function (alias) {
  await expect(this.page.locator('tr').filter({ hasText: alias })).toHaveCount(0);
  const res = await this.page.request.fetch(`${API}/aliases`, {
    method: 'GET',
    headers: { Authorization: `Bearer ${this.authKey}` },
  });
  const body = await res.json();
  expect(body.aliases.find((a) => a.alias === alias)).toBeFalsy();
});
