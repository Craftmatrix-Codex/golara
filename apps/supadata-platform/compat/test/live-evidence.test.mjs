import assert from 'node:assert/strict'
import test from 'node:test'

import { assertStableAttestation, redactLog } from '../live-evidence-lib.mjs'

const revision = 'a'.repeat(40)
const good = {
  id: 'f'.repeat(64),
  name: 'poslsmflcfylqkhdl131pj6o-20261029',
  image: `poslsmflcfylqkhdl131pj6o:${revision}`,
  health: 'healthy',
  volume: 'poslsmflcfylqkhdl131pj6o-golara-stable-data',
}

test('attestation requires one healthy exact revision and the persistent volume', () => {
  assert.deepEqual(assertStableAttestation([good], revision), good)
  for (const invalid of [
    [],
    [good, good],
    [{ ...good, health: 'starting' }],
    [{ ...good, image: `poslsmflcfylqkhdl131pj6o:${'b'.repeat(40)}` }],
    [{ ...good, volume: 'different' }],
  ]) {
    assert.throws(() => assertStableAttestation(invalid, revision))
  }
  assert.throws(() => assertStableAttestation([good], 'abc'))
})

test('attestation rejects a container replacement during a test run', () => {
  assert.throws(() =>
    assertStableAttestation(
      [{ ...good, name: 'poslsmflcfylqkhdl131pj6o-replaced' }],
      revision,
      good
    )
  )
  assert.throws(() => assertStableAttestation([{ ...good, id: 'e'.repeat(64) }], revision, good))
})

test('evidence logs redact configured credentials without rewriting nonsecret values', () => {
  const config = {
    anonKey: 'secret-anon',
    serviceRoleKey: 'secret-service',
    studioPassword: 'password-value',
    studioUsername: 'login-name',
    publicURL: 'https://golara.craftmatrix.org',
  }
  const output = redactLog(
    'secret-anon secret-service password-value login-name https://golara.craftmatrix.org',
    config
  )
  assert.equal(output, '[REDACTED] [REDACTED] [REDACTED] [REDACTED] https://golara.craftmatrix.org')
})

test('successful commands still require all expected tests to run without skips', async () => {
  const { summarizeStage } = await import('../live-evidence-lib.mjs')
  assert.deepEqual(
    summarizeStage(
      'sdk',
      '# tests 16\n# pass 16\n# fail 0\n# skipped 0\n# cancelled 0\n# todo 0\n'
    ),
    { total: 16, passed: 16 }
  )
  assert.deepEqual(summarizeStage('browser', '  18 passed (1.4m)'), { total: 18, passed: 18 })
  assert.throws(() =>
    summarizeStage('sdk', '# tests 16\n# pass 15\n# fail 0\n# skipped 1\n# cancelled 0\n# todo 0\n')
  )
  assert.throws(() => summarizeStage('browser', '  18 passed, 1 skipped'))
  assert.throws(() => summarizeStage('browser', '  2 passed'))
  assert.throws(() => summarizeStage('sdk', ''))
})
