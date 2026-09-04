import assert from 'node:assert/strict'
import test from 'node:test'

import { createControlPlaneHandler } from './server.mjs'

function request(path, options = {}) {
  return new Request(`http://localhost${path}`, options)
}

test('health is public while project management requires bearer auth', async () => {
  const registry = { listProjects: async () => [] }
  const handler = createControlPlaneHandler(registry, { token: 'secret' })

  assert.equal((await handler(request('/health'))).status, 200)
  assert.equal((await handler(request('/api/projects'))).status, 401)
  assert.equal(
    (await handler(request('/api/projects', { headers: { authorization: 'Bearer secret' } })))
      .status,
    200
  )
})

test('missing control-plane token rejects protected requests even with a bearer header', async () => {
  const handler = createControlPlaneHandler({ listProjects: async () => [] }, { token: undefined })
  assert.equal(
    (await handler(request('/api/projects', { headers: { authorization: 'Bearer anything' } })))
      .status,
    401
  )
})

test('proxy requests also require the control-plane bearer token', async () => {
  const handler = createControlPlaneHandler(
    { currentProject: async () => null },
    { token: 'secret' }
  )
  assert.equal((await handler(request('/proxy/rest/v1/items'))).status, 401)
  assert.equal((await handler(request('/proxy-meta'))).status, 401)
})
