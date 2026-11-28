import { defineConfig, devices } from '@playwright/test';

export default defineConfig({
  testDir: './e2e',
  fullyParallel: false, // Run sequentially for predictable DB state
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 2 : 0,
  workers: 1, // Single worker avoids database write contention on SQLite
  reporter: [['list'], ['html', { open: 'never' }]],
  globalSetup: './e2e/global-setup.ts',
  globalTeardown: './e2e/global-teardown.ts',
  use: {
    baseURL: 'http://127.0.0.1:8085',
    trace: 'on-first-retry',
    screenshot: 'only-on-failure',
  },
  projects: [
    {
      name: 'chromium',
      use: { ...devices['Desktop Chrome'] },
    },
  ],
  webServer: {
    command: 'go run main.go',
    url: 'http://127.0.0.1:8085/login',
    reuseExistingServer: !process.env.CI,
    timeout: 60000,
    env: {
      PORT: '8085',
      DATABASE_URL: 'series_tkd_e2e.db',
    },
  },
});
