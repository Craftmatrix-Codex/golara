import { timingSafeEqual } from 'node:crypto'

export function hasValidBearerToken(configuredToken, authorization) {
  if (!configuredToken || typeof authorization !== 'string') return false
  const prefix = 'Bearer '
  if (!authorization.startsWith(prefix)) return false
  const provided = Buffer.from(authorization.slice(prefix.length))
  const expected = Buffer.from(configuredToken)
  return provided.length === expected.length && timingSafeEqual(provided, expected)
}
