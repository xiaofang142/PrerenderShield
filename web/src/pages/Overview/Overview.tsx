import React, { useState, useEffect } from 'react'
import { Card, Row, Col, Statistic, Spin, Select, Table } from 'antd'
import { ArrowUpOutlined, ArrowDownOutlined } from '@ant-design/icons'
import BaseChart from '../../components/charts/BaseChart'
import { overviewApi, monitoringApi, sitesApi } from '../../services/api'

const { Option } = Select

const Overview: React.FC = () => {
  const [sites, setSites] = useState<any[]>([])
  const [selectedSite, setSelectedSite] = useState<string>('all')
  const [stats, setStats] = useState({
    totalRequests: 0,
    crawlerRequests: 0,
    blockedRequests: 0,
    cacheHitRate: 0,
    activeBrowsers: 0,
    sslCertificates: 0,
    activeSites: 0,
    firewallEnabled: false,
    prerenderEnabled: false,
  })
  const [systemStats, setSystemStats] = useState({
    cpuUsage: 0,
    memoryUsage: 0,
    diskUsage: 0,
    requestsPerSecond: 0,
  })
  const [loading, setLoading] = useState(true)
  
  // 站点统计表格列配置
  const siteStatsColumns = [
    {
      title: '站点名称',
      dataIndex: 'name',
      key: 'name',
    },
    {
      title: '域名',
      dataIndex: 'domain',
      key: 'domain',
      ellipsis: true,
    },
    {
      title: '状态',
      dataIndex: 'status',
      key: 'status',
      render: (text: string) => {
        const color = text === 'active' ? '#52c41a' : '#faad14'
        return <span style={{ color }}>{text}</span>
      },
    },
    {
      title: '今日请求数',
      dataIndex: 'requests',
      key: 'requests',
    },
    {
      title: '爬虫请求数',
      dataIndex: 'crawlerRequests',
      key: 'crawlerRequests',
    },
    {
      title: '拦截请求数',
      dataIndex: 'blockedRequests',
      key: 'blockedRequests',
    },
  ]
  
  // 格式化站点统计数据
  const formatSiteStats = () => {
    return sites.map(site => ({
      key: site.name,
      name: site.name,
      domain: site.domain,
      status: 'active',
      requests: Math.floor(Math.random() * 1000),
      crawlerRequests: Math.floor(Math.random() * 300),
      blockedRequests: Math.floor(Math.random() * 100),
    }))
  }

  // 模拟流量数据
  const trafficData = [
    { time: '00:00', totalRequests: 120, crawlerRequests: 30, blockedRequests: 10 },
    { time: '04:00', totalRequests: 80, crawlerRequests: 20, blockedRequests: 5 },
    { time: '08:00', totalRequests: 200, crawlerRequests: 60, blockedRequests: 15 },
    { time: '12:00', totalRequests: 350, crawlerRequests: 120, blockedRequests: 30 },
    { time: '16:00', totalRequests: 420, crawlerRequests: 150, blockedRequests: 45 },
    { time: '20:00', totalRequests: 280, crawlerRequests: 90, blockedRequests: 25 },
  ]

  const chartOption = {
    tooltip: {
      trigger: 'axis',
      axisPointer: {
        type: 'cross',
      },
    },
    legend: {
      data: ['总请求数', '爬虫请求数', '攻击拦截数'],
      bottom: 0,
    },
    grid: {
      left: '3%',
      right: '4%',
      bottom: '15%',
      top: '3%',
      containLabel: true,
    },
    xAxis: {
      type: 'category',
      boundaryGap: false,
      data: trafficData.map(item => item.time),
      axisLabel: {
        rotate: 45,
      },
    },
    yAxis: {
      type: 'value',
    },
    series: [
      {
        name: '总请求数',
        type: 'line',
        data: trafficData.map(item => item.totalRequests),
        smooth: true,
        lineStyle: {
          color: '#1890ff',
        },
      },
      {
        name: '爬虫请求数',
        type: 'line',
        data: trafficData.map(item => item.crawlerRequests),
        smooth: true,
        lineStyle: {
          color: '#52c41a',
        },
      },
      {
        name: '攻击拦截数',
        type: 'line',
        data: trafficData.map(item => item.blockedRequests),
        smooth: true,
        lineStyle: {
          color: '#f5222d',
        },
      },
    ],
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

  // 获取概览数据
  const fetchData = async () => {
    try {
      setLoading(true)
      const [overviewRes, monitoringRes] = await Promise.all([
        overviewApi.getStats(),
        monitoringApi.getStats(),
      ])
      
      if (overviewRes.code === 200) {
        setStats(overviewRes.data)
      }
      
      if (monitoringRes.code === 200) {
        setSystemStats(monitoringRes.data)
      }
    } catch (error) {
      console.error('Failed to fetch overview data:', error)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    fetchSites()
    fetchData()
    // 每30秒刷新一次数据
    const interval = setInterval(() => {
      fetchSites()
      fetchData()
    }, 30000)
    return () => clearInterval(interval)
  }, [])

  return (
    <Spin spinning={loading} tip="加载中...">
      <div>
        <h1 className="page-title">概览</h1>
        
        {/* 站点选择器 */}
        <Card className="card" style={{ marginBottom: 16 }}>
          <Row align="middle">
            <Col span={8}>
              <label style={{ marginRight: 8 }}>站点视图：</label>
              <Select
                value={selectedSite}
                onChange={setSelectedSite}
                style={{ width: 200 }}
              >
                <Option value="all">所有站点</Option>
                {sites.map((site) => (
                  <Option key={site.name} value={site.name}>
                    {site.name} ({site.domain})
                  </Option>
                ))}
              </Select>
            </Col>
          </Row>
        </Card>
        
        {/* Statistics Cards */}
        <Row gutter={[16, 16]}>
          <Col span={6}>
            <Card className="stat-card">
              <Statistic
                title="总请求数"
                value={stats.totalRequests}
                prefix={<ArrowUpOutlined />}
                valueStyle={{ color: '#3f8600' }}
                suffix="今日"
              />
            </Card>
          </Col>
          <Col span={6}>
            <Card className="stat-card">
              <Statistic
                title="爬虫请求数"
                value={stats.crawlerRequests}
                prefix={<ArrowUpOutlined />}
                valueStyle={{ color: '#3f8600' }}
                suffix="今日"
              />
            </Card>
          </Col>
          <Col span={6}>
            <Card className="stat-card">
              <Statistic
                title="攻击拦截数"
                value={stats.blockedRequests}
                prefix={<ArrowDownOutlined />}
                valueStyle={{ color: '#cf1322' }}
                suffix="今日"
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
              />
            </Card>
          </Col>
          <Col span={6}>
            <Card className="stat-card">
              <Statistic
                title="活跃浏览器数"
                value={stats.activeBrowsers}
                valueStyle={{ color: '#faad14' }}
              />
            </Card>
          </Col>
          <Col span={6}>
            <Card className="stat-card">
              <Statistic
                title="活跃站点数"
                value={stats.activeSites}
                valueStyle={{ color: '#52c41a' }}
              />
            </Card>
          </Col>
          <Col span={6}>
            <Card className="stat-card">
              <Statistic
                title="SSL证书数"
                value={stats.sslCertificates}
                valueStyle={{ color: '#1890ff' }}
              />
            </Card>
          </Col>
        </Row>
        
        {/* Traffic Trend Chart */}
        <Card className="card">
          <h3 style={{ marginBottom: 16 }}>请求趋势</h3>
          <div style={{ height: 300 }}>
            <BaseChart option={chartOption} />
          </div>
        </Card>
        
        {/* 站点统计表格 */}
        <Card className="card" style={{ marginTop: 16 }}>
          <h3 style={{ marginBottom: 16 }}>站点统计</h3>
          <Table
            columns={siteStatsColumns}
            dataSource={formatSiteStats()}
            rowKey="key"
            pagination={{ pageSize: 5 }}
            size="middle"
          />
        </Card>
        
        {/* System Status */}
        <Row gutter={[16, 16]}>
          <Col span={12}>
            <Card className="card">
              <h3 style={{ marginBottom: 16 }}>系统状态</h3>
              <Row gutter={[16, 16]}>
                <Col span={12}>
                  <Statistic title="CPU使用率" value={systemStats.cpuUsage} suffix="%" />
                </Col>
                <Col span={12}>
                  <Statistic title="内存使用率" value={systemStats.memoryUsage} suffix="%" />
                </Col>
                <Col span={12}>
                  <Statistic title="磁盘使用率" value={systemStats.diskUsage} suffix="%" />
                </Col>
                <Col span={12}>
                  <Statistic title="请求/秒" value={systemStats.requestsPerSecond} />
                </Col>
              </Row>
            </Card>
          </Col>
          <Col span={12}>
            <Card className="card">
              <h3 style={{ marginBottom: 16 }}>服务状态</h3>
              <Row gutter={[16, 16]}>
                <Col span={12}>
                  <Statistic title="API服务" value="运行中" valueStyle={{ color: '#52c41a' }} />
                </Col>
                <Col span={12}>
                  <Statistic 
                    title="防火墙" 
                    value={stats.firewallEnabled ? "已启用" : "已禁用"} 
                    valueStyle={{ color: stats.firewallEnabled ? '#52c41a' : '#faad14' }} 
                  />
                </Col>
                <Col span={12}>
                  <Statistic 
                    title="预渲染" 
                    value={stats.prerenderEnabled ? "已启用" : "已禁用"} 
                    valueStyle={{ color: stats.prerenderEnabled ? '#52c41a' : '#faad14' }} 
                  />
                </Col>
                <Col span={12}>
                  <Statistic title="监控服务" value="运行中" valueStyle={{ color: '#52c41a' }} />
                </Col>
              </Row>
            </Card>
          </Col>
        </Row>
      </div>
    </Spin>
  )
}

export default Overview