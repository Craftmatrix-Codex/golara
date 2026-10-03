import { expect, Page, TestInfo } from '@playwright/test'

const ignoredResponsePaths = ['/api/telemetry']

export function requireCanarySecrets(testInfo: TestInfo, names: string[]) {
  const missing = names.filter((name) => !process.env[name])
  testInfo.skip(missing.length > 0, `Missing Canary environment variables: ${missing.join(', ')}`)
}

export function attachRuntimeGuards(page: Page) {
  const pageErrors: string[] = []
  const consoleErrors: string[] = []
  const failedRequests: string[] = []
  const serverErrors: string[] = []

  page.on('pageerror', (error) => pageErrors.push(error.message))
  page.on('console', (message) => {
    if (message.type() === 'error') consoleErrors.push(message.text())
  })
  page.on('requestfailed', (request) => {
    failedRequests.push(`${request.method()} ${request.url()}: ${request.failure()?.errorText}`)
  })
  page.on('response', (response) => {
    if (response.status() < 500) return
    if (ignoredResponsePaths.some((path) => response.url().includes(path))) return
    serverErrors.push(`${response.status()} ${response.request().method()} ${response.url()}`)
  })

  return {
    assertClean() {
      expect(pageErrors, 'No uncaught page errors should occur').toEqual([])
      expect(consoleErrors, 'No browser console errors should occur').toEqual([])
      expect(failedRequests, 'No network requests should fail').toEqual([])
      expect(serverErrors, 'No application request should return 5xx').toEqual([])
    },
  }
}

export async function expectSettledStudio(page: Page) {
  await expect(page.locator('body')).not.toContainText('Taking longer than expected?')
  await expect(page.locator('body')).not.toContainText('Bad Gateway')
  await expect(page.getByAltText('Golara')).toBeVisible()
}
