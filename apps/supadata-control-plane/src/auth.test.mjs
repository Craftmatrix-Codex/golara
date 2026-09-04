import assert from 'node:assert/strict'
import test from 'node:test'

import { hasValidBearerToken } from './auth.mjs'

test('bearer authentication requires a configured token and exact match', () => {
  assert.equal(hasValidBearerToken(undefined, 'Bearer secret'), false)
  assert.equal(hasValidBearerToken('secret', undefined), false)
  assert.equal(hasValidBearerToken('secret', 'Basic secret'), false)
  assert.equal(hasValidBearerToken('secret', 'Bearer wrong'), false)
  assert.equal(hasValidBearerToken('secret', 'Bearer secret'), true)
})

test('bearer authentication does not accept a token with a different length', () => {
  assert.equal(hasValidBearerToken('secret', 'Bearer secret-longer'), false)
})
