import React, { useState, useEffect, createContext, useContext } from 'react'
import { Switch, Tooltip } from 'antd'
import { BulbOutlined, BulbFilled } from '@ant-design/icons'

// 主题类型
type ThemeType = 'light' | 'dark'

// 主题上下文
interface ThemeContextType {
  theme: ThemeType
  toggleTheme: () => void
  isDark: boolean
}

const ThemeContext = createContext<ThemeContextType>({
  theme: 'light',
  toggleTheme: () => {},
  isDark: false,
})

// 主题 Provider
export const ThemeProvider: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const [theme, setTheme] = useState<ThemeType>(() => {
    // 从 localStorage 读取主题
    if (typeof window !== 'undefined') {
      const stored = localStorage.getItem('theme')
      if (stored === 'dark' || stored === 'light') {
        return stored
      }
      // 检测系统主题
      if (window.matchMedia && window.matchMedia('(prefers-color-scheme: dark)').matches) {
        return 'dark'
      }
    }
    return 'light'
  })

  useEffect(() => {
    // 保存到 localStorage
    localStorage.setItem('theme', theme)
    
    // 设置 HTML 属性
    document.documentElement.setAttribute('data-theme', theme)
    
    // 更新 body 类名
    if (theme === 'dark') {
      document.body.classList.add('dark-theme')
      document.body.classList.remove('light-theme')
    } else {
      document.body.classList.add('light-theme')
      document.body.classList.remove('dark-theme')
    }
  }, [theme])

  const toggleTheme = () => {
    setTheme(prev => prev === 'light' ? 'dark' : 'light')
  }

  return (
    <ThemeContext.Provider value={{ theme, toggleTheme, isDark: theme === 'dark' }}>
      {children}
    </ThemeContext.Provider>
  )
}

// 使用主题 Hook
export const useTheme = () => useContext(ThemeContext)

// 主题切换按钮
export const ThemeToggle: React.FC = () => {
  const { isDark, toggleTheme } = useTheme()

  return (
    <Tooltip title={isDark ? '切换到浅色模式' : '切换到深色模式'}>
      <Switch
        checked={isDark}
        onChange={toggleTheme}
        checkedChildren={<BulbFilled />}
        unCheckedChildren={<BulbOutlined />}
      />
    </Tooltip>
  )
}

export default ThemeContext
