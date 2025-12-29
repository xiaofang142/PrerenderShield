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
  return (
    <ConfigProvider locale={zhCN}>
      <Router>
        <MainLayout>
          <Routes>
            <Route path="/" element={<Overview />} />
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