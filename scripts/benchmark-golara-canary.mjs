import { mkdir, writeFile } from 'node:fs/promises'
import path from 'node:path'
import { performance } from 'node:perf_hooks'

const baseURL = (process.env.GOLARA_BASE_URL ?? 'https://go-alpha.craftmatrix.org').replace(
  /\/$/,
  ''
)
const username = process.env.GOLARA_STUDIO_USERNAME
const password = process.env.GOLARA_STUDIO_PASSWORD
const anonKey = process.env.GOLARA_ANON_KEY
const iterations = Number(process.env.GOLARA_BENCH_ITERATIONS ?? 12)

if (!username || !password || !anonKey) {
  throw new Error(
    'GOLARA_STUDIO_USERNAME, GOLARA_STUDIO_PASSWORD, and GOLARA_ANON_KEY are required'
  )
}
if (!Number.isInteger(iterations) || iterations < 3 || iterations > 100) {
  throw new Error('GOLARA_BENCH_ITERATIONS must be an integer between 3 and 100')
}

const basic = `Basic ${Buffer.from(`${username}:${password}`).toString('base64')}`
const checks = [
  { name: 'Studio root', path: '/', headers: { Authorization: basic } },
  {
    name: 'Project overview',
    path: '/project/default',
    headers: { Authorization: basic },
  },
  {
    name: 'Auth users shell',
    path: '/project/default/auth/users',
    headers: { Authorization: basic },
  },
  {
    name: 'Project registry API',
    path: '/api/platform/projects',
    headers: { Authorization: basic },
  },
  { name: 'Auth health', path: '/auth/v1/health', headers: {} },
  {
    name: 'Project GraphQL',
    path: '/api/projects/default/graphql/v1',
    method: 'POST',
    headers: { apikey: anonKey, 'content-type': 'application/json' },
    body: JSON.stringify({ query: '{ __typename }' }),
  },
]

const percentile = (values, quantile) => {
  const sorted = [...values].sort((a, b) => a - b)
  return sorted[Math.min(sorted.length - 1, Math.ceil(sorted.length * quantile) - 1)]
}

const results = []
for (const check of checks) {
  const timings = []
  const statuses = []
  let bytes = 0

  for (let index = 0; index < iterations; index += 1) {
    const started = performance.now()
    const response = await fetch(`${baseURL}${check.path}`, {
      method: check.method ?? 'GET',
      headers: check.headers,
      body: check.body,
      redirect: 'manual',
    })
    const body = await response.arrayBuffer()
    timings.push(performance.now() - started)
    statuses.push(response.status)
    bytes = body.byteLength
  }

  results.push({
    name: check.name,
    path: check.path,
    samples: iterations,
    statuses: [...new Set(statuses)],
    bytes,
    minMs: Number(Math.min(...timings).toFixed(1)),
    medianMs: Number(percentile(timings, 0.5).toFixed(1)),
    p95Ms: Number(percentile(timings, 0.95).toFixed(1)),
    maxMs: Number(Math.max(...timings).toFixed(1)),
  })
}

const generatedAt = new Date().toISOString()
const report = { baseURL, generatedAt, iterations, results }
const outputDir = path.resolve('artifacts/golara-canary/benchmarks')
await mkdir(outputDir, { recursive: true })
await writeFile(path.join(outputDir, 'latest.json'), `${JSON.stringify(report, null, 2)}\n`)
await writeFile(
  path.join(outputDir, 'latest.md'),
  [
    '# Golara Canary benchmark',
    '',
    `Generated: ${generatedAt}`,
    `Samples per endpoint: ${iterations}`,
    '',
    '| Flow | Status | Bytes | Min | Median | p95 | Max |',
    '| --- | ---: | ---: | ---: | ---: | ---: | ---: |',
    ...results.map(
      (result) =>
        `| ${result.name} | ${result.statuses.join(', ')} | ${result.bytes} | ${result.minMs} ms | ${result.medianMs} ms | ${result.p95Ms} ms | ${result.maxMs} ms |`
    ),
    '',
  ].join('\n')
)

console.log(JSON.stringify(report, null, 2))
if (results.some((result) => result.statuses.some((status) => status >= 400))) process.exitCode = 1
