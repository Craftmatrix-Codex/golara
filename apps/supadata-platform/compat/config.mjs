import { readFileSync } from 'node:fs'
import { createClient } from '@supabase/supabase-js'

const clients = new Set()

export function validateConfig(input) {
  if (!input.publicURL) throw new Error('publicURL must be explicitly configured')
  const url = new URL(input.publicURL)
  const local = ['localhost', '127.0.0.1', '[::1]'].includes(url.hostname)
  if (url.protocol !== 'https:' && !(local && url.protocol === 'http:')) {
    throw new Error('Live publicURL must use HTTPS')
  }
  if (url.username || url.password || url.search || url.hash || url.pathname !== '/') {
    throw new Error('publicURL must be an origin without credentials, path, query, or fragment')
  }
  if (!input.anonKey || !input.serviceRoleKey)
    throw new Error('anonKey and serviceRoleKey are required')
  if (!/^golara_sdk_[a-z0-9_]+$/.test(input.namespace ?? '')) {
    throw new Error('A disposable golara_sdk_ namespace is required')
  }
  const project = input.project ?? 'default'
  if (!/^[a-zA-Z0-9_-]+$/.test(project)) throw new Error('Invalid project reference')
  return {
    ...input,
    publicURL: url.origin,
    project,
    sdkURL: `${url.origin}/api/projects/${project}`,
  }
}

export function loadConfig() {
  if (!process.env.GOLARA_CONFIG_FILE)
    throw new Error('GOLARA_CONFIG_FILE must name a protected test configuration file')
  return validateConfig(JSON.parse(readFileSync(process.env.GOLARA_CONFIG_FILE, 'utf8')))
}

export function sdkClient(config, role = 'anon') {
  const client = createClient(
    config.sdkURL,
    role === 'service' ? config.serviceRoleKey : config.anonKey,
    {
      auth: { persistSession: false, autoRefreshToken: false, detectSessionInUrl: false },
    }
  )
  clients.add(client)
  return client
}

export async function disposeClients() {
  for (const client of clients) {
    await client.removeAllChannels()
    await client.realtime.disconnect()
    await client.auth.dispose()
  }
  clients.clear()
}
