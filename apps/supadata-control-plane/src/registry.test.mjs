import assert from 'node:assert/strict'
import { mkdtemp, readFile } from 'node:fs/promises'
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
