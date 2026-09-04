import { createServer } from 'node:http'

import { hasValidBearerToken } from './auth.mjs'
import { createRegistry } from './registry.mjs'

function jsonResponse(status, payload, origin = '*') {
  return new Response(status === 204 ? null : JSON.stringify(payload), {
    status,
    headers: {
      'access-control-allow-headers': 'content-type, authorization, apikey, x-client-info',
      'access-control-allow-methods': 'DELETE,GET,OPTIONS,POST,PATCH,PUT',
      'access-control-allow-origin': origin,
      'content-type': 'application/json; charset=utf-8',
    },
  })
}

function credentialMetadata(credentials) {
  return Object.fromEntries(
    Object.entries(credentials).flatMap(([name, value]) => {
      if (!value || typeof value !== 'object') return [[name, value]]
      const { password: _password, value: _value, ...metadata } = value
      return [[name, metadata]]
    })
  )
}

async function parseBody(request) {
  const text = await request.text()
  if (!text) return {}
  try {
    return JSON.parse(text)
  } catch {
    throw new Error('request body must be valid JSON')
  }
}

async function proxy(request, targetBase, targetPath) {
  const headers = new Headers(request.headers)
  headers.delete('host')
  headers.delete('content-length')
  const upstream = await fetch(`${targetBase}${targetPath}`, {
    method: request.method,
    headers,
    body:
      request.method === 'GET' || request.method === 'HEAD'
        ? undefined
        : await request.arrayBuffer(),
  })
  return new Response(await upstream.arrayBuffer(), {
    status: upstream.status,
    headers: {
      'access-control-allow-origin': process.env.SUPADATA_ALLOWED_ORIGIN || '*',
      'content-type': upstream.headers.get('content-type') || 'application/octet-stream',
    },
  })
}

export function createControlPlaneHandler(
  registry,
  {
    token = process.env.SUPADATA_CONTROL_PLANE_TOKEN,
    proxyHost = process.env.SUPADATA_PROXY_TARGET_HOST ||
      process.env.SUPADATA_PUBLIC_HOST ||
      '127.0.0.1',
  } = {}
) {
  return async function handle(request) {
    const origin = process.env.SUPADATA_ALLOWED_ORIGIN || '*'
    try {
      const url = new URL(request.url)
      const parts = url.pathname.split('/').filter(Boolean)
      if (request.method === 'OPTIONS') return jsonResponse(204, {}, origin)
      if (request.method === 'GET' && url.pathname === '/health')
        return jsonResponse(200, { status: 'ok' }, origin)

      const isProjectManagement = url.pathname.startsWith('/api/projects')
      const isProxy = url.pathname === '/proxy' || url.pathname.startsWith('/proxy/')
      const isMetaProxy = url.pathname === '/proxy-meta' || url.pathname.startsWith('/proxy-meta/')
      if (
        (isProjectManagement || isProxy || isMetaProxy) &&
        !hasValidBearerToken(token, request.headers.get('authorization'))
      )
        return jsonResponse(401, { error: 'unauthorized' }, origin)

      if (request.method === 'GET' && url.pathname === '/api/projects')
        return jsonResponse(200, { projects: await registry.listProjects() }, origin)
      if (request.method === 'GET' && url.pathname === '/api/projects/current')
        return jsonResponse(200, { project: await registry.currentProject() }, origin)
      if (request.method === 'POST' && url.pathname === '/api/projects')
        return jsonResponse(
          201,
          { project: await registry.createProject(await parseBody(request)) },
          origin
        )
      if (
        request.method === 'GET' &&
        parts.length === 4 &&
        parts[0] === 'api' &&
        parts[1] === 'projects' &&
        parts[3] === 'credentials'
      )
        return jsonResponse(
          200,
          credentialMetadata(await registry.getProjectCredentials(parts[2])),
          origin
        )
      if (
        request.method === 'POST' &&
        parts.length === 6 &&
        parts[0] === 'api' &&
        parts[1] === 'projects' &&
        parts[3] === 'credentials' &&
        parts[5] === 'rotate'
      )
        return jsonResponse(200, await registry.rotateProjectCredential(parts[2], parts[4]), origin)
      if (
        request.method === 'POST' &&
        parts.length === 4 &&
        parts[0] === 'api' &&
        parts[1] === 'projects' &&
        parts[3] === 'provision'
      )
        return jsonResponse(200, { project: await registry.provisionProject(parts[2]) }, origin)
      if (
        request.method === 'POST' &&
        parts.length === 4 &&
        parts[0] === 'api' &&
        parts[1] === 'projects' &&
        parts[3] === 'select'
      )
        return jsonResponse(
          200,
          { project: { ...(await registry.selectProject(parts[2])), current: true } },
          origin
        )
      if (
        request.method === 'DELETE' &&
        parts.length === 3 &&
        parts[0] === 'api' &&
        parts[1] === 'projects'
      ) {
        await registry.deleteProject(parts[2])
        return jsonResponse(204, {}, origin)
      }

      const project = await registry.currentProject()
      if (project && isProxy)
        return proxy(
          request,
          `http://${proxyHost}:${project.gatewayPort}`,
          url.pathname.slice('/proxy'.length) || '/'
        )
      if (project && isMetaProxy)
        return proxy(
          request,
          `http://${proxyHost}:${project.metaPort}`,
          url.pathname.slice('/proxy-meta'.length) || '/'
        )
      return jsonResponse(404, { error: 'not found' }, origin)
    } catch (error) {
      const message = error instanceof Error ? error.message : 'request failed'
      const status = /required|invalid|already exists|not found|valid JSON/.test(message)
        ? 400
        : 502
      return jsonResponse(status, { error: message }, origin)
    }
  }
}

const port = Number(process.env.PORT || 8090)
const dataDir = process.env.SUPADATA_DATA_DIR || './.supadata'
const registry = await createRegistry({
  dataDir,
  composeCommand: process.env.SUPADATA_COMPOSE_COMMAND || 'docker-compose',
})
const handler = createControlPlaneHandler(registry)
const server = createServer(async (request, response) => {
  const chunks = []
  for await (const chunk of request) chunks.push(chunk)
  const webRequest = new Request(`http://${request.headers.host || 'localhost'}${request.url}`, {
    method: request.method,
    headers: request.headers,
    body:
      chunks.length && request.method !== 'GET' && request.method !== 'HEAD'
        ? Buffer.concat(chunks)
        : undefined,
  })
  const result = await handler(webRequest)
  response.writeHead(result.status, Object.fromEntries(result.headers))
  response.end(Buffer.from(await result.arrayBuffer()))
})

if (process.argv[1] === new URL(import.meta.url).pathname) {
  server.listen(port, () =>
    console.log(`Supadata control plane listening on http://localhost:${port}`)
  )
  process.on('SIGTERM', () => server.close())
  process.on('SIGINT', () => server.close())
}
