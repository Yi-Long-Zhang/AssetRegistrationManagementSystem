import { defineConfig } from '@playwright/test'

export default defineConfig({
  testDir: './tests/e2e',
  fullyParallel: false,
  workers: 1,
  retries: process.env.CI ? 1 : 0,
  reporter: process.env.CI ? 'github' : 'list',
  use: {
    baseURL: 'http://127.0.0.1:5173',
    channel: process.env.CI ? undefined : (process.env.PLAYWRIGHT_CHANNEL || 'msedge'),
    trace: 'retain-on-failure',
    screenshot: 'only-on-failure'
  },
  webServer: [
    {
      command: 'node tests/e2e/start-backend.mjs',
      url: 'http://127.0.0.1:18080/readyz',
      timeout: 120000,
      reuseExistingServer: false
    },
    {
      command: 'node node_modules/vite/bin/vite.js --host 127.0.0.1 --port 5173',
      url: 'http://127.0.0.1:5173',
      env: { VITE_PROXY_TARGET: 'http://127.0.0.1:18080' },
      timeout: 120000,
      reuseExistingServer: false
    }
  ]
})
