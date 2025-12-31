import React from 'react'
import { Layout, Menu, ConfigProvider } from 'antd'
import { MenuUnfoldOutlined, MenuFoldOutlined, DashboardOutlined, SecurityScanOutlined, CodeOutlined, BarChartOutlined, FileTextOutlined, BugOutlined } from '@ant-design/icons'
import { Link, useLocation } from 'react-router-dom'

const { Header, Sider, Content } = Layout

const MainLayout: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const [collapsed, setCollapsed] = React.useState(false)
  const location = useLocation()

  return (
    <Layout style={{ minHeight: '100vh', background: '#ffffff' }}>
      {/* 左侧导航栏 - 纯白主题 */}
      <Sider 
        trigger={null} 
        collapsible 
        collapsed={collapsed}
        style={{
          background: '#ffffff',
          borderRight: '1px solid #e8e8e8',
        }}
      >
        {/* Logo区域 - 雷池风格 */}
        <div 
          className="logo" 
          style={{
            height: 32, 
            margin: 16, 
            background: '#2f855a', // 中碧蓝
            borderRadius: 6,
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            fontSize: 16,
            fontWeight: 'bold',
            color: '#ffffff',
            boxShadow: '0 2px 8px rgba(47, 133, 90, 0.3)'
          }} 
        >
          {collapsed ? 'PS' : 'PrerenderShield'}
        </div>
        
        {/* 菜单 - 纯白主题 */}
        <Menu 
          theme="light" 
          mode="inline" 
          selectedKeys={[location.pathname]}
          style={{
            background: '#ffffff',
            borderRight: 'none'
          }}
          items={[
            {
              key: '/',
              icon: <DashboardOutlined style={{ color: '#2f855a' }} />,
              label: <Link to="/" style={{ color: '#333333' }}>概览</Link>
            },
            {
              key: '/sites',
              icon: <FileTextOutlined style={{ color: '#2f855a' }} />,
              label: <Link to="/sites" style={{ color: '#333333' }}>站点管理</Link>
            },
            {
              key: '/firewall',
              icon: <SecurityScanOutlined style={{ color: '#2f855a' }} />,
              label: <Link to="/firewall" style={{ color: '#333333' }}>防火墙</Link>
            },
            {
              key: '/prerender',
              icon: <CodeOutlined style={{ color: '#2f855a' }} />,
              label: <Link to="/prerender" style={{ color: '#333333' }}>渲染预热</Link>
            },

            {
              key: '/monitoring',
              icon: <BarChartOutlined style={{ color: '#2f855a' }} />,
              label: <Link to="/monitoring" style={{ color: '#333333' }}>监控告警</Link>
            },
            {
              key: '/crawler',
              icon: <BugOutlined style={{ color: '#2f855a' }} />,
              label: <Link to="/crawler" style={{ color: '#333333' }}>爬虫访问</Link>
            },

          ]}
        />
      </Sider>
      
      {/* 主内容区域 */}
      <Layout className="site-layout">
        {/* 顶部导航栏 - 纯白主题 */}
        <Header 
          className="site-layout-background" 
          style={{
            padding: 0, 
            display: 'flex', 
            alignItems: 'center', 
            justifyContent: 'space-between',
            background: '#ffffff',
            borderBottom: '1px solid #e8e8e8',
            boxShadow: '0 2px 8px rgba(0, 0, 0, 0.08)',
          }}
        >
          {/* 左侧触发器和标题 */}
          <div style={{ display: 'flex', alignItems: 'center' }}>
            {React.createElement(collapsed ? MenuUnfoldOutlined : MenuFoldOutlined, {
              className: 'trigger',
              onClick: () => setCollapsed(!collapsed),
              style: { marginLeft: 16, fontSize: 18, cursor: 'pointer', color: '#333333' }
            })}
            <h1 style={{ margin: 0, marginLeft: 16, fontSize: 18, color: '#333333' }}>PrerenderShield</h1>
          </div>
          
          {/* 右侧用户信息 */}
          <div style={{ marginRight: 16, color: '#333333', display: 'flex', alignItems: 'center' }}>
            <span style={{ marginRight: 8 }}>管理员</span>
            <div 
              style={{
                width: 32,
                height: 32,
                borderRadius: '50%',
                background: '#2f855a', // 中碧蓝
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                fontSize: 14,
                fontWeight: 'bold',
                color: '#ffffff'
              }}
            >
              A
            </div>
          </div>
        </Header>
        
        {/* 内容区域 */}
        <Content
          className="site-layout-background"
          style={{
            margin: '24px 16px',
            padding: 24,
            minHeight: 280,
            background: '#ffffff',
            borderRadius: 8,
            border: '1px solid #e8e8e8',
            boxShadow: '0 2px 8px rgba(0, 0, 0, 0.08)',
          }}
        >
          {children}
        </Content>
      </Layout>
    </Layout>
  )
}

export default MainLayout