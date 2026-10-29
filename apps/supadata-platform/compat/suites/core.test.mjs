import assert from 'node:assert/strict'
import { randomUUID } from 'node:crypto'
import test, { after, before } from 'node:test'

import { disposeClients, loadConfig, sdkClient } from '../config.mjs'

const cfg = loadConfig()
const admin = sdkClient(cfg, 'service')
const marker = randomUUID()
after(disposeClients)
before(async () => {
  const deadline = Date.now() + 30000
  while (Date.now() < deadline) {
    try {
      const health = await fetch(`${cfg.publicURL}/health`, { signal: AbortSignal.timeout(2000) })
      if (health.status === 200) return
    } catch {}
    await new Promise((resolve) => setTimeout(resolve, 200))
  }
  throw new Error('Configured target did not become healthy within 30 seconds')
})

test('SDK Auth rejects invalid credentials with a usable AuthApiError', async () => {
  const { data, error } = await sdkClient(cfg).auth.signInWithPassword({
    email: `missing-${marker}@example.invalid`,
    password: 'invalid-password',
  })
  assert.equal(data.session, null)
  assert.equal(error?.name, 'AuthApiError')
  assert.match(error.message, /invalid.*credentials/i)
})

test('SDK Auth admin lists users with a valid pagination result', async () => {
  const { data, error } = await admin.auth.admin.listUsers({ page: 1, perPage: 2 })
  assert.equal(error, null)
  assert.ok(Array.isArray(data.users))
})

test('SDK Auth signup returns a user, enforces confirmation, and allows cleanup', async () => {
  const email = `golara-sdk-${marker}@example.invalid`
  const password = `Sdk-${marker}!`
  const client = sdkClient(cfg)
  const { data, error } = await client.auth.signUp({
    email,
    password,
    options: { data: { fixture: cfg.namespace } },
  })
  assert.equal(error, null)
  assert.equal(data.user?.email, email)
  assert.ok(data.user.id)
  try {
    if (!data.session) {
      const login = await client.auth.signInWithPassword({ email, password })
      assert.equal(login.data.session, null)
      assert.equal(login.error?.code, 'email_not_confirmed')
    }
  } finally {
    assert.equal((await admin.auth.admin.deleteUser(data.user.id)).error, null)
  }
})

test('SDK Auth admin creates a confirmed user; password, getUser, refresh, and signOut work', async () => {
  const client = sdkClient(cfg)
  const email = `golara-sdk-session-${marker}@example.invalid`
  const password = `Sdk-${marker}!`
  const created = await admin.auth.admin.createUser({
    email,
    password,
    email_confirm: true,
    user_metadata: { fixture: cfg.namespace },
  })
  assert.equal(created.error, null)
  const user = created.data.user
  try {
    const login = await client.auth.signInWithPassword({ email, password })
    assert.equal(login.error, null)
    assert.ok(login.data.session?.access_token)
    const current = await client.auth.getUser()
    assert.equal(current.error, null)
    assert.equal(current.data.user.id, user.id)
    const refresh = await client.auth.refreshSession()
    assert.equal(refresh.error, null)
    assert.notEqual(refresh.data.session.refresh_token, login.data.session.refresh_token)
    const replay = await sdkClient(cfg).auth.refreshSession({
      refresh_token: login.data.session.refresh_token,
    })
    assert.ok(replay.error, 'A rotated refresh token must not be reusable')
    assert.equal(replay.data.session, null)
    assert.equal((await client.auth.signOut()).error, null)
    assert.equal((await client.auth.getSession()).data.session, null)
    const revokedUser = await sdkClient(cfg).auth.getUser(refresh.data.session.access_token)
    assert.ok(revokedUser.error, 'A revoked session must fail server-side user lookup')
    const revokedRefresh = await sdkClient(cfg).auth.refreshSession({
      refresh_token: refresh.data.session.refresh_token,
    })
    assert.ok(revokedRefresh.error, 'A revoked session must not refresh')
    assert.equal(revokedRefresh.data.session, null)
  } finally {
    assert.equal((await admin.auth.admin.deleteUser(user.id)).error, null)
  }
})

