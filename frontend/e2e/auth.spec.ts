import { test, expect } from '@playwright/test';
import { shot } from './helpers';

// The dashboard Models page lives at /dashboard/models (not /models) so it no
// longer collides with the OpenAI-compatible GET /models API endpoint.
const PROTECTED_ROUTES = ['/', '/providers', '/logs', '/settings', '/dashboard/models'];

test('unauthenticated access to dashboard routes redirects to /login', async ({ page, } , testInfo) => {
  // Fresh context: no access_key in localStorage.
  for (const route of PROTECTED_ROUTES) {
    await page.goto(route);
    await page.waitForURL('**/login');
    expect(page.url()).toContain('/login');
  }
  await shot(page, testInfo, 'auth-redirect-login');
});

test('login with an invalid access key shows an error and stays on /login', async ({ page, } , testInfo) => {
  await page.goto('/login');
  await page.locator('#access-key').fill('wrong-key');
  await page.getByRole('button', { name: /Sign in/i }).click();

  await expect(page.getByText('Invalid access key')).toBeVisible();
  await expect(page.url()).toContain('/login');
  await shot(page, testInfo, 'auth-invalid-key');
});

test('login with a valid access key succeeds and lands on the dashboard', async ({ page, } , testInfo) => {
  await page.goto('/login');
  await page.locator('#access-key').fill('e2e-key');
  await page.getByRole('button', { name: /Sign in/i }).click();

  await page.waitForURL('**/');
  await expect(page.locator('h1')).toContainText('Dashboard');
  await shot(page, testInfo, 'auth-login-success');
});
