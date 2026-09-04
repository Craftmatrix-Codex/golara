import { execFile } from 'node:child_process'
import { randomBytes, randomUUID } from 'node:crypto'
import { cp, mkdir, mkdtemp, readFile, rename, rm, writeFile } from 'node:fs/promises'
import net from 'node:net'
import os from 'node:os'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { promisify } from 'node:util'

const PROJECT_ID = /^[a-z0-9]+(?:-[a-z0-9]+)*$/
const execFileAsync = promisify(execFile)
const REPOSITORY_ROOT = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../../..')
const SETUP_SCRIPT = path.join(REPOSITORY_ROOT, 'docker', 'setup.sh')
const BUNDLED_ENVOY_ENTRYPOINT =
  process.env.SUPADATA_ENVOY_ENTRYPOINT ||
  path.join(REPOSITORY_ROOT, 'docker', 'volumes', 'api', 'envoy', 'docker-entrypoint.sh')
const LEGACY_NESTED_GATEWAY_PORT = '${API_GW_HTTP_PORT:-${KONG_HTTP_PORT:-8000}}'
const VALID_GATEWAY_PORT = '${API_GW_HTTP_PORT:-8000}'

async function portAvailable(port) {
  try {
    const { stdout } = await execFileAsync('docker', ['ps', '--format', '{{.Ports}}'])
    if ([...stdout.matchAll(/:(\d+)->/g)].some((match) => Number(match[1]) === port)) return false
  } catch {
    // Fall back to a socket probe when Docker is unavailable in unit tests.
  }
  return await new Promise((resolve) => {
    const server = net.createServer()
    server.once('error', () => resolve(false))
    server.listen(port, '0.0.0.0', () => server.close(() => resolve(true)))
  })
}

async function findPort(start, count) {
  for (let port = start; ; port += 10) {
    const available = await Promise.all(
      Array.from({ length: count }, (_, i) => portAvailable(port + i))
    )
    if (available.every(Boolean)) return port
  }
}

export function slugify(value) {
  return String(value)
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '')
    .slice(0, 48)
}

function secret() {
  return `${randomUUID().replaceAll('-', '')}${randomUUID().replaceAll('-', '')}`
}

function envValue(value) {
  return String(value).replaceAll('\\', '\\\\').replaceAll('\n', '\\n')
}

function upsertEnv(contents, values) {
  const lines = contents.split('\n')
  for (const [key, value] of Object.entries(values)) {
    const index = lines.findIndex((line) => line.startsWith(`${key}=`))
    const replacement = `${key}=${envValue(value)}`
    if (index >= 0) lines[index] = replacement
    else lines.push(replacement)
  }
  return `${lines.filter((line, index, all) => !(line === '' && index === all.length - 1)).join('\n')}\n`
}

async function repairLegacyCompose(composeFile) {
  const contents = await readFile(composeFile, 'utf8')
  const repaired = contents.replaceAll(LEGACY_NESTED_GATEWAY_PORT, VALID_GATEWAY_PORT)
  if (repaired !== contents) await writeFile(composeFile, repaired, 'utf8')
}