test('SDK Auth signOut scopes revoke the intended sessions only', async () => {
  const email = `golara-sdk-scopes-${marker}@example.invalid`
  const password = `Sdk-${marker}!`
  const created = await admin.auth.admin.createUser({ email, password, email_confirm: true })
  assert.equal(created.error, null)
  const observer = sdkClient(cfg)
  const login = async () => {
    const client = sdkClient(cfg)
    const result = await client.auth.signInWithPassword({ email, password })
    assert.equal(result.error, null)
    return {
      client,
      token: result.data.session.access_token,
      refreshToken: result.data.session.refresh_token,
    }
  }
  const active = async (session) => (await observer.auth.getUser(session.token)).error === null
  try {
    const first = await login()
    const second = await login()
    assert.equal((await first.client.auth.signOut({ scope: 'others' })).error, null)
    assert.equal(await active(first), true, 'others must preserve the current session')
    assert.equal(await active(second), false, 'others must revoke other sessions')
    const secondRefresh = await sdkClient(cfg).auth.refreshSession({
      refresh_token: second.refreshToken,
    })
    assert.ok(secondRefresh.error, 'others must revoke the other session refresh token')
    const firstRefresh = await sdkClient(cfg).auth.refreshSession({
      refresh_token: first.refreshToken,
    })
    assert.equal(firstRefresh.error, null, 'others must preserve the current refresh token')
    const third = await login()
    assert.equal((await first.client.auth.signOut({ scope: 'local' })).error, null)
    assert.equal(await active(first), false, 'local must revoke the current session')
    assert.equal(await active(third), true, 'local must preserve other sessions')
    const fourth = await login()
    assert.equal((await third.client.auth.signOut()).error, null)
    assert.equal(await active(third), false, 'global must revoke the current session')
    assert.equal(await active(fourth), false, 'global must revoke other sessions')
    const fourthRefresh = await sdkClient(cfg).auth.refreshSession({
      refresh_token: fourth.refreshToken,
    })
    assert.ok(fourthRefresh.error, 'global must revoke other session refresh tokens')
  } finally {
    assert.equal((await admin.auth.admin.deleteUser(created.data.user.id)).error, null)
  }
})

test('Auth admin rejects malformed deletion requests before changing the user', async () => {
  const email = `golara-sdk-delete-${marker}@example.invalid`
  const password = `Sdk-${marker}!`
  const created = await admin.auth.admin.createUser({ email, password, email_confirm: true })
  assert.equal(created.error, null)
  const id = created.data.user.id
  try {
    for (const body of ['{} garbage', '{} {"should_soft_delete":true}']) {
      const response = await fetch(`${cfg.sdkURL}/auth/v1/admin/users/${id}`, {
        method: 'DELETE',
        headers: {
          apikey: cfg.serviceRoleKey,
          Authorization: `Bearer ${cfg.serviceRoleKey}`,
          'Content-Type': 'application/json',
        },
        body,
        signal: AbortSignal.timeout(10000),
      })
      await response.text()
      assert.equal(response.status, 400)
    }
    const client = sdkClient(cfg)
    const login = await client.auth.signInWithPassword({ email, password })
    assert.equal(login.error, null, 'Malformed deletion must not remove the account')
    assert.equal(login.data.user.id, id)
    assert.equal((await client.auth.signOut()).error, null)
  } finally {
    assert.equal((await admin.auth.admin.deleteUser(id)).error, null)
  }
})

test('SDK database insert/select/update/upsert/delete returns real fixture rows', async () => {
  const row = { marker, name: 'sdk-contract', quantity: 3 }
  try {
    const insert = await admin.from(cfg.namespace).insert(row).select()
    assert.equal(insert.error, null)
    assert.equal(insert.data[0].marker, marker)
    const select = await admin
      .from(cfg.namespace)
      .select('marker,name,quantity')
      .eq('marker', marker)
    assert.equal(select.error, null)
    assert.equal(select.data.length, 1)
    assert.equal(select.data[0].quantity, 3)
    const update = await admin
      .from(cfg.namespace)
      .update({ quantity: 4 })
      .eq('marker', marker)
      .select()
    assert.equal(update.error, null)
    assert.equal(update.data[0].quantity, 4)
    const upsert = await admin
      .from(cfg.namespace)
      .upsert({ ...row, quantity: 5 }, { onConflict: 'marker' })
      .select()
    assert.equal(upsert.error, null)
    assert.equal(upsert.data[0].quantity, 5)
  } finally {
    assert.equal((await admin.from(cfg.namespace).delete().eq('marker', marker)).error, null)
  }
})

