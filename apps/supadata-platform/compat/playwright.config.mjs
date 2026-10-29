import { defineConfig } from '@playwright/test'

import { loadConfig } from './config.mjs'

const cfg = loadConfig()
export default defineConfig({
  testDir: './browser',
  timeout: 60000,
  expect: { timeout: 15000 },
  workers: 1,
  retries: 0,
  reporter: [['list']],
  outputDir: '../../../output/playwright/golara-stable',
  use: {
    baseURL: cfg.publicURL,
    headless: true,
    ignoreHTTPSErrors: false,
    trace: 'off',
    video: 'off',
    screenshot: 'only-on-failure',
  },
})
