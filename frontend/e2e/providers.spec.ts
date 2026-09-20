import { test, expect } from '@playwright/test';
import { authInit, api, shot } from './helpers';

test('provider rename end-to-end', async ({ page, context, } , testInfo) => {
  await authInit(page);

  const prov = await api<any>(context.request, '/admin/providers', 'POST', {
    name: 'E2E Provider',
    base_url: 'http://127.0.0.1:8000/v1',
  });
  const pid = prov.provider.id;

  await page.goto('/providers');
  await expect(page.getByText('E2E Provider').first()).toBeVisible();
  await shot(page, testInfo, '06-providers-list');

  // Click the Edit link on the row -> detail page.
  const row = page.locator('tr').filter({ hasText: 'E2E Provider' });
  await row.getByText('Edit').click();

  await page.waitForURL('**/providers/' + pid);
  await expect(page.locator('#edit-name')).toBeVisible();
  await shot(page, testInfo, '07-provider-detail');

  await page.locator('#edit-name').fill('Renamed Provider');
  await page.getByRole('button', { name: 'Save Changes' }).click();

  // Heading reflects the updated name.
  await expect(page.locator('h1').first()).toContainText('Renamed Provider');
  await shot(page, testInfo, '08-provider-renamed');

  // Verify persisted via API.
  const detail = await api<any>(context.request, `/admin/providers/${pid}`, 'GET');
  expect(detail.provider.name).toBe('Renamed Provider');
});
