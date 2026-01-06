import React from 'react'
import { BrowserRouter as Router, Routes, Route } from 'react-router-dom'
import { ConfigProvider } from 'antd'
import zhCN from 'antd/locale/zh_CN'

// Import Auth Context
import { AuthProvider } from './context/AuthContext'

// Import Private Route
import PrivateRoute from './components/PrivateRoute/PrivateRoute'

// Import pages
import Login from './pages/Login/Login'
import Overview from './pages/Overview/Overview'
import Firewall from './pages/Firewall/Firewall'
import Prerender from './pages/Prerender/Prerender'
import Preheat from './pages/Prerender/Preheat'
import Push from './pages/Prerender/Push'
import Monitoring from './pages/Monitoring/Monitoring'
import Logs from './pages/Logs/Logs'
import Sites from './pages/Sites/Sites'
import Crawler from './pages/Crawler/Crawler'
import SystemConfig from './pages/System/SystemConfig'

// Import layout
import MainLayout from './components/layouts/MainLayout'

function App() {
  // 自定义主题配置，参考雷池设计风格
  const theme = {
    token: {
      // 中碧蓝主色调 - 雷池风格
      colorPrimary: '#2f855a', // 中碧蓝
      colorPrimaryHover: '#389e6d',
      colorPrimaryActive: '#1d684f',
      
      // 背景色 - 纯白
      colorBgLayout: '#ffffff', // 纯白背景
      colorBgContainer: '#ffffff', // 容器背景
      colorBgElevated: '#ffffff', // 悬浮背景
      
      // 文字颜色 - 黑色灰色
      colorTextPrimary: '#000000', // 主文字黑色
      colorTextSecondary: '#666666', // 次要文字灰色
      colorTextTertiary: '#999999', // 三级文字
      
      // 边框和分割线
      colorBorder: '#e8e8e8', // 边框颜色
      colorSplit: '#e8e8e8', // 分割线颜色
      
      // 扁平化设计
      borderRadius: 6, // 适中的圆角
      boxShadow: '0 2px 8px rgba(0, 0, 0, 0.1)', // 轻微阴影
      
      // 卡片样式
      cardShadow: '0 2px 8px rgba(0, 0, 0, 0.08)',
      
      // 按钮样式
      controlHeight: 36,
      borderRadiusButton: 4,
    },
  }

  return (
    <ConfigProvider locale={zhCN} theme={theme}>
      <AuthProvider>
        <Router>
          <Routes>
            {/* 登录路由 - 不需要认证 */}
            <Route path="/login" element={<Login />} />
            
            {/* 需要认证的路由 */}
            <Route 
              path="/" 
              element={
                <PrivateRoute>
                  <MainLayout>
                    <Overview />
                  </MainLayout>
                </PrivateRoute>
              } 
            />
            <Route 
              path="/sites" 
              element={
                <PrivateRoute>
                  <MainLayout>
                    <Sites />
                  </MainLayout>
                </PrivateRoute>
              } 
            />
            <Route 
              path="/firewall" 
              element={
                <PrivateRoute>
                  <MainLayout>
                    <Firewall />
                  </MainLayout>
                </PrivateRoute>
              } 
            />
            <Route 
              path="/prerender" 
              element={
                <PrivateRoute>
                  <MainLayout>
                    <Prerender />
                  </MainLayout>
                </PrivateRoute>
              } 
            />
            <Route 
              path="/prerender/preheat" 
              element={
                <PrivateRoute>
                  <MainLayout>
                    <Preheat />
                  </MainLayout>
                </PrivateRoute>
              } 
            />
            <Route 
              path="/prerender/push" 
              element={
                <PrivateRoute>
                  <MainLayout>
                    <Push />
                  </MainLayout>
                </PrivateRoute>
              } 
            />
            <Route 
              path="/monitoring" 
              element={
                <PrivateRoute>
                  <MainLayout>
                    <Monitoring />
                  </MainLayout>
                </PrivateRoute>
              } 
            />
            <Route 
              path="/logs" 
              element={
                <PrivateRoute>
                  <MainLayout>
                    <Logs />
                  </MainLayout>
                </PrivateRoute>
              } 
            />
            <Route 
              path="/crawler" 
              element={
                <PrivateRoute>
                  <MainLayout>
                    <Crawler />
                  </MainLayout>
                </PrivateRoute>
              } 
            />
            <Route 
              path="/system" 
              element={
                <PrivateRoute>
                  <MainLayout>
                    <SystemConfig />
                  </MainLayout>
                </PrivateRoute>
              } 
            />
          </Routes>
        </Router>
      </AuthProvider>
    </ConfigProvider>
  )
}

export default App