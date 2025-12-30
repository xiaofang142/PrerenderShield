import React from 'react'
import { BrowserRouter as Router, Routes, Route } from 'react-router-dom'
import { ConfigProvider } from 'antd'
import zhCN from 'antd/locale/zh_CN'

// Import pages
import Overview from './pages/Overview/Overview'
import Firewall from './pages/Firewall/Firewall'
import Prerender from './pages/Prerender/Prerender'
import SSL from './pages/SSL/SSL'
import Monitoring from './pages/Monitoring/Monitoring'
import Logs from './pages/Logs/Logs'
import Settings from './pages/Settings/Settings'
import Sites from './pages/Sites/Sites'

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
      <Router>
        <MainLayout>
          <Routes>
            <Route path="/" element={<Overview />} />
            <Route path="/sites" element={<Sites />} />
            <Route path="/firewall" element={<Firewall />} />
            <Route path="/prerender" element={<Prerender />} />
            <Route path="/ssl" element={<SSL />} />
            <Route path="/monitoring" element={<Monitoring />} />
            <Route path="/logs" element={<Logs />} />
            <Route path="/settings" element={<Settings />} />
          </Routes>
        </MainLayout>
      </Router>
    </ConfigProvider>
  )
}

export default App