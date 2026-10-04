import { mkdir } from 'node:fs/promises'
import path from 'node:path'
import { expect, test } from '@playwright/test'

import { attachRuntimeGuards, expectSettledStudio, requireCanarySecrets } from '../helpers.js'

test.describe('Golara Canary mobile smoke', () => {
  test.beforeEach(async ({}, testInfo) => {
    requireCanarySecrets(testInfo, ['GOLARA_STUDIO_USERNAME', 'GOLARA_STUDIO_PASSWORD'])
  })

  test('project shell stays within the mobile viewport', async ({ page }) => {
    const runtime = attachRuntimeGuards(page)
    await page.goto('/project/default')
    await expect(page.getByRole('heading', { level: 1 })).toBeVisible()

    const geometry = await page.evaluate(() => ({
      viewportWidth: window.innerWidth,
      documentWidth: document.documentElement.scrollWidth,
    }))
    expect(geometry.documentWidth).toBeLessThanOrEqual(geometry.viewportWidth + 1)
    runtime.assertClean()
  })

  test('Auth users route remains usable on a mobile viewport', async ({ page }) => {
    const runtime = attachRuntimeGuards(page)
    await page.goto('/project/default/auth/users')

    await expect(page.getByRole('heading', { name: 'Users' })).toBeVisible()
    await expect(page.getByRole('button', { name: 'Add user' })).toBeVisible()
    await expectSettledStudio(page)
    await expect(page.locator('body')).not.toContainText('Taking longer than expected?')
    await expect(page.locator('body')).not.toContainText('Loading...')
    const screenshotDir = path.resolve('artifacts/golara-canary/screenshots')
    await mkdir(screenshotDir, { recursive: true })
    await page.screenshot({
      path: path.join(screenshotDir, 'auth-users-mobile.png'),
      fullPage: true,
    })
    runtime.assertClean()
  })
})
