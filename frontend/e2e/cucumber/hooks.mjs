import { Before, After, AfterStep } from '@cucumber/cucumber';
import { chromium } from '@playwright/test';

Before(async function () {
  this.browser = await chromium.launch({ headless: true });
  this.context = await this.browser.newContext();
  this.page = await this.context.newPage();
});

// Wait for the page to settle so screenshots capture the rendered UI rather
// than a blank/white frame. Returns false when there is no real page to capture
// (e.g. about:blank in pure API scenarios), so we skip those screenshots.
async function waitForRender(page) {
  const url = page.url();
  if (!url || url === 'about:blank') return false;
  // Wait for in-flight fetches (SPA route transitions, async data) to settle,
  // then give the DOM a moment to commit before capturing.
  await page.waitForLoadState('networkidle').catch(() => {});
  await page.waitForTimeout(250);
  return true;
}

// Attach a screenshot after every step so the HTML report shows the page state
// at each page visit / UI action.
AfterStep(async function () {
  if (this.page && !this.page.isClosed() && (await waitForRender(this.page))) {
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
