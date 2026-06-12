import React, { useState, useEffect } from 'react'
import { Card, Table, Select, message, Spin, Row, Col, Statistic, Tabs, Input, DatePicker, Button, Space } from 'antd'
import { SearchOutlined, DownloadOutlined, BarChartOutlined, GlobalOutlined } from '@ant-design/icons'
import { firewallApi } from '../../services/api'
import dayjs from 'dayjs'
import ExportButton from '../../components/common/ExportButton'

const { Option } = Select
const { TabPane } = Tabs
const { RangePicker } = DatePicker

const Logs: React.FC = () => {
  const [logs, setLogs] = useState<any[]>([])
  const [total, setTotal] = useState(0)
  const [loading, setLoading] = useState(false)
  const [currentPage, setCurrentPage] = useState(1)
  const [pageSize, setPageSize] = useState(20)
  
  // 筛选条件
  const [filterIP, setFilterIP] = useState('')
  const [filterMethod, setFilterMethod] = useState('')
  const [filterStatus, setFilterStatus] = useState('')
  const [dateRange, setDateRange] = useState<[dayjs.Dayjs | null, dayjs.Dayjs | null]>([null, null])
  
  // 统计数据
  const [topIPs, setTopIPs] = useState<{ ip: string; count: number }[]>([])
  const [topURLs, setTopURLs] = useState<{ url: string; count: number }[]>([])
  const [methodStats, setMethodStats] = useState<{ method: string; count: number }[]>([])
  const [statusStats, setStatusStats] = useState<{ status: number; count: number }[]>([])

  const columns = [
    { title: '时间', dataIndex: 'time', key: 'time', render: (t: string) => dayjs(t).format('YYYY-MM-DD HH:mm:ss') },
    { title: 'IP', dataIndex: 'ip', key: 'ip', render: (text: string) => <span style={{ fontFamily: 'monospace' }}>{text}</span> },
    { title: '方法', dataIndex: 'method', key: 'method', width: 80, render: (text: string) => {
      const colorMap: Record<string, string> = {
        GET: '#52c41a',
        POST: '#1890ff',
        PUT: '#faad14',
        DELETE: '#ff4d4f',
        PATCH: '#722ed1',
      }
      return <span style={{ color: colorMap[text] || '#333', fontWeight: 500 }}>{text}</span>
    }},
    { title: '路径', dataIndex: 'path', key: 'path', ellipsis: true },
    { title: '状态码', dataIndex: 'status', key: 'status', width: 80, render: (text: number) => {
      const color = text >= 200 && text < 300 ? '#52c41a' 
        : text >= 300 && text < 400 ? '#1890ff'
        : text >= 400 && text < 500 ? '#faad14'
        : '#ff4d4f'
      return <span style={{ color, fontWeight: 500 }}>{text}</span>
    }},
    { title: '耗时(ms)', dataIndex: 'duration', key: 'duration', width: 100, render: (text: number) => {
      const color = text < 100 ? '#52c41a' : text < 500 ? '#faad14' : '#ff4d4f'
      return <span style={{ color }}>{text}</span>
    }},
  ]

  const fetchLogs = async (page = 1) => {
    try {
      setLoading(true)
      const res = await firewallApi.getAccessLogs({ page, limit: pageSize })
      if (res.code === 200) {
        const logData = res.data.logs || []
        setLogs(logData)
        setTotal(res.data.total || 0)
        setCurrentPage(page)
        
        // 计算统计数据
        calculateStats(logData)
      }
    } catch (error) {
      console.error('Failed to fetch access logs:', error)
    } finally {
      setLoading(false)
    }
  }

  // 计算统计数据
  const calculateStats = (logData: any[]) => {
    // 统计IP
    const ipCount: Record<string, number> = {}
    logData.forEach(log => {
      if (log.ip) {
        ipCount[log.ip] = (ipCount[log.ip] || 0) + 1
      }
    })
    const topIPsList = Object.entries(ipCount)
      .map(([ip, count]) => ({ ip, count }))
      .sort((a, b) => b.count - a.count)
      .slice(0, 10)
    setTopIPs(topIPsList)
    
    // 统计URL
    const urlCount: Record<string, number> = {}
    logData.forEach(log => {
      if (log.path) {
        urlCount[log.path] = (urlCount[log.path] || 0) + 1
      }
    })
    const topURLsList = Object.entries(urlCount)
      .map(([url, count]) => ({ url, count }))
      .sort((a, b) => b.count - a.count)
      .slice(0, 10)
    setTopURLs(topURLsList)
    
    // 统计方法
    const methodCount: Record<string, number> = {}
    logData.forEach(log => {
      if (log.method) {
        methodCount[log.method] = (methodCount[log.method] || 0) + 1
      }
    })
    const methodStatsList = Object.entries(methodCount)
      .map(([method, count]) => ({ method, count }))
      .sort((a, b) => b.count - a.count)
    setMethodStats(methodStatsList)
    
    // 统计状态码
    const statusCount: Record<number, number> = {}
    logData.forEach(log => {
      if (log.status) {
        statusCount[log.status] = (statusCount[log.status] || 0) + 1
      }
    })
    const statusStatsList = Object.entries(statusCount)
      .map(([status, count]) => ({ status: parseInt(status), count }))
      .sort((a, b) => b.count - a.count)
    setStatusStats(statusStatsList)
  }

  useEffect(() => { fetchLogs() }, [pageSize])

  // 筛选后的日志
  const filteredLogs = logs.filter(log => {
    if (filterIP && !log.ip?.includes(filterIP)) return false
    if (filterMethod && log.method !== filterMethod) return false
    if (filterStatus && log.status !== parseInt(filterStatus)) return false
    return true
  })

  return (
    <Spin spinning={loading}>
      <div>
        <h1 className="page-title">日志管理</h1>
        
        <Tabs defaultActiveKey="logs">
          {/* 访问日志 */}
          <TabPane 
            tab={
              <Space>
                <BarChartOutlined />
                <span>访问日志</span>
              </Space>
            } 
            key="logs"
          >
            {/* 筛选条件 */}
            <Card className="card" style={{ marginBottom: 16 }}>
              <Row gutter={16} align="middle">
                <Col>
                  <Input
                    placeholder="筛选 IP"
                    prefix={<GlobalOutlined />}
                    value={filterIP}
                    onChange={(e) => setFilterIP(e.target.value)}
                    style={{ width: 150 }}
                    allowClear
                  />
                </Col>
                <Col>
                  <Select
                    placeholder="请求方法"
                    value={filterMethod || undefined}
                    onChange={(value) => setFilterMethod(value || '')}
                    style={{ width: 120 }}
                    allowClear
                  >
                    <Option value="GET">GET</Option>
                    <Option value="POST">POST</Option>
                    <Option value="PUT">PUT</Option>
                    <Option value="DELETE">DELETE</Option>
                    <Option value="PATCH">PATCH</Option>
                  </Select>
                </Col>
                <Col>
                  <Select
                    placeholder="状态码"
                    value={filterStatus || undefined}
                    onChange={(value) => setFilterStatus(value || '')}
                    style={{ width: 120 }}
                    allowClear
                  >
                    <Option value="200">200</Option>
                    <Option value="301">301</Option>
                    <Option value="404">404</Option>
                    <Option value="500">500</Option>
                  </Select>
                </Col>
                <Col>
                  <RangePicker 
                    value={dateRange as any}
                    onChange={(dates) => setDateRange(dates as any)}
                  />
                </Col>
                <Col>
                  <Space>
                    <Button 
                      icon={<SearchOutlined />} 
                      onClick={() => fetchLogs(1)}
                    >
                      搜索
                    </Button>
                    <ExportButton 
                      data={filteredLogs} 
                      columns={columns}
                      filename="access_logs"
                    />
                  </Space>
                </Col>
              </Row>
            </Card>

            {/* 日志列表 */}
            <Card className="card">
              <Table
                columns={columns}
                dataSource={filteredLogs}
                rowKey="id"
                pagination={{
                  current: currentPage,
                  pageSize,
                  total,
                  onChange: (p, ps) => { setCurrentPage(p); setPageSize(ps); fetchLogs(p) },
                  showSizeChanger: true,
                  showTotal: (t) => `共 ${t} 条记录`,
                }}
                size="middle"
              />
            </Card>
          </TabPane>

          {/* 统计分析 */}
          <TabPane 
            tab={
              <Space>
                <BarChartOutlined />
                <span>统计分析</span>
              </Space>
            } 
            key="stats"
          >
            <Row gutter={[16, 16]}>
              {/* 热门IP */}
              <Col span={12}>
                <Card className="card" title="热门 IP (Top 10)">
                  {topIPs.length > 0 ? (
                    <Table
                      columns={[
                        { title: '排名', key: 'rank', render: (_: any, __: any, index: number) => index + 1, width: 60 },
                        { title: 'IP 地址', dataIndex: 'ip', key: 'ip' },
                        { title: '请求数', dataIndex: 'count', key: 'count', render: (text: number) => <span style={{ fontWeight: 500 }}>{text}</span> },
                      ]}
                      dataSource={topIPs}
                      rowKey="ip"
                      pagination={false}
                      size="small"
                    />
                  ) : (
                    <div style={{ textAlign: 'center', padding: '40px', color: '#999' }}>暂无数据</div>
                  )}
                </Card>
              </Col>

              {/* 热门URL */}
              <Col span={12}>
                <Card className="card" title="热门 URL (Top 10)">
                  {topURLs.length > 0 ? (
                    <Table
                      columns={[
                        { title: '排名', key: 'rank', render: (_: any, __: any, index: number) => index + 1, width: 60 },
                        { title: 'URL', dataIndex: 'url', key: 'url', ellipsis: true },
                        { title: '请求数', dataIndex: 'count', key: 'count', render: (text: number) => <span style={{ fontWeight: 500 }}>{text}</span> },
                      ]}
                      dataSource={topURLs}
                      rowKey="url"
                      pagination={false}
                      size="small"
                    />
                  ) : (
                    <div style={{ textAlign: 'center', padding: '40px', color: '#999' }}>暂无数据</div>
                  )}
                </Card>
              </Col>

              {/* 请求方法分布 */}
              <Col span={12}>
                <Card className="card" title="请求方法分布">
                  {methodStats.length > 0 ? (
                    <Table
                      columns={[
                        { title: '方法', dataIndex: 'method', key: 'method', render: (text: string) => {
                          const colorMap: Record<string, string> = {
                            GET: '#52c41a',
                            POST: '#1890ff',
                            PUT: '#faad14',
                            DELETE: '#ff4d4f',
                          }
                          return <span style={{ color: colorMap[text] || '#333', fontWeight: 500 }}>{text}</span>
                        }},
                        { title: '请求数', dataIndex: 'count', key: 'count' },
                        { title: '占比', key: 'percent', render: (_: any, record: any) => {
                          const total = methodStats.reduce((sum, m) => sum + m.count, 0)
                          const percent = ((record.count / total) * 100).toFixed(1)
                          return `${percent}%`
                        }},
                      ]}
                      dataSource={methodStats}
                      rowKey="method"
                      pagination={false}
                      size="small"
                    />
                  ) : (
                    <div style={{ textAlign: 'center', padding: '40px', color: '#999' }}>暂无数据</div>
                  )}
                </Card>
              </Col>

              {/* 状态码分布 */}
              <Col span={12}>
                <Card className="card" title="状态码分布">
                  {statusStats.length > 0 ? (
                    <Table
                      columns={[
                        { title: '状态码', dataIndex: 'status', key: 'status', render: (text: number) => {
                          const color = text >= 200 && text < 300 ? '#52c41a' 
                            : text >= 300 && text < 400 ? '#1890ff'
                            : text >= 400 && text < 500 ? '#faad14'
                            : '#ff4d4f'
                          return <span style={{ color, fontWeight: 500 }}>{text}</span>
                        }},
                        { title: '请求数', dataIndex: 'count', key: 'count' },
                        { title: '占比', key: 'percent', render: (_: any, record: any) => {
                          const total = statusStats.reduce((sum, s) => sum + s.count, 0)
                          const percent = ((record.count / total) * 100).toFixed(1)
                          return `${percent}%`
                        }},
                      ]}
                      dataSource={statusStats}
                      rowKey="status"
                      pagination={false}
                      size="small"
                    />
                  ) : (
                    <div style={{ textAlign: 'center', padding: '40px', color: '#999' }}>暂无数据</div>
                  )}
                </Card>
              </Col>
            </Row>
          </TabPane>
        </Tabs>
      </div>
    </Spin>
  )
}

export default Logs
