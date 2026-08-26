import React, { useState, useEffect, useRef } from 'react'
import { Card, Row, Col, Statistic, Spin, Select, Table, Radio, Tabs } from 'antd'
import { ArrowUpOutlined } from '@ant-design/icons'
import { useTranslation } from 'react-i18next'
import BaseChart from '../../components/charts/BaseChart'
import { crawlerApi } from '../../services/api'
import { useSites } from '../../hooks/useSites'
import dayjs from 'dayjs'
import type { EChartsOption } from 'echarts'

const { Option } = Select
const { TabPane } = Tabs

const Crawler: React.FC = () => {
  const { t } = useTranslation()
  const { sites, selectedSiteId: selectedSite, setSelectedSiteId: setSelectedSite } = useSites()
  const [granularity, setGranularity] = useState<string>('day') // day, week, month
  const [logs, setLogs] = useState<any[]>([])
  const [totalLogs, setTotalLogs] = useState<number>(0)
  const [page, setPage] = useState<number>(1)
  const [pageSize, setPageSize] = useState<number>(10)
  const [stats, setStats] = useState({
    totalRequests: 0,
    cacheHitRate: 0.0,
    topUAs: [],
    trafficByHour: []
  })
  const [loading, setLoading] = useState<boolean>(true)
  // 竞态防护：筛选条件快速变化时，旧请求的响应不再写入 state
  const requestVersionRef = useRef(0)

  // 日志表格列配置
  const logColumns = [
    {
      title: t('crawlerPage.columns.time'),
      dataIndex: 'time',
      key: 'time',
      render: (text: string) => {
        return dayjs(text).format('YYYY-MM-DD HH:mm:ss')
      }
    },
    {
      title: t('crawlerPage.columns.site'),
      dataIndex: 'site',
      key: 'site'
    },
    {
      title: t('crawlerPage.columns.ip'),
      dataIndex: 'ip',
      key: 'ip'
    },
    {
      title: t('crawlerPage.columns.route'),
      dataIndex: 'route',
      key: 'route',
      ellipsis: true
    },
    {
      title: t('crawlerPage.columns.ua'),
      dataIndex: 'ua',
      key: 'ua',
      ellipsis: true,
      width: 300
    },
    {
      title: t('crawlerPage.columns.status'),
      dataIndex: 'status',
      key: 'status',
      render: (text: number) => {
        const color = text === 200 ? '#52c41a' : '#ff4d4f'
        return <span style={{ color }}>{text}</span>
      }
    },
    {
      title: t('crawlerPage.columns.hitCache'),
      dataIndex: 'hitCache',
      key: 'hitCache',
      render: (text: boolean) => {
        const color = text ? '#52c41a' : '#faad14'
        const label = text ? t('crawlerPage.yes') : t('crawlerPage.no')
        return <span style={{ color }}>{label}</span>
      }
    },
    {
      title: t('crawlerPage.columns.renderTime'),
      dataIndex: 'renderTime',
      key: 'renderTime',
      render: (text: number) => {
        return `${(text * 1000).toFixed(2)}ms`
      }
    }
  ]

  // 处理图表数据，直接使用后端返回的数据
  const processChartData = () => {
    const { trafficByHour = [] } = stats;

    // 直接使用后端返回的数据，后端已经根据不同的粒度返回了相应格式的数据
    return {
      time: trafficByHour.map((item: any) => item.time),
      totalRequests: trafficByHour.map((item: any) => item.totalRequests),
      cacheHits: trafficByHour.map((item: any) => item.cacheHits),
      cacheMisses: trafficByHour.map((item: any) => item.cacheMisses)
    };
  };

  // 使用useMemo缓存图表配置，当stats变化时重新计算
  const chartOption = React.useMemo(() => {
    const chartData = processChartData();
    return {
      tooltip: {
        trigger: 'axis',
        axisPointer: {
          type: 'cross'
        }
      },
      legend: {
        data: [
          t('crawlerPage.chartLegend.requests'),
          t('crawlerPage.chartLegend.cacheHits'),
          t('crawlerPage.chartLegend.cacheMisses')
        ],
        bottom: 0
      },
      grid: {
        left: '3%',
        right: '4%',
        bottom: '15%',
        top: '3%',
        containLabel: true
      },
      xAxis: {
        type: 'category',
        boundaryGap: false,
        data: chartData.time,
        axisLabel: {
          rotate: 45
        }
      },
      yAxis: {
        type: 'value'
      },
      series: [
        {
          name: t('crawlerPage.chartLegend.requests'),
          type: 'line',
          data: chartData.totalRequests,
          smooth: true,
          lineStyle: {
            color: '#1890ff'
          }
        },
        {
          name: t('crawlerPage.chartLegend.cacheHits'),
          type: 'line',
          data: chartData.cacheHits,
          smooth: true,
          lineStyle: {
            color: '#52c41a'
          }
        },
        {
          name: t('crawlerPage.chartLegend.cacheMisses'),
          type: 'line',
          data: chartData.cacheMisses,
          smooth: true,
          lineStyle: {
            color: '#f5222d'
          }
        }
      ]
    } as EChartsOption;
  }, [stats, t]); // 当stats或语言变化时重新计算图表配置

  // 获取爬虫访问日志
  const fetchLogs = async () => {
    try {
      setLoading(true)
      // 使用默认时间范围：最近7天
      const startTime = dayjs().subtract(7, 'day').format('YYYY-MM-DDTHH:mm:ssZ')
      const endTime = dayjs().format('YYYY-MM-DDTHH:mm:ssZ')

      const res = await crawlerApi.getLogs({
        site: selectedSite === 'all' ? '' : selectedSite,
        startTime,
        endTime,
        page,
        pageSize
      })

      if (res.code === 200) {
        setLogs(res.data.items)
        setTotalLogs(res.data.total)
      }
    } catch (error) {
      console.error('Failed to fetch crawler logs:', error)
    } finally {
      setLoading(false)
    }
  }

  // 获取爬虫统计数据
  const fetchStats = async () => {
    const version = ++requestVersionRef.current
    try {
      // 使用默认时间范围：最近7天
      const startTime = dayjs().subtract(7, 'day').format('YYYY-MM-DDTHH:mm:ssZ')
      const endTime = dayjs().format('YYYY-MM-DDTHH:mm:ssZ')

      const res = await crawlerApi.getStats({
        site: selectedSite === 'all' ? '' : selectedSite,
        startTime,
        endTime,
        granularity
      })

      if (version !== requestVersionRef.current) return
      if (res.code === 200) {
        setStats(res.data)
      }
    } catch (error) {
      console.error('Failed to fetch crawler stats:', error)
    }
  }

  // 处理站点变化
  const handleSiteChange = (value: string) => {
    setSelectedSite(value)
    setPage(1)
  }

  // 处理粒度变化
  const handleGranularityChange = (e: any) => {
    setGranularity(e.target.value)
  }

  // 处理分页变化
  const handlePageChange = (page: number, pageSize: number) => {
    setPage(page)
    setPageSize(pageSize)
  }

  // 更新所有数据
  const updateData = async () => {
    await Promise.all([fetchLogs(), fetchStats()])
  }

  useEffect(() => {
    // 站点列表由 useSites 自动加载一次，此处仅随筛选条件刷新日志/统计
    updateData()
  }, [selectedSite, page, pageSize, granularity])

  return (
    <Spin spinning={loading} tip={t('common.loading')}>
      <div>
        <h1 className="page-title">{t('crawlerPage.title')}</h1>

        {/* 筛选条件 */}
        <Card className="card" style={{ marginBottom: 16 }}>
          <Row gutter={[16, 16]} align="middle">
            <Col span={8}>
              <div style={{ display: 'flex', alignItems: 'center' }}>
                <label style={{ marginRight: 8, width: 100, textAlign: 'right' }}>{t('crawlerPage.site')}</label>
                <Select
                  value={selectedSite}
                  onChange={handleSiteChange}
                  style={{ width: 200 }}
                >
                  <Option value="all">{t('crawlerPage.allSites')}</Option>
                  {sites.map((site) => (
                    <Option key={site.id} value={site.id}>
                      {site.name}
                    </Option>
                  ))}
                </Select>
              </div>
            </Col>
            <Col span={8}>
              <div style={{ display: 'flex', alignItems: 'center' }}>
                <label style={{ marginRight: 8, width: 100, textAlign: 'right' }}>{t('crawlerPage.granularity')}</label>
                <Radio.Group value={granularity} onChange={handleGranularityChange}>
                  <Radio.Button value="day">{t('crawlerPage.granularityDay')}</Radio.Button>
                  <Radio.Button value="week">{t('crawlerPage.granularityWeek')}</Radio.Button>
                  <Radio.Button value="month">{t('crawlerPage.granularityMonth')}</Radio.Button>
                </Radio.Group>
              </div>
            </Col>
          </Row>
        </Card>

        {/* 统计卡片 */}
        <Row gutter={[16, 16]}>
          <Col span={6}>
            <Card className="stat-card">
              <Statistic
                title={t('crawlerPage.totalRequests')}
                value={stats.totalRequests || 0}
                prefix={<ArrowUpOutlined />}
                valueStyle={{ color: '#3f8600' }}
                suffix={t('crawlerPage.countUnit')}
              />
            </Card>
          </Col>
          <Col span={6}>
            <Card className="stat-card">
              <Statistic
                title={t('crawlerPage.cacheHitRate')}
                value={stats.cacheHitRate || 0}
                prefix={<ArrowUpOutlined />}
                valueStyle={{ color: '#1890ff' }}
                suffix="%"
                precision={2}
              />
            </Card>
          </Col>
          <Col span={6}>
            <Card className="stat-card">
              <Statistic
                title={t('crawlerPage.avgRenderTime')}
                value={(stats.trafficByHour || []).length > 0 ?
                  ((stats.trafficByHour || []).reduce((sum: any, item: any) => sum + (item.renderTime || 0), 0) / (stats.trafficByHour || []).length * 1000).toFixed(2) : 0}
                valueStyle={{ color: '#faad14' }}
                suffix="ms"
                precision={2}
              />
            </Card>
          </Col>
          <Col span={6}>
            <Card className="stat-card">
              <Statistic
                title={t('crawlerPage.activeUAs')}
                value={(stats.topUAs || []).length}
                valueStyle={{ color: '#722ed1' }}
                suffix={t('crawlerPage.kindUnit')}
              />
            </Card>
          </Col>
        </Row>

        {/* 标签页 */}
        <Tabs defaultActiveKey="chart">
          {/* 图表标签页 */}
          <TabPane tab={t('crawlerPage.tabTrend')} key="chart">
            <Card className="card">
              <div style={{ height: 400 }}>
                <BaseChart option={chartOption} />
              </div>
            </Card>
          </TabPane>

          {/* 日志列表标签页 */}
          <TabPane tab={t('crawlerPage.tabLogs')} key="logs">
            <Card className="card">
              <Table
                columns={logColumns}
                dataSource={logs}
                rowKey="id"
                pagination={{
                  current: page,
                  pageSize: pageSize,
                  total: totalLogs,
                  onChange: handlePageChange,
                  showSizeChanger: true,
                  showTotal: (total) => t('crawlerPage.totalRecords', { total })
                }}
                size="middle"
              />
            </Card>
          </TabPane>
        </Tabs>
      </div>
    </Spin>
  )
}

export default Crawler
