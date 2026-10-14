import { defineConfig } from '@playwright/test';
export default defineConfig({
  testDir: './tests/browser', timeout: 30000, fullyParallel: true,
  use: {
    baseURL: 'http://127.0.0.1:18741', viewport: { width: 1200, height: 700 },
    launchOptions: process.env.CHROMIUM_PATH ? { executablePath: process.env.CHROMIUM_PATH } : {},
  },
  webServer: { command: 'node scripts/serve-history-test.mjs', url: 'http://127.0.0.1:18741', reuseExistingServer: false },
});
