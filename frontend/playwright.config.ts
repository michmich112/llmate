import { defineConfig } from '@playwright/test';

export default defineConfig({
  testDir: './e2e',
  timeout: 120000,
  fullyParallel: false,
  workers: 1,
  outputDir: './e2e/test-results',
  reporter: [['list'], ['html', { outputFolder: 'e2e/playwright-report', open: 'never' }]],
  use: {
    baseURL: 'http://127.0.0.1:8099',
    headless: true,
    screenshot: 'only-on-failure',
    trace: 'retain-on-failure',
  },
  webServer: {
    command: 'bash e2e/start-gateway.sh',
    url: 'http://127.0.0.1:8099',
    timeout: 120000,
    reuseExistingServer: false,
  },
});
