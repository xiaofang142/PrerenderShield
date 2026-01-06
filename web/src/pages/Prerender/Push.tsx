import React, { useState, useEffect } from 'react'
import { Card, Row, Col, Statistic, Button, Table, Select, message } from 'antd'
import { ReloadOutlined, UploadOutlined, BarChartOutlined } from '@ant-design/icons'
import { sitesApi, pushApi } from '../../services/api'

const { Option } = Select

const Push: React.FC = () => {
  const [sites, setSites] = useState<any[]>([])
  const [selectedSiteId, setSelectedSiteId] = useState<string>('')
  const [stats, setStats] = useState({
    total: 0,
    success: 0,
    failed: 0,
  })
  const [loading, setLoading] = useState(false)
  const [logList, setLogList] = useState<any[]>([])
  const [logLoading, setLogLoading] = useState(false)
  const [currentPage, setCurrentPage] = useState(1)
  const [pageSize, setPageSize] = useState(20)
  const [total, setTotal] = useState(0)

  // 日志表格列配置
  const columns = [
    {
      title: '站点名称',
      dataIndex: 'siteName',
      key: 'siteName',
    },
    {
      title: '推送URL',
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
      title: '路由',
      dataIndex: 'route',
      key: 'route',
      ellipsis: true,
    },
    {
      title: '搜索引擎',
      dataIndex: 'searchEngine',
      key: 'searchEngine',
      render: (engine: string) => {
        const engineMap: { [key: string]: string } = {
          'baidu': '百度',
          'bing': '必应',
        }
        return engineMap[engine] || engine
      }
    },
    {
      title: '状态',
      dataIndex: 'status',
      key: 'status',
      render: (status: string) => {
        const statusMap: { [key: string]: { text: string; color: string } } = {
          'success': { text: '成功', color: '#52c41a' },
          'failed': { text: '失败', color: '#f5222d' },
        }
        const config = statusMap[status] || { text: '未知', color: '#8c8c8c' }
        return <span style={{ color: config.color }}>{config.text}</span>
      }
    },
    {
      title: '推送时间',
      dataIndex: 'pushTime',
      key: 'pushTime',
      render: (time: string) => {
        if (!time) return '-'
        const date = new Date(time)
        return date.toLocaleString()
      }
    },
    {
      title: '消息',
      dataIndex: 'message',
      key: 'message',
      ellipsis: true,
      render: (message: string) => (
        <span title={message}>{message}</span>
      )
    },
  ]

  // 获取站点列表
  const fetchSites = async () => {
    try {
      setLoading(true)
      const res = await sitesApi.getSites()
      if (res.code === 200) {
        setSites(res.data)
        if (res.data.length > 0 && !selectedSiteId) {
          setSelectedSiteId(res.data[0].id)
        }
      }
    } catch (error) {
      console.error('Failed to fetch sites:', error)
      message.error('获取站点列表失败')
    } finally {
      setLoading(false)
    }
  }

  // 获取推送统计数据
  const fetchStats = async () => {
    try {
      setLoading(true)
      const res = await pushApi.getStats(selectedSiteId)
      if (res.code === 200) {
        setStats({
          total: res.data.stats.total || 0,
          success: res.data.stats.success || 0,
          failed: res.data.stats.failed || 0,
        })
      }
    } catch (error) {
      console.error('Failed to fetch stats:', error)
      message.error('获取推送统计数据失败')
    } finally {
      setLoading(false)
    }
  }

  // 获取推送日志
  const fetchLogs = async (page: number = 1, size: number = 20) => {
    try {
      setLogLoading(true)
      const res = await pushApi.getLogs(selectedSiteId, page, size)
      if (res.code === 200) {
        setLogList(res.data.list || [])
        setTotal(res.data.total || 0)
        setCurrentPage(page)
        setPageSize(size)
      }
    } catch (error) {
      console.error('Failed to fetch logs:', error)
      message.error('获取推送日志失败')
    } finally {
      setLogLoading(false)
    }
  }

  // 初始化数据
  useEffect(() => {
    fetchSites()
  }, [])

  // 当选中站点变化时，重新获取统计数据和日志列表
  useEffect(() => {
    if (selectedSiteId) {
      fetchStats()
      fetchLogs()
    }
  }, [selectedSiteId])

  // 刷新统计数据
  const handleRefreshStats = () => {
    fetchStats()
    fetchLogs(currentPage, pageSize)
    message.success('统计数据已刷新')
  }

  // 处理分页变化
  const handlePageChange = (page: number, size: number) => {
    setCurrentPage(page)
    setPageSize(size)
    fetchLogs(page, size)
  }

  return (
    <div>
      <h1 className="page-title">推送管理</h1>
      
      {/* 站点选择栏 */}
      <Card className="card" style={{ marginBottom: 16 }}>
        <Row align="middle" gutter={16}>
          <Col span={8}>
            <label style={{ marginRight: 8, fontWeight: 'bold' }}>选择站点：</label>
            <Select
              value={selectedSiteId}
              onChange={(value) => {
                setSelectedSiteId(value)
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
          <Col span={8}>
            <Statistic
              title="推送总数"
              value={stats.total}
              prefix={<UploadOutlined />}
              valueStyle={{ color: '#1890ff' }}
            />
          </Col>
          <Col span={8}>
            <Statistic
              title="成功数"
              value={stats.success}
              prefix={<BarChartOutlined />}
              valueStyle={{ color: '#52c41a' }}
            />
          </Col>
          <Col span={8}>
            <Statistic
              title="失败数"
              value={stats.failed}
              prefix={<BarChartOutlined />}
              valueStyle={{ color: '#f5222d' }}
            />
          </Col>
        </Row>
      </Card>
      
      {/* 推送日志列表 */}
      <Card className="card" title="推送日志">
        <Table
          columns={columns}
          dataSource={logList}
          rowKey="id"
          loading={logLoading}
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

export default Push
