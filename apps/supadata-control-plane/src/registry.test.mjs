import assert from 'node:assert/strict'
import { chmod, mkdir, mkdtemp, readFile, writeFile } from 'node:fs/promises'
import os from 'node:os'
import path from 'node:path'
import test from 'node:test'

import { createRegistry, slugify } from './registry.mjs'

test('slugify creates stable project ids', () => {
  assert.equal(slugify('  Sales & Analytics  '), 'sales-analytics')
})

test('registry creates isolated project compose files and switches projects', async () => {
  const dataDir = await mkdtemp(path.join(os.tmpdir(), 'supadata-'))
  const registry = await createRegistry({ dataDir })

  const first = await registry.createProject({ name: 'Project Alpha' })
  const second = await registry.createProject({ name: 'Project Beta', id: 'beta' })

  assert.equal(first.id, 'project-alpha')
  assert.equal(first.current, true)
  assert.equal(second.current, false)
  assert.deepEqual((await registry.currentProject()).id, 'project-alpha')

  const compose = await readFile(
    path.join(dataDir, 'projects', 'project-alpha', 'compose.yml'),
    'utf8'
  )
  assert.match(compose, /services:/)
  assert.match(compose, /studio:/)
  assert.match(compose, /rest:/)
  assert.match(compose, /GLOBAL_S3_BUCKET: \$\{GLOBAL_S3_BUCKET\}/)
  assert.match(compose, /GLOBAL_S3_ENDPOINT: http:\/\/supadata-seaweedfs:8333/)
  assert.doesNotMatch(compose, /^  seaweedfs:/m)
  assert.match(compose, /supadata-storage/)
  assert.match(compose, /supavisor:/)
  assert.match(compose, /meta:/)
  assert.match(compose, /META_HTTP_PORT/)
  assert.equal(typeof first.metaPort, 'number')

  await registry.selectProject('beta')
  assert.equal((await registry.currentProject()).id, 'beta')
  assert.equal(
    (await registry.listProjects()).find((project) => project.id === 'beta').current,
    true
  )
})

test('registry rejects duplicate projects', async () => {
  const dataDir = await mkdtemp(path.join(os.tmpdir(), 'supadata-'))
  const registry = await createRegistry({ dataDir })
  await registry.createProject({ name: 'Duplicate' })
  await assert.rejects(() => registry.createProject({ name: 'Duplicate' }), /already exists/)
})

test('registry provisions and deletes a project through compose', async () => {
  const dataDir = await mkdtemp(path.join(os.tmpdir(), 'supadata-'))
  const registry = await createRegistry({
    dataDir,
    composeCommand: 'true',
    storageMode: 'isolated',
  })
  await registry.createProject({ name: 'Provisioned' })

  assert.equal((await registry.provisionProject('provisioned')).status, 'running')
  await registry.deleteProject('provisioned')
  assert.deepEqual(await registry.listProjects(), [])
})

test('registry deletes legacy projects with invalid nested compose interpolation', async () => {
  const dataDir = await mkdtemp(path.join(os.tmpdir(), 'supadata-legacy-'))
  const projectDir = path.join(dataDir, 'projects', 'create-proof')
  const composeFile = path.join(projectDir, 'compose.yml')
  const composeCommand = path.join(dataDir, 'compose-check.sh')
  await mkdir(projectDir, { recursive: true })
  await writeFile(composeFile, 'services:\n  api-gw:\n    ports:\n      - ${API_GW_HTTP_PORT:-${KONG_HTTP_PORT:-8000}}:8000/tcp\n')
  await writeFile(path.join(dataDir, 'registry.json'), `${JSON.stringify({ currentProjectId: 'create-proof', projects: [{ id: 'create-proof', name: 'Create Proof', status: 'running', composeFile, composeEnvFile: path.join(projectDir, '.env') }] })}\n`)
  await writeFile(composeCommand, '#!/bin/sh\nfile=""\nwhile [ "$#" -gt 0 ]; do\n  [ "$1" = "--file" ] && file="$2"\n  shift\ndone\nif grep -q \'${API_GW_HTTP_PORT:-${KONG_HTTP_PORT:-8000}}\' "$file"; then\n  echo "invalid interpolation format" >&2\n  exit 1\nfi\nexit 0\n')
  await chmod(composeCommand, 0o755)
  const registry = await createRegistry({ dataDir, composeCommand, databaseMode: 'isolated' })
  await registry.deleteProject('create-proof')
  assert.deepEqual(await registry.listProjects(), [])
  assert.equal(await readFile(composeFile, 'utf8'), 'services:\n  api-gw:\n    ports:\n      - ${API_GW_HTTP_PORT:-8000}:8000/tcp\n')
})

test('shared database mode uses one postgres service and isolated databases', async () => {
  const dataDir = await mkdtemp(path.join(os.tmpdir(), 'supadata-shared-'))
  const registry = await createRegistry({ dataDir, databaseMode: 'shared' })
  const project = await registry.createProject({ name: 'Shared Analytics' })
  const compose = await readFile(project.composeFile, 'utf8')
  const env = await readFile(project.composeEnvFile, 'utf8')
  const shared = await readFile(path.join(dataDir, 'shared-postgres', 'compose.yml'), 'utf8')

  assert.doesNotMatch(compose, /^  db:\n/m)
  assert.match(env, /^POSTGRES_HOST=supadata-postgres$/m)
  assert.match(compose, /POSTGRES_DB: \$\{POSTGRES_DB\}/)
  assert.match(shared, /supabase\/postgres/)
  assert.match(shared, /supadata-postgres/)
})

test('registry rotates project credentials without listing secrets', async () => {
  const dataDir = await mkdtemp(path.join(os.tmpdir(), 'supadata-credentials-'))
  const registry = await createRegistry({ dataDir, composeCommand: 'true', databaseMode: 'isolated' })
  await registry.createProject({ name: 'Credentials' })
  const api = await registry.rotateProjectCredential('credentials', 'api-key')
  const deploy = await registry.rotateProjectCredential('credentials', 'deployable-password')
  assert.equal(api.type, 'api-key')
  assert.equal(deploy.type, 'deployable-password')
  assert.notEqual(api.value, deploy.value)
  assert.equal('apiKey' in (await registry.listProjects())[0], false)
  assert.equal((await registry.getProjectCredentials('credentials')).postgres.database, 'credentials')
})