function isolateCompose(contents, project, { storageMode }) {
  const seaweed =
    storageMode === 'shared'
      ? ''
      : `  seaweedfs:\n    image: chrislusf/seaweedfs:3.80\n    command: server -dir=/data -s3\n    restart: unless-stopped\n    volumes:\n      - ${project.id}-seaweedfs:/data\n    expose: [8333]\n\n`
  const storageHost = storageMode === 'shared' ? 'supadata-seaweedfs' : 'seaweedfs'
  return contents
    .replace(/^name: supabase\n/m, '')
    .replace(
      /^  studio:\n/m,
      '  studio:\n    extra_hosts:\n      - "host.docker.internal:host-gateway"\n'
    )
    .replace(/^    container_name: .*\n/gm, '')
    .replace(
      /^  api-gw:\n/m,
      `  api-gw:\n    # Public entrypoint for ${project.id}; all other services stay private.\n`
    )
    .replace(
      '      STUDIO_PG_META_URL: http://meta:8080',
      '      STUDIO_PG_META_URL: ${SUPADATA_META_PROXY_URL:-http://meta:8080}'
    )
    .replace(
      '      SUPABASE_URL: http://api-gw:8000',
      '      SUPABASE_URL: ${SUPADATA_PROXY_URL:-http://api-gw:8000}'
    )
    .replace(/^  meta:\n/m, `  meta:\n    ports:\n      - \${META_HTTP_PORT}:8080\n`)
    .replace(
      /^  storage:\n/m,
      storageMode === 'shared'
        ? `${seaweed}  storage:\n    networks:\n      - default\n      - supadata-storage\n`
        : `${seaweed}  storage:\n`
    )
    .replace(
      /      STORAGE_BACKEND: file\n/,
      `      STORAGE_BACKEND: s3\n      GLOBAL_S3_ENDPOINT: http://${storageHost}:8333\n      GLOBAL_S3_PROTOCOL: http\n      GLOBAL_S3_FORCE_PATH_STYLE: \"true\"\n      AWS_ACCESS_KEY_ID: \${S3_PROTOCOL_ACCESS_KEY_ID}\n      AWS_SECRET_ACCESS_KEY: \${S3_PROTOCOL_ACCESS_KEY_SECRET}\n`
    )
    .replace(
      /(^  storage:\n[\s\S]*?)\n    depends_on:\n      db:\n        # Disable this if you are using an external Postgres database\n        condition: service_healthy\n      rest:/m,
      (_match, prefix) =>
        storageMode === 'shared'
          ? `${prefix}\n    depends_on:\n      rest:`
          : `${prefix}\n    depends_on:\n      db:\n        # Disable this if you are using an external Postgres database\n        condition: service_healthy\n      seaweedfs:\n        condition: service_started\n      rest:`
    )
    .replaceAll(':***@', ':${POSTGRES_PASSWORD}@')
    .replace(/\$\{POSTGRES_PORT\}:5432/g, '${POOLER_PUBLIC_PORT}:5432')
    .replace(/\nvolumes:\n$/, `\nvolumes:\n`)
    .replace(
      /  db-config:\n  deno-cache:\n/,
      storageMode === 'shared'
        ? '  db-config:\n  deno-cache:\n'
        : `  db-config:\n  deno-cache:\n  ${project.id}-seaweedfs:\n`
    )
}

