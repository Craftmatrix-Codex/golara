import assert from 'node:assert/strict'
import test from 'node:test'

import { validateConfig } from '../config.mjs'

test('configuration requires an explicit target and rejects non-HTTPS live origins', () => {
  assert.throws(() => validateConfig({}), /publicURL/)
  assert.throws(() => validateConfig({ publicURL: 'http://golara.craftmatrix.org' }), /HTTPS/)
})

test('configuration requires a disposable namespace for live writes', () => {
  const base = {
    publicURL: 'https://golara.craftmatrix.org',
    anonKey: 'anon',
    serviceRoleKey: 'service',
  }
  assert.throws(() => validateConfig(base), /namespace/)
  const cfg = validateConfig({ ...base, namespace: 'golara_sdk_contract', project: 'default' })
  assert.equal(cfg.sdkURL, 'https://golara.craftmatrix.org/api/projects/default')
  assert.equal(cfg.namespace, 'golara_sdk_contract')
})

test('local integration targets are explicit and allow loopback HTTP', () => {
  const cfg = validateConfig({
    publicURL: 'http://127.0.0.1:18080',
    anonKey: 'anon',
    serviceRoleKey: 'service',
    namespace: 'golara_sdk_local',
  })
  assert.equal(cfg.project, 'default')
  assert.throws(() => validateConfig({ ...cfg, namespace: 'public' }), /namespace/)
})
