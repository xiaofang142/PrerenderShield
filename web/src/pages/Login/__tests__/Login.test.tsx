import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor, fireEvent } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import api from '../../../services/api'

// Mock i18n before importing Login
vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string) => key,
    i18n: { language: 'zh' },
  }),
}))

vi.mock('../../../services/api', () => ({
  default: { get: vi.fn(), post: vi.fn() },
}))

vi.mock('../../../context/AuthContext', async () => {
  const actual = await vi.importActual('../../../context/AuthContext')
  return { ...(actual as object), useAuth: vi.fn() }
})

const Login = (await import('../Login')).default
const { useAuth } = await import('../../../context/AuthContext')
const mockLogin = vi.fn()

beforeEach(() => {
  vi.clearAllMocks()
  vi.mocked(useAuth).mockReturnValue({
    isAuthenticated: false, username: null, token: null,
    login: mockLogin, logout: vi.fn(),
  })
  vi.mocked(api.get).mockResolvedValue({
    code: 200, message: 'success', data: { isFirstRun: false }
  })
})

describe('Login', () => {
  it('renders login form', async () => {
    render(<MemoryRouter><Login /></MemoryRouter>)
    await waitFor(() => {
      expect(screen.getByPlaceholderText('login.username')).toBeInTheDocument()
    })
    expect(screen.getByPlaceholderText('login.password')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'login.loginBtn' })).toBeInTheDocument()
  })

  it('validates required fields', async () => {
    render(<MemoryRouter><Login /></MemoryRouter>)
    fireEvent.click(await screen.findByRole('button', { name: 'login.loginBtn' }))
    await waitFor(() => {
      expect(screen.getAllByText('login.inputUsername')[0]).toBeInTheDocument()
    })
  })

  it('calls API and authLogin on submit', async () => {
    vi.mocked(api.post).mockResolvedValue({
      code: 200,
      message: 'Login successful',
      data: { token: 't', username: 'u', force_change_password: false },
    })

    render(<MemoryRouter><Login /></MemoryRouter>)

    const usernameInput = await screen.findByPlaceholderText('login.username')
    const passwordInput = await screen.findByPlaceholderText('login.password')
    const submitButton = await screen.findByRole('button', { name: 'login.loginBtn' })

    fireEvent.change(usernameInput, { target: { value: 'username' } })
    fireEvent.change(passwordInput, { target: { value: 'password' } })
    fireEvent.click(submitButton)

    await waitFor(() => {
      expect(api.post).toHaveBeenCalledWith('/auth/login', { username: 'username', password: 'password' })
    })
    expect(mockLogin).toHaveBeenCalledWith('t', 'u')
  })
})
