import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, test, vi } from 'vitest'

import { SupadataProjectSelector } from '@/components/layouts/AppLayout/SupadataProjectSelector'
import { customRender } from '@/tests/lib/custom-render'

const push = vi.fn()
vi.mock('next/router', () => ({ useRouter: () => ({ push }) }))

describe('SupadataProjectSelector credentials', () => {
  test('shows credential metadata and keeps rotated secrets hidden until revealed', async () => {
    const user = userEvent.setup()
    const fetchMock = vi.spyOn(globalThis, 'fetch').mockImplementation(async (input, init) => {
      const url = String(input)
      if (url.endsWith('/projects')) {
        return new Response(
          JSON.stringify({
            projects: [{ id: 'demo', name: 'Demo', status: 'running', current: true }],
          })
        )
      }
      if (url.endsWith('/projects/demo/credentials')) {
        return new Response(
          JSON.stringify({
            apiKey: { createdAt: '2026-09-04T12:00:00Z' },
            deployablePassword: { createdAt: null },
            postgres: { host: '127.0.0.1', port: 5433, database: 'demo', username: 'postgres' },
          })
        )
      }
      if (url.endsWith('/projects/demo/credentials/api-key/rotate')) {
        expect(init?.method).toBe('POST')
        return new Response(JSON.stringify({ type: 'api-key', value: 'secret-api-key' }))
      }
      return new Response('{}', { status: 404 })
    })

    customRender(<SupadataProjectSelector currentId="demo" currentName="Demo" />)
    await waitFor(() =>
      expect(screen.getByRole('combobox', { name: 'Select Supadata project' })).toBeEnabled()
    )
    await user.click(screen.getByRole('combobox', { name: 'Select Supadata project' }))
    await user.click(screen.getByRole('option', { name: 'Credentials' }))

    expect(await screen.findByText('127.0.0.1:5433')).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Rotate API key' }))
    expect(await screen.findByText('Secret generated')).toBeInTheDocument()
    expect(screen.queryByText('secret-api-key')).not.toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Reveal secret' }))
    expect(screen.getByText('secret-api-key')).toBeInTheDocument()

    fetchMock.mockRestore()
  })
})
