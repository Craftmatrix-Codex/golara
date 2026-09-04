import { timingSafeEqual } from 'node:crypto'
import {
  sentryGlobalFunctionMiddleware,
  sentryGlobalRequestMiddleware,
} from '@sentry/tanstackstart-react'
import { createMiddleware, createStart } from '@tanstack/react-start'

import { BASE_PATH, IS_PLATFORM } from '@/lib/constants'
import { isHostedSupportedApiPath } from '@/lib/hosted-api-allowlist'

// Self-hosted-only API routes must 404 in platform (hosted) mode. Under the
// Next pages router this lives in middleware (proxy.ts), but TanStack Start
// has no middleware runtime, so the guard is migrated here as a global
// request middleware sharing the same allowlist (lib/hosted-api-allowlist.ts).
// On Vercel our `/api/*` (and `/_serverFn/*`) requests are rewritten to the
// api/server.js function which runs the Start handler, so createStartHandler
// runs this server-side for every API request — even though pages are served
// as a static SPA shell. The guard therefore covers all API routes from a
// single place.

const studioAuthMiddleware = createMiddleware({ type: 'request' }).server(({ request, next }) => {
  const { pathname } = new URL(request.url)
  if (process.env.SUPADATA_STUDIO_BUILDING === '1') return next()
  if (pathname === '/api/get-utc-time' || pathname === '/health') return next()

  const username = process.env.SUPADATA_STUDIO_AUTH_USERNAME
  const password = process.env.SUPADATA_STUDIO_AUTH_PASSWORD
  if (!username || !password) {
    return new Response('Studio authentication is not configured', { status: 503 })
  }

  const encoded = request.headers.get('authorization')?.match(/^Basic (.+)$/i)?.[1]
  let valid = false
  if (encoded) {
    try {
      const decoded = Buffer.from(encoded, 'base64').toString('utf8')
      const expected = `${username}:${password}`
      const actualBytes = Buffer.from(decoded)
      const expectedBytes = Buffer.from(expected)
      valid =
        actualBytes.length === expectedBytes.length && timingSafeEqual(actualBytes, expectedBytes)
    } catch {
      valid = false
    }
  }
  if (!valid) {
    return new Response('Authentication required', {
      status: 401,
      headers: { 'www-authenticate': 'Basic realm="Supadata Studio", charset="UTF-8"' },
    })
  }
  return next()
})

const platformApiGuard = createMiddleware({ type: 'request' }).server(({ request, next }) => {
  const { pathname } = new URL(request.url)
  // Path relative to the configured basePath — mirrors Next's basePath-
  // relative middleware matcher.
  const relativePath =
    BASE_PATH && pathname.startsWith(BASE_PATH) ? pathname.slice(BASE_PATH.length) : pathname

  if (IS_PLATFORM && relativePath.startsWith('/api/') && !isHostedSupportedApiPath(relativePath)) {
    return Response.json(
      { success: false, message: 'Endpoint not supported on hosted' },
      { status: 404 }
    )
  }

  return next()
})

// Sentry's global middlewares go at the FRONT so they wrap the whole request /
// server-function lifecycle — including errors that downstream code swallows
// into a 500, which the manual `@sentry/nextjs` approach never sees. The SDK's
// Vite plugin can auto-wrap these arrays instead, but we disable that
// (`autoInstrumentMiddleware: false` in vite.config.ts) and wire them
// explicitly so the instrumentation is visible in source.
export const startInstance = createStart(() => ({
  requestMiddleware: [sentryGlobalRequestMiddleware, studioAuthMiddleware, platformApiGuard],
  functionMiddleware: [sentryGlobalFunctionMiddleware],
}))
