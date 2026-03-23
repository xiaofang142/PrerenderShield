import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import Dashboard from '../Dashboard'
import { overviewApi } from '../../services/api'

vi.mock('../../services/api', () => ({
  overviewApi: { getStats: vi.fn() },
}))

const mockGetStats = vi.mocked(overviewApi.getStats)

describe('Dashboard', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('renders stats after loading', async () => {
    mockGetStats.mockResolvedValue({
      code: 200,
      data: {
        totalRequests: 1000,
        crawlerRequests: 200,
        blockedRequests: 50,
        activeSites: 10,
        firewallEnabled: true,
        prerenderEnabled: true,
        trafficData: [],
        accessStats: { pv: 5000, uv: 1000, ip: 800 },
      },
    })

    render(<MemoryRouter><Dashboard /></MemoryRouter>)

    await waitFor(() => {
      expect(screen.getByText(/总请求量/i)).toBeInTheDocument()
    }, { timeout: 10000 })

    expect(screen.getByText('1000')).toBeInTheDocument()
    expect(screen.getByText('50')).toBeInTheDocument()
    expect(screen.getByText('200')).toBeInTheDocument()
    expect(screen.getByText('10')).toBeInTheDocument()
    expect(screen.getByText('5000')).toBeInTheDocument()
  })

  it('renders system status', async () => {
    mockGetStats.mockResolvedValue({
      code: 200,
      data: {
        totalRequests: 0,
        crawlerRequests: 0,
        blockedRequests: 0,
        activeSites: 0,
        firewallEnabled: true,
        prerenderEnabled: false,
        trafficData: [],
        accessStats: { pv: 0, uv: 0, ip: 0 },
      },
    })

    render(<MemoryRouter><Dashboard /></MemoryRouter>)

    await waitFor(() => {
      expect(screen.getByText(/系统状态/i)).toBeInTheDocument()
    }, { timeout: 10000 })
  })
})
