import { test, expect } from '@playwright/test';
import { authInit, api, shot } from './helpers';

test('model alias: create, edit, delete end-to-end', async ({ page, context, } , testInfo) => {
  await authInit(page);

  // Seed a provider with two models via the real admin API.
  const prov = await api<any>(context.request, '/admin/providers', 'POST', {
    name: 'E2E Ollama',
    base_url: 'http://127.0.0.1:11434',
  });
  const pid = prov.provider.id;
  await api(context.request, `/admin/providers/${pid}/models`, 'POST', { model_id: 'llama3' });
  await api(context.request, `/admin/providers/${pid}/models`, 'POST', { model_id: 'claude-3' });

  await page.goto('/');
  await page.getByRole('link', { name: 'Models' }).click();
  await page.waitForURL('**/dashboard/models');
  await expect(page.getByRole('button', { name: 'Add Alias' })).toBeVisible();
  await shot(page, testInfo, '01-models-initial');

  // ---- Add ----
  await page.getByRole('button', { name: 'Add Alias' }).click();
  await page.locator('#form-alias').fill('gpt-4');
  await page.locator('#form-provider').selectOption({ label: 'E2E Ollama' });
  await page.locator('#form-model option[value="claude-3"]').waitFor({ state: 'attached' });
  await page.locator('#form-model').selectOption('llama3');
  await shot(page, testInfo, '02-add-dialog');
  await page.getByRole('button', { name: 'Create Alias' }).click();
  await expect(page.locator('tr').filter({ hasText: 'gpt-4' })).toBeVisible();
  await shot(page, testInfo, '03-after-add');

  // ---- Edit (rename alias + switch model) ----
  await page.locator('tr').filter({ hasText: 'gpt-4' }).getByRole('button', { name: 'Edit' }).click();
  await page.locator('#form-alias').fill('claude');
  await page.locator('#form-provider').selectOption({ label: 'E2E Ollama' });
  await page.locator('#form-model option[value="claude-3"]').waitFor({ state: 'attached' });
  await page.locator('#form-model').selectOption('claude-3');
  await page.getByRole('button', { name: 'Save Changes' }).click();
  await expect(page.locator('tr').filter({ hasText: 'claude' })).toBeVisible();
  await expect(page.locator('tr').filter({ hasText: 'claude-3' })).toBeVisible();
  await shot(page, testInfo, '04-after-edit');

  // Verify persisted via API.
  const aliases = await api<any>(context.request, '/admin/aliases', 'GET');
  const edited = aliases.aliases.find((a: any) => a.alias === 'claude');
  expect(edited).toBeTruthy();
  expect(edited.model_id).toBe('claude-3');

  // ---- Delete ----
  await page.locator('tr').filter({ hasText: 'claude' }).getByRole('button', { name: 'Delete' }).click();
  await page.getByRole('dialog').getByRole('button', { name: 'Delete' }).click();
  await expect(page.locator('tr').filter({ hasText: 'claude' })).toHaveCount(0);
  await shot(page, testInfo, '05-after-delete');

  const after = await api<any>(context.request, '/admin/aliases', 'GET');
  expect(after.aliases.find((a: any) => a.alias === 'claude')).toBeFalsy();
});
