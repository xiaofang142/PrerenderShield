import React from 'react'
import { Layout, Menu, Button, message, Dropdown } from 'antd'
import { MenuUnfoldOutlined, MenuFoldOutlined, DashboardOutlined, SecurityScanOutlined, CodeOutlined, BarChartOutlined, FileTextOutlined, BugOutlined, LogoutOutlined, CloudUploadOutlined, SettingOutlined, GlobalOutlined } from '@ant-design/icons'
import { Link, useLocation, useNavigate } from 'react-router-dom'
import { useAuth } from '../../context/AuthContext'
import { useTranslation } from 'react-i18next'
import { systemApi } from '../../services/api'

const { Header, Sider, Content } = Layout

const MainLayout: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const { t, i18n } = useTranslation()
  const [collapsed, setCollapsed] = React.useState(false)
  const location = useLocation()
  const navigate = useNavigate()
  const { logout, username } = useAuth()
  const [appInfo, setAppInfo] = React.useState<{ version: string, official_url: string }>({ version: '', official_url: '' })

  React.useEffect(() => {
    const fetchVersion = async () => {
      try {
        const res = await systemApi.version()
        if (res.data) {
          setAppInfo({
            version: res.data.version || '1.0.0',
            official_url: res.data.official_url || '#'
          })
        }
      } catch (error) {
        console.error('Failed to fetch version info:', error)
      }
    }
    fetchVersion()
  }, [])


  // 退出登录处理函数
  const handleLogout = () => {
    logout()
    message.success(t('common.success'))
    navigate('/login')
  }

  // 语言切换菜单项
  const langItems = [
    { key: 'zh', label: '简体中文' },
    { key: 'en', label: 'English' },
    { key: 'ar', label: 'العربية' },
    { key: 'fr', label: 'Français' },
    { key: 'ru', label: 'Русский' },
    { key: 'es', label: 'Español' },
  ]

  const handleLangChange = (key: string) => {
    i18n.changeLanguage(key)
    message.success(t('common.success'))
  }

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
        <div style={{ height: '100%', display: 'flex', flexDirection: 'column' }}>
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
        
        <div style={{ display: 'flex', flexDirection: 'column', flex: 1, overflowY: 'auto', overflowX: 'hidden' }}>
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
              label: <Link to="/" style={{ color: '#333333' }}>{t('menu.overview')}</Link>
            },
            {
              key: '/sites',
              icon: <FileTextOutlined style={{ color: '#2f855a' }} />,
              label: <Link to="/sites" style={{ color: '#333333' }}>{t('menu.sites')}</Link>
            },
            {
              key: '/firewall',
              icon: <SecurityScanOutlined style={{ color: '#2f855a' }} />,
              label: <Link to="/firewall" style={{ color: '#333333' }}>{t('menu.firewall')}</Link>
            },
            {
              key: '/prerender/preheat',
              icon: <CodeOutlined style={{ color: '#2f855a' }} />,
              label: <Link to="/prerender/preheat" style={{ color: '#333333' }}>{t('sites.prerenderConfig')}</Link>
            },
            {
              key: '/prerender/push',
              icon: <CloudUploadOutlined style={{ color: '#2f855a' }} />,
              label: <Link to="/prerender/push" style={{ color: '#333333' }}>{t('sites.pushConfig')}</Link>
            },

            {
              key: '/monitoring',
              icon: <BarChartOutlined style={{ color: '#2f855a' }} />,
              label: <Link to="/monitoring" style={{ color: '#333333' }}>{t('menu.monitor')}</Link>
            },
            {
              key: '/crawler',
              icon: <BugOutlined style={{ color: '#2f855a' }} />,
              label: <Link to="/crawler" style={{ color: '#333333' }}>{t('menu.crawler')}</Link>
            },
            {
              key: '/system',
              icon: <SettingOutlined style={{ color: '#2f855a' }} />,
              label: <Link to="/system" style={{ color: '#333333' }}>{t('menu.settings')}</Link>
            },

          ]}
        />
        
        {/* 底部版本信息 */}
        {!collapsed && (
          <div style={{ 
            padding: '16px 0', 
            textAlign: 'center', 
            borderTop: '1px solid #f0f0f0',
            backgroundColor: '#fff'
          }}>
            <div style={{ fontSize: '12px', color: '#999', marginBottom: '4px' }}>
              {appInfo.version ? `v${appInfo.version}` : ''}
            </div>
            <div>
              <a 
                href={appInfo.official_url} 
                target="_blank" 
                rel="noopener noreferrer"
                style={{ fontSize: '12px', color: '#2f855a', textDecoration: 'none' }}
              >
                {t('system.visitOfficial')}
              </a>
            </div>
          </div>
        )}
        </div>
        </div>
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
            {/* 语言切换 */}
            <Dropdown 
              menu={{ 
                items: langItems, 
                onClick: ({ key }) => handleLangChange(key) 
              }} 
              placement="bottomRight"
            >
              <Button type="text" icon={<GlobalOutlined />} style={{ marginRight: 16 }}>
                {langItems.find(i => i.key === (i18n.language.split('-')[0]))?.label || 'Language'}
              </Button>
            </Dropdown>

            <span style={{ marginRight: 16 }}>{username || '管理员'}</span>
            <Button 
              type="text" 
              icon={<LogoutOutlined />} 
              onClick={handleLogout}
              style={{ marginRight: 8 }}
            >
              {t('common.cancel')}
            </Button>
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
              {username ? username.charAt(0).toUpperCase() : 'A'}
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