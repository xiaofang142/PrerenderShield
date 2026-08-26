import React, { useState, useEffect, useRef } from 'react'
import { Card, Row, Col, Statistic, Button, Select, Space, message, Table } from 'antd'
import { ReloadOutlined, FireOutlined, PlayCircleOutlined, DeleteOutlined, SearchOutlined } from '@ant-design/icons'
import { useSites } from '../../hooks/useSites'
import { prerenderApi } from '../../services/api'
import { useTranslation } from 'react-i18next'

const { Option } = Select

const Preheat: React.FC = () => {
  const { t } = useTranslation()
  const { sites, selectedSiteId, setSelectedSiteId } = useSites({
    autoSelectFirst: true,
    filter: (site) => site.mode === 'static',
    onFetchError: (msg) => message.error(msg),
  })
  // 选中站点名称由列表派生，无需独立 state
  const selectedSiteName = sites.find((s) => s.id === selectedSiteId)?.name || sites.find((s) => s.id === selectedSiteId)?.Name || ''
  const [stats, setStats] = useState({
    urlCount: 0,
    cacheCount: 0,
    totalCacheSize: 0,
    browserPoolSize: 0,
  })
  const [loading, setLoading] = useState(false)
  const [urlList, setUrlList] = useState<any[]>([])
  const [urlLoading, setUrlLoading] = useState(false)
  const [currentPage, setCurrentPage] = useState(1)
  const [pageSize, setPageSize] = useState(20)
  const [total, setTotal] = useState(0)
  const [isPreheating, setIsPreheating] = useState(false)
  const [isClearingCache, setIsClearingCache] = useState(false)
  // 竞态防护：站点快速切换时，旧请求的响应不再写入 state
  const requestVersionRef = useRef(0)

  // 表格列配置
  const columns = [
    {
      title: 'URL',
      dataIndex: 'url',
      key: 'url',
      ellipsis: true,
      render: (url: string) => (
        <a href={url} target="_blank" rel="noopener noreferrer">
          {url}
        </a>
      )
    },
    {
      title: t('preheat.columns.siteName'),
      dataIndex: 'siteName',
      key: 'siteName',
      render: () => selectedSiteName || '-'
    },
    {
      title: t('preheat.columns.updatedAt'),
      dataIndex: 'updatedAt',
      key: 'updatedAt',
      render: (time: string) => {
        if (!time) return '-'
        const date = new Date(parseInt(time) * 1000)
        return date.toLocaleString()
      }
    },
  ]

  // 获取预热统计数据
   const fetchStats = async () => {
    const version = ++requestVersionRef.current
    try {
      setLoading(true)
      const res = await prerenderApi.getPreheatStats(selectedSiteId)
      if (version !== requestVersionRef.current) return
      if (res.code === 200) {
        setStats({
          urlCount: res.data.urlCount || 0,
          cacheCount: res.data.cacheCount || 0,
          totalCacheSize: res.data.totalCacheSize || 0,
          browserPoolSize: res.data.browserPoolSize || 0,
        })
      }
    } catch (error) {
      console.error('Failed to fetch stats:', error)
      message.error(t('preheat.fetchStatsFailed'))
    } finally {
      setLoading(false)
    }
  }

  // 获取URL列表
  const fetchUrls = async (page: number = 1, size: number = 20) => {
    const version = ++requestVersionRef.current
    try {
      setUrlLoading(true)
      const res = await prerenderApi.getUrls(selectedSiteId, page, size)
      if (version !== requestVersionRef.current) return
      if (res.code === 200) {
        setUrlList(res.data.list || [])
        setTotal(res.data.total || 0)
        setCurrentPage(page)
        setPageSize(size)
      }
    } catch (error) {
      console.error('Failed to fetch URLs:', error)
      message.error(t('preheat.fetchUrlsFailed'))
    } finally {
      setUrlLoading(false)
    }
  }



  // 初始化数据
  useEffect(() => {
    // 站点列表由 useSites 自动加载
  }, [])

  // 当选中站点变化时，重新获取统计数据和URL列表
  useEffect(() => {
    if (selectedSiteId) {
      fetchStats()
      fetchUrls()
    }
  }, [selectedSiteId])

  // 刷新统计数据
  const handleRefreshStats = () => {
    fetchStats()
    fetchUrls(currentPage, pageSize)
    message.success(t('preheat.refreshed'))
  }

  // 触发站点预热
  const handleTriggerPreheat = async () => {
    if (!selectedSiteId) {
      message.warning(t('preheat.selectSiteFirst'))
      return
    }

    try {
      setIsPreheating(true)
      const res = await prerenderApi.triggerPreheat(selectedSiteId)
      if (res.code === 200) {
        message.success(t('preheat.preheatCreated'))
      }
    } catch (error) {
      console.error('Failed to trigger preheat:', error)
      message.error(t('preheat.triggerPreheatFailed'))
    } finally {
      setIsPreheating(false)
    }
  }

  // 清除站点缓存
  const handleClearCache = async () => {
    if (!selectedSiteId) {
      message.warning(t('preheat.selectSiteFirst'))
      return
    }

    try {
      setIsClearingCache(true)
      const res = await prerenderApi.clearCache(selectedSiteId)
      if (res.code === 200) {
        message.success(t('preheat.clearCacheSuccess', { count: res.data.clearedCount }))
        // 刷新统计数据和URL列表
        fetchStats()
        fetchUrls(currentPage, pageSize)
      }
    } catch (error) {
      console.error('Failed to clear cache:', error)
      message.error(t('preheat.clearCacheFailed'))
    } finally {
      setIsClearingCache(false)
    }
  }



  // 处理分页变化
  const handlePageChange = (page: number, size: number) => {
    setCurrentPage(page)
    setPageSize(size)
    fetchUrls(page, size)
  }

  return (
    <div>
      <h1 className="page-title">{t('preheat.title')}</h1>
      
      {/* 站点选择栏 */}
      <Card className="card" style={{ marginBottom: 16 }}>
        <Row align="middle" gutter={16}>
          <Col span={8}>
            <label style={{ marginRight: 8, fontWeight: 'bold' }}>{t('preheat.selectSite')}</label>
            <Select
              value={selectedSiteId}
              onChange={(value) => {
                setSelectedSiteId(value)
              }}
              style={{ width: 200 }}
              loading={loading}
              placeholder={t('preheat.sitePlaceholder')}
            >
              {sites.map((site: any) => (
                <Option key={site.id} value={site.id}>
                  {site.name || site.Name} ({site.domain || site.Domains?.[0] || site.id})
                </Option>
              ))}
            </Select>
          </Col>
          <Col span={12}>
            <Space>
              <Button type="primary" icon={<ReloadOutlined />} onClick={handleRefreshStats} loading={loading}>
                {t('preheat.refreshData')}
              </Button>
              <Button type="primary" icon={<FireOutlined />} onClick={handleTriggerPreheat} loading={isPreheating}>
                {t('preheat.triggerPreheat')}
              </Button>
              <Button danger icon={<DeleteOutlined />} onClick={handleClearCache} loading={isClearingCache}>
                {t('preheat.clearCache')}
              </Button>
            </Space>
          </Col>
        </Row>
      </Card>
      
      {/* 统计数据卡片 */}
      <Card className="card" style={{ marginBottom: 16 }}>
        <Row gutter={[16, 16]}>
          <Col span={6}>
            <Statistic
              title={t('preheat.stats.urlCount')}
              value={stats.urlCount}
              prefix={<SearchOutlined />}
              valueStyle={{ color: '#1890ff' }}
            />
          </Col>
          <Col span={6}>
            <Statistic
              title={t('preheat.stats.cacheCount')}
              value={stats.cacheCount}
              prefix={<FireOutlined />}
              valueStyle={{ color: '#52c41a' }}
            />
          </Col>
          <Col span={6}>
            <Statistic
              title={t('preheat.stats.cacheSize')}
              value={stats.totalCacheSize}
              prefix={<DeleteOutlined />}
              valueStyle={{ color: '#faad14' }}
              formatter={(value) => {
                const numValue = Number(value)
                if (numValue < 1024) return `${numValue} B`
                if (numValue < 1024 * 1024) return `${(numValue / 1024).toFixed(2)} KB`
                return `${(numValue / (1024 * 1024)).toFixed(2)} MB`
              }}
            />
          </Col>
          <Col span={6}>
              <Statistic
                title={t('preheat.stats.browserPoolSize')}
                value={stats.browserPoolSize}
                prefix={<PlayCircleOutlined />}
                valueStyle={{ color: '#722ed1' }}
              />
            </Col>
        </Row>
      </Card>
      
      {/* URL列表 */}
      <Card className="card" title={t('preheat.urlListTitle')}>
        <Table
          columns={columns}
          dataSource={urlList}
          rowKey="url"
          loading={urlLoading}
          pagination={{
            current: currentPage,
            pageSize: pageSize,
            total: total,
            onChange: handlePageChange,
            showSizeChanger: true,
            pageSizeOptions: ['20', '50', '100'],
            showTotal: (total) => t('preheat.totalRecords', { total }),
          }}
        />
      </Card>
    </div>
  )
}

export default Preheat