test('SDK database combines filters, quoted IN values, and repeated ordering', async () => {
  const rows = [
    { marker: `${marker}-filters,one`, name: 'Alpha first', quantity: 31 },
    { marker: `${marker}-filters-two`, name: 'Alpha second', quantity: 32 },
    { marker: `${marker}-filters-three`, name: 'blocked', quantity: 33 },
    { marker: `${marker}-filters-a`, name: 'Alpha tie', quantity: 32 },
  ]
  const markers = rows.map((row) => row.marker)
  try {
    assert.equal((await admin.from(cfg.namespace).insert(rows)).error, null)
    const quoted = await admin.from(cfg.namespace).select('marker').in('marker', markers)
    assert.equal(quoted.error, null)
    assert.deepEqual(quoted.data.map((row) => row.marker).sort(), [...markers].sort())
    const combined = await admin
      .from(cfg.namespace)
      .select('marker,name,quantity')
      .in('marker', markers)
      .gt('quantity', 31)
      .lte('quantity', 33)
      .not('name', 'eq', 'blocked')
      .or('quantity.eq.32,quantity.eq.33')
      .order('quantity', { ascending: false })
      .order('marker')
    assert.equal(combined.error, null)
    assert.deepEqual(combined.data, [rows[3], rows[1]])
    const ordered = await admin
      .from(cfg.namespace)
      .select('marker,name,quantity')
      .in('marker', markers)
      .lte('quantity', 32)
      .order('quantity', { ascending: false })
      .order('marker')
    assert.equal(ordered.error, null)
    assert.deepEqual(ordered.data, [rows[3], rows[1], rows[0]])
    const alternatives = await admin
      .from(cfg.namespace)
      .select('marker,name,quantity')
      .in('marker', markers)
      .or('quantity.eq.31,quantity.eq.33')
      .order('quantity', { ascending: false })
    assert.equal(alternatives.error, null)
    assert.deepEqual(alternatives.data, [rows[2], rows[0]])
    const pattern = await admin
      .from(cfg.namespace)
      .select('marker')
      .in('marker', markers)
      .ilike('name', '%alpha%')
    assert.equal(pattern.error, null)
    assert.deepEqual(
      pattern.data.map((row) => row.marker).sort(),
      [markers[0], markers[1], markers[3]].sort()
    )
  } finally {
    assert.equal((await admin.from(cfg.namespace).delete().in('marker', markers)).error, null)
  }
})

test('SDK database supports single, maybeSingle, HEAD counts, ordering and ranges', async () => {
  const one = await admin
    .from(cfg.namespace)
    .insert({ marker: `${marker}-count`, name: 'count', quantity: 2 })
    .select()
    .single()
  assert.equal(one.error, null)
  try {
    assert.equal(one.data.marker, `${marker}-count`)
    const count = await admin
      .from(cfg.namespace)
      .select('*', { count: 'exact', head: true })
      .eq('marker', `${marker}-count`)
    assert.equal(count.error, null)
    assert.equal(count.count, 1)
    assert.equal(count.data, null)
    const absent = await admin
      .from(cfg.namespace)
      .select()
      .eq('marker', `${marker}-absent`)
      .maybeSingle()
    assert.equal(absent.error, null)
    assert.equal(absent.data, null)
    const range = await admin
      .from(cfg.namespace)
      .select()
      .order('quantity', { ascending: false })
      .range(0, 0)
    assert.equal(range.error, null)
    assert.equal(range.data.length, 1)
  } finally {
    assert.equal(
      (await admin.from(cfg.namespace).delete().eq('marker', `${marker}-count`)).error,
      null
    )
  }
})

