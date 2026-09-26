import { Given, When, Then } from '@cucumber/cucumber';
import { expect } from '@playwright/test';

Given('I am authenticated', async function () {
  await this.page.addInitScript(() => {
    localStorage.setItem('access_key', 'e2e-key');
  });
});

Given('I am not authenticated', async function () {
  // Fresh browser context per scenario has no access_key in localStorage.
});

Given('I am on the login page', async function () {
  await this.page.goto(`${this.baseURL}/login`);
  await this.page.waitForLoadState('networkidle');
});

When('I visit the route {string}', async function (route) {
  await this.page.goto(this.baseURL + route);
  await this.page.waitForLoadState('networkidle');
});

When('I visit the protected routes {string}', async function (routes) {
  for (const r of routes.split(',').map((s) => s.trim())) {
    await this.page.goto(this.baseURL + r);
    await this.page.waitForLoadState('networkidle');
  }
});

When('I fill the access key input with {string}', async function (key) {
  await this.page.locator('#access-key').fill(key);
});

When('I click the {string} button', async function (name) {
  await this.page.getByRole('button', { name, exact: true }).click();
});

Then('I am redirected to \\/login', async function () {
  expect(this.page.url()).toContain('/login');
});

Then('I remain on \\/login', async function () {
  expect(this.page.url()).toContain('/login');
});

Then('I see the text {string}', async function (text) {
  await expect(this.page.getByText(text)).toBeVisible();
});

Then('I land on the dashboard', async function () {
  await expect(this.page.locator('h1')).toContainText('Dashboard');
});

Then('I take a screenshot named {string}', async function (name) {
  await this.screenshot(name);
});

Then('no console errors were logged', async function () {
  expect(this.consoleErrors).toHaveLength(0);
});
