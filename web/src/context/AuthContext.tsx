import React, { createContext, useState, useEffect, useContext, useCallback } from 'react'
import api from '../services/api'

interface AuthContextType {
  isAuthenticated: boolean
  username: string | null
  token: string | null
  login: (token: string, username: string) => void
  logout: () => void
}

const AuthContext = createContext<AuthContextType | undefined>(undefined)

function isTokenExpired(token: string): boolean {
  try {
    const payload = JSON.parse(atob(token.split('.')[1]))
    return payload.exp * 1000 < Date.now()
  } catch {
    return true
  }
}

export const AuthProvider: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const [isAuthenticated, setIsAuthenticated] = useState(false)
  const [username, setUsername] = useState<string | null>(null)
  const [token, setToken] = useState<string | null>(null)

  useEffect(() => {
    const storedToken = localStorage.getItem('token')
    const storedUsername = localStorage.getItem('username')
    
    if (storedToken && storedUsername && !isTokenExpired(storedToken)) {
      setIsAuthenticated(true)
      setUsername(storedUsername)
      setToken(storedToken)
    } else {
      localStorage.removeItem('token')
      localStorage.removeItem('username')
    }
  }, [])

  const login = (token: string, username: string) => {
    localStorage.setItem('token', token)
    localStorage.setItem('username', username)
    setIsAuthenticated(true)
    setUsername(username)
    setToken(token)
  }

  const logout = useCallback(async () => {
    try {
      await api.post('/auth/logout')
    } catch {
      // ignore network errors on logout
    }
    localStorage.removeItem('token')
    localStorage.removeItem('username')
    setIsAuthenticated(false)
    setUsername(null)
    setToken(null)
  }, [])

  const value: AuthContextType = {
    isAuthenticated,
    username,
    token,
    login,
    logout
  }

  return (
    <AuthContext.Provider value={value}>
      {children}
    </AuthContext.Provider>
  )
}

// 自定义hook，方便组件使用AuthContext
export const useAuth = () => {
  const context = useContext(AuthContext)
  if (context === undefined) {
    throw new Error('useAuth must be used within an AuthProvider')
  }
  return context
}
