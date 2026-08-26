import { describe, it, expect, beforeEach } from 'vitest'
import { render, screen } from '@testing-library/react'
import { MemoryRouter, Routes, Route } from 'react-router-dom'
import { AuthProvider } from '../../../context/AuthContext'
import PrivateRoute from '../PrivateRoute'

const LoginPage = () => <div data-testid="login-page">LoginPage</div>

const renderWithAuth = (token?: { token: string; username: string }) => {
  if (token) {
    localStorage.setItem('token', token.token)
    localStorage.setItem('username', token.username)
  } else {
    localStorage.clear()
  }

  return render(
    <MemoryRouter initialEntries={['/dashboard']}>
      <AuthProvider>
        <Routes>
          <Route path="/login" element={<LoginPage />} />
          <Route
            path="*"
            element={
              <PrivateRoute>
                <div data-testid="child">Protected</div>
              </PrivateRoute>
            }
          />
        </Routes>
      </AuthProvider>
    </MemoryRouter>
  )
}

const validToken = () => {
  const payload = btoa(JSON.stringify({ exp: Math.floor(Date.now() / 1000) + 3600 }))
  return `header.${payload}.signature`
}

describe('PrivateRoute', () => {
  beforeEach(() => {
    localStorage.clear()
  })

  it('redirects to login when not authenticated', () => {
    renderWithAuth()

    expect(screen.getByTestId('login-page')).toBeInTheDocument()
    expect(screen.queryByTestId('child')).not.toBeInTheDocument()
  })

  it('renders children when authenticated', () => {
    renderWithAuth({ token: validToken(), username: 'user' })

    expect(screen.getByTestId('child')).toBeInTheDocument()
    expect(screen.queryByTestId('login-page')).not.toBeInTheDocument()
  })

  it('redirects to login when token is expired', () => {
    const expiredPayload = btoa(JSON.stringify({ exp: Math.floor(Date.now() / 1000) - 10 }))
    renderWithAuth({ token: `header.${expiredPayload}.signature`, username: 'user' })

    expect(screen.getByTestId('login-page')).toBeInTheDocument()
    expect(screen.queryByTestId('child')).not.toBeInTheDocument()
  })
})
