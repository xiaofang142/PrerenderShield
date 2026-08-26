import React, { useState } from 'react'
import { Card, Table, Select, Spin, Row, Col, Tabs, Input, DatePicker, Button, Space, Switch } from 'antd'
import { SearchOutlined, BarChartOutlined, GlobalOutlined, SyncOutlined } from '@ant-design/icons'
import dayjs from 'dayjs'
import ExportButton from '../../components/common/ExportButton'
import { usePolling } from '@prerender/utils'
import { pollingIntervals } from '@prerender/design-tokens'
import {
  useLogs,
  filterLogs,
  formatLogForExport,
} from './useLogs'
import { useTranslation } from 'react-i18next'

const { Option } = Select
const { TabPane } = Tabs
const { RangePicker } = DatePicker

const methodColorMap: Record<string, string> = {
  GET: '#52c41a',
  POST: '#1890ff',
  PUT: '#faad14',
  DELETE: '#ff4d4f',
  PATCH: '#722ed1',
}

const statusColor = (status: number): string =>
  status >= 200 && status < 300 ? '#52c41a'
    : status >= 300 && status < 400 ? '#1890ff'
    : status >= 400 && status < 500 ? '#faad14'
    : '#ff4d4f'

const durationColor = (ms: number): string =>
  ms < 100 ? '#52c41a' : ms < 500 ? '#faad14' : '#ff4d4f'

const EmptyHint: React.FC = () => {
  const { t } = useTranslation()
  return (
    <div style={{ textAlign: 'center', padding: '40px', color: '#999' }}>{t('logs.noData')}</div>
  )
}

interface RankColumn {
  title: string
  key: string
  width: number
  render: (_: unknown, __: unknown, index: number) => number
}
const rankColumn: RankColumn = {
  title: '排名',
  key: 'rank',
  width: 60,
  render: (_: unknown, __: unknown, index: number) => index + 1,
}

