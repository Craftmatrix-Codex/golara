import { createClient } from '@supabase/supabase-js'

// This file is inspected by TypeScript; it is never executed or sent to a server.
const client = createClient('https://inventory.invalid', 'inventory-only')
export const surfaces = {
  client,
  auth: client.auth,
  authMFA: client.auth.mfa,
  authOAuth: client.auth.oauth,
  authAdmin: client.auth.admin,
  authAdminMFA: client.auth.admin.mfa,
  authAdminOAuth: client.auth.admin.oauth,
  authAdminCustomProviders: client.auth.admin.customProviders,
  authAdminPasskey: client.auth.admin.passkey,
  database: client.from('inventory'),
  databaseQuery: client.from('inventory').select(),
  functions: client.functions,
  realtime: client.realtime,
  realtimeChannel: client.channel('inventory'),
  storage: client.storage,
  storageFiles: client.storage.from('inventory'),
  storageVectors: client.storage.vectors,
  storageVectorBucket: client.storage.vectors.from('inventory'),
  storageVectorIndex: client.storage.vectors.from('inventory').index('inventory'),
  storageAnalytics: client.storage.analytics,
}
