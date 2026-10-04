import { randomUUID } from 'node:crypto'
import { expect, test } from '@playwright/test'

import { requireCanarySecrets } from '../helpers.js'

const projectRoot = '/api/projects/default'
const storageBucket = process.env.GOLARA_STORAGE_BUCKET ?? 'supadata-default'

test.describe('Golara Canary data plane', () => {
  test('GraphQL executes against the project database', async ({ request }, testInfo) => {
    requireCanarySecrets(testInfo, ['GOLARA_ANON_KEY'])
    const response = await request.post(`${projectRoot}/graphql/v1`, {
      headers: { apikey: process.env.GOLARA_ANON_KEY as string },
      data: { query: '{ __typename }' },
    })

    expect(response.status(), await response.text()).toBe(200)
    expect(await response.json()).toEqual({ data: { __typename: 'Query' } })
  })

  test('storage uploads, downloads, lists, and deletes an isolated object', async ({
    request,
  }, testInfo) => {
    requireCanarySecrets(testInfo, ['GOLARA_ANON_KEY', 'GOLARA_SERVICE_ROLE_KEY'])
    const anonKey = process.env.GOLARA_ANON_KEY as string
    const serviceRoleKey = process.env.GOLARA_SERVICE_ROLE_KEY as string
    const objectKey = `e2e/${randomUUID()}.txt`
    const objectPath = `${projectRoot}/storage/v1/object/${storageBucket}/${objectKey}`
    const content = `golara-canary-${randomUUID()}`

    const upload = await request.post(objectPath, {
      headers: { apikey: serviceRoleKey, 'content-type': 'text/plain' },
      data: content,
    })
    expect(upload.status(), await upload.text()).toBe(200)

    try {
      const download = await request.get(objectPath, { headers: { apikey: anonKey } })
      expect(download.status(), await download.text()).toBe(200)
      expect(await download.text()).toBe(content)
      expect(download.headers()['content-type']).toContain('text/plain')

      const list = await request.post(`${projectRoot}/storage/v1/object/list/${storageBucket}`, {
        headers: { apikey: anonKey },
        data: { prefix: 'e2e/', limit: 100, offset: 0 },
      })
      expect(list.status(), await list.text()).toBe(200)
      expect(await list.json()).toEqual(
        expect.arrayContaining([expect.objectContaining({ name: objectKey })])
      )
    } finally {
      const cleanup = await request.delete(objectPath, {
        headers: { apikey: serviceRoleKey },
      })
      expect(cleanup.status(), await cleanup.text()).toBe(200)
    }
  })

  test('anonymous storage mutation is rejected', async ({ request }, testInfo) => {
    requireCanarySecrets(testInfo, ['GOLARA_ANON_KEY'])
    const response = await request.post(
      `${projectRoot}/storage/v1/object/${storageBucket}/denied.txt`,
      {
        headers: { apikey: process.env.GOLARA_ANON_KEY as string },
        data: 'denied',
      }
    )
    expect(response.status()).toBe(401)
  })

  test('CORS preflight declares supported methods and headers', async ({ request }) => {
    const response = await request.fetch(`${projectRoot}/auth/v1/settings`, {
      method: 'OPTIONS',
      headers: {
        Origin: 'https://client.example.invalid',
        'Access-Control-Request-Method': 'GET',
        'Access-Control-Request-Headers': 'apikey,authorization',
      },
    })

    expect(response.status()).toBe(204)
    expect(response.headers()['access-control-allow-methods']).toContain('GET')
    expect(response.headers()['access-control-allow-headers']).toContain('apikey')
  })
})
