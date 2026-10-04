import { screen } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { FunctionsEmptyState } from './FunctionsEmptyState'
import { customRender as render } from '@/tests/lib/custom-render'

const { mockIsPlatform, mockUseDeploymentMode, mockUseTrack, mockUseIsFeatureEnabled } = vi.hoisted(
  () => ({
    mockIsPlatform: { value: true },
    mockUseDeploymentMode: vi.fn(),
    mockUseTrack: vi.fn(),
    mockUseIsFeatureEnabled: vi.fn(),
  })
)

vi.mock('@/lib/constants', async () => {
  const actual = await vi.importActual<Record<string, unknown>>('@/lib/constants')
  return {
    ...actual,
    get IS_PLATFORM() {
      return mockIsPlatform.value
    },
  }
})

vi.mock('@/hooks/misc/useDeploymentMode', () => ({
  useDeploymentMode: mockUseDeploymentMode,
}))

vi.mock('@/lib/telemetry/track', () => ({
  useTrack: mockUseTrack,
}))

vi.mock('@/hooks/misc/useIsFeatureEnabled', () => ({
  useIsFeatureEnabled: mockUseIsFeatureEnabled,
}))

describe('FunctionsEmptyState', () => {
  beforeEach(() => {
    mockIsPlatform.value = true
    mockUseDeploymentMode.mockReturnValue({
      isPlatform: true,
      isCli: false,
      isSelfHosted: false,
    })
    mockUseTrack.mockReturnValue(vi.fn())
    mockUseIsFeatureEnabled.mockReturnValue(false)
  })

  it('renders the templates section on platform', () => {
    render(<FunctionsEmptyState />)

    expect(screen.getByText('Start with a template')).toBeInTheDocument()
  })

  it('renders the browser editor and templates on self-hosted Golara', () => {
    mockIsPlatform.value = false
    mockUseDeploymentMode.mockReturnValue({
      isPlatform: false,
      isCli: false,
      isSelfHosted: true,
    })

    render(<FunctionsEmptyState />)

    expect(screen.getByRole('button', { name: 'Open Editor' })).toBeInTheDocument()
    expect(screen.getByText('Start with a template')).toBeInTheDocument()
    expect(screen.queryByText('Self-Hosted')).not.toBeInTheDocument()
  })

  it('hides the templates section on CLI mode', () => {
    mockIsPlatform.value = false
    mockUseDeploymentMode.mockReturnValue({
      isPlatform: false,
      isCli: true,
      isSelfHosted: false,
    })

    render(<FunctionsEmptyState />)

    expect(screen.queryByText('Start with a template')).not.toBeInTheDocument()
  })

  it('does not show obsolete restart instructions on self-hosted Golara', () => {
    mockIsPlatform.value = false
    mockUseDeploymentMode.mockReturnValue({
      isPlatform: false,
      isCli: false,
      isSelfHosted: true,
    })

    render(<FunctionsEmptyState />)

    expect(screen.queryByText(/restart the functions service/i)).not.toBeInTheDocument()
  })
})
