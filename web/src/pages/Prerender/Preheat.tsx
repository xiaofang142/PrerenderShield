import React, { useState, useEffect } from 'react'
import { Card, Row, Col, Statistic, Button, Input, message, Table, Select, Space, Form, Modal, Spin } from 'antd'
import { ReloadOutlined, FireOutlined, PlayCircleOutlined, DeleteOutlined, SearchOutlined } from '@ant-design/icons'
import { sitesApi, prerenderApi } from '../../services/api'

const { Option } = Select
const { TextArea } = Input

const Preheat: React.FC = () => {
  const [sites, setSites] = useState<any[]>([])
  const [selectedSiteId, setSelectedSiteId] = useState<string>('')
  const [selectedSiteName, setSelectedSiteName] = useState<string>('')
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
      title: '站点名称',
      dataIndex: 'siteName',
      key: 'siteName',
      render: () => selectedSiteName || '-'
    },
    {
      title: '更新时间',
      dataIndex: 'updatedAt',
      key: 'updatedAt',
      render: (time: string) => {
        if (!time) return '-'
        const date = new Date(parseInt(time) * 1000)
        return date.toLocaleString()
      }
    },
  ]

  // 获取静态网站列表
  const fetchSites = async () => {
    try {
      setLoading(true)
      const res = await sitesApi.getSites()
      if (res.code === 200) {
        // 只保留静态模式的站点
        const staticSites = res.data.filter((site: any) => site.mode === 'static')
        setSites(staticSites)
        if (staticSites.length > 0 && !selectedSiteId) {
          setSelectedSiteId(staticSites[0].id)
          setSelectedSiteName(staticSites[0].name || staticSites[0].Name || '')
        }
      }
    } catch (error) {
      console.error('Failed to fetch sites:', error)
      message.error('获取站点列表失败')
    } finally {
      setLoading(false)
    }
  }

  // 获取预热统计数据
   const fetchStats = async () => {
    try {
      setLoading(true)
      const res = await prerenderApi.getPreheatStats(selectedSiteId)
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
      message.error('获取统计数据失败')
    } finally {
      setLoading(false)
    }
  }

  // 获取URL列表
  const fetchUrls = async (page: number = 1, size: number = 20) => {
    try {
      setUrlLoading(true)
      const res = await prerenderApi.getUrls(selectedSiteId, page, size)
      if (res.code === 200) {
        setUrlList(res.data.list || [])
        setTotal(res.data.total || 0)
        setCurrentPage(page)
        setPageSize(size)
      }
    } catch (error) {
      console.error('Failed to fetch URLs:', error)
      message.error('获取URL列表失败')
    } finally {
      setUrlLoading(false)
    }
  }



  // 初始化数据
  useEffect(() => {
    fetchSites()
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
    message.success('数据已刷新')
  }

  // 触发站点预热
  const handleTriggerPreheat = async () => {
    if (!selectedSiteId) {
      message.warning('请先选择站点')
      return
    }

    try {
      setIsPreheating(true)
      const res = await prerenderApi.triggerPreheat(selectedSiteId)
      if (res.code === 200) {
        message.success('预热任务已创建成功，请稍后查看')
      }
    } catch (error) {
      console.error('Failed to trigger preheat:', error)
      message.error('触发预热失败')
    } finally {
      setIsPreheating(false)
      setPreheatProgress(0)
    }
  }

  // 删除URL
  const handleRemoveURL = async (_url: string) => {
    try {
      // 这里简化处理，实际应该调用API删除URL
      message.success('URL已删除')
      fetchUrls(currentPage, pageSize)
    } catch (error) {
      console.error('Failed to remove URL:', error)
      message.error('删除URL失败')
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
      <h1 className="page-title">渲染预热</h1>
      
      {/* 站点选择栏 */}
      <Card className="card" style={{ marginBottom: 16 }}>
        <Row align="middle" gutter={16}>
          <Col span={8}>
            <label style={{ marginRight: 8, fontWeight: 'bold' }}>选择站点：</label>
            <Select
              value={selectedSiteId}
              onChange={(value) => {
                const site = sites.find((s: any) => s.id === value)
                setSelectedSiteId(value)
                setSelectedSiteName(site?.name || site?.Name || '')
              }}
              style={{ width: 200 }}
              loading={loading}
              placeholder="请选择站点"
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
                刷新数据
              </Button>
              <Button type="primary" icon={<FireOutlined />} onClick={handleTriggerPreheat} loading={isPreheating}>
                触发站点预热
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
              title="URL总数"
              value={stats.urlCount}
              prefix={<SearchOutlined />}
              valueStyle={{ color: '#1890ff' }}
            />
          </Col>
          <Col span={6}>
            <Statistic
              title="缓存数"
              value={stats.cacheCount}
              prefix={<FireOutlined />}
              valueStyle={{ color: '#52c41a' }}
            />
          </Col>
          <Col span={6}>
            <Statistic
              title="缓存大小"
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
                title="浏览器池大小"
                value={stats.browserPoolSize}
                prefix={<PlayCircleOutlined />}
                valueStyle={{ color: '#722ed1' }}
              />
            </Col>
        </Row>
      </Card>
      
      {/* URL列表 */}
      <Card className="card" title="URL列表">
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
            showTotal: (total) => `共 ${total} 条记录`,
          }}
        />
      </Card>
    </div>
  )
}

export default Preheat
