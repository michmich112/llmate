import { When, Then } from '@cucumber/cucumber';
import { expect } from '@playwright/test';

When('I switch to Lifetime mode', async function () {
  await this.page.getByRole('button', { name: 'Lifetime', exact: true }).click();
});

Then('the lifetime cost card shows a non-zero dollar amount', async function () {
  await expect(this.page.getByText('Est. Total Cost')).toBeVisible();
  const costCard = this.page.locator('p.text-3xl.font-bold.tabular-nums');
  await expect(costCard).toBeVisible();
  const text = (await costCard.innerText()).trim();
  expect(text.startsWith('$')).toBe(true);
  expect(text).not.toBe('$0.0000');
});
