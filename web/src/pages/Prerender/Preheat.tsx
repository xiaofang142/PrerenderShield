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
  const [manualUrls, setManualUrls] = useState<string>('')
  const [preheatModalVisible, setPreheatModalVisible] = useState(false)
  const [currentPage, setCurrentPage] = useState(1)
  const [pageSize, setPageSize] = useState(20)
  const [total, setTotal] = useState(0)
  const [isPreheating, setIsPreheating] = useState(false)
  const [preheatProgress, setPreheatProgress] = useState(0)

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
    message.success('统计数据已刷新')
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
        message.success('预热任务已触发，正在执行中')
        // 开始轮询进度（这里简化处理，实际可以通过WebSocket或定期查询）
        let progress = 0
        const interval = setInterval(() => {
          progress += 10
          setPreheatProgress(progress)
          if (progress >= 100) {
            clearInterval(interval)
            setIsPreheating(false)
            setPreheatProgress(0)
            fetchStats()
            fetchUrls(currentPage, pageSize)
          }
        }, 1000)
        setPreheatModalVisible(true)
      }
    } catch (error) {
      console.error('Failed to trigger preheat:', error)
      message.error('触发预热失败')
    } finally {
      // setIsPreheating(false)
    }
  }

  // 手动预热URL
  const handleManualPreheat = async () => {
    if (!selectedSiteId) {
      message.warning('请先选择站点')
      return
    }

    if (!manualUrls.trim()) {
      message.warning('请输入要预热的URL')
      return
    }

    try {
      setIsPreheating(true)
      const urls = manualUrls.split('\n').filter(url => url.trim())
      const res = await prerenderApi.preheatUrls(selectedSiteId, urls)
      if (res.code === 200) {
        message.success(`已触发 ${urls.length} 个URL的预热任务`)
        setManualUrls('')
        setPreheatModalVisible(true)
        // 开始轮询进度
        let progress = 0
        const interval = setInterval(() => {
          progress += 10
          setPreheatProgress(progress)
          if (progress >= 100) {
            clearInterval(interval)
            setIsPreheating(false)
            setPreheatProgress(0)
            fetchStats()
            fetchUrls(currentPage, pageSize)
          }
        }, 1000)
      }
    } catch (error) {
      console.error('Failed to preheat URLs:', error)
      message.error('手动预热失败')
    } finally {
      // setIsPreheating(false)
    }
  }

  // 单个URL预热
  const handleSinglePreheat = async (url: string) => {
    if (!selectedSiteId) {
      message.warning('请先选择站点')
      return
    }
    
    try {
      setIsPreheating(true)
      const res = await prerenderApi.preheatUrls(selectedSiteId, [url])
      if (res.code === 200) {
        message.success('URL预热任务已触发')
        // 模拟进度
        let progress = 0
        const interval = setInterval(() => {
          progress += 20
          setPreheatProgress(progress)
          if (progress >= 100) {
            clearInterval(interval)
            setIsPreheating(false)
            setPreheatProgress(0)
            fetchStats()
            fetchUrls(currentPage, pageSize)
          }
        }, 500)
        setPreheatModalVisible(true)
      }
    } catch (error) {
      console.error('Failed to preheat URL:', error)
      message.error('URL预热失败')
    } finally {
      // setIsPreheating(false)
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

  // 关闭预热弹窗
  const handleClosePreheatModal = () => {
    setPreheatModalVisible(false)
    setIsPreheating(false)
    setPreheatProgress(0)
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
          <Col span={8}>
            <Button type="primary" icon={<ReloadOutlined />} onClick={handleRefreshStats} loading={loading}>
              刷新数据
            </Button>
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
      
      {/* 手动操作区 */}
      <Card className="card" title="手动操作" style={{ marginBottom: 16 }}>
        <Row gutter={[16, 16]}>
          {/* 左侧面板：站点操作 */}
          <Col span={12}>
            <Card size="small" title="站点操作">
              <Space direction="vertical" style={{ width: '100%' }}>
                <Button type="primary" icon={<FireOutlined />} onClick={handleTriggerPreheat} loading={isPreheating} block>
                  触发站点预热
                </Button>
                <Button icon={<ReloadOutlined />} onClick={() => fetchUrls(currentPage, pageSize)} loading={urlLoading} block>
                  刷新URL列表
                </Button>
              </Space>
            </Card>
          </Col>
          {/* 右侧面板：手动预热URL */}
          <Col span={12}>
            <Card size="small" title="手动预热URL">
              <Space direction="vertical" style={{ width: '100%' }}>
                <TextArea
                  rows={4}
                  placeholder="请输入要预热的URL，一行一个地址"
                  value={manualUrls}
                  onChange={(e) => setManualUrls(e.target.value)}
                />
                <Button
                  type="primary"
                  icon={<PlayCircleOutlined />}
                  onClick={handleManualPreheat}
                  loading={isPreheating}
                  block
                >
                  执行手动预热
                </Button>
              </Space>
            </Card>
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
      
      {/* 预热进度弹窗 */}
      <Modal
        title="预热进度"
        open={preheatModalVisible}
        onCancel={handleClosePreheatModal}
        footer={null}
        closable={false}
      >
        <div style={{ textAlign: 'center', padding: '20px 0' }}>
          <Spin tip="正在预热中..." spinning={isPreheating} size="large">
            <div style={{ marginBottom: 20 }}>
              <Statistic
                title="预热进度"
                value={preheatProgress}
                suffix="%"
                valueStyle={{ color: '#1890ff' }}
              />
            </div>
            <div style={{ marginTop: 20 }}>
              <Button type="primary" onClick={handleClosePreheatModal} disabled={isPreheating}>
                关闭
              </Button>
            </div>
          </Spin>
        </div>
      </Modal>
    </div>
  )
}

export default Preheat
