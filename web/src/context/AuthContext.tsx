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
  // 惰性初始化：同步读取 localStorage，避免首帧 isAuthenticated=false 导致的闪跳重定向
  const [isAuthenticated, setIsAuthenticated] = useState(() => {
    try {
      const storedToken = localStorage.getItem('token')
      const storedUsername = localStorage.getItem('username')
      return !!(storedToken && storedUsername && !isTokenExpired(storedToken))
    } catch {
      return false
    }
  })
  const [username, setUsername] = useState<string | null>(() => {
    try {
      const storedToken = localStorage.getItem('token')
      const storedUsername = localStorage.getItem('username')
      if (storedToken && storedUsername && !isTokenExpired(storedToken)) {
        return storedUsername
      }
    } catch {
      // fallthrough
    }
    return null
  })
  const [token, setToken] = useState<string | null>(() => {
    try {
      const storedToken = localStorage.getItem('token')
      if (storedToken && localStorage.getItem('username') && !isTokenExpired(storedToken)) {
        return storedToken
      }
    } catch {
      // fallthrough
    }
    return null
  })

  useEffect(() => {
    // 清理过期/无效的本地凭证
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
