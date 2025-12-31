import React, { useState, useEffect } from 'react'
import { Card, Row, Col, Statistic, Spin, Select, Table, DatePicker, Radio, Tabs } from 'antd'
import { ArrowUpOutlined, ArrowDownOutlined } from '@ant-design/icons'
import BaseChart from '../../components/charts/BaseChart'
import { crawlerApi, sitesApi } from '../../services/api'
import dayjs from 'dayjs'

const { Option } = Select
const { RangePicker } = DatePicker
const { TabPane } = Tabs

const Crawler: React.FC = () => {
  const [sites, setSites] = useState<any[]>([])
  const [selectedSite, setSelectedSite] = useState<string>('all')
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

  // 日志表格列配置
  const logColumns = [
    {
      title: '时间',
      dataIndex: 'time',
      key: 'time',
      render: (text: string) => {
        return dayjs(text).format('YYYY-MM-DD HH:mm:ss')
      }
    },
    {
      title: '站点',
      dataIndex: 'site',
      key: 'site'
    },
    {
      title: 'IP地址',
      dataIndex: 'ip',
      key: 'ip'
    },
    {
      title: '路由',
      dataIndex: 'route',
      key: 'route',
      ellipsis: true
    },
    {
      title: 'User-Agent',
      dataIndex: 'ua',
      key: 'ua',
      ellipsis: true,
      width: 300
    },
    {
      title: '状态',
      dataIndex: 'status',
      key: 'status',
      render: (text: number) => {
        const color = text === 200 ? '#52c41a' : '#ff4d4f'
        return <span style={{ color }}>{text}</span>
      }
    },
    {
      title: '缓存命中',
      dataIndex: 'hit_cache',
      key: 'hit_cache',
      render: (text: boolean) => {
        const color = text ? '#52c41a' : '#faad14'
        const label = text ? '是' : '否'
        return <span style={{ color }}>{label}</span>
      }
    },
    {
      title: '渲染时间',
      dataIndex: 'render_time',
      key: 'render_time',
      render: (text: number) => {
        return `${(text * 1000).toFixed(2)}ms`
      }
    }
  ]

  // 处理图表数据，直接使用后端返回的数据
  const processChartData = () => {
    const { trafficByHour } = stats;
    
    // 直接使用后端返回的数据，后端已经根据不同的粒度返回了相应格式的数据
    return {
      time: trafficByHour.map((item: any) => item.time),
      totalRequests: trafficByHour.map((item: any) => item.totalRequests),
      cacheHits: trafficByHour.map((item: any) => item.cacheHits),
      cacheMisses: trafficByHour.map((item: any) => item.cacheMisses)
    };
  };

  // 请求趋势图表配置
  const chartOption = {
    tooltip: {
      trigger: 'axis',
      axisPointer: {
        type: 'cross'
      }
    },
    legend: {
      data: ['爬虫请求数', '缓存命中数', '缓存未命中数'],
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
      data: processChartData().time,
      axisLabel: {
        rotate: 45
      }
    },
    yAxis: {
      type: 'value'
    },
    series: [
      {
        name: '爬虫请求数',
        type: 'line',
        data: processChartData().totalRequests,
        smooth: true,
        lineStyle: {
          color: '#1890ff'
        }
      },
      {
        name: '缓存命中数',
        type: 'line',
        data: processChartData().cacheHits,
        smooth: true,
        lineStyle: {
          color: '#52c41a'
        }
      },
      {
        name: '缓存未命中数',
        type: 'line',
        data: processChartData().cacheMisses,
        smooth: true,
        lineStyle: {
          color: '#f5222d'
        }
      }
    ]
  }

  // 获取站点列表
  const fetchSites = async () => {
    try {
      const res = await sitesApi.getSites()
      if (res.code === 200) {
        setSites(res.data)
      }
    } catch (error) {
      console.error('Failed to fetch sites:', error)
    }
  }

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
        setLogs(res.data.logs)
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
      
      if (res.code === 200) {
        console.log('Fetched stats:', res.data)
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
    fetchSites()
    updateData()
  }, [selectedSite, page, pageSize, granularity])

  return (
    <Spin spinning={loading} tip="加载中...">
      <div>
        <h1 className="page-title">爬虫访问</h1>
        
        {/* 筛选条件 */}
        <Card className="card" style={{ marginBottom: 16 }}>
          <Row gutter={[16, 16]} align="middle">
            <Col span={8}>
              <div style={{ display: 'flex', alignItems: 'center' }}>
                <label style={{ marginRight: 8, width: 100, textAlign: 'right' }}>站点：</label>
                <Select
                  value={selectedSite}
                  onChange={handleSiteChange}
                  style={{ width: 200 }}
                >
                  <Option value="all">所有站点</Option>
                  {sites.map((site) => (
                    <Option key={site.name} value={site.name}>
                      {site.name}
                    </Option>
                  ))}
                </Select>
              </div>
            </Col>
            <Col span={8}>
              <div style={{ display: 'flex', alignItems: 'center' }}>
                <label style={{ marginRight: 8, width: 100, textAlign: 'right' }}>时间粒度：</label>
                <Radio.Group value={granularity} onChange={handleGranularityChange}>
                  <Radio.Button value="day">日</Radio.Button>
                  <Radio.Button value="week">周</Radio.Button>
                  <Radio.Button value="month">月</Radio.Button>
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
                title="总请求数"
                value={stats.totalRequests}
                prefix={<ArrowUpOutlined />}
                valueStyle={{ color: '#3f8600' }}
                suffix="条"
              />
            </Card>
          </Col>
          <Col span={6}>
            <Card className="stat-card">
              <Statistic
                title="缓存命中率"
                value={stats.cacheHitRate}
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
                title="平均渲染时间"
                value={stats.trafficByHour.length > 0 ? 
                  (stats.trafficByHour.reduce((sum: any, item: any) => sum + item.renderTime, 0) / stats.trafficByHour.length * 1000).toFixed(2) : 0}
                valueStyle={{ color: '#faad14' }}
                suffix="ms"
                precision={2}
              />
            </Card>
          </Col>
          <Col span={6}>
            <Card className="stat-card">
              <Statistic
                title="活跃爬虫UA数"
                value={stats.topUAs.length}
                valueStyle={{ color: '#722ed1' }}
                suffix="种"
              />
            </Card>
          </Col>
        </Row>
        
        {/* 标签页 */}
        <Tabs defaultActiveKey="chart">
          {/* 图表标签页 */}
          <TabPane tab="访问趋势" key="chart">
            <Card className="card">
              <div style={{ height: 400 }}>
                <BaseChart option={chartOption} />
              </div>
            </Card>
          </TabPane>
          
          {/* 日志列表标签页 */}
          <TabPane tab="访问记录" key="logs">
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
                  showTotal: (total) => `共 ${total} 条记录`
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
