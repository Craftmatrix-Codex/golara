import { mkdir } from 'node:fs/promises'
import path from 'node:path'
import { expect, test } from '@playwright/test'

import { attachRuntimeGuards, expectSettledStudio, requireCanarySecrets } from '../helpers.js'

test.describe('Golara Canary visual regression', () => {
  const screenshotDir = path.resolve('artifacts/golara-canary/screenshots')

  test.beforeEach(async ({}, testInfo) => {
    requireCanarySecrets(testInfo, ['GOLARA_STUDIO_USERNAME', 'GOLARA_STUDIO_PASSWORD'])
  })

  test('create-user dialog is accessible, branded, and visually captured', async ({ page }) => {
    const runtime = attachRuntimeGuards(page)
    await page.goto('/project/default/auth/users')
    await expectSettledStudio(page)

    await page.getByRole('button', { name: 'Add user' }).click()
    await page.getByText('Create new user', { exact: true }).click()

    const dialog = page.getByRole('dialog', { name: 'Create a new user' })
    await expect(dialog).toBeVisible()
    await expect(dialog.getByLabel('Email address')).toBeVisible()
    await expect(dialog.getByLabel('User password')).toHaveAttribute('type', 'password')
    await expect(dialog.getByRole('checkbox', { name: 'Auto-confirm user' })).toBeChecked()
    await expect(dialog).toContainText('Skip email verification for this account.')
    await expect(dialog).toContainText('No confirmation email is sent when this option is enabled.')

    await dialog.getByRole('button', { name: 'Show password' }).click()
    await expect(dialog.getByLabel('User password')).toHaveAttribute('type', 'text')
    await dialog.getByRole('button', { name: 'Hide password' }).click()
    await expect(dialog.getByLabel('User password')).toHaveAttribute('type', 'password')

    await mkdir(screenshotDir, { recursive: true })
    await page.screenshot({
      path: path.join(screenshotDir, 'create-user-dialog-desktop.png'),
      fullPage: true,
    })

    runtime.assertClean()
  })

  test('branding, Auth users, and Table Editor are visually captured', async ({ page }) => {
    const runtime = attachRuntimeGuards(page)
    await mkdir(screenshotDir, { recursive: true })

    await page.goto('/project/default')
    await expect(page).toHaveTitle(/Golara/)
    await expect(page.getByRole('heading', { level: 1 })).toBeVisible()
    await expectSettledStudio(page)
    await page.screenshot({
      path: path.join(screenshotDir, 'golara-branding-desktop.png'),
      fullPage: true,
    })

    await page.goto('/project/default/auth/users')
    await expect(page.getByRole('heading', { name: 'Users' })).toBeVisible()
    await expect(page.getByRole('columnheader', { name: 'First name' })).toBeVisible()
    await expectSettledStudio(page)
    await page.screenshot({
      path: path.join(screenshotDir, 'auth-users-desktop.png'),
      fullPage: true,
    })

    await page.goto('/project/default/editor')
    await expectSettledStudio(page)
    await expect(page.locator('body')).not.toContainText('Something went wrong')
    await page.screenshot({
      path: path.join(screenshotDir, 'table-editor-desktop.png'),
      fullPage: true,
    })

    runtime.assertClean()
  })

  test('Edge Functions and scheduled Jobs are visually captured', async ({ page }) => {
    const runtime = attachRuntimeGuards(page)
    await mkdir(screenshotDir, { recursive: true })

    await page.goto('/project/default/functions')
    await expectSettledStudio(page)
    await expect(page.locator('body')).toContainText('Edge Functions')
    await page.screenshot({
      path: path.join(screenshotDir, 'edge-functions-desktop.png'),
      fullPage: true,
    })

    await page.goto('/project/default/integrations/cron/jobs')
    await expectSettledStudio(page)
    await expect(page.locator('body')).toContainText('No cron jobs in your project')
    await page.screenshot({
      path: path.join(screenshotDir, 'scheduled-jobs-desktop.png'),
      fullPage: true,
    })

    runtime.assertClean()
  })
})
