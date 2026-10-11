import { defineConfig, devices } from '@playwright/test';

const PORT = 3099;
const BASE_URL = `http://127.0.0.1:${PORT}`;

export default defineConfig({
  testDir: './tests/e2e',
  fullyParallel: false,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 2 : 0,
  workers: 1,
  reporter: 'list',
  use: {
    baseURL: BASE_URL,
    trace: 'on-first-retry',
  },
  projects: [
    {
      name: 'chromium',
      use: { ...devices['Desktop Chrome'] },
    },
  ],
  webServer: {
    command: `rm -rf tmp/e2e-data && mkdir -p tmp/e2e-data && cp -r data/bikes/* tmp/e2e-data/ && PORT=${PORT} DATA_DIR=tmp/e2e-data go run ./cmd/server`,
    url: BASE_URL,
    reuseExistingServer: !process.env.CI,
    timeout: 120 * 1000,
  },
});
