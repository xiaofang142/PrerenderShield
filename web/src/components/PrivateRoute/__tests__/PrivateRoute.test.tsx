import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen } from '@testing-library/react'
import { MemoryRouter, useLocation } from 'react-router-dom'

vi.mock('../../../context/AuthContext', () => ({
  useAuth: vi.fn(),
}))

const { useAuth } = await import('../../../context/AuthContext')
const PrivateRoute = (await import('../PrivateRoute')).default

const TestChild = () => <div data-testid="child">Protected</div>
const LocationDisplay = () => <div data-testid="location">{useLocation().pathname}</div>

describe('PrivateRoute', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('redirects to login when not authenticated', () => {
    vi.mocked(useAuth).mockReturnValue({
      isAuthenticated: false,
      username: null,
      token: null,
      login: vi.fn(),
      logout: vi.fn(),
    })

    render(
      <MemoryRouter initialEntries={['/dashboard']}>
        <PrivateRoute><TestChild /></PrivateRoute>
        <LocationDisplay />
      </MemoryRouter>
    )

    expect(screen.getByTestId('location').textContent).toBe('/login')
  })

  it('renders children when authenticated', () => {
    vi.mocked(useAuth).mockReturnValue({
      isAuthenticated: true,
      username: 'user',
      token: 'token',
      login: vi.fn(),
      logout: vi.fn(),
    })

    render(
      <MemoryRouter>
        <PrivateRoute><TestChild /></PrivateRoute>
      </MemoryRouter>
    )

    expect(screen.getByTestId('child')).toBeInTheDocument()
  })
})