test('SDK RPC passes named arguments and consumes its JSON result', async () => {
  const { data, error } = await admin.rpc(`${cfg.namespace}_echo`, { message: 'sdk-compatible' })
  assert.equal(error, null)
  assert.deepEqual(data, [{ echo: 'sdk-compatible' }])
})

test('SDK singleton mutation failure rolls back all inserted rows', async () => {
  const markers = [`${marker}-rollback-a`, `${marker}-rollback-b`]
  try {
    const result = await admin
      .from(cfg.namespace)
      .insert(markers.map((value) => ({ marker: value, name: 'rollback', quantity: 1 })))
      .select()
      .single()
    assert.equal(result.error?.code, 'PGRST116')
    const readback = await admin.from(cfg.namespace).select('marker').in('marker', markers)
    assert.equal(readback.error, null)
    assert.deepEqual(readback.data, [])
  } finally {
    assert.equal((await admin.from(cfg.namespace).delete().in('marker', markers)).error, null)
  }
})

test('SDK anonymous requests cannot read the service-only fixture table', async () => {
  const result = await sdkClient(cfg).from(cfg.namespace).select('marker')
  assert.ok(result.error, 'Anonymous requests must not inherit database administrator privileges')
  assert.equal(result.data, null)
})

test('SDK Storage uploads, lists, downloads, and removes a real object', async () => {
  assert.ok(cfg.bucket, 'bucket must name the project-owned disposable object bucket')
  const bucket = admin.storage.from(cfg.bucket)
  const path = `${cfg.namespace}/${marker}.txt`
  const upload = await bucket.upload(path, new Blob(['golara-sdk-content'], { type: 'text/plain' }))
  assert.equal(upload.error, null)
  try {
    const list = await bucket.list(cfg.namespace)
    assert.equal(list.error, null)
    assert.ok(list.data.some((entry) => entry.name === `${marker}.txt`))
    const download = await bucket.download(path)
    assert.equal(download.error, null)
    assert.equal(await download.data.text(), 'golara-sdk-content')
    const anonymous = sdkClient(cfg).storage.from(cfg.bucket)
    assert.ok(
      (await anonymous.download(path)).error,
      'Private object download must require authentication'
    )
    assert.ok(
      (await anonymous.list(cfg.namespace)).error,
      'Private object names must not be disclosed'
    )
    const publicURL = anonymous.getPublicUrl(path).data.publicUrl
    assert.equal(
      (await fetch(publicURL)).status,
      404,
      'Public URL must not expose a private bucket'
    )
  } finally {
    assert.equal((await bucket.remove([path])).error, null)
  }
})

test('SDK Realtime channel subscribes and removes cleanly', { timeout: 15000 }, async () => {
  const client = sdkClient(cfg)
  const channel = client.channel(`${cfg.namespace}-${marker}`)
  try {
    await new Promise((resolve, reject) => {
      const timeout = setTimeout(() => reject(new Error('Realtime subscribe timed out')), 10000)
      channel.subscribe((status, error) => {
        if (status === 'SUBSCRIBED') {
          clearTimeout(timeout)
          resolve()
        }
        if (['CHANNEL_ERROR', 'TIMED_OUT'].includes(status)) {
          clearTimeout(timeout)
          reject(error ?? new Error(status))
        }
      })
    })
    assert.equal(await client.removeChannel(channel), 'ok')
  } finally {
    await client.removeAllChannels()
  }
})

test('SDK Functions invokes a deployed disposable fixture', async () => {
  const { data, error } = await admin.functions.invoke(`${cfg.namespace}-echo`, {
    body: { message: 'sdk-compatible' },
  })
  assert.equal(error, null)
  assert.deepEqual(data, { message: 'sdk-compatible' })
})

test('SDK Functions receives the public project SDK URL and request URL', async () => {
  const { data, error } = await admin.functions.invoke(`${cfg.namespace}-echo?check=public%20url`, {
    body: { inspectEnvironment: true },
  })
  assert.equal(error, null)
  assert.equal(data.url, cfg.sdkURL)
  assert.equal(data.project, cfg.project)
  assert.equal(
    data.requestURL,
    `${cfg.sdkURL}/functions/v1/${cfg.namespace}-echo?check=public%20url`
  )
})