const Logs: React.FC = () => {
  const { t } = useTranslation()
  const [pageSize, setPageSize] = useState(20)
  const [autoRefresh, setAutoRefresh] = useState(false)

  // 筛选条件
  const [filterIP, setFilterIP] = useState('')
  const [filterMethod, setFilterMethod] = useState('')
  const [filterStatus, setFilterStatus] = useState('')
  const [dateRange, setDateRange] = useState<[dayjs.Dayjs | null, dayjs.Dayjs | null]>([null, null])

  const { logs, total, loading, currentPage, stats, fetchLogs } = useLogs({ pageSize })

  // 自动刷新：usePolling 统一生命周期管理（enabled 开关）
  usePolling(() => fetchLogs(currentPage), {
    interval: pollingIntervals.realtime,
    enabled: autoRefresh,
    immediate: false,
  })

  const rankCol: RankColumn = { ...rankColumn, title: t('logs.columns.rank') }

  const columns = [
    { title: t('logs.columns.time'), dataIndex: 'time', key: 'time', render: (t: string) => dayjs(t).format('YYYY-MM-DD HH:mm:ss') },
    { title: t('logs.columns.ip'), dataIndex: 'ip', key: 'ip', render: (text: string) => <span style={{ fontFamily: 'monospace' }}>{text}</span> },
    { title: t('logs.columns.method'), dataIndex: 'method', key: 'method', width: 80, render: (text: string) => (
      <span style={{ color: methodColorMap[text] || '#333', fontWeight: 500 }}>{text}</span>
    )},
    { title: t('logs.columns.path'), dataIndex: 'path', key: 'path', ellipsis: true },
    { title: t('logs.columns.status'), dataIndex: 'status', key: 'status', width: 80, render: (text: number) => (
      <span style={{ color: statusColor(text), fontWeight: 500 }}>{text}</span>
    )},
    { title: t('logs.columns.duration'), dataIndex: 'duration', key: 'duration', width: 100, render: (text: number) => (
      <span style={{ color: durationColor(text) }}>{text}</span>
    )},
  ]

  const filteredLogs = filterLogs(logs, { ip: filterIP, method: filterMethod, status: filterStatus })

  return (
    <Spin spinning={loading}>
      <div>
        <h1 className="page-title">{t('logs.title')}</h1>

        <div style={{ marginBottom: 16, display: 'flex', justifyContent: 'flex-end' }}>
          <Space>
            <SyncOutlined spin={autoRefresh} />
            <span>{t('logs.autoRefresh')}</span>
            <Switch checked={autoRefresh} onChange={setAutoRefresh} size="small" />
          </Space>
        </div>

        <Tabs defaultActiveKey="logs">
          {/* 访问日志 */}
          <TabPane
            tab={
              <Space>
                <BarChartOutlined />
                <span>{t('logs.tabAccessLogs')}</span>
              </Space>
            }
            key="logs"
          >
            {/* 筛选条件 */}
            <Card className="card" style={{ marginBottom: 16 }}>
              <Row gutter={16} align="middle">
                <Col>
                  <Input
                    placeholder={t('logs.filterIp')}
                    prefix={<GlobalOutlined />}
                    value={filterIP}
                    onChange={(e) => setFilterIP(e.target.value)}
                    style={{ width: 150 }}
                    allowClear
                  />
                </Col>
                <Col>
                  <Select
                    placeholder={t('logs.filterMethod')}
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
                    placeholder={t('logs.filterStatus')}
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
                      {t('common.search')}
                    </Button>
                    <ExportButton
                      data={filteredLogs.map(formatLogForExport)}
                      columns={[
                        { title: t('logs.columns.time'), dataIndex: '时间', key: 'time' },
                        { title: t('logs.columns.ip'), dataIndex: 'IP', key: 'ip' },
                        { title: t('logs.columns.method'), dataIndex: '方法', key: 'method' },
                        { title: t('logs.columns.path'), dataIndex: '路径', key: 'path' },
                        { title: t('logs.columns.status'), dataIndex: '状态码', key: 'status' },
                        { title: t('logs.columns.duration'), dataIndex: '耗时ms', key: 'duration' },
                      ]}
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
                  onChange: (p, ps) => { setPageSize(ps); fetchLogs(p) },
                  showSizeChanger: true,
                  showTotal: (total: number) => t('logs.totalRecords', { total }),
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
                <span>{t('logs.tabStats')}</span>
              </Space>
            }
            key="stats"
          >
            <Row gutter={[16, 16]}>
              {/* 热门IP */}
              <Col span={12}>
                <Card className="card" title={t('logs.topIps')}>
                  {stats.topIPs.length > 0 ? (
                    <Table
                      columns={[
                        rankCol,
                        { title: t('logs.ipAddress'), dataIndex: 'ip', key: 'ip' },
                        { title: t('logs.requestCount'), dataIndex: 'count', key: 'count', render: (text: number) => <span style={{ fontWeight: 500 }}>{text}</span> },
                      ]}
                      dataSource={stats.topIPs}
                      rowKey="ip"
                      pagination={false}
                      size="small"
                    />
                  ) : (
                    <EmptyHint />
                  )}
                </Card>
              </Col>

              {/* 热门URL */}
              <Col span={12}>
                <Card className="card" title={t('logs.topUrls')}>
                  {stats.topURLs.length > 0 ? (
                    <Table
                      columns={[
                        rankCol,
                        { title: t('logs.url'), dataIndex: 'url', key: 'url', ellipsis: true },
                        { title: t('logs.requestCount'), dataIndex: 'count', key: 'count', render: (text: number) => <span style={{ fontWeight: 500 }}>{text}</span> },
                      ]}
                      dataSource={stats.topURLs}
                      rowKey="url"
                      pagination={false}
                      size="small"
                    />
                  ) : (
                    <EmptyHint />
                  )}
                </Card>
              </Col>

              {/* 请求方法分布 */}
              <Col span={12}>
                <Card className="card" title={t('logs.methodDistribution')}>
                  {stats.methodStats.length > 0 ? (
                    <Table
                      columns={[
                        { title: t('logs.columns.method'), dataIndex: 'method', key: 'method', render: (text: string) => (
                          <span style={{ color: methodColorMap[text] || '#333', fontWeight: 500 }}>{text}</span>
                        )},
                        { title: t('logs.requestCount'), dataIndex: 'count', key: 'count' },
                        { title: t('logs.percent'), key: 'percent', render: (_: unknown, record: { count: number }) => {
                          const totalCount = stats.methodStats.reduce((sum, m) => sum + m.count, 0)
                          return `${((record.count / totalCount) * 100).toFixed(1)}%`
                        }},
                      ]}
                      dataSource={stats.methodStats}
                      rowKey="method"
                      pagination={false}
                      size="small"
                    />
                  ) : (
                    <EmptyHint />
                  )}
                </Card>
              </Col>

              {/* 状态码分布 */}
              <Col span={12}>
                <Card className="card" title={t('logs.statusDistribution')}>
                  {stats.statusStats.length > 0 ? (
                    <Table
                      columns={[
                        { title: t('logs.columns.status'), dataIndex: 'status', key: 'status', render: (text: number) => (
                          <span style={{ color: statusColor(text), fontWeight: 500 }}>{text}</span>
                        )},
                        { title: t('logs.requestCount'), dataIndex: 'count', key: 'count' },
                        { title: t('logs.percent'), key: 'percent', render: (_: unknown, record: { count: number }) => {
                          const totalCount = stats.statusStats.reduce((sum, s) => sum + s.count, 0)
                          return `${((record.count / totalCount) * 100).toFixed(1)}%`
                        }},
                      ]}
                      dataSource={stats.statusStats}
                      rowKey="status"
                      pagination={false}
                      size="small"
                    />
                  ) : (
                    <EmptyHint />
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
