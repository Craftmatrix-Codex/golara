import { expect, test } from '@playwright/test'

import { loadConfig } from '../config.mjs'

const cfg = loadConfig()
const local = new URL(cfg.publicURL).protocol === 'http:'

for (const viewport of [
  { width: 1440, height: 900 },
  { width: 390, height: 844 },
]) {
  test(`public sign-in renders at ${viewport.width}px using the public origin`, async ({
    page,
  }, testInfo) => {
    await page.setViewportSize(viewport)
    const errors = []
    const localRequests = []
    page.on('pageerror', (error) => errors.push(error.message))
    page.on('request', (request) => {
      if (!local && /localhost|127\.0\.0\.1|0\.0\.0\.0/.test(request.url()))
        localRequests.push(request.url())
    })
    const response = await page.goto('/sign-in')
    expect(response.status()).toBe(200)
    await expect(page.getByRole('heading', { name: 'Sign in to GoLara' })).toBeVisible()
    await expect(page.getByLabel('Email or username', { exact: true })).toBeVisible()
    await expect(page.getByLabel('Password', { exact: true })).toBeVisible()
    await expect(page.getByAltText(/space engineer mascot/)).toBeVisible()
    expect(await page.locator('body').innerText()).not.toMatch(/localhost/i)
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true)
    await page.screenshot({
      path: testInfo.outputPath(`sign-in-${viewport.width}.png`),
      fullPage: true,
    })
    expect(errors).toEqual([])
    expect(localRequests).toEqual([])
  })
}

test('browser sign-in opens the real Studio; protected responses use public configuration', async ({
  page,
}, testInfo) => {
  const errors = []
  const failedResponses = []
  const localRequests = []
  page.on('pageerror', (error) => errors.push(error.message))
  page.on('response', (response) => {
    if (response.status() >= 400 && !response.url().includes('/api/telemetry'))
      failedResponses.push(`${response.status()} ${new URL(response.url()).pathname}`)
  })
  page.on('request', (request) => {
    if (!local && /localhost|127\.0\.0\.1|0\.0\.0\.0/.test(request.url()))
      localRequests.push(request.url())
  })
  await page.goto('/sign-in')
  await page.getByLabel('Email or username', { exact: true }).fill(cfg.studioUsername)
  await page.getByLabel('Password', { exact: true }).fill(cfg.studioPassword)
  await page.getByRole('button', { name: 'Sign in', exact: true }).click()
  await expect(page).toHaveURL(/\/project\/default/)
  await expect(page.getByRole('link', { name: /Database/ }).first()).toBeVisible()
  await expect(page.locator('body')).not.toContainText('localhost')
  const metadata = await page.request.get('/api/platform/projects/default')
  expect(metadata.status()).toBe(200)
  expect(await metadata.text()).not.toMatch(/localhost/i)
  if (!local) {
    const settings = await page.request.get('/api/platform/projects/default/settings')
    expect(settings.status()).toBe(200)
    expect((await settings.json()).app_config).toMatchObject({
      protocol: 'https',
      endpoint: `${new URL(cfg.publicURL).host}/api/projects/default`,
    })
    await expect(page.locator('main').first()).toContainText(
      `${cfg.publicURL}/api/projects/default`
    )
  }
  await page.screenshot({ path: testInfo.outputPath('studio-project.png'), fullPage: true })
  expect(errors).toEqual([])
  expect(failedResponses).toEqual([])
  expect(localRequests).toEqual([])
  await page.getByRole('button', { name: 'Settings', exact: true }).click({ timeout: 15000 })
  await expect(page.getByRole('menuitem', { name: 'Sign out', exact: true })).toBeVisible()
  const logout = page.waitForResponse(
    (response) =>
      new URL(response.url()).pathname === '/api/studio-auth/logout' &&
      response.request().method() === 'POST'
  )
  await page.getByRole('menuitem', { name: 'Sign out', exact: true }).click()
  expect((await logout).status()).toBe(200)
  await expect(page).toHaveURL(/\/sign-in$/)
  await expect(page.getByRole('heading', { name: 'Sign in to GoLara' })).toBeVisible()
  expect((await page.request.get('/api/platform/projects')).status()).toBe(401)
})

test('failed Studio logout offers retry and preserves authenticated access', async ({ page }) => {
  await page.goto('/sign-in')
  await page.getByLabel('Email or username', { exact: true }).fill(cfg.studioUsername)
  await page.getByLabel('Password', { exact: true }).fill(cfg.studioPassword)
  await page.getByRole('button', { name: 'Sign in', exact: true }).click()
  await expect(page.getByRole('link', { name: /Database/ }).first()).toBeVisible()
  await page.route('**/api/studio-auth/logout', (route) =>
    route.fulfill({
      status: 503,
      contentType: 'application/json',
      body: '{"message":"Temporarily unavailable"}',
    })
  )
  await page.goto('/logout')
  await expect(page.getByRole('alert')).toHaveText('Unable to sign out. Try again.')
  expect((await page.request.get('/api/platform/projects')).status()).toBe(200)
  await page.unroute('**/api/studio-auth/logout')
  await page.getByRole('button', { name: 'Try again', exact: true }).click()
  await expect(page).toHaveURL(/\/sign-in$/)
  expect((await page.request.get('/api/platform/projects')).status()).toBe(401)
})

// Exercise real application pages beyond the sign-in/landing shell.
test.describe('Studio feature navigation', () => {
  let context
  test.beforeAll(async ({ browser }) => {
    context = await browser.newContext({ baseURL: cfg.publicURL })
    const page = await context.newPage()
    await page.goto('/sign-in')
    await page.getByLabel('Email or username', { exact: true }).fill(cfg.studioUsername)
    await page.getByLabel('Password', { exact: true }).fill(cfg.studioPassword)
    await page.getByRole('button', { name: 'Sign in', exact: true }).click()
    await expect(page.getByRole('link', { name: /Database/ }).first()).toBeVisible()
    await page.close()
  })
  test.afterAll(async () => {
    await context?.close()
  })

  for (const [name, path] of [
    ['Table Editor', 'editor'],
    ['SQL Editor', 'sql'],
    ['Database', 'database/schemas'],
    ['Authentication', 'auth/users'],
    ['Storage', 'storage/files'],
    ['Edge Functions', 'functions'],
    ['Realtime', 'realtime/inspector'],
    ['Advisors', 'advisors/security'],
    ['Observability', 'observability'],
    ['Logs', 'logs/explorer'],
    ['Integrations', 'integrations'],
    ['Project Settings', 'settings/general'],
  ]) {
    test(`${name} loads without backend or browser errors`, async () => {
      const page = await context.newPage()
      const failures = [],
        errors = [],
        loopback = []
      page.on('response', (response) => {
        if (response.status() >= 400 && !response.url().includes('/api/telemetry'))
          failures.push(`${response.status()} ${new URL(response.url()).pathname}`)
      })
      page.on('pageerror', (error) => errors.push(error.message))
      page.on('request', (request) => {
        if (!local && /localhost|127\.0\.0\.1|0\.0\.0\.0/.test(request.url()))
          loopback.push(request.url())
      })
      try {
        expect((await page.goto(`/project/default/${path}`)).status()).toBe(200)
        await expect(page.getByRole('heading', { name, exact: true }).first()).toBeVisible()
        await page.waitForLoadState('networkidle')
        expect(failures).toEqual([])
        expect(errors).toEqual([])
        expect(loopback).toEqual([])
        expect(await page.locator('body').innerText()).not.toMatch(/localhost/i)
      } finally {
        await page.close()
      }
    })
  }
})
