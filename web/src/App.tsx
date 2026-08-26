import { BrowserRouter as Router, Routes, Route, Outlet } from 'react-router-dom'
import { Suspense, lazy } from 'react'
import { ConfigProvider, Spin } from 'antd'
import zhCN from 'antd/locale/zh_CN'
import enUS from 'antd/locale/en_US'
import jaJP from 'antd/locale/ja_JP'
import koKR from 'antd/locale/ko_KR'
import { useTranslation } from 'react-i18next'

// Import Auth Context
import { AuthProvider } from './context/AuthContext'

// Import Error Boundary
import ErrorBoundary from './components/common/ErrorBoundary'

// Import Private Route
import PrivateRoute from './components/PrivateRoute/PrivateRoute'

// 路由级代码分割：页面组件按需加载，减小主 bundle 体积
const Login = lazy(() => import('./pages/Login/Login'))
const Overview = lazy(() => import('./pages/Overview/Overview'))
const Firewall = lazy(() => import('./pages/Firewall/Firewall'))
const FirewallRules = lazy(() => import('./pages/Firewall/FirewallRules'))
const Prerender = lazy(() => import('./pages/Prerender/Prerender'))
const Preheat = lazy(() => import('./pages/Prerender/Preheat'))
const Push = lazy(() => import('./pages/Prerender/Push'))
const Monitoring = lazy(() => import('./pages/Monitoring/Monitoring'))
const AlertConfig = lazy(() => import('./pages/Monitoring/AlertConfig'))
const Logs = lazy(() => import('./pages/Logs/Logs'))
const Sites = lazy(() => import('./pages/Sites/Sites'))
const Crawler = lazy(() => import('./pages/Crawler/Crawler'))
const SystemConfig = lazy(() => import('./pages/System/SystemConfig'))
const WAFSettings = lazy(() => import('./pages/WAFSettings'))
const Dashboard = lazy(() => import('./pages/Dashboard'))
const SSLPage = lazy(() => import('./pages/SSL'))
const SettingsPage = lazy(() => import('./pages/Settings'))

// Import layout
import MainLayout from './components/layouts/MainLayout'

// 懒加载页面时的全屏 loading 占位
const PageLoading = () => (
  <div style={{ display: 'flex', justifyContent: 'center', alignItems: 'center', minHeight: '60vh' }}>
    <Spin size="large" />
  </div>
)

function App() {
  const { i18n } = useTranslation()

  // 根据当前语言获取 Ant Design 语言包
  const getAntdLocale = () => {
    switch (i18n.language) {
      case 'zh':
      case 'zh-CN':
        return zhCN
      case 'en':
        return enUS
      case 'ja':
        return jaJP
      case 'ko':
        return koKR
      default:
        return zhCN
    }
  }

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
    <ConfigProvider locale={getAntdLocale()} theme={theme}>
      <ErrorBoundary>
        <AuthProvider>
          <Router>
            <Suspense fallback={<PageLoading />}>
              <Routes>
                <Route path="/login" element={<Login />} />
                <Route element={<PrivateRoute><MainLayout><Outlet /></MainLayout></PrivateRoute>}>
                <Route path="/" element={<Overview />} />
                <Route path="/sites" element={<Sites />} />
                <Route path="/sites/:id/waf" element={<WAFSettings />} />
                <Route path="/dashboard" element={<Dashboard />} />
                <Route path="/firewall" element={<Firewall />} />
                <Route path="/firewall/rules" element={<FirewallRules />} />
                <Route path="/prerender" element={<Prerender />} />
                <Route path="/prerender/preheat" element={<Preheat />} />
                <Route path="/prerender/push" element={<Push />} />
                <Route path="/monitoring" element={<Monitoring />} />
                <Route path="/monitoring/alerts" element={<AlertConfig />} />
                <Route path="/logs" element={<Logs />} />
                <Route path="/crawler" element={<Crawler />} />
                <Route path="/system" element={<SystemConfig />} />
                <Route path="/ssl" element={<SSLPage />} />
                <Route path="/settings" element={<SettingsPage />} />
              </Route>
              </Routes>
            </Suspense>
          </Router>
        </AuthProvider>
      </ErrorBoundary>
    </ConfigProvider>
  )
}

export default App