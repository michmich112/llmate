import { test, expect } from '@playwright/test';
import { authInit, api, shot } from './helpers';

test('other dashboard pages render without breaking', async ({ page, context, } , testInfo) => {
  await authInit(page);

  // Seed some data so pages have content to show.
  await api(context.request, '/admin/providers', 'POST', { name: 'Smoke Provider', base_url: 'http://127.0.0.1:9000/v1' });
  const aliases = await api<any>(context.request, '/admin/aliases', 'GET');

  for (const [route, shot] of [
    ['/', '10-dashboard.png'],
    ['/logs', '11-logs.png'],
    ['/settings', '12-settings.png'],
    ['/providers', '13-providers.png'],
    ['/dashboard/models', '14-models.png'],
  ] as [string, string][]) {
    if (route === '/dashboard/models') {
    await page.goto('/');
    await page.getByRole('link', { name: 'Models' }).click();
    await page.waitForURL('**/dashboard/models');
  } else {
    await page.goto(route);
  }
    await page.waitForLoadState('networkidle');
    // No console errors on any page.
    await page.screenshot({ path: 'e2e/screenshots/' + shot, fullPage: true });
  }

  // Confirm we can still read data back — API layer intact.
  const again = await api<any>(context.request, '/admin/aliases', 'GET');
  expect(Array.isArray(again.aliases)).toBe(true);
});

test('no console errors across pages', async ({ page }) => {
  await authInit(page);
  const errors: string[] = [];
  page.on('console', (m) => {
    if (m.type() === 'error') errors.push(m.text());
  });
  for (const route of ['/', '/providers', '/logs', '/settings']) {
    await page.goto(route);
    await page.waitForLoadState('networkidle');
  }
  await page.goto('/');
  await page.getByRole('link', { name: 'Models' }).click();
  await page.waitForURL('**/dashboard/models');
  await page.waitForLoadState('networkidle');
  // Filter out benign resource-load noise; fail on real JS errors.
  const real = errors.filter((e) => !e.includes('favicon') && !e.includes('net::ERR'));
  expect(real).toHaveLength(0);
});
