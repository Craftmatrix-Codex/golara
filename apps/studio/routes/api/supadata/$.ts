import { createFileRoute } from '@tanstack/react-router'

import { proxySupadataRequest } from '@/app/api/supadata/supadataProxy'

const controlPlaneUrl = process.env.SUPADATA_CONTROL_PLANE_URL || 'http://control-plane:8090'

async function handle({ request, params }: { request: Request; params: { _splat?: string } }) {
  const path = params._splat ? `/${params._splat}` : '/'
  return proxySupadataRequest(request, {
    baseUrl: controlPlaneUrl,
    token: process.env.SUPADATA_CONTROL_PLANE_TOKEN,
    path,
  })
}

export const Route = createFileRoute('/api/supadata/$')({
  server: {
    handlers: {
      GET: handle,
      POST: handle,
      DELETE: handle,
      PATCH: handle,
      PUT: handle,
      OPTIONS: handle,
    },
  },
})
