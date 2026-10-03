import { existsSync } from 'node:fs'
import { defineConfig, devices } from '@playwright/test'

const baseURL = process.env.GOLARA_BASE_URL ?? 'https://go-alpha.craftmatrix.org'
const username = process.env.GOLARA_STUDIO_USERNAME
const password = process.env.GOLARA_STUDIO_PASSWORD
const systemChromium = process.env.GOLARA_CHROMIUM_EXECUTABLE ?? '/snap/bin/chromium'

export default defineConfig({
  testDir: './golara-canary/specs',
  outputDir: './artifacts/golara-canary/test-results',
  timeout: 60_000,
  expect: { timeout: 15_000 },
  fullyParallel: false,
  workers: 1,
  retries: process.env.CI ? 2 : 0,
  forbidOnly: Boolean(process.env.CI),
  reporter: [
    ['list'],
    ['json', { outputFile: './artifacts/golara-canary/results.json' }],
    ['html', { outputFolder: './artifacts/golara-canary/html', open: 'never' }],
  ],
  use: {
    baseURL,
    httpCredentials: username && password ? { username, password } : undefined,
    headless: true,
    screenshot: 'only-on-failure',
    trace: 'retain-on-failure',
    video: 'retain-on-failure',
    actionTimeout: 15_000,
    navigationTimeout: 30_000,
    ignoreHTTPSErrors: false,
    launchOptions: existsSync(systemChromium) ? { executablePath: systemChromium } : undefined,
  },
  projects: [
    {
      name: 'desktop-chromium',
      testIgnore: /.*mobile\.spec\.ts/,
      use: { ...devices['Desktop Chrome'], viewport: { width: 1440, height: 900 } },
    },
    {
      name: 'mobile-chromium',
      testMatch: /.*mobile\.spec\.ts/,
      use: { ...devices['Pixel 7'] },
    },
  ],
})
