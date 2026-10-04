import { randomUUID } from 'node:crypto'
import { expect, test } from '@playwright/test'

import { requireCanarySecrets } from '../helpers.js'

const projectApi = '/api/projects/default/auth/v1'

test.describe('Golara Canary auth and session lifecycle', () => {
  test('Studio rejects missing HTTP credentials', async ({ baseURL }) => {
    const response = await fetch(new URL('/project/default', baseURL), { redirect: 'manual' })
    expect(response.status).toBe(401)
    expect(response.headers.get('www-authenticate')).toContain('Basic')
  })

  test('register, login, refresh, current user, logout, and cleanup', async ({
    request,
  }, testInfo) => {
    requireCanarySecrets(testInfo, ['GOLARA_ANON_KEY', 'GOLARA_SERVICE_ROLE_KEY'])

    const anonKey = process.env.GOLARA_ANON_KEY as string
    const serviceRoleKey = process.env.GOLARA_SERVICE_ROLE_KEY as string
    const marker = randomUUID()
    const email = `golara-e2e-${marker}@example.invalid`
    const password = `G0lara-${randomUUID()}!`
    let userId: string | undefined

    try {
      const signup = await request.post(`${projectApi}/signup`, {
        headers: { apikey: anonKey },
        data: {
          email,
          password,
          data: { first_name: 'Golara', middle_name: 'Canary', last_name: 'Test' },
        },
      })
      expect(signup.status(), await signup.text()).toBe(200)
      const signupBody = await signup.json()
      userId = signupBody.user?.id
      expect(userId).toMatch(/^[0-9a-f-]{36}$/i)

      // Registration correctly returns no session while email confirmation is required.
      // Confirm this disposable Canary user through the authenticated control plane so the
      // same test can continue through login, refresh, and logout.
      expect(signupBody.access_token).toBeFalsy()
      expect(signupBody.refresh_token).toBeFalsy()
      const confirmation = await request.post('/api/platform/pg-meta/default/query', {
        data: {
          query: `UPDATE auth.users SET confirmed_at = now(), email_confirmed_at = now() WHERE id = '${userId}'::uuid RETURNING id`,
        },
      })
      expect(confirmation.status(), await confirmation.text()).toBe(200)
      expect(await confirmation.json()).toEqual([{ id: userId }])

      const login = await request.post(`${projectApi}/token?grant_type=password`, {
        headers: { apikey: anonKey },
        data: { email, password },
      })
      expect(login.status(), await login.text()).toBe(200)
      const loginBody = await login.json()
      expect(loginBody.user?.email).toBe(email)

      const currentUser = await request.get(`${projectApi}/user`, {
        headers: { apikey: anonKey, Authorization: `Bearer ${loginBody.access_token}` },
      })
      expect(currentUser.status(), await currentUser.text()).toBe(200)
      expect((await currentUser.json()).id).toBe(userId)

      const refresh = await request.post(`${projectApi}/token?grant_type=refresh_token`, {
        headers: { apikey: anonKey },
        data: { refresh_token: loginBody.refresh_token },
      })
      expect(refresh.status(), await refresh.text()).toBe(200)
      const refreshBody = await refresh.json()
      expect(refreshBody.refresh_token).toBeTruthy()
      expect(refreshBody.refresh_token).not.toBe(loginBody.refresh_token)

      const logout = await request.post(`${projectApi}/logout`, {
        headers: { apikey: anonKey, Authorization: `Bearer ${refreshBody.access_token}` },
      })
      expect(logout.status()).toBe(204)

      const afterLogout = await request.get(`${projectApi}/user`, {
        headers: { apikey: anonKey, Authorization: `Bearer ${refreshBody.access_token}` },
      })
      expect(afterLogout.status()).toBe(401)
    } finally {
      if (userId) {
        const cleanup = await request.delete(`${projectApi}/admin/users/${userId}`, {
          headers: {
            apikey: serviceRoleKey,
            Authorization: `Bearer ${serviceRoleKey}`,
          },
        })
        expect(cleanup.status(), await cleanup.text()).toBe(204)
      }
    }
  })

  test('invalid credentials return a bounded error without revealing account state', async ({
    request,
  }, testInfo) => {
    requireCanarySecrets(testInfo, ['GOLARA_ANON_KEY'])
    const response = await request.post(`${projectApi}/token?grant_type=password`, {
      headers: { apikey: process.env.GOLARA_ANON_KEY as string },
      data: { email: `missing-${randomUUID()}@example.invalid`, password: 'invalid-password' },
    })

    expect(response.status()).toBe(400)
    expect(await response.json()).toEqual({ error: 'invalid login credentials' })
  })
})
