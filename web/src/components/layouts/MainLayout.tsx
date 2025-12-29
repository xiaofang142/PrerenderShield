import React from 'react'
import { Layout, Menu, ConfigProvider } from 'antd'
import { MenuUnfoldOutlined, MenuFoldOutlined, DashboardOutlined, SecurityScanOutlined, CodeOutlined, LockOutlined, BarChartOutlined, FileTextOutlined, SettingOutlined } from '@ant-design/icons'
import { Link, useLocation } from 'react-router-dom'

const { Header, Sider, Content } = Layout

const MainLayout: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const [collapsed, setCollapsed] = React.useState(false)
  const location = useLocation()

  return (
    <Layout style={{ minHeight: '100vh' }}>
      <Sider trigger={null} collapsible collapsed={collapsed}>
        <div className="logo" style={{ height: 32, margin: 16, background: 'rgba(255, 255, 255, 0.2)', borderRadius: 6 }} />
        <Menu theme="dark" mode="inline" selectedKeys={[location.pathname]}>
          <Menu.Item key="/" icon={<DashboardOutlined />}>
            <Link to="/">概览</Link>
          </Menu.Item>
          <Menu.Item key="/firewall" icon={<SecurityScanOutlined />}>
            <Link to="/firewall">防火墙</Link>
          </Menu.Item>
          <Menu.Item key="/prerender" icon={<CodeOutlined />}>
            <Link to="/prerender">预渲染</Link>
          </Menu.Item>
          <Menu.Item key="/ssl" icon={<LockOutlined />}>
            <Link to="/ssl">SSL管理</Link>
          </Menu.Item>
          <Menu.Item key="/monitoring" icon={<BarChartOutlined />}>
            <Link to="/monitoring">监控告警</Link>
          </Menu.Item>
          <Menu.Item key="/logs" icon={<FileTextOutlined />}>
            <Link to="/logs">日志管理</Link>
          </Menu.Item>
          <Menu.Item key="/settings" icon={<SettingOutlined />}>
            <Link to="/settings">系统设置</Link>
          </Menu.Item>
        </Menu>
      </Sider>
      <Layout className="site-layout">
        <Header className="site-layout-background" style={{ padding: 0, display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
          <div style={{ display: 'flex', alignItems: 'center' }}>
            {React.createElement(collapsed ? MenuUnfoldOutlined : MenuFoldOutlined, {
              className: 'trigger',
              onClick: () => setCollapsed(!collapsed),
              style: { marginLeft: 16, fontSize: 18, cursor: 'pointer' }
            })}
            <h1 style={{ margin: 0, marginLeft: 16, fontSize: 18, color: '#fff' }}>PrerenderShield</h1>
          </div>
          <div style={{ marginRight: 16, color: '#fff' }}>
            管理员
          </div>
        </Header>
        <Content
          className="site-layout-background"
          style={{
            margin: '24px 16px',
            padding: 24,
            minHeight: 280,
          }}
        >
          {children}
        </Content>
      </Layout>
    </Layout>
  )
}

export default MainLayout