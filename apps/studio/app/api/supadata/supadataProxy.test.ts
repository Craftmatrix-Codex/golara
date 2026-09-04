import { describe, expect, it, vi } from 'vitest'

import { proxySupadataRequest } from './supadataProxy'

describe('proxySupadataRequest', () => {
  it('forwards the request to the control plane with the server-only bearer token', async () => {
    const fetchImpl = vi.fn().mockResolvedValue(new Response('{"projects":[]}', { status: 200 }))
    const request = new Request('http://studio.test/api/supadata/projects', {
      headers: { authorization: 'Bearer browser-token', 'content-type': 'application/json' },
    })

    const response = await proxySupadataRequest(request, {
      baseUrl: 'http://control-plane.test',
      token: 'server-secret',
      fetchImpl,
      path: '/api/projects',
    })

    expect(response.status).toBe(200)
    expect(fetchImpl).toHaveBeenCalledWith(
      'http://control-plane.test/api/projects',
      expect.anything()
    )
    const forwardedHeaders = fetchImpl.mock.calls[0][1].headers as Headers
    expect(forwardedHeaders.get('authorization')).toBe('Bearer server-secret')
    expect(forwardedHeaders.get('authorization')).not.toBe('Bearer browser-token')
  })

  it('preserves bodyless upstream responses such as 204', async () => {
    const fetchImpl = vi.fn().mockResolvedValue(new Response(null, { status: 204 }))
    const response = await proxySupadataRequest(new Request('http://studio.test/delete'), {
      baseUrl: 'http://control-plane.test',
      token: 'server-secret',
      fetchImpl,
      path: '/api/projects/demo',
    })
    expect(response.status).toBe(204)
    expect(await response.text()).toBe('')
  })
})
