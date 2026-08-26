import React, { useState, useEffect } from 'react'
import { Tabs, Dropdown } from 'antd'
import { HomeOutlined, ReloadOutlined } from '@ant-design/icons'
import { useLocation, useNavigate } from 'react-router-dom'
import { useTranslation } from 'react-i18next'

// 路由到标签名 i18n key 的映射
// 注：/firewall 复用 menu.firewall（与侧边栏一致），其余走 multiTab 命名空间
const routeTitleKeyMap: Record<string, [string, string]> = {
  '/': ['multiTab', 'overview'],
  '/sites': ['multiTab', 'sites'],
  '/firewall': ['menu', 'firewall'],
  '/firewall/rules': ['multiTab', 'firewallRules'],
  '/prerender': ['multiTab', 'prerender'],
  '/prerender/preheat': ['multiTab', 'preheat'],
  '/prerender/push': ['multiTab', 'push'],
  '/monitoring': ['multiTab', 'monitoring'],
  '/monitoring/alerts': ['multiTab', 'alertConfig'],
  '/crawler': ['multiTab', 'crawler'],
  '/logs': ['multiTab', 'logs'],
  '/system': ['multiTab', 'system'],
  '/ssl': ['multiTab', 'ssl'],
  '/settings': ['multiTab', 'settings'],
  '/dashboard': ['multiTab', 'dashboard'],
}

interface TabItem {
  key: string
  label: string
  closable: boolean
}

interface MultiTabProps {
  children: React.ReactNode
}

const MultiTab: React.FC<MultiTabProps> = ({ children }) => {
  const location = useLocation()
  const navigate = useNavigate()
  const { t } = useTranslation()
  const [tabs, setTabs] = useState<TabItem[]>([
    { key: '/', label: t('multiTab.overview'), closable: false }
  ])
  const [activeKey, setActiveKey] = useState('/')

  // 当路由变化时，添加或激活标签
  useEffect(() => {
    const path = location.pathname
    
    // 检查是否已存在该标签
    const existingTab = tabs.find(tab => tab.key === path)
    
    if (!existingTab) {
      // 添加新标签
      const entry = routeTitleKeyMap[path]
      const title = entry ? t(`${entry[0]}.${entry[1]}`) : path.split('/').pop() || t('multiTab.page')
      setTabs(prev => [...prev, {
        key: path,
        label: title,
        closable: path !== '/'
      }])
    }
    
    setActiveKey(path)
  }, [location.pathname])

  // 切换标签
  const handleTabChange = (key: string) => {
    navigate(key)
  }

  // 关闭标签
  const handleTabEdit = (targetKey: React.MouseEvent | React.KeyboardEvent | string, action: 'add' | 'remove') => {
    if (action === 'remove' && typeof targetKey === 'string') {
      const index = tabs.findIndex(tab => tab.key === targetKey)
      if (index === -1) return
      
      // 不允许关闭首页标签
      if (targetKey === '/') return
      
      const newTabs = tabs.filter(tab => tab.key !== targetKey)
      setTabs(newTabs)
      
      // 如果关闭的是当前激活的标签，切换到前一个标签
      if (activeKey === targetKey) {
        const newActiveKey = newTabs[index - 1]?.key || '/'
        navigate(newActiveKey)
      }
    }
  }

  // 关闭其他标签
  const handleCloseOtherTabs = () => {
    setTabs(tabs.filter(tab => !tab.closable || tab.key === activeKey))
  }

  // 关闭所有标签
  const handleCloseAllTabs = () => {
    setTabs(tabs.filter(tab => !tab.closable))
    navigate('/')
  }

  // 刷新当前标签
  const handleRefreshTab = () => {
    // 通过重新导航来刷新页面
    navigate(0)
  }

  // 右键菜单
  const getContextMenuItems = () => {
    return [
      {
        key: 'refresh',
        label: t('multiTab.refreshPage'),
        icon: <ReloadOutlined />,
        onClick: handleRefreshTab,
      },
      {
        key: 'closeOthers',
        label: t('multiTab.closeOthers'),
        onClick: handleCloseOtherTabs,
      },
      {
        key: 'closeAll',
        label: t('multiTab.closeAll'),
        onClick: handleCloseAllTabs,
      },
    ]
  }

  // 渲染标签标题
  const renderTabLabel = (tab: TabItem) => {
    return (
      <Dropdown
        menu={{ items: getContextMenuItems() }}
        trigger={['contextMenu']}
      >
        <span style={{ padding: '0 8px' }}>
          {tab.key === '/' && <HomeOutlined style={{ marginRight: 4 }} />}
          {tab.label}
        </span>
      </Dropdown>
    )
  }

  return (
    <div>
      {/* 标签栏 */}
      <div
        style={{
          background: '#fff',
          borderBottom: '1px solid #e8e8e8',
          padding: '0 16px',
          boxShadow: '0 1px 4px rgba(0, 0, 0, 0.05)',
        }}
      >
        <Tabs
          type="editable-card"
          hideAdd
          activeKey={activeKey}
          onChange={handleTabChange}
          onEdit={handleTabEdit}
          items={tabs.map(tab => ({
            key: tab.key,
            label: renderTabLabel(tab),
            closable: tab.closable,
          }))}
          style={{ marginBottom: 0 }}
          tabBarStyle={{ marginBottom: 0 }}
        />
      </div>
      
      {/* 页面内容 */}
      <div style={{ padding: '16px 0' }}>
        {children}
      </div>
    </div>
  )
}

export default MultiTab
