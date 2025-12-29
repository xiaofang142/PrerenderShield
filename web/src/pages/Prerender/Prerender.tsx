import React, { useState, useEffect } from 'react'
import { Card, Row, Col, Statistic, Button, Modal, Input, message, Table, Select } from 'antd'
import { CodeOutlined, PlayCircleOutlined, FireOutlined, ReloadOutlined } from '@ant-design/icons'
import { prerenderApi, sitesApi } from '../../services/api'

const { Search } = Input
const { Option } = Select

const Prerender: React.FC = () => {
  const [sites, setSites] = useState<any[]>([])
  const [selectedSite, setSelectedSite] = useState<string>('')
  const [status, setStatus] = useState({
    enabled: false,
    poolSize: 5,
    timeout: 30,
    cacheTTL: 3600,
    preheat: {
      enabled: false,
      sitemapURL: '',
      schedule: '0 0 * * *',
    },
  })
  const [loading, setLoading] = useState(true)
  const [renderModalVisible, setRenderModalVisible] = useState(false)
  const [preheatModalVisible, setPreheatModalVisible] = useState(false)
  const [renderUrl, setRenderUrl] = useState('')
  const [renderLoading, setRenderLoading] = useState(false)
  const [preheatLoading, setPreheatLoading] = useState(false)
  const [renderHistory, setRenderHistory] = useState<any[]>([])

  // 表格列配置
  const columns = [
    {
      title: 'URL',
      dataIndex: 'url',
      key: 'url',
      ellipsis: true,
    },
    {
      title: '站点',
      dataIndex: 'site',
      key: 'site',
    },
    {
      title: '状态',
      dataIndex: 'status',
      key: 'status',
      render: (text: string) => {
        const color = text === 'success' ? '#52c41a' : '#f5222d'
        return <span style={{ color }}>{text}</span>
      },
    },
    {
      title: '耗时(ms)',
      dataIndex: 'duration',
      key: 'duration',
    },
    {
      title: '时间',
      dataIndex: 'time',
      key: 'time',
    },
  ]

  // 获取站点列表
  const fetchSites = async () => {
    try {
      const res = await sitesApi.getSites()
      if (res.code === 200) {
        setSites(res.data)
        if (res.data.length > 0) {
          setSelectedSite(res.data[0].name)
        }
      }
    } catch (error) {
      console.error('Failed to fetch sites:', error)
      message.error('获取站点列表失败')
    }
  }

  // 获取预渲染状态
  const fetchStatus = async () => {
    if (!selectedSite) return
    
    try {
      setLoading(true)
      const res = await prerenderApi.getStatus(selectedSite)
      if (res.code === 200) {
        // 处理单站点数据结构
        const statusData = typeof res.data === 'object' && res.data.enabled !== undefined ? res.data : res.data[selectedSite]
        setStatus(statusData)
      }
    } catch (error) {
      console.error('Failed to fetch prerender status:', error)
      message.error('获取预渲染状态失败')
    } finally {
      setLoading(false)
    }
  }

  // 初始化数据
  useEffect(() => {
    fetchSites()
  }, [])

  // 当选择的站点变化时，重新获取状态
  useEffect(() => {
    if (selectedSite) {
      fetchStatus()
    }
  }, [selectedSite])

  // 手动触发渲染
  const handleRender = async () => {
    if (!selectedSite) {
      message.warning('请选择站点')
      return
    }
    
    if (!renderUrl) {
      message.warning('请输入要渲染的URL')
      return
    }

    try {
      setRenderLoading(true)
      const startTime = Date.now()
      const res = await prerenderApi.render({ site: selectedSite, url: renderUrl })
      const duration = Date.now() - startTime
      
      if (res.code === 200) {
        message.success('渲染成功')
        // 添加到渲染历史
        setRenderHistory(prev => [
          {
            site: selectedSite,
            url: renderUrl,
            status: res.data.success ? 'success' : 'error',
            duration,
            time: new Date().toLocaleString(),
          },
          ...prev.slice(0, 9), // 只保留最近10条
        ])
        setRenderModalVisible(false)
        setRenderUrl('')
      } else {
        message.error('渲染失败')
      }
    } catch (error) {
      console.error('Failed to render:', error)
      message.error('渲染失败')
    } finally {
      setRenderLoading(false)
    }
  }

  // 触发缓存预热
  const handlePreheat = async () => {
    if (!selectedSite) {
      message.warning('请选择站点')
      return
    }

    try {
      setPreheatLoading(true)
      const res = await prerenderApi.preheat({ site: selectedSite })
      if (res.code === 200) {
        message.success('缓存预热已触发')
        setPreheatModalVisible(false)
      } else {
        message.error('缓存预热触发失败')
      }
    } catch (error) {
      console.error('Failed to trigger preheat:', error)
      message.error('缓存预热触发失败')
    } finally {
      setPreheatLoading(false)
    }
  }

  return (
    <div>
      <h1 className="page-title">预渲染</h1>
      
      {/* 站点选择器 */}
      <Card className="card" style={{ marginBottom: 16 }}>
        <Row align="middle">
          <Col span={8}>
            <label style={{ marginRight: 8 }}>选择站点：</label>
            <Select
              value={selectedSite}
              onChange={setSelectedSite}
              style={{ width: 200 }}
              loading={sites.length === 0}
            >
              {sites.map((site) => (
                <Option key={site.name} value={site.name}>
                  {site.name} ({site.domain})
                </Option>
              ))}
            </Select>
          </Col>
          <Col span={8}>
            <Button type="primary" icon={<ReloadOutlined />} onClick={fetchStatus} loading={loading}>
              刷新状态
            </Button>
          </Col>
        </Row>
      </Card>
      
      {/* 预渲染状态卡片 */}
      <Card className="card">
        <Row gutter={[16, 16]}>
          <Col span={6}>
            <Statistic
              title="预渲染状态"
              value={status.enabled ? '已启用' : '已禁用'}
              prefix={<CodeOutlined />}
              valueStyle={{ color: status.enabled ? '#52c41a' : '#faad14' }}
            />
          </Col>
          <Col span={6}>
            <Statistic
              title="浏览器池大小"
              value={status.poolSize}
              valueStyle={{ color: '#1890ff' }}
            />
          </Col>
          <Col span={6}>
            <Statistic
              title="渲染超时(秒)"
              value={status.timeout}
              valueStyle={{ color: '#faad14' }}
            />
          </Col>
          <Col span={6}>
            <Statistic
              title="缓存TTL(秒)"
              value={status.cacheTTL}
              valueStyle={{ color: '#52c41a' }}
            />
          </Col>
        </Row>
      </Card>

      {/* 操作按钮 */}
      <Row gutter={[16, 16]} style={{ marginBottom: 16 }}>
        <Col span={12}>
          <Card className="card" title="手动操作">
            <Row gutter={[16, 16]}>
              <Col span={12}>
                <Button
                  type="primary"
                  icon={<PlayCircleOutlined />}
                  onClick={() => setRenderModalVisible(true)}
                  block
                >
                  手动渲染
                </Button>
              </Col>
              <Col span={12}>
                <Button
                  type="default"
                  icon={<FireOutlined />}
                  onClick={() => setPreheatModalVisible(true)}
                  block
                >
                  缓存预热
                </Button>
              </Col>
            </Row>
          </Card>
        </Col>
        <Col span={12}>
          <Card className="card" title="缓存预热配置">
            <Row gutter={[16, 16]}>
              <Col span={24}>
                <Statistic
                  title="预热状态"
                  value={status.preheat.enabled ? '已启用' : '已禁用'}
                  valueStyle={{ color: status.preheat.enabled ? '#52c41a' : '#faad14' }}
                />
              </Col>
              <Col span={24}>
                <Statistic
                  title="预热计划"
                  value={status.preheat.schedule}
                  valueStyle={{ color: '#1890ff' }}
                />
              </Col>
            </Row>
          </Card>
        </Col>
      </Row>

      {/* 渲染历史 */}
      <Card className="card" title="渲染历史">
        <Table
          columns={columns}
          dataSource={renderHistory}
          rowKey={(record, index) => index.toString()}
          pagination={{ pageSize: 10 }}
          emptyText="暂无渲染历史"
        />
      </Card>

      {/* 手动渲染模态框 */}
      <Modal
        title="手动渲染"
        open={renderModalVisible}
        onOk={handleRender}
        onCancel={() => setRenderModalVisible(false)}
        confirmLoading={renderLoading}
        okText="开始渲染"
        cancelText="取消"
      >
        <div style={{ marginBottom: 16 }}>
          <label style={{ display: 'block', marginBottom: 8 }}>URL地址：</label>
          <Search
            placeholder="请输入要渲染的URL"
            allowClear
            value={renderUrl}
            onChange={(e) => setRenderUrl(e.target.value)}
            onPressEnter={handleRender}
            style={{ width: '100%' }}
          />
        </div>
        <p style={{ color: '#666', fontSize: 12 }}>
          注意：渲染可能需要一段时间，请耐心等待
        </p>
      </Modal>

      {/* 缓存预热模态框 */}
      <Modal
        title="缓存预热"
        open={preheatModalVisible}
        onOk={handlePreheat}
        onCancel={() => setPreheatModalVisible(false)}
        confirmLoading={preheatLoading}
        okText="开始预热"
        cancelText="取消"
      >
        <p>确定要触发缓存预热吗？</p>
        <p style={{ color: '#666', fontSize: 12, marginTop: 16 }}>
          注意：预热可能需要较长时间，取决于站点规模
        </p>
      </Modal>
    </div>
  )
}

export default Prerender