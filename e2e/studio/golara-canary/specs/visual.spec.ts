import { mkdir } from 'node:fs/promises'
import path from 'node:path'
import { expect, test } from '@playwright/test'

import { attachRuntimeGuards, expectSettledStudio, requireCanarySecrets } from '../helpers.js'

test.describe('Golara Canary visual regression', () => {
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

    const screenshotDir = path.resolve('artifacts/golara-canary/screenshots')
    await mkdir(screenshotDir, { recursive: true })
    await page.screenshot({
      path: path.join(screenshotDir, 'create-user-dialog-desktop.png'),
      fullPage: true,
    })

    runtime.assertClean()
  })
})
