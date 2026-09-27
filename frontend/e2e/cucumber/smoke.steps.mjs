import { When, Then } from '@cucumber/cucumber';
import { expect } from '@playwright/test';

When('I visit each dashboard page {string}', async function (routes) {
  for (const r of routes.split(',').map((s) => s.trim())) {
    await this.page.goto(this.baseURL + r);
    await this.page.waitForLoadState('networkidle');
  }
});

// Mirrors smoke.spec.ts: ignore benign resource-load noise (favicon / net::ERR),
// fail on real JS console errors.
Then('no real console errors were logged', async function () {
  const real = this.consoleErrors.filter(
    (e) => !e.includes('favicon') && !e.includes('net::ERR')
  );
  expect(real).toHaveLength(0);
});
