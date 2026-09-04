type FetchImplementation = typeof fetch

export async function proxySupadataRequest(
  request: Request,
  {
    baseUrl,
    token,
    path,
    fetchImpl = fetch,
  }: { baseUrl: string; token?: string; path: string; fetchImpl?: FetchImplementation }
) {
  if (!baseUrl || !token)
    return new Response(JSON.stringify({ error: 'Supadata proxy is not configured' }), {
      status: 503,
    })

  const headers = new Headers(request.headers)
  headers.delete('authorization')
  headers.set('authorization', `Bearer ${token}`)
  headers.delete('host')
  headers.delete('content-length')
  const response = await fetchImpl(`${baseUrl.replace(/\/$/, '')}${path}`, {
    method: request.method,
    headers,
    body:
      request.method === 'GET' || request.method === 'HEAD'
        ? undefined
        : await request.arrayBuffer(),
  })
  return new Response(await response.arrayBuffer(), {
    status: response.status,
    headers: { 'content-type': response.headers.get('content-type') ?? 'application/json' },
  })
}
