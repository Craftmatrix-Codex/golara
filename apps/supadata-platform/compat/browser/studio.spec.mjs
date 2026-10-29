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
  await page.screenshot({ path: testInfo.outputPath('studio-project.png'), fullPage: true })
  expect(errors).toEqual([])
  expect(failedResponses).toEqual([])
  expect(localRequests).toEqual([])
  expect((await page.request.post('/api/studio-auth/logout')).status()).toBe(200)
  expect((await page.request.get('/api/platform/projects')).status()).toBe(401)
})
