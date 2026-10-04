import { expect, test } from '@playwright/test'

import { attachRuntimeGuards, expectSettledStudio, requireCanarySecrets } from '../helpers.js'

test.describe('Golara Canary smoke', () => {
  test.beforeEach(async ({}, testInfo) => {
    requireCanarySecrets(testInfo, ['GOLARA_STUDIO_USERNAME', 'GOLARA_STUDIO_PASSWORD'])
  })

  test('health endpoint reports ready', async ({ request }) => {
    const response = await request.get('/health')
    expect(response.status()).toBe(200)
    await expect(response.json()).resolves.toEqual({ status: 'ok' })
  })

  test('project overview settles without runtime failures', async ({ page }) => {
    const runtime = attachRuntimeGuards(page)
    await page.goto('/project/default')

    await expect(page.getByRole('heading', { level: 1 })).toBeVisible()
    await expectSettledStudio(page)
    await expect(page).toHaveTitle(/Golara/)
    runtime.assertClean()
  })

  test('Auth users page settles and exposes split name columns', async ({ page }) => {
    const runtime = attachRuntimeGuards(page)
    await page.goto('/project/default/auth/users')

    await expect(page.getByRole('heading', { name: 'Users' })).toBeVisible()
    await expect(page.getByRole('columnheader', { name: 'First name' })).toBeVisible()
    await expect(page.getByRole('columnheader', { name: 'Middle name' })).toBeVisible()
    await expect(page.getByRole('columnheader', { name: 'Last name' })).toBeVisible()
    await expect(page.getByRole('columnheader', { name: 'Display name' })).toHaveCount(0)
    await expectSettledStudio(page)
    runtime.assertClean()
  })

  test('critical Studio routes render without 5xx responses', async ({ context }) => {
    const routes = [
      '/project/default/editor',
      '/project/default/sql',
      '/project/default/storage/buckets',
      '/project/default/functions',
      '/project/default/integrations/cron/jobs',
      '/project/default/settings/api',
    ]

    for (const route of routes) {
      await test.step(route, async () => {
        const routePage = await context.newPage()
        const runtime = attachRuntimeGuards(routePage)
        const response = await routePage.goto(route)
        expect(response?.status(), `${route} document should load`).toBe(200)
        await expect(routePage.locator('body')).not.toContainText('Bad Gateway')
        await expectSettledStudio(routePage)
        runtime.assertClean()
        await routePage.close()
      })
    }
  })
})
