import { createServer } from 'node:http'

import { createRegistry } from './registry.mjs'

const port = Number(process.env.PORT || 8090)
const dataDir = process.env.SUPADATA_DATA_DIR || './.supadata'
const registry = await createRegistry({
  dataDir,
  composeCommand: process.env.SUPADATA_COMPOSE_COMMAND || 'docker-compose',
})

async function body(request) {
  const chunks = []
  for await (const chunk of request) chunks.push(chunk)
  if (!chunks.length) return {}
  try {
    return JSON.parse(Buffer.concat(chunks).toString('utf8'))
  } catch {
    throw new Error('request body must be valid JSON')
  }
}

function send(response, status, payload) {
  response.writeHead(status, {
    'access-control-allow-headers': 'content-type, authorization, apikey, x-client-info',
    'access-control-allow-methods': 'DELETE,GET,OPTIONS,POST,PATCH,PUT',
    'access-control-allow-origin': process.env.SUPADATA_ALLOWED_ORIGIN || '*',
    'content-type': 'application/json; charset=utf-8',
  })
  response.end(status === 204 ? '' : JSON.stringify(payload))
}

async function proxy(request, response, targetBase, targetPath) {
  const headers = new Headers()
  for (const [key, value] of Object.entries(request.headers)) {
    if (key !== 'host' && key !== 'content-length' && typeof value === 'string')
      headers.set(key, value)
  }
  const chunks = []
  for await (const chunk of request) chunks.push(chunk)
  const upstream = await fetch(`${targetBase}${targetPath}`, {
    method: request.method,
    headers,
    body: chunks.length ? Buffer.concat(chunks) : undefined,
  })
  response.writeHead(upstream.status, {
    'access-control-allow-origin': process.env.SUPADATA_ALLOWED_ORIGIN || '*',
    'content-type': upstream.headers.get('content-type') || 'application/octet-stream',
  })
  response.end(Buffer.from(await upstream.arrayBuffer()))
}

const server = createServer(async (request, response) => {
  try {
    const url = new URL(request.url, `http://${request.headers.host || 'localhost'}`)
    const parts = url.pathname.split('/').filter(Boolean)
    if (request.method === 'OPTIONS') return send(response, 204, {})
    if (request.method === 'GET' && url.pathname === '/health')
      return send(response, 200, { status: 'ok' })
    if (request.method === 'GET' && url.pathname === '/api/projects')
      return send(response, 200, { projects: await registry.listProjects() })
    if (request.method === 'GET' && url.pathname === '/api/projects/current')
      return send(response, 200, { project: await registry.currentProject() })
    if (request.method === 'POST' && url.pathname === '/api/projects')
      return send(response, 201, { project: await registry.createProject(await body(request)) })
    if (
      request.method === 'POST' &&
      parts.length === 4 &&
      parts[0] === 'api' &&
      parts[1] === 'projects' &&
      parts[3] === 'provision'
    )
      return send(response, 200, { project: await registry.provisionProject(parts[2]) })
    if (
      request.method === 'POST' &&
      parts.length === 4 &&
      parts[0] === 'api' &&
      parts[1] === 'projects' &&
      parts[3] === 'select'
    )
      return send(response, 200, {
        project: { ...(await registry.selectProject(parts[2])), current: true },
      })
    if (
      request.method === 'DELETE' &&
      parts.length === 3 &&
      parts[0] === 'api' &&
      parts[1] === 'projects'
    ) {
      await registry.deleteProject(parts[2])
      return send(response, 204, {})
    }

    const project = await registry.currentProject()
    if (project && (url.pathname === '/proxy' || url.pathname.startsWith('/proxy/'))) {
      return proxy(
        request,
        response,
        `http://127.0.0.1:${project.gatewayPort}`,
        url.pathname.slice('/proxy'.length) || '/'
      )
    }
    if (project && (url.pathname === '/proxy-meta' || url.pathname.startsWith('/proxy-meta/'))) {
      return proxy(
        request,
        response,
        `http://127.0.0.1:${project.metaPort}`,
        url.pathname.slice('/proxy-meta'.length) || '/'
      )
    }
    return send(response, 404, { error: 'not found' })
  } catch (error) {
    const status = /required|invalid|already exists|not found|valid JSON/.test(error.message)
      ? 400
      : 502
    return send(response, status, { error: error.message })
  }
})

server.listen(port, () =>
  console.log(`Supadata control plane listening on http://localhost:${port}`)
)
process.on('SIGTERM', () => server.close())
process.on('SIGINT', () => server.close())
