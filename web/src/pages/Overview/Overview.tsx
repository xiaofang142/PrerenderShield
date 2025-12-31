import React, { useState, useEffect } from 'react'
import { Card, Row, Col, Statistic, Spin, Select, Table, Tabs, Tag } from 'antd'
import { ArrowUpOutlined, ArrowDownOutlined } from '@ant-design/icons'
import BaseChart from '../../components/charts/BaseChart'
import { overviewApi, monitoringApi, sitesApi } from '../../services/api'

const { Option } = Select

const Overview: React.FC = () => {
  const [sites, setSites] = useState<any[]>([])
  const [selectedSite, setSelectedSite] = useState<string>('all')
  const [mapType, setMapType] = useState<string>('bar') // 地图类型：2d, 3d, bar，默认使用柱状图避免地图数据缺失问题
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
    geoData: {
      countryData: [],
      mapData: []
    },
    trafficData: [],
    accessStats: {
      pv: 0,
      uv: 0,
      ip: 0
    }
  })
  const [systemStats, setSystemStats] = useState({
    cpuUsage: 0,
    memoryUsage: 0,
    diskUsage: 0,
    requestsPerSecond: 0,
  })
  const [accessStats, setAccessStats] = useState({
    pv: 74100,
    uv: 192,
    ip: 294,
    countryData: [
      { country: '中国', count: 891800 },
      { country: '美国', count: 2300 },
      { country: '爱尔兰', count: 461 },
      { country: '澳大利亚', count: 361 },
      { country: '新加坡', count: 221 },
      { country: '印度', count: 157 },
      { country: '日本', count: 133 },
    ],
    mapData: [
      { name: '中国', value: 891800 },
      { name: '美国', value: 2300 },
      { name: '爱尔兰', value: 461 },
      { name: '澳大利亚', value: 361 },
      { name: '新加坡', value: 221 },
      { name: '印度', value: 157 },
      { name: '日本', value: 133 },
    ]
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

  // 请求趋势图表配置
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
      data: (stats.trafficData || []).map((item: any) => item.time),
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
        data: (stats.trafficData || []).map((item: any) => item.totalRequests),
        smooth: true,
        lineStyle: {
          color: '#1890ff',
        },
      },
      { 
        name: '爬虫请求数',
        type: 'line',
        data: (stats.trafficData || []).map((item: any) => item.crawlerRequests),
        smooth: true,
        lineStyle: {
          color: '#52c41a',
        },
      },
      { 
        name: '攻击拦截数',
        type: 'line',
        data: (stats.trafficData || []).map((item: any) => item.blockedRequests),
        smooth: true,
        lineStyle: {
          color: '#f5222d',
        },
      },
    ],
  }

  // 2D地图配置
  const map2DOption = {
    tooltip: {
      trigger: 'item',
      formatter: '{b}: {c} ({d}%)'
    },
    visualMap: {
      type: 'continuous',
      left: 'left',
      bottom: '3%',
      min: 0,
      max: 1000000,
      text: ['高', '低'],
      calculable: true,
      color: ['#ff0000', '#ffa500', '#ffff00', '#90ee90', '#00ff00']
    },
    geo: {
      map: 'world',
      roam: true,
      emphasis: {
        label: {
          show: true
        }
      },
      itemStyle: {
        areaColor: '#f0f0f0',
        borderColor: '#999'
      }
    },
    series: [
      {
        name: '访问数量',
        type: 'scatter',
        coordinateSystem: 'geo',
        data: accessStats.mapData.map(item => ({
          name: item.name,
          value: [item.name, item.value]
        })),
        symbolSize: function(val: any) {
          return Math.sqrt(val[1]) / 100;
        },
        label: {
          formatter: '{b}',
          position: 'right',
          show: false
        },
        emphasis: {
          label: {
            show: true
          }
        }
      }
    ]
  }

  // 3D地图配置
  const map3DOption = {
    tooltip: {
      trigger: 'item',
      formatter: '{b}: {c} ({d}%)'
    },
    visualMap: {
      type: 'continuous',
      left: 'left',
      bottom: '3%',
      min: 0,
      max: 1000000,
      text: ['高', '低'],
      calculable: true,
      color: ['#ff0000', '#ffa500', '#ffff00', '#90ee90', '#00ff00']
    },
    geo3D: {
      map: 'world',
      roam: true,
      shading: 'lambert',
      light: {
        main: {
          intensity: 1.2,
          shadow: true
        },
        ambient: {
          intensity: 0.6
        }
      },
      itemStyle: {
        areaColor: '#f0f0f0',
        borderColor: '#999'
      }
    },
    series: [
      {
        name: '访问数量',
        type: 'bar3D',
        coordinateSystem: 'geo3D',
        data: accessStats.mapData.map(item => ({
          name: item.name,
          value: [item.name, 0, item.value]
        })),
        shading: 'lambert',
        label: {
          show: false
        },
        emphasis: {
          label: {
            show: true
          }
        }
      }
    ]
  }

  // 柱状图配置（作为备选）
  const barOption = {
    tooltip: {
      trigger: 'axis',
      axisPointer: {
        type: 'shadow'
      },
      formatter: '{b}<br/>30天内访问数量: {c}',
    },
    xAxis: {
      type: 'category',
      data: accessStats.countryData.map(item => item.country),
      axisLabel: {
        rotate: 45,
        interval: 0
      }
    },
    yAxis: {
      type: 'value',
      name: '访问数量'
    },
    series: [
      {
        name: '访问数量',
        type: 'bar',
        data: accessStats.countryData.map(item => item.count),
        itemStyle: {
          color: {
            type: 'linear',
            x: 0,
            y: 0,
            x2: 0,
            y2: 1,
            colorStops: [
              {
                offset: 0,
                color: '#1890ff'
              },
              {
                offset: 1,
                color: '#69c0ff'
              }
            ]
          }
        },
        emphasis: {
          itemStyle: {
            color: '#40a9ff'
          }
        }
      }
    ]
  }

  // PV趋势图表配置
  const pvTrendOption = {
    xAxis: {
      type: 'category',
      data: ['00:00', '04:00', '08:00', '12:00', '16:00', '20:00'],
      axisLabel: {
        rotate: 45,
      },
    },
    yAxis: {
      type: 'value',
      show: false,
    },
    grid: {
      left: '0%',
      right: '0%',
      top: '0%',
      bottom: '0%',
      containLabel: false,
    },
    series: [
      {
        data: [12000, 15000, 30000, 25000, 35000, 28000],
        type: 'line',
        smooth: true,
        lineStyle: {
          color: '#1890ff',
          width: 2,
        },
        areaStyle: {
          color: {
            type: 'linear',
            x: 0,
            y: 0,
            x2: 0,
            y2: 1,
            colorStops: [
              {
                offset: 0,
                color: 'rgba(24, 144, 255, 0.3)',
              },
              {
                offset: 1,
                color: 'rgba(24, 144, 255, 0.05)',
              },
            ],
          },
        },
        symbol: 'none',
      },
    ],
  }

  // UV趋势图表配置
  const uvTrendOption = {
    xAxis: {
      type: 'category',
      data: ['00:00', '04:00', '08:00', '12:00', '16:00', '20:00'],
      axisLabel: {
        rotate: 45,
      },
    },
    yAxis: {
      type: 'value',
      show: false,
    },
    grid: {
      left: '0%',
      right: '0%',
      top: '0%',
      bottom: '0%',
      containLabel: false,
    },
    series: [
      {
        data: [20, 30, 60, 80, 120, 90],
        type: 'line',
        smooth: true,
        lineStyle: {
          color: '#52c41a',
          width: 2,
        },
        areaStyle: {
          color: {
            type: 'linear',
            x: 0,
            y: 0,
            x2: 0,
            y2: 1,
            colorStops: [
              {
                offset: 0,
                color: 'rgba(82, 196, 26, 0.3)',
              },
              {
                offset: 1,
                color: 'rgba(82, 196, 26, 0.05)',
              },
            ],
          },
        },
        symbol: 'none',
      },
    ],
  }

  // IP趋势图表配置
  const ipTrendOption = {
    xAxis: {
      type: 'category',
      data: ['00:00', '04:00', '08:00', '12:00', '16:00', '20:00'],
      axisLabel: {
        rotate: 45,
      },
    },
    yAxis: {
      type: 'value',
      show: false,
    },
    grid: {
      left: '0%',
      right: '0%',
      top: '0%',
      bottom: '0%',
      containLabel: false,
    },
    series: [
      {
        data: [30, 50, 80, 120, 180, 150],
        type: 'line',
        smooth: true,
        lineStyle: {
          color: '#faad14',
          width: 2,
        },
        areaStyle: {
          color: {
            type: 'linear',
            x: 0,
            y: 0,
            x2: 0,
            y2: 1,
            colorStops: [
              {
                offset: 0,
                color: 'rgba(250, 173, 20, 0.3)',
              },
              {
                offset: 1,
                color: 'rgba(250, 173, 20, 0.05)',
              },
            ],
          },
        },
        symbol: 'none',
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
        
        {/* 访问统计 - 首屏第一个元素 */}
        <Card className="card" style={{ marginTop: 16 }}>
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16 }}>
            <h3 style={{ margin: 0 }}>网站访问量地域分布</h3>
            <div>
              <span style={{ marginRight: 8 }}>地图类型：</span>
              <Select
                value={mapType}
                onChange={setMapType}
                style={{ width: 120 }}
                size="small"
              >
                <Option value="2d">2D地图</Option>
                <Option value="3d">3D地图</Option>
                <Option value="bar">柱状图</Option>
              </Select>
            </div>
          </div>
          <Tabs 
            defaultActiveKey="map"
            items={[
              {
                key: 'map',
                label: '访问分布',
                children: (
                  <Row gutter={[16, 16]}>
                    <Col span={16}>
                      <div style={{ height: 400 }}>
                        <BaseChart option={mapType === '2d' ? map2DOption : mapType === '3d' ? map3DOption : barOption} />
                      </div>
                    </Col>
                    <Col span={8}>
                      <Card title="国家访问排行" variant="outlined">
                        <div style={{ maxHeight: 400, overflowY: 'auto' }}>
                          {(stats.geoData?.countryData || accessStats.countryData).map((item: any, index: number) => (
                            <div key={index} style={{ 
                              display: 'flex', 
                              justifyContent: 'space-between', 
                              alignItems: 'center',
                              padding: '8px 0',
                              borderBottom: '1px solid #f0f0f0'
                            }}>
                              <span style={{ display: 'flex', alignItems: 'center' }}>
                                <Tag color="blue" style={{ marginRight: 8 }}>{index + 1}</Tag>
                                {item.country}
                              </span>
                              <span style={{ fontWeight: 'bold' }}>{item.count}</span>
                            </div>
                          ))}
                        </div>
                      </Card>
                    </Col>
                  </Row>
                )
              },
              {
                key: 'trend',
                label: '访问趋势',
                children: (
                  <Row gutter={[16, 16]}>
                    <Col span={8}>
                      <Card variant="outlined">
                        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16 }}>
                          <h4 style={{ margin: 0 }}>PV (访问量)</h4>
                          <Statistic value={stats.accessStats?.pv || accessStats.pv} valueStyle={{ color: '#1890ff' }} />
                        </div>
                        <div style={{ height: 150 }}>
                          <BaseChart option={pvTrendOption} />
                        </div>
                      </Card>
                    </Col>
                    <Col span={8}>
                      <Card variant="outlined">
                        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16 }}>
                          <h4 style={{ margin: 0 }}>UV (独立访客)</h4>
                          <Statistic value={stats.accessStats?.uv || accessStats.uv} valueStyle={{ color: '#52c41a' }} />
                        </div>
                        <div style={{ height: 150 }}>
                          <BaseChart option={uvTrendOption} />
                        </div>
                      </Card>
                    </Col>
                    <Col span={8}>
                      <Card variant="outlined">
                        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16 }}>
                          <h4 style={{ margin: 0 }}>IP (独立IP)</h4>
                          <Statistic value={stats.accessStats?.ip || accessStats.ip} valueStyle={{ color: '#faad14' }} />
                        </div>
                        <div style={{ height: 150 }}>
                          <BaseChart option={ipTrendOption} />
                        </div>
                      </Card>
                    </Col>
                  </Row>
                )
              }
            ]}
          />
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
                precision={2}
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
                  <Statistic title="CPU使用率" value={systemStats.cpuUsage} suffix="%" precision={2} />
                </Col>
                <Col span={12}>
                  <Statistic title="内存使用率" value={systemStats.memoryUsage} suffix="%" precision={2} />
                </Col>
                <Col span={12}>
                  <Statistic title="磁盘使用率" value={systemStats.diskUsage} suffix="%" precision={2} />
                </Col>
                <Col span={12}>
                  <Statistic title="请求/秒" value={systemStats.requestsPerSecond} precision={2} />
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