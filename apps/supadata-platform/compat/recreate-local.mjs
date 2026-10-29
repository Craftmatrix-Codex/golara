import assert from 'node:assert/strict'
import { execFileSync } from 'node:child_process'
import { chromium } from '@playwright/test'

import { loadConfig } from './config.mjs'

const cfg = loadConfig()
assert.equal(
  cfg.publicURL,
  'http://127.0.0.1:18080',
  'Container replacement is restricted to the disposable local harness'
)
assert.ok(
  process.env.GOLARA_LOCAL_ENV_FILE,
  'GOLARA_LOCAL_ENV_FILE must name the protected local environment file'
)
const container = 'golara-sdk-slim'
const previous = `${container}-previous`
const docker = (...args) =>
  execFileSync('docker', args, { encoding: 'utf8', stdio: ['ignore', 'pipe', 'pipe'] })
const inspect = JSON.parse(docker('inspect', container))[0]
assert.equal(inspect.HostConfig.PortBindings['8080/tcp'][0].HostIp, '127.0.0.1')
assert.ok(
  inspect.Mounts.some(
    (m) => m.Name === 'golara-sdk-app-data' && m.Destination === '/var/lib/supadata'
  )
)
const browser = await chromium.launch({ headless: true })
const context = await browser.newContext({ baseURL: cfg.publicURL })
let replaced = false
try {
  const page = await context.newPage()
  await page.goto('/sign-in')
  await page.getByLabel('Email or username', { exact: true }).fill(cfg.studioUsername)
  await page.getByLabel('Password', { exact: true }).fill(cfg.studioPassword)
  await page.getByRole('button', { name: 'Sign in', exact: true }).click()
  await page
    .getByRole('link', { name: /Database/ })
    .first()
    .waitFor()
  assert.equal((await context.request.get('/api/platform/projects')).status(), 200)
  docker('stop', '--time=5', container)
  docker('rename', container, previous)
  replaced = true
  docker(
    'run',
    '-d',
    '--name',
    container,
    '--network',
    'golara-sdk-validation',
    '--env-file',
    process.env.GOLARA_LOCAL_ENV_FILE,
    '-p',
    '127.0.0.1:18080:8080',
    '-v',
    'golara-sdk-app-data:/var/lib/supadata',
    inspect.Config.Image
  )
  const deadline = Date.now() + 30000
  while (true) {
    if (
      await fetch(`${cfg.publicURL}/health`, {
        signal: AbortSignal.timeout(Math.min(2000, Math.max(1, deadline - Date.now()))),
      })
        .then((r) => r.ok)
        .catch(() => false)
    )
      break
    if (Date.now() > deadline) throw new Error('Replacement container did not become healthy')
    await new Promise((resolve) => setTimeout(resolve, 250))
  }
  assert.equal(
    (await context.request.get('/api/platform/projects')).status(),
    200,
    'Studio session must survive replacement of the container while keeping its named volume'
  )
  docker('rm', previous)
  replaced = false
  console.log(
    'Authenticated Studio session survives a fresh container using the same named volume.'
  )
} catch (error) {
  if (replaced) {
    try {
      docker('rm', '-f', container)
    } catch {}
    docker('rename', previous, container)
    docker('start', container)
  }
  throw error
} finally {
  await browser.close()
}
