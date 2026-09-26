import { Before, After, AfterStep } from '@cucumber/cucumber';
import { chromium } from '@playwright/test';

Before(async function () {
  this.browser = await chromium.launch({ headless: true });
  this.context = await this.browser.newContext();
  this.page = await this.context.newPage();
});

// Attach a screenshot after every step so the HTML report shows the page state
// at each page visit / UI action.
AfterStep(async function () {
  if (this.page && !this.page.isClosed()) {
    const buf = await this.page.screenshot({ fullPage: false });
    await this.attach(buf, 'image/png');
  }
});

After(async function ({ result }) {
  if (result && result.status === 'FAILED') {
    const buf = await this.page.screenshot({ fullPage: true });
    await this.attach(buf, 'image/png');
  }
  if (this.browser) await this.browser.close();
});
