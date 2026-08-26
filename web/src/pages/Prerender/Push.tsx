import React, { useState, useEffect, useRef } from 'react'
import { Card, Row, Col, Statistic, Button, Table, Select, message } from 'antd'
import { ReloadOutlined, UploadOutlined, BarChartOutlined } from '@ant-design/icons'
import { useTranslation } from 'react-i18next'
import BaseChart from '../../components/charts/BaseChart'
import { pushApi } from '../../services/api'
import { useSites } from '../../hooks/useSites'

const { Option } = Select

const Push: React.FC = () => {
  const { t } = useTranslation()
  const { sites, selectedSiteId, setSelectedSiteId } = useSites({
    autoSelectFirst: true,
    onFetchError: (msg) => message.error(t('push.fetchSitesFailed') + ': ' + msg),
  })
  const [trendData, setTrendData] = useState<Record<string, number>>({})
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
  // 竞态防护：站点快速切换时，旧请求的响应不再写入 state
  const requestVersionRef = useRef(0)

  // 日志表格列配置
  const columns = [
    {
      title: t('push.columns.siteName'),
      dataIndex: 'siteName',
      key: 'siteName',
    },
    {
      title: t('push.columns.url'),
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
      title: t('push.columns.route'),
      dataIndex: 'route',
      key: 'route',
      ellipsis: true,
    },
    {
      title: t('push.columns.searchEngine'),
      dataIndex: 'searchEngine',
      key: 'searchEngine',
      render: (engine: string) => {
        const engineMap: { [key: string]: string } = {
          'baidu': t('push.engines.baidu'),
          'bing': t('push.engines.bing'),
        }
        return engineMap[engine] || engine
      }
    },
    {
      title: t('push.columns.status'),
      dataIndex: 'status',
      key: 'status',
      render: (status: string) => {
        const statusMap: { [key: string]: { text: string; color: string } } = {
          'success': { text: t('push.statusText.success'), color: '#52c41a' },
          'failed': { text: t('push.statusText.failed'), color: '#f5222d' },
        }
        const config = statusMap[status] || { text: t('push.statusText.unknown'), color: '#8c8c8c' }
        return <span style={{ color: config.color }}>{config.text}</span>
      }
    },
    {
      title: t('push.columns.pushTime'),
      dataIndex: 'pushTime',
      key: 'pushTime',
      render: (time: string) => {
        if (!time) return '-'
        const date = new Date(time)
        return date.toLocaleString()
      }
    },
    {
      title: t('push.columns.message'),
      dataIndex: 'message',
      key: 'message',
      ellipsis: true,
      render: (message: string) => (
        <span title={message}>{message}</span>
      )
    },
  ]

  // 获取推送统计数据
  const fetchStats = async () => {
    try {
      setLoading(true)
      const [statsRes, trendRes] = await Promise.all([
        pushApi.getStats(selectedSiteId),
        pushApi.getTrend(selectedSiteId)
      ])
      if (statsRes.code === 200) {
        setStats({
          total: statsRes.data.stats.total || 0,
          success: statsRes.data.stats.success || 0,
          failed: statsRes.data.stats.failed || 0,
        })
      }
      if (trendRes.code === 200) {
        setTrendData(trendRes.data || {})
      }
    } catch (error) {
      console.error('Failed to fetch stats:', error)
      message.error(t('push.fetchStatsFailed'))
    } finally {
      setLoading(false)
    }
  }

  // 获取推送日志
  const fetchLogs = async (page: number = 1, size: number = 20) => {
    const version = ++requestVersionRef.current
    try {
      setLogLoading(true)
      const res = await pushApi.getLogs(selectedSiteId, page, size)
      if (version !== requestVersionRef.current) return
      if (res.code === 200) {
        setLogList(res.data.list || [])
        setTotal(res.data.total || 0)
        setCurrentPage(page)
        setPageSize(size)
      }
    } catch (error) {
      console.error('Failed to fetch logs:', error)
      message.error(t('push.fetchLogsFailed'))
    } finally {
      setLogLoading(false)
    }
  }

  // 初始化数据
  useEffect(() => {
    // 站点列表由 useSites 自动加载
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
    message.success(t('push.refreshed'))
  }

  // 处理分页变化
  const handlePageChange = (page: number, size: number) => {
    setCurrentPage(page)
    setPageSize(size)
    fetchLogs(page, size)
  }

  return (
    <div>
      <h1 className="page-title">{t('push.title')}</h1>

      {/* 站点选择栏 */}
      <Card className="card" style={{ marginBottom: 16 }}>
        <Row align="middle" gutter={16}>
          <Col span={8}>
            <label style={{ marginRight: 8, fontWeight: 'bold' }}>{t('push.selectSite')}</label>
            <Select
              value={selectedSiteId}
              onChange={(value) => {
                setSelectedSiteId(value)
              }}
              style={{ width: 200 }}
              loading={loading}
              placeholder={t('push.sitePlaceholder')}
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
              {t('push.refresh')}
            </Button>
          </Col>
        </Row>
      </Card>

      {/* 统计数据卡片 */}
      <Card className="card" style={{ marginBottom: 16 }}>
        <Row gutter={[16, 16]}>
          <Col span={8}>
            <Statistic
              title={t('push.totalPushes')}
              value={stats.total}
              prefix={<UploadOutlined />}
              valueStyle={{ color: '#1890ff' }}
            />
          </Col>
          <Col span={8}>
            <Statistic
              title={t('push.successCount')}
              value={stats.success}
              prefix={<BarChartOutlined />}
              valueStyle={{ color: '#52c41a' }}
            />
          </Col>
          <Col span={8}>
            <Statistic
              title={t('push.failedCount')}
              value={stats.failed}
              prefix={<BarChartOutlined />}
              valueStyle={{ color: '#f5222d' }}
            />
          </Col>
        </Row>
      </Card>

      {/* 推送趋势图 */}
      <Card className="card" style={{ marginBottom: 16 }}>
        <h3 style={{ marginBottom: 16 }}>{t('push.trendTitle')}</h3>
        <BaseChart option={{
          tooltip: { trigger: 'axis' },
          xAxis: { type: 'category', data: Object.keys(trendData) },
          yAxis: { type: 'value', name: t('push.pushCount') },
          series: [{
            name: t('push.pushCount'), type: 'line',
            data: Object.values(trendData),
            smooth: true,
            itemStyle: { color: '#1890ff' },
            areaStyle: { color: 'rgba(24,144,255,0.1)' }
          }]
        }} style={{ height: 300 }} />
      </Card>

      {/* 推送日志列表 */}
      <Card className="card" title={t('push.logsTitle')}>
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
            showTotal: (total) => t('push.totalRecords', { total }),
          }}
        />
      </Card>
    </div>
  )
}

export default Push
