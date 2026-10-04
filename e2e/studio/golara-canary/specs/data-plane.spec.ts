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

  test('Realtime broadcasts between two project-scoped websocket clients', async ({ page }) => {
    const anonKey = process.env.GOLARA_ANON_KEY!
    await page.goto('/project/default')
    const received = await page.evaluate(
      async ({ anonKey }) => {
        const endpoint = `${location.origin.replace(/^http/, 'ws')}/api/projects/default/realtime/v1/websocket?apikey=${encodeURIComponent(anonKey)}&vsn=1.0.0`
        const connect = () =>
          new Promise<WebSocket>((resolve, reject) => {
            const socket = new WebSocket(endpoint)
            socket.addEventListener('open', () => resolve(socket), { once: true })
            socket.addEventListener(
              'error',
              () => reject(new Error('websocket connection failed')),
              {
                once: true,
              }
            )
          })
        const nextMessage = (socket: WebSocket) =>
          new Promise<unknown[]>((resolve, reject) => {
            const timer = setTimeout(() => reject(new Error('websocket message timeout')), 10_000)
            socket.addEventListener(
              'message',
              (event) => {
                clearTimeout(timer)
                resolve(JSON.parse(String(event.data)))
              },
              { once: true }
            )
          })
        const first = await connect()
        const second = await connect()
        try {
          const topic = 'realtime:public:e2e-room'
          first.send(JSON.stringify([null, '1', topic, 'phx_join', {}]))
          await nextMessage(first)
          second.send(JSON.stringify([null, '2', topic, 'phx_join', {}]))
          await nextMessage(second)
          const broadcast = nextMessage(second)
          first.send(
            JSON.stringify([
              null,
              '3',
              topic,
              'broadcast',
              { event: 'message', payload: { text: 'hello' } },
            ])
          )
          return await broadcast
        } finally {
          first.close()
          second.close()
        }
      },
      { anonKey }
    )

    expect(received[3]).toBe('broadcast')
    expect(received[4]).toEqual({ event: 'message', payload: { text: 'hello' } })
  })

  test('Realtime streams database inserts to a service-role subscription', async ({
    page,
    request,
  }) => {
    const serviceRoleKey = process.env.GOLARA_SERVICE_ROLE_KEY!
    const table = `golara_realtime_probe_${Date.now()}`
    const marker = randomUUID()
    const query = async (sql: string) => {
      const response = await request.post('/api/platform/pg-meta/default/query', {
        data: { query: sql },
      })
      expect(response.status(), await response.text()).toBe(200)
    }

    await query(
      `create table public.${table} (id bigint generated by default as identity primary key, marker text not null)`
    )
    try {
      await page.goto('/project/default')
      const received = await page.evaluate(
        async ({ marker, serviceRoleKey, table }) => {
          const endpoint = `${location.origin.replace(/^http/, 'ws')}/api/projects/default/realtime/v1/websocket?apikey=${encodeURIComponent(serviceRoleKey)}&vsn=1.0.0`
          const socket = await new Promise<WebSocket>((resolve, reject) => {
            const value = new WebSocket(endpoint)
            value.addEventListener('open', () => resolve(value), { once: true })
            value.addEventListener(
              'error',
              () => reject(new Error('websocket connection failed')),
              {
                once: true,
              }
            )
          })
          const nextMessage = () =>
            new Promise<unknown[]>((resolve, reject) => {
              const timer = setTimeout(() => reject(new Error('database change timeout')), 10_000)
              socket.addEventListener(
                'message',
                (event) => {
                  clearTimeout(timer)
                  resolve(JSON.parse(String(event.data)))
                },
                { once: true }
              )
            })
          try {
            const topic = `realtime:public:${table}`
            socket.send(
              JSON.stringify([
                null,
                '1',
                topic,
                'phx_join',
                {
                  config: {
                    postgres_changes: [{ event: 'INSERT', schema: 'public', table }],
                  },
                },
              ])
            )
            const joined = await nextMessage()
            if ((joined[4] as { status?: string }).status !== 'ok') {
              throw new Error(`postgres subscription rejected: ${JSON.stringify(joined)}`)
            }
            const pending = nextMessage()
            const inserted = await fetch(
              `/api/projects/default/rest/v1/${encodeURIComponent(table)}`,
              {
                method: 'POST',
                headers: {
                  apikey: serviceRoleKey,
                  Authorization: `Bearer ${serviceRoleKey}`,
                  'Content-Type': 'application/json',
                  Prefer: 'return=representation',
                },
                body: JSON.stringify({ marker }),
              }
            )
            if (!inserted.ok) throw new Error(`insert failed: ${inserted.status}`)
            return await pending
          } finally {
            socket.close()
          }
        },
        { marker, serviceRoleKey, table }
      )
      expect(received[3]).toBe('postgres_changes')
      expect(received[4]).toMatchObject({ data: { type: 'INSERT', record: { marker } } })
    } finally {
      await query(`drop table if exists public.${table}`)
    }
  })

  test('Edge Function deploys, executes in Deno, lists, and deletes', async ({ request }) => {
    const serviceRoleKey = process.env.GOLARA_SERVICE_ROLE_KEY!
    const slug = `e2e-${Date.now()}`
    const source = `Deno.serve(async (request) => {
  const payload = await request.json()
  return Response.json({ message: \`Hello \${payload.name}\`, runtime: 'deno' }, { status: 201 })
})\n`

    try {
      const deployed = await request.post(
        `/api/v1/projects/default/functions/deploy?slug=${slug}`,
        {
          multipart: {
            metadata: JSON.stringify({ entrypoint_path: 'index.ts', verify_jwt: false }),
            file: { name: 'index.ts', mimeType: 'text/plain', buffer: Buffer.from(source) },
          },
        }
      )
      expect(deployed.status()).toBe(201)
      await expect(deployed.json()).resolves.toMatchObject({ slug, status: 'ACTIVE', version: 1 })

      const listed = await request.get('/api/v1/projects/default/functions')
      expect(listed.status()).toBe(200)
      expect((await listed.json()).some((item: { slug: string }) => item.slug === slug)).toBe(true)

      const invoked = await request.post(`/api/projects/default/functions/v1/${slug}`, {
        headers: { apikey: serviceRoleKey, 'content-type': 'application/json' },
        data: { name: 'Canary' },
      })
      expect(invoked.status()).toBe(201)
      await expect(invoked.json()).resolves.toEqual({ message: 'Hello Canary', runtime: 'deno' })
    } finally {
      const deleted = await request.delete(`/api/v1/projects/default/functions/${slug}`)
      expect([200, 404]).toContain(deleted.status())
    }
  })

  test('database cron job schedules, executes, and cleans up', async ({ request }) => {
    const suffix = Date.now().toString()
    const table = `golara_e2e_job_${suffix}`
    const job = `golara-e2e-${suffix}`
    let jobId: number | undefined
    const query = async (sql: string) => {
      const response = await request.post('/api/platform/pg-meta/default/query', {
        data: { query: sql },
      })
      expect(response.status(), await response.text()).toBe(200)
      return (await response.json()) as Record<string, unknown>[]
    }

    try {
      await query(`create table public.${table} (ran_at timestamptz not null default now())`)
      const scheduled = await query(
        `select cron.schedule('${job}', '1 second', $$insert into public.${table} default values$$) as jobid`
      )
      jobId = Number(scheduled[0]?.jobid)
      expect(jobId).toBeGreaterThan(0)

      await expect
        .poll(
          async () =>
            Number((await query(`select count(*)::int as count from public.${table}`))[0]?.count),
          {
            timeout: 15_000,
          }
        )
        .toBeGreaterThan(0)
    } finally {
      if (jobId) await query(`select cron.unschedule(${jobId})`)
      await query(`drop table if exists public.${table}`)
    }
  })
})
