import { APIRequestContext, Page, TestInfo } from '@playwright/test';

export const ACCESS_KEY = 'e2e-key';

export async function authInit(page: Page): Promise<void> {
  await page.addInitScript((key: string) => {
    localStorage.setItem('access_key', key);
  }, ACCESS_KEY);
}

export async function api<T>(
  ctx: APIRequestContext,
  path: string,
  method: 'GET' | 'POST' | 'PUT' | 'DELETE' = 'GET',
  body?: unknown
): Promise<T> {
  const res = await ctx.fetch(`http://127.0.0.1:8099${path}`, {
    method,
    headers: { Authorization: `Bearer ${ACCESS_KEY}`, 'Content-Type': 'application/json' },
    data: body ? JSON.stringify(body) : undefined,
  });
  const status = res.status();
  let data: any = null;
  try { data = await res.json(); } catch { data = null; }
  if (status < 200 || status >= 300) {
    throw new Error(`API ${method} ${path} failed (${status}): ${JSON.stringify(data)}`);
  }
  return data as T;
}

/** Capture a screenshot, save it to e2e/screenshots/, and attach it to the HTML report. */
export async function shot(
  page: Page,
  testInfo: TestInfo,
  name: string
): Promise<void> {
  const p = `e2e/screenshots/${name}.png`;
  await page.screenshot({ path: p, fullPage: true });
  await testInfo.attach(name, { path: p, contentType: 'image/png' });
}

/** Authenticate a page by setting the access key in localStorage, then go to /. */
export async function login(page: Page): Promise<void> {
  await page.addInitScript(() => localStorage.setItem('access_key', 'e2e-key'));
  await page.goto('/');
  await page.waitForURL('**/');
}
