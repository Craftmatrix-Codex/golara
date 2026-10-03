import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { SupadataProjectSelector } from './SupadataProjectSelector'
import { render } from '@/tests/helpers'

const mockPush = vi.hoisted(() => vi.fn())

vi.mock('next/router', () => ({
  useRouter: () => ({ push: mockPush }),
}))

describe('SupadataProjectSelector', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue(
        new Response(
          JSON.stringify({
            projects: [{ id: 'default', name: 'Default Project', status: 'ready', current: true }],
          }),
          { status: 200, headers: { 'content-type': 'application/json' } }
        )
      )
    )
  })

  it('keeps the Create button enabled after the form is opened and filled', async () => {
    const user = userEvent.setup()
    render(<SupadataProjectSelector currentId="default" currentName="Default Project" />)

    await waitFor(() => expect(fetch).toHaveBeenCalledWith('/api/platform/projects'))
    await user.click(screen.getByRole('combobox', { name: 'Select Golara project' }))
    await user.click(screen.getByText('New project', { exact: true }))
    await user.type(screen.getByLabelText('Project name'), 'Video Project Demo')

    expect(screen.getByRole('button', { name: 'Create' })).toBeEnabled()
  })
})
