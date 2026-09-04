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

test('proxy requests use the configured host while preserving project ports', async () => {
  const originalFetch = globalThis.fetch
  const requests = []
  globalThis.fetch = async (url) => {
    requests.push(url)
    return new Response('ok', { status: 200 })
  }
  try {
    const handler = createControlPlaneHandler(
      { currentProject: async () => ({ gatewayPort: 8100, metaPort: 8103 }) },
      { token: 'secret', proxyHost: '203.0.113.10' }
    )
    const headers = { authorization: 'Bearer secret' }
    assert.equal((await handler(request('/proxy/rest/v1/items', { headers }))).status, 200)
    assert.equal((await handler(request('/proxy-meta/tables', { headers }))).status, 200)
    assert.deepEqual(requests, [
      'http://203.0.113.10:8100/rest/v1/items',
      'http://203.0.113.10:8103/tables',
    ])
  } finally {
    globalThis.fetch = originalFetch
  }
})
