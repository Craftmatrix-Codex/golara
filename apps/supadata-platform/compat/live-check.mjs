import { spawnSync } from 'node:child_process'
import { randomUUID } from 'node:crypto'
import { chmodSync, mkdirSync, mkdtempSync, realpathSync, writeFileSync } from 'node:fs'
import { dirname, join, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { loadConfig } from './config.mjs'
import { assertStableAttestation, redactLog, summarizeStage } from './live-evidence-lib.mjs'

const root = resolve(dirname(fileURLToPath(import.meta.url)), '../../..')
const cfg = loadConfig()
const revision = process.env.GOLARA_EXPECTED_REVISION ?? ''
if (cfg.publicURL !== 'https://golara.craftmatrix.org' || cfg.project !== 'default')
  throw new Error('This Stable verification targets only the configured default project')
if (!/^[a-f0-9]{40}$/.test(revision))
  throw new Error('GOLARA_EXPECTED_REVISION must be the full deployed Git SHA')
if (!cfg.studioUsername || !cfg.studioPassword || !cfg.bucket)
  throw new Error('Studio credentials and the disposable storage bucket are required')
const configPath = realpathSync(process.env.GOLARA_CONFIG_FILE)
const outputBase = join(root, 'output', 'playwright', 'golara-live-receipts')
mkdirSync(outputBase, { recursive: true, mode: 0o700 })
chmodSync(outputBase, 0o700)
const output = mkdtempSync(join(outputBase, `${revision.slice(0, 10)}-`))
chmodSync(output, 0o700)
const python = `import json, subprocess
items=[]
for line in subprocess.check_output(['docker','ps','--format','json']).decode().splitlines():
 x=json.loads(line)
 if x['Names'].startswith('poslsmflcfylqkhdl131pj6o-'):
  d=json.loads(subprocess.check_output(['docker','inspect',x['Names']]))[0]
  mounts=[m.get('Name') for m in d['Mounts'] if m['Destination']=='/var/lib/supadata']
  items.append({'id':d['Id'],'name':x['Names'],'image':d['Config']['Image'],'health':d['State'].get('Health',{}).get('Status'),'volume':mounts[0] if len(mounts)==1 else None})
print(json.dumps(items))`
const report = {
  target: cfg.publicURL,
  project: cfg.project,
  revision,
  startedAt: new Date().toISOString(),
  stages: {},
}
const save = () =>
  writeFileSync(join(output, 'report.json'), JSON.stringify(report, null, 2) + '\n', {
    mode: 0o600,
  })
function attest(initial) {
  const result = spawnSync(
    'ssh',
    [
      '-o',
      'BatchMode=yes',
      'craftmatrix-codex',
      'ssh',
      '-o',
      'BatchMode=yes',
      'app-vps',
      'python3',
      '-',
    ],
    { input: python, encoding: 'utf8', timeout: 30000 }
  )
  if (result.status !== 0) throw new Error('Could not read Stable container state through SSH')
  return assertStableAttestation(JSON.parse(result.stdout), revision, initial)
}
function run(stage, browser) {
  const containerName = `golara-live-test-${randomUUID()}`
  const args = [
    'run',
    '--rm',
    '--name',
    containerName,
    '--network',
    'bridge',
    '-v',
    `${root}:/src:ro`,
    '-v',
    `${configPath}:/run/golara-config.json:ro`,
    '-w',
    '/src',
    '-e',
    'GOLARA_CONFIG_FILE=/run/golara-config.json',
  ]
  if (browser)
    args.push(
      '--shm-size=512m',
      '-v',
      `${output}:/evidence`,
      '-e',
      'PLAYWRIGHT_BROWSERS_PATH=/ms-playwright'
    )
  args.push(
    'mcr.microsoft.com/playwright:v1.64.1-noble',
    'npm',
    'run',
    browser ? 'test:browser' : 'test:sdk',
    '--prefix',
    'apps/supadata-platform/compat'
  )
  if (browser) args.push('--', '--output=/evidence/browser')
  console.log(`Running ${stage} against ${cfg.publicURL}`)
  const started = Date.now()
  const result = spawnSync('docker', args, {
    encoding: 'utf8',
    timeout: 300000,
    maxBuffer: 4 * 1024 * 1024,
  })
  if (result.error) {
    const exists = spawnSync('docker', ['inspect', containerName], {
      stdio: 'ignore',
      timeout: 10000,
    })
    if (exists.status === 0) {
      const cleanup = spawnSync('docker', ['rm', '--force', containerName], {
        stdio: 'ignore',
        timeout: 30000,
      })
      if (cleanup.status !== 0) throw new Error('Timed-out test container could not be removed')
    }
  }
  const log = redactLog(`${result.stdout ?? ''}\n${result.stderr ?? ''}`, cfg)
  writeFileSync(join(output, `${stage}.log`), log, { mode: 0o600 })
  report.stages[stage] = {
    passed: false,
    exitCode: result.status,
    durationMs: Date.now() - started,
  }
  if (result.status === 0 && !result.error) {
    try {
      report.stages[stage].counts = summarizeStage(stage, log)
      report.stages[stage].passed = true
    } catch (error) {
      report.stages[stage].error = error.message
    }
  }
  save()
  console.log(
    `${stage}: ${report.stages[stage].passed ? 'passed' : 'failed'}; details saved locally`
  )
}
try {
  const initial = attest()
  report.attestation = { before: initial }
  save()
  run('sdk', false)
  report.attestation.afterSdk = attest(initial)
  save()
  run('browser', true)
  report.attestation.afterBrowser = attest(initial)
  report.passed = report.stages.sdk.passed && report.stages.browser.passed
} catch (error) {
  report.passed = false
  report.error = redactLog(error.message, cfg)
  console.error(`Live verification stopped: ${report.error}`)
} finally {
  report.finishedAt = new Date().toISOString()
  save()
  console.log(`Evidence: ${output}`)
}
if (!report.passed) process.exitCode = 1