export async function createRegistry({
  dataDir,
  composeCommand = 'docker-compose',
  databaseMode = process.env.SUPADATA_DATABASE_MODE || 'isolated',
  storageMode = process.env.SUPADATA_STORAGE_MODE || 'shared',
  publicHost = process.env.SUPADATA_PUBLIC_HOST || '13.140.160.208',
  publicProtocol = process.env.SUPADATA_PUBLIC_PROTOCOL || 'http',
}) {
  const registryPath = path.join(dataDir, 'registry.json')
  const projectsDir = path.join(dataDir, 'projects')
  const sharedDatabaseDir = path.join(dataDir, 'shared-postgres')
  const sharedStorageDir = path.join(dataDir, 'shared-seaweedfs')

  async function readRegistry() {
    try {
      return JSON.parse(await readFile(registryPath, 'utf8'))
    } catch (error) {
      if (error.code === 'ENOENT') return { currentProjectId: null, projects: [] }
      throw error
    }
  }

  async function saveRegistry(registry) {
    const temporaryPath = `${registryPath}.${process.pid}.${randomUUID()}.tmp`
    await writeFile(temporaryPath, `${JSON.stringify(registry, null, 2)}\n`, 'utf8')
    await rename(temporaryPath, registryPath)
  }

  let sharedInfrastructurePromise

  async function waitForHealthy(container) {
    for (let attempt = 0; attempt < 60; attempt += 1) {
      const { stdout } = await execFileAsync('docker', [
        'inspect',
        '-f',
        '{{.State.Health.Status}}',
        container,
      ])
      if (stdout.trim() === 'healthy') return
      if (stdout.trim() === 'unhealthy') throw new Error(`${container} became unhealthy`)
      await new Promise((resolve) => setTimeout(resolve, 2000))
    }
    throw new Error(`${container} did not become healthy within 120 seconds`)
  }

  async function ensureSharedInfrastructure() {
    if (!sharedInfrastructurePromise) {
      sharedInfrastructurePromise = (async () => {
        if (storageMode === 'shared') {
          await execFileAsync(
            composeCommand,
            [
              '--project-name',
              'supadata-shared-storage',
              '--env-file',
              path.join(sharedStorageDir, '.env'),
              '--file',
              path.join(sharedStorageDir, 'compose.yml'),
              'up',
              '--detach',
            ],
            { cwd: sharedStorageDir }
          )
          await waitForHealthy('supadata-seaweedfs')
        }
        if (databaseMode === 'shared') {
          await execFileAsync(
            composeCommand,
            [
              '--project-name',
              'supadata-shared',
              '--env-file',
              path.join(sharedDatabaseDir, '.env'),
              '--file',
              path.join(sharedDatabaseDir, 'compose.yml'),
              'up',
              '--detach',
            ],
            { cwd: sharedDatabaseDir }
          )
          await waitForHealthy('supadata-postgres')
        }
      })().catch((error) => {
        sharedInfrastructurePromise = undefined
        throw error
      })
    }
    return sharedInfrastructurePromise
  }

  let sharedDatabaseMutation = Promise.resolve()

  async function ensureProjectDatabase(id) {
    const operation = sharedDatabaseMutation.then(async () => {
      const databaseCheck = await execFileAsync('docker', [
        'exec',
        'supadata-postgres',
        'psql',
        '-U',
        'supabase_admin',
        '-d',
        'postgres',
        '-Atc',
        `SELECT 1 FROM pg_database WHERE datname = '${id}'`,
      ])
      if (databaseCheck.stdout.trim()) return
      await execFileAsync('docker', [
        'exec',
        'supadata-postgres',
        'psql',
        '-U',
        'supabase_admin',
        '-d',
        'postgres',
        '-c',
        `ALTER DATABASE postgres CONNECTION LIMIT 0; SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = 'postgres' AND pid <> pg_backend_pid()`,
      ])
      await execFileAsync('docker', [
        'exec',
        'supadata-postgres',
        'psql',
        '-U',
        'supabase_admin',
        '-d',
        'postgres',
        '-c',
        `CREATE DATABASE "${id}" TEMPLATE postgres`,
      ])
      await execFileAsync('docker', [
        'exec',
        'supadata-postgres',
        'psql',
        '-U',
        'supabase_admin',
        '-d',
        id,
        '-v',
        'ON_ERROR_STOP=1',
        '-c',
        `DO $do$ BEGIN IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'supabase_auth_admin') THEN CREATE ROLE supabase_auth_admin; END IF; IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'supabase_storage_admin') THEN CREATE ROLE supabase_storage_admin; END IF; IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'supabase_functions_admin') THEN CREATE ROLE supabase_functions_admin; END IF; IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'authenticator') THEN CREATE ROLE authenticator; END IF; END $do$; CREATE SCHEMA IF NOT EXISTS auth AUTHORIZATION supabase_auth_admin; CREATE SCHEMA IF NOT EXISTS extensions AUTHORIZATION supabase_admin; CREATE SCHEMA IF NOT EXISTS graphql_public AUTHORIZATION supabase_admin; CREATE SCHEMA IF NOT EXISTS _realtime AUTHORIZATION supabase_admin; CREATE SCHEMA IF NOT EXISTS realtime AUTHORIZATION supabase_admin; GRANT ALL PRIVILEGES ON DATABASE "${id}" TO supabase_storage_admin; GRANT ALL PRIVILEGES ON DATABASE "${id}" TO supabase_auth_admin; GRANT ALL PRIVILEGES ON DATABASE "${id}" TO supabase_admin; GRANT USAGE, CREATE ON SCHEMA public, extensions, graphql_public, realtime TO authenticator, supabase_admin, supabase_auth_admin, supabase_functions_admin, supabase_storage_admin; ALTER DATABASE "${id}" SET search_path TO public, extensions, realtime`,
      ])
      await execFileAsync('docker', [
        'exec',
        'supadata-postgres',
        'psql',
        '-U',
        'supabase_admin',
        '-d',
        'postgres',
        '-c',
        'ALTER DATABASE postgres CONNECTION LIMIT -1',
      ])
    })
    sharedDatabaseMutation = operation.catch(() => undefined)
    return operation
  }

  async function createFullStack(project, projectDir, port) {
    const setupWorkspace = await mkdtemp(path.join(os.tmpdir(), 'supadata-setup-'))
    const setupRoot = `supabase-${project.id}`
    try {
      await execFileAsync(
        'sh',
        [SETUP_SCRIPT, '-y', '--skip-deps', '--project-dir', setupRoot, '--ref', 'stable'],
        {
          cwd: setupWorkspace,
          stdio: 'ignore',
          env: {
            ...process.env,
            SUPABASE_LOCAL_DOCKER_DIR: path.dirname(SETUP_SCRIPT),
            SUPABASE_REPO_URL: 'https://github.com/renzaspiras/supabase',
          },
        }
      )
      const generatedDir = path.join(setupWorkspace, setupRoot)
      await cp(generatedDir, projectDir, { recursive: true })
      const generatedEntrypoint = path.join(projectDir, 'volumes/api/envoy/docker-entrypoint.sh')
      await rm(generatedEntrypoint, { recursive: true, force: true })
      await cp(BUNDLED_ENVOY_ENTRYPOINT, generatedEntrypoint)
      const generatedComposeFile = path.join(projectDir, 'docker-compose.yml')
      const composeFile = path.join(projectDir, 'compose.yml')
      const envFile = path.join(projectDir, '.env')
      const [compose, env] = await Promise.all([
        readFile(generatedComposeFile, 'utf8'),
        readFile(envFile, 'utf8'),
      ])
      let sharedPassword
      let sharedStorageAccessKey
      let sharedStorageSecretKey
      if (databaseMode === 'shared') {
        await mkdir(path.join(sharedDatabaseDir, 'volumes', 'db'), { recursive: true })
        await cp(
          path.join(projectDir, 'volumes', 'db'),
          path.join(sharedDatabaseDir, 'volumes', 'db'),
          {
            recursive: true,
            force: true,
          }
        )
        await writeFile(
          path.join(sharedDatabaseDir, 'volumes', 'db', '00-supadata-roles.sql'),
          `SELECT 'CREATE DATABASE _supabase WITH OWNER supabase_admin' WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = '_supabase')\\gexec\n\\connect _supabase\nCREATE SCHEMA IF NOT EXISTS _analytics AUTHORIZATION supabase_admin;\nCREATE SCHEMA IF NOT EXISTS _supavisor AUTHORIZATION supabase_admin;\n\\connect postgres\n\\set pgpass \`echo "$POSTGRES_PASSWORD"\`\nDO $do$ BEGIN\n  IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'authenticator') THEN CREATE ROLE authenticator LOGIN; END IF;\n  IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'pgbouncer') THEN CREATE ROLE pgbouncer LOGIN; END IF;\n  IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'supabase_admin') THEN CREATE ROLE supabase_admin LOGIN; END IF;\n  IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'supabase_auth_admin') THEN CREATE ROLE supabase_auth_admin LOGIN; END IF;\n  IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'supabase_functions_admin') THEN CREATE ROLE supabase_functions_admin LOGIN; END IF;\n  IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'supabase_storage_admin') THEN CREATE ROLE supabase_storage_admin LOGIN; END IF;\nEND $do$;\nALTER ROLE authenticator WITH PASSWORD :'pgpass';\nALTER ROLE pgbouncer WITH PASSWORD :'pgpass';\nALTER ROLE supabase_admin WITH PASSWORD :'pgpass';\nALTER ROLE supabase_auth_admin WITH PASSWORD :'pgpass';\nALTER ROLE supabase_functions_admin WITH PASSWORD :'pgpass';\nALTER ROLE supabase_storage_admin WITH PASSWORD :'pgpass';\n`,
          'utf8'
        )
        const sharedEnvFile = path.join(sharedDatabaseDir, '.env')
        try {
          const sharedEnv = await readFile(sharedEnvFile, 'utf8')
          sharedPassword = sharedEnv.match(/^POSTGRES_PASSWORD=(.+)$/m)?.[1]
        } catch (error) {
          if (error.code !== 'ENOENT') throw error
        }
        if (!sharedPassword) {
          sharedPassword = secret()
          await writeFile(
            sharedEnvFile,
            `POSTGRES_PASSWORD=${sharedPassword}\nJWT_SECRET=${secret()}\n`,
            {
              mode: 0o600,
            }
          )
          await writeFile(
            path.join(sharedDatabaseDir, 'compose.yml'),
            `services:\n  postgres:\n    image: supabase/postgres:17.6.1.136\n    container_name: supadata-postgres\n    restart: unless-stopped\n    environment:\n      POSTGRES_PASSWORD: \${POSTGRES_PASSWORD}\n      POSTGRES_DB: postgres\n    volumes:\n      - ./volumes/db/realtime.sql:/docker-entrypoint-initdb.d/migrations/99-realtime.sql:ro\n      - ./volumes/db/webhooks.sql:/docker-entrypoint-initdb.d/init-scripts/98-webhooks.sql:ro\n      - ./volumes/db/roles.sql:/docker-entrypoint-initdb.d/init-scripts/99-roles.sql:ro\n      - ./volumes/db/jwt.sql:/docker-entrypoint-initdb.d/init-scripts/99-jwt.sql:ro\n      - ./volumes/db/_supabase.sql:/docker-entrypoint-initdb.d/migrations/97-_supabase.sql:ro\n      - ./volumes/db/logs.sql:/docker-entrypoint-initdb.d/migrations/99-logs.sql:ro\n      - ./volumes/db/pooler.sql:/docker-entrypoint-initdb.d/migrations/99-pooler.sql:ro\n      - ./volumes/db/00-supadata-roles.sql:/docker-entrypoint-initdb.d/init-scripts/00-supadata-roles.sql:ro\n      - supadata-postgres-data:/var/lib/postgresql/data\n    networks: [supadata-postgres]\n    healthcheck:\n      test: [\"CMD\", \"pg_isready\", \"-U\", \"supabase_admin\", \"-d\", \"postgres\"]\n      interval: 5s\n      timeout: 5s\n      retries: 20\nnetworks:\n  supadata-postgres:\n    name: supadata-postgres\nvolumes:\n  supadata-postgres-data:\n`,
            'utf8'
          )
        }
      }
      if (storageMode === 'shared') {
        await mkdir(path.join(sharedStorageDir, 'data'), { recursive: true })
        const sharedStorageEnvFile = path.join(sharedStorageDir, '.env')
        let sharedStorageEnv = ''
        try {
          sharedStorageEnv = await readFile(sharedStorageEnvFile, 'utf8')
        } catch (error) {
          if (error.code !== 'ENOENT') throw error
        }
        sharedStorageAccessKey = sharedStorageEnv.match(/^S3_ACCESS_KEY=(.+)$/m)?.[1]
        sharedStorageSecretKey = sharedStorageEnv.match(/^S3_SECRET_KEY=(.+)$/m)?.[1]
        if (!sharedStorageAccessKey || !sharedStorageSecretKey) {
          sharedStorageAccessKey = 'supadata_shared'
          sharedStorageSecretKey = secret()
          await writeFile(
            sharedStorageEnvFile,
            `S3_ACCESS_KEY=${sharedStorageAccessKey}\nS3_SECRET_KEY=${sharedStorageSecretKey}\n`,
            { mode: 0o600 }
          )
          await writeFile(
            path.join(sharedStorageDir, 's3.json'),
            JSON.stringify(
              {
                identities: [
                  {
                    name: 'supadata',
                    credentials: [
                      { accessKey: sharedStorageAccessKey, secretKey: sharedStorageSecretKey },
                    ],
                    actions: ['Read', 'Write', 'List', 'Tagging'],
                  },
                ],
              },
              null,
              2
            ) + '\n',
            { mode: 0o600 }
          )
          await writeFile(
            path.join(sharedStorageDir, 'compose.yml'),
            `services:\n  seaweedfs:\n    image: chrislusf/seaweedfs:3.80\n    container_name: supadata-seaweedfs\n    command: server -dir=/data -s3 -s3.config=/etc/seaweedfs/s3.json\n    restart: unless-stopped\n    volumes:\n      - ./data:/data\n      - ./s3.json:/etc/seaweedfs/s3.json:ro\n    expose: [8333]
    networks: [supadata-storage]\n    healthcheck:\n      test: [\"CMD\", \"weed\", \"version\"]\n      interval: 5s\n      timeout: 5s\n      retries: 20\nnetworks:\n  supadata-storage:\n    name: supadata-storage\n`,
            'utf8'
          )
        }
      }
      const gatewayPort = port
      const postgresPort = port + 1
      const poolerPort = port + 2
      const values = {
        STUDIO_DEFAULT_ORGANIZATION: 'Supadata',
        STUDIO_DEFAULT_PROJECT: project.name,
        API_EXTERNAL_URL: `${publicProtocol}://${publicHost}:${gatewayPort}`,
        SUPABASE_PUBLIC_URL: `${publicProtocol}://${publicHost}:${gatewayPort}`,
        SITE_URL: `${publicProtocol}://${publicHost}:${gatewayPort}`,
        API_GW_HTTP_PORT: gatewayPort,
        KONG_HTTP_PORT: gatewayPort,
        POSTGRES_PORT: 5432,
        POOLER_TENANT_ID: project.id,
        POOLER_PUBLIC_PORT: postgresPort,
        META_HTTP_PORT: port + 3,
        POOLER_PROXY_PORT_TRANSACTION: poolerPort,
        POSTGRES_DB: project.id,
        GLOBAL_S3_BUCKET: `supadata-${project.id}`,
        STORAGE_TENANT_ID: project.id,
        MINIO_ROOT_USER: `supadata_${project.id.replaceAll('-', '_')}`,
        MINIO_ROOT_PASSWORD: secret(),
        S3_PROTOCOL_ACCESS_KEY_ID:
          storageMode === 'shared'
            ? sharedStorageAccessKey
            : `supadata_${project.id.replaceAll('-', '_')}`,
        S3_PROTOCOL_ACCESS_KEY_SECRET: storageMode === 'shared' ? sharedStorageSecretKey : secret(),
        SUPABASE_PROJECT_ID: project.id,
        SUPADATA_PROXY_URL:
          process.env.SUPADATA_PROXY_URL || 'http://host.docker.internal:8090/proxy',
        SUPADATA_META_PROXY_URL:
          process.env.SUPADATA_META_PROXY_URL || 'http://host.docker.internal:8090/proxy-meta',
      }
      let projectCompose = isolateCompose(compose, project, { storageMode })
      const projectValues =
        databaseMode === 'shared'
          ? { ...values, POSTGRES_HOST: 'supadata-postgres', POSTGRES_PASSWORD: sharedPassword }
          : values
      if (databaseMode === 'shared') {
        projectCompose = projectCompose
          .replaceAll('PG_META_DB_USER: postgres', 'PG_META_DB_USER: supabase_admin')
          .replaceAll(
            'POSTGRES_USER_READ_WRITE: postgres',
            'POSTGRES_USER_READ_WRITE: supabase_admin'
          )
          .replace(/\n  db:\n[\s\S]*?\n  supavisor:/, '\n  supavisor:')
          .replace(
            /\n    depends_on:\n      db:\n(?:        #.*\n)?        condition: service_healthy\n/g,
            '\n'
          )
          .replace(/\n      seaweedfs:\n/, '\n    depends_on:\n      seaweedfs:\n')
          .concat(
            storageMode === 'shared'
              ? '\nnetworks:\n  default:\n    name: supadata-postgres\n    external: true\n  supadata-storage:\n    name: supadata-storage\n    external: true\n'
              : ''
          )
      }
      if (storageMode === 'shared' && databaseMode !== 'shared') {
        projectCompose = projectCompose.concat(
          '\nnetworks:\n  supadata-storage:\n    name: supadata-storage\n    external: true\n'
        )
      }
      await writeFile(composeFile, projectCompose, 'utf8')
      await writeFile(envFile, upsertEnv(env, projectValues), { mode: 0o600 })
      return {
        composeFile,
        composeEnvFile: envFile,
        gatewayPort,
        postgresPort,
        metaPort: port + 3,
        poolerPort,
      }
    } finally {
      await rm(setupWorkspace, { recursive: true, force: true })
    }
  }

  async function updateProject(id, update) {
    const registry = await readRegistry()
    const index = registry.projects.findIndex((project) => project.id === id)
    if (index < 0) throw new Error(`project '${id}' not found`)
    registry.projects[index] = { ...registry.projects[index], ...update }
    await saveRegistry(registry)
    return registry.projects[index]
  }

  async function getProjectCredentials(id) {
    const registry = await readRegistry()
    const project = registry.projects.find((candidate) => candidate.id === id)
    if (!project) throw new Error(`project '${id}' not found`)
    const env = Object.fromEntries(
      (await readFile(project.composeEnvFile, 'utf8')).split('\n').flatMap((line) => {
        const m = line.match(/^([A-Z0-9_]+)=(.*)$/)
        return m ? [[m[1], m[2]]] : []
      })
    )
    return {
      id,
      apiKey: null,
      deployablePassword: null,
      postgres: {
        host: publicHost,
        port: Number(env.POOLER_PUBLIC_PORT || env.POSTGRES_PORT || project.postgresPort),
        database: env.POSTGRES_DB || id,
        username: databaseMode === 'shared' ? `supadata_${id.replaceAll('-', '_')}` : 'postgres',
        password: env.SUPADATA_POSTGRES_PASSWORD || env.POSTGRES_PASSWORD || null,
      },
    }
  }

  async function rotateProjectCredential(id, type) {
    const registry = await readRegistry()
    const project = registry.projects.find((candidate) => candidate.id === id)
    if (!project) throw new Error(`project '${id}' not found`)
    const normalized = type === 'apiKey' ? 'api-key' : type
    const key = {
      'api-key': 'SUPADATA_API_KEY',
      'deployable-password': 'SUPADATA_DEPLOYABLE_PASSWORD',
      postgres: 'SUPADATA_POSTGRES_PASSWORD',
      'postgres-password': 'SUPADATA_POSTGRES_PASSWORD',
    }[normalized]
    if (!key) throw new Error('credential type must be api-key, deployable-password, or postgres')
    const value = randomBytes(32).toString('base64url')
    await writeFile(
      project.composeEnvFile,
      upsertEnv(await readFile(project.composeEnvFile, 'utf8'), { [key]: value }),
      { mode: 0o600 }
    )
    return { type: normalized === 'postgres-password' ? 'postgres' : normalized, value }
  }

  async function createProject({ name, id: requestedId }) {
    const cleanName = String(name ?? '').trim()
    if (!cleanName) throw new Error('name is required')
    const id = slugify(requestedId || cleanName)
    if (!PROJECT_ID.test(id)) throw new Error('id must be lowercase kebab-case')
    const registry = await readRegistry()
    if (registry.projects.some((project) => project.id === id))
      throw new Error(`project '${id}' already exists`)
    const projectDir = path.join(projectsDir, id)
    const project = {
      id,
      name: cleanName,
      status: 'provisioning',
      createdAt: new Date().toISOString(),
    }
    await mkdir(projectDir, { recursive: true })
    const port = await findPort(8100, 4)
    const stack = await createFullStack(project, projectDir, port)
    Object.assign(project, stack)
    registry.projects.push(project)
    if (!registry.currentProjectId) registry.currentProjectId = id
    await saveRegistry(registry)
    return { ...project, current: registry.currentProjectId === id }
  }

  async function listProjects() {
    const registry = await readRegistry()
    return registry.projects.map((project) => ({
      ...project,
      current: project.id === registry.currentProjectId,
    }))
  }

  async function selectProject(id) {
    if (!PROJECT_ID.test(id)) throw new Error('invalid project id')
    const registry = await readRegistry()
    const project = registry.projects.find((candidate) => candidate.id === id)
    if (!project) throw new Error(`project '${id}' not found`)
    registry.currentProjectId = id
    await saveRegistry(registry)
    return project
  }

  async function currentProject() {
    const registry = await readRegistry()
    return registry.projects.find((project) => project.id === registry.currentProjectId) ?? null
  }

  async function provisionProject(id) {
    const project = (await listProjects()).find((candidate) => candidate.id === id)
    if (!project) throw new Error(`project '${id}' not found`)
    await updateProject(id, { status: 'provisioning', error: null })
    try {
      await ensureSharedInfrastructure()
      if (databaseMode === 'shared') {
        await ensureProjectDatabase(id)
      }
      await execFileAsync(
        composeCommand,
        [
          '--project-name',
          `supadata-${id}`,
          '--env-file',
          project.composeEnvFile,
          '--file',
          project.composeFile,
          'up',
          '--detach',
        ],
        { cwd: path.dirname(project.composeFile) }
      )
      return updateProject(id, { status: 'running', error: null })
    } catch (error) {
      return updateProject(id, {
        status: 'error',
        error: error instanceof Error ? error.message : String(error),
      })
    }
  }

  async function deleteProject(id) {
    const project = (await listProjects()).find((candidate) => candidate.id === id)
    if (!project) throw new Error(`project '${id}' not found`)
    if (project.status === 'running' || project.status === 'provisioning') {
      await repairLegacyCompose(project.composeFile)
      await execFileAsync(
        composeCommand,
        [
          '--project-name',
          `supadata-${id}`,
          '--env-file',
          project.composeEnvFile,
          '--file',
          project.composeFile,
          'down',
          '--volumes',
        ],
        { cwd: path.dirname(project.composeFile) }
      )
    }
    if (databaseMode === 'shared') {
      try {
        await execFileAsync('docker', [
          'exec',
          'supadata-postgres',
          'psql',
          '-U',
          'supabase_admin',
          '-d',
          'postgres',
          '-c',
          `DROP DATABASE IF EXISTS "${id}"`,
        ])
      } catch (error) {
        if (
          !/No such container: supadata-postgres/.test(
            error instanceof Error ? error.message : String(error)
          )
        )
          throw error
      }
    }
    const registry = await readRegistry()
    registry.projects = registry.projects.filter((candidate) => candidate.id !== id)
    if (registry.currentProjectId === id)
      registry.currentProjectId = registry.projects[0]?.id ?? null
    await saveRegistry(registry)
  }

  await mkdir(projectsDir, { recursive: true })
  return {
    createProject,
    listProjects,
    selectProject,
    currentProject,
    provisionProject,
    deleteProject,
    getProjectCredentials,
    rotateProjectCredential,
  }
}
