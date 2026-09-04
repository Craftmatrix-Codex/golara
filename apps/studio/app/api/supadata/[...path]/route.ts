import { proxySupadataRequest } from '../supadataProxy'

const controlPlaneUrl = process.env.SUPADATA_CONTROL_PLANE_URL || 'http://control-plane:8090'

async function handle(request: Request, context: { params: Promise<{ path: string[] }> }) {
  const { path } = await context.params
  return proxySupadataRequest(request, {
    baseUrl: controlPlaneUrl,
    token: process.env.SUPADATA_CONTROL_PLANE_TOKEN,
    path: `/${path.join('/')}`,
  })
}

export const GET = handle
export const POST = handle
export const DELETE = handle
export const PATCH = handle
export const PUT = handle
