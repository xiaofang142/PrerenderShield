import React, { createContext, useState, useEffect, useContext } from 'react'

interface AuthContextType {
  isAuthenticated: boolean
  username: string | null
  token: string | null
  login: (token: string, username: string) => void
  logout: () => void
}

const AuthContext = createContext<AuthContextType | undefined>(undefined)

export const AuthProvider: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const [isAuthenticated, setIsAuthenticated] = useState(false)
  const [username, setUsername] = useState<string | null>(null)
  const [token, setToken] = useState<string | null>(null)

  // 初始化时检查localStorage中的token
  useEffect(() => {
    const storedToken = localStorage.getItem('token')
    const storedUsername = localStorage.getItem('username')
    
    if (storedToken && storedUsername) {
      setIsAuthenticated(true)
      setUsername(storedUsername)
      setToken(storedToken)
    }
  }, [])

  const login = (token: string, username: string) => {
    // 保存到localStorage
    localStorage.setItem('token', token)
    localStorage.setItem('username', username)
    
    // 更新状态
    setIsAuthenticated(true)
    setUsername(username)
    setToken(token)
  }

  const logout = () => {
    // 从localStorage移除
    localStorage.removeItem('token')
    localStorage.removeItem('username')
    
    // 更新状态
    setIsAuthenticated(false)
    setUsername(null)
    setToken(null)
  }

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
