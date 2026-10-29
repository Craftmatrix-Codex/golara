import assert from 'node:assert/strict'
import { createHmac } from 'node:crypto'
import { readFileSync, writeFileSync } from 'node:fs'
import { performance } from 'node:perf_hooks'

import { loadConfig } from './config.mjs'

const cfg = loadConfig()
assert.equal(
  cfg.publicURL,
  'http://127.0.0.1:18080',
  'Benchmarks only target the disposable local harness'
)
assert.ok(process.env.GOLARA_LOCAL_ENV_FILE, 'A protected local environment file is required')
const env = Object.fromEntries(
  readFileSync(process.env.GOLARA_LOCAL_ENV_FILE, 'utf8')
    .split('\n')
    .filter((l) => l.includes('='))
    .map((l) => {
      const i = l.indexOf('=')
      return [l.slice(0, i), l.slice(i + 1)]
    })
)
assert.ok(env.JWT_SECRET)
const encode = (v) => Buffer.from(JSON.stringify(v)).toString('base64url')
const payload = `${encode({ alg: 'HS256', typ: 'JWT' })}.${encode({ role: 'service_role', exp: Math.floor(Date.now() / 1000) + 3600 })}`
const referenceToken = `${payload}.${createHmac('sha256', env.JWT_SECRET).update(payload).digest('base64url')}`
const targets = {
  golara: {
    url: 'http://golara-sdk-slim:8090/api/projects/default/rest/v1',
    token: cfg.serviceRoleKey,
  },
  postgrest: { url: 'http://golara-sdk-reference:3000', token: referenceToken },
}
const workloads = [
  { name: 'read_100', path: `/${cfg.namespace}?select=id,name,quantity&order=id.asc&limit=100` },
  { name: 'filtered_read', path: `/${cfg.namespace}?select=id,name,quantity&quantity=eq.123` },
  {
    name: 'head_exact_count',
    path: `/${cfg.namespace}?quantity=gte.1000`,
    method: 'HEAD',
    headers: { Prefer: 'count=exact' },
  },
  {
    name: 'rpc',
    path: `/rpc/${cfg.namespace}_echo`,
    method: 'POST',
    body: JSON.stringify({ message: 'benchmark' }),
  },
]
const request = async (target, workload) => {
  const response = await fetch(target.url + workload.path, {
    method: workload.method ?? 'GET',
    body: workload.body,
    headers: {
      apikey: cfg.serviceRoleKey,
      Authorization: `Bearer ${target.token}`,
      Accept: 'application/json',
      'Content-Type': 'application/json',
      ...workload.headers,
    },
    signal: AbortSignal.timeout(5000),
  })
  const text = await response.text()
  assert.ok(
    response.status === 200 || response.status === 206,
    `Benchmark request returned ${response.status}`
  )
  if (workload.method === 'HEAD') {
    assert.match(response.headers.get('content-range') ?? '', /\/1001$/)
    return null
  }
  return JSON.parse(text)
}
const results = []
for (const workload of workloads) {
  assert.deepEqual(
    await request(targets.golara, workload),
    await request(targets.postgrest, workload),
    'Reference and GoLara response data must match before measuring'
  )
  for (const concurrency of [1, 10]) {
    for (let round = 1; round <= 4; round++) {
      const order = round % 2 === 1 ? ['postgrest', 'golara'] : ['golara', 'postgrest']
      for (const name of order) {
        const target = targets[name]
        for (let i = 0; i < 20; i++) await request(target, workload)
        const samples = [],
          count = 200
        let next = 0
        const start = performance.now()
        await Promise.all(
          Array.from({ length: concurrency }, async () => {
            while (next++ < count) {
              const t = performance.now()
              await request(target, workload)
              samples.push(performance.now() - t)
            }
          })
        )
        const elapsed = performance.now() - start
        samples.sort((a, b) => a - b)
        const percentile = (p) =>
          samples[Math.min(samples.length - 1, Math.ceil(samples.length * p) - 1)]
        results.push({
          round,
          target: name,
          workload: workload.name,
          concurrency,
          requests: samples.length,
          p50_ms: percentile(0.5),
          p95_ms: percentile(0.95),
          requests_per_second: (1000 * samples.length) / elapsed,
        })
      }
    }
  }
}
const report = {
  scope:
    'Local direct REST/RPC endpoints on the same PostgreSQL fixture. Excludes Auth, Storage, Realtime, Functions and Studio. This is not a whole-Supabase performance comparison.',
  reference: 'postgrest/postgrest:v14.12 (pinned by repository docker/docker-compose.yml)',
  fixture_rows: 2000,
  rounds: 4,
  ordering: 'Alternating target order each round, with 20 warmup requests before each measurement',
  results,
}
if (process.env.GOLARA_BENCH_REPORT)
  writeFileSync(process.env.GOLARA_BENCH_REPORT, JSON.stringify(report, null, 2) + '\n')
console.log(JSON.stringify(report, null, 2))
