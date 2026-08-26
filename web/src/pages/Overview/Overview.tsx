import React, { useState, useEffect } from 'react'
import { Card, Row, Col, Statistic, Spin, Select, Tag, Progress } from 'antd'
import echarts from '../../components/charts/echarts'
import type { EChartsOption } from 'echarts'
import BaseChart from '../../components/charts/BaseChart'
import { overviewApi, monitoringApi, firewallApi } from '../../services/api'
import { usePolling } from '@prerender/utils'
import { pollingIntervals } from '@prerender/design-tokens'
import { useTranslation } from 'react-i18next'

const { Option } = Select

interface GeoDataItem {
  name: string;
  value: number;
}

interface CountryDataItem {
  country: string;
  count: number;
}

interface OverviewStats {
  totalRequests: number;
  crawlerRequests: number;
  blockedRequests: number;
  cacheHitRate: number;
  activeBrowsers: number;
  sslCertificates: number;
  activeSites: number;
  firewallEnabled: boolean;
  prerenderEnabled: boolean;
  geoData: {
    countryData: CountryDataItem[];
    mapData: GeoDataItem[];
    globeData: any[];
  };
  trafficData: any[];
  accessStats: {
    pv: number;
    uv: number;
    ip: number;
  };
}

interface SecurityEvent {
  type: string;
  count: number;
  timestamp?: string;
}

interface RenderStats {
  successRate: number;
  avgRenderTime: number;
  cacheHitRate: number;
  renderTimeDistribution?: { range: string; count: number }[];
}

const Overview: React.FC = () => {
  const { t } = useTranslation()
  const [mapType, setMapType] = useState<string>('2d') // 地图类型：2d, 3d, bar，默认使用2D地图
  const [stats, setStats] = useState<OverviewStats>({
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
      mapData: [],
      globeData: []
    },
    trafficData: [],
    accessStats: {
      pv: 0,
      uv: 0,
      ip: 0
    }
  })
  
  const accessStats = { pv: 0, uv: 0, ip: 0, countryData: [], mapData: [] }
  const [loading, setLoading] = useState(true)
  const [systemHealth, setSystemHealth] = useState({ cpu: 0, memory: 0, disk: 0 })

  const [isMapLoaded, setIsMapLoaded] = useState(false)
  const [securityEvents, setSecurityEvents] = useState<SecurityEvent[]>([])
  const [renderStats, setRenderStats] = useState<RenderStats>({
    successRate: 0,
    avgRenderTime: 0,
    cacheHitRate: 0,
    renderTimeDistribution: []
  })



  // 如果地图未加载完成，暂时不渲染地图相关的 Option
  // 或者修改 series type 为 'map'，这样不需要经纬度数据，直接用 name 匹配
  const mapSeriesOption: EChartsOption = {
    tooltip: {
        trigger: 'item',
        formatter: (params: any) => {
          const value = params.value || 0;
          return `${params.name}: ${isNaN(value) ? 0 : value}`;
        }
    },
    visualMap: {
        min: 0,
        max: 10000,
        text: [t('overview.high'), t('overview.low')],
        realtime: false,
        calculable: true,
        inRange: {
            color: ['#lightskyblue', 'yellow', 'orangered']
        }
    },
    series: [
        {
            name: t('overview.mapSeries'),
            type: 'map',
            map: 'world', // 必须与 registerMap 的名字一致
            roam: true,
            emphasis: {
                label: {
                    show: true
                }
            },
            data: (stats.geoData?.mapData || accessStats.mapData).length > 0 
              ? (stats.geoData?.mapData || accessStats.mapData).map(item => ({
                  name: item.name,
                  value: isNaN(item.value) ? 0 : item.value
              }))
              : [] // 确保为空时是空数组
        }
    ]
  };

  // 柱状图配置（作为备选）
  const barOption: EChartsOption = {
    tooltip: {
      trigger: 'axis',
      axisPointer: {
        type: 'shadow'
      },
      formatter: t('overview.barTooltip'),
    },
    xAxis: {
      type: 'category',
      data: (stats.geoData?.countryData || accessStats.countryData).length > 0
        ? (stats.geoData?.countryData || accessStats.countryData).map(item => item.country)
        : [t('overview.noData')], // 空数据占位
      axisLabel: {
        rotate: 45,
        interval: 0
      }
    },
    yAxis: {
      type: 'value',
      name: t('overview.visitCount')
    },
    series: [
      {
        name: t('overview.visitCount'),
        type: 'bar',
        data: (stats.geoData?.countryData || accessStats.countryData).length > 0
          ? (stats.geoData?.countryData || accessStats.countryData).map(item => item.count)
          : [0], // 空数据占位
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

  // 安全事件类型分布饼图
  const securityPieOption: EChartsOption = {
    tooltip: {
      trigger: 'item',
      formatter: '{b}: {c} ({d}%)'
    },
    legend: {
      orient: 'vertical',
      left: 'left',
      top: 'center'
    },
    series: [
      {
        name: t('overview.attackTypeDistribution'),
        type: 'pie',
        radius: ['40%', '70%'],
        center: ['55%', '50%'],
        avoidLabelOverlap: true,
        itemStyle: {
          borderRadius: 6,
          borderColor: '#fff',
          borderWidth: 2
        },
        label: {
          show: true,
          formatter: '{b}\n{d}%'
        },
        emphasis: {
          label: {
            show: true,
            fontSize: 14,
            fontWeight: 'bold'
          }
        },
        data: securityEvents.length > 0 
          ? securityEvents.reduce((acc: any[], event) => {
              const existing = acc.find(a => a.name === event.type)
              if (existing) {
                existing.value += event.count
              } else {
                acc.push({ name: event.type, value: event.count })
              }
              return acc
            }, [])
          : [{ name: t('overview.noData'), value: 0 }]
      }
    ]
  }

  // 渲染性能图表 - 渲染时间分布
  const renderPerformanceOption: EChartsOption = {
    tooltip: {
      trigger: 'axis',
      axisPointer: {
        type: 'shadow'
      }
    },
    xAxis: {
      type: 'category',
      data: (renderStats.renderTimeDistribution?.length ?? 0) > 0 
        ? renderStats.renderTimeDistribution!.map(d => d.range)
        : ['<100ms', '100-500ms', '500ms-1s', '1-2s', '>2s'],
      axisLabel: {
        rotate: 0
      }
    },
    yAxis: {
      type: 'value',
      name: t('overview.requestCount')
    },
    series: [
      {
        name: t('overview.requestCount'),
        type: 'bar',
        data: (renderStats.renderTimeDistribution?.length ?? 0) > 0 
          ? renderStats.renderTimeDistribution!.map(d => d.count)
          : [0, 0, 0, 0, 0],
        itemStyle: {
          color: function(params: any) {
            const colors = ['#52c41a', '#73d13d', '#95de64', '#faad14', '#ff4d4f']
            return colors[params.dataIndex] || '#1890ff'
          }
        },
        barWidth: '60%'
      }
    ]
  }

  // 缓存命中率趋势图
  const cacheHitTrendOption: EChartsOption = {
    tooltip: {
      trigger: 'axis',
      formatter: (params: any) => {
        const data = params[0]
        return t('overview.cacheHitTooltip', { time: data.axisValue, value: data.value })
      }
    },
    xAxis: {
      type: 'category',
      data: (stats.trafficData || []).map((d: any) => d.time || '').filter((v: string) => v),
      boundaryGap: false
    },
    yAxis: {
      type: 'value',
      name: t('overview.hitRatePercent'),
      min: 0,
      max: 100,
      axisLabel: {
        formatter: '{value}%'
      }
    },
    series: [
      {
        name: t('overview.cacheHitRate'),
        type: 'line',
        data: (stats.trafficData || []).map((d: any) => d.cacheHitRate || renderStats.cacheHitRate || 0),
        smooth: true,
        areaStyle: {
          color: {
            type: 'linear',
            x: 0,
            y: 0,
            x2: 0,
            y2: 1,
            colorStops: [
              { offset: 0, color: 'rgba(82, 196, 26, 0.4)' },
              { offset: 1, color: 'rgba(82, 196, 26, 0.05)' }
            ]
          }
        },
        lineStyle: {
          color: '#52c41a',
          width: 2
        },
        itemStyle: {
          color: '#52c41a'
        }
      }
    ]
  }

  // 获取概览数据
  const fetchData = async () => {
    try {
      setLoading(true)
      const [overviewRes, monitoringRes, securityRes] = await Promise.all([
        overviewApi.getStats(),
        monitoringApi.getStats(),
        firewallApi.getAttackLogs({ site_id: '', page: 1, limit: 100 }).catch(() => ({ code: 200, data: { list: [] } }))
      ])
      
      if (overviewRes.code === 200) {
        setStats(overviewRes.data)
      }
      if (monitoringRes.code === 200 && monitoringRes.data) {
        setSystemHealth({
          cpu: monitoringRes.data.cpuUsage || 0,
          memory: monitoringRes.data.memoryUsage || 0,
          disk: monitoringRes.data.diskUsage || 0,
        })
        setRenderStats({
          successRate: monitoringRes.data.renderSuccessRate || 0,
          avgRenderTime: monitoringRes.data.avgRenderTime || 0,
          cacheHitRate: monitoringRes.data.cacheHitRate || 0,
          renderTimeDistribution: monitoringRes.data.renderTimeDistribution || []
        })
      }
      if (securityRes.code === 200 && securityRes.data?.list) {
        const events: SecurityEvent[] = securityRes.data.list.map((item: any) => ({
          type: item.type || item.attack_type || 'unknown',
          count: item.count || 1,
          timestamp: item.timestamp || item.created_at
        }))
        setSecurityEvents(events)
      }
    } catch (error) {
      console.error('Failed to fetch overview data:', error)
    } finally {
      setLoading(false)
    }
  }

  // 注册地图（一次性）：本地优先，失败回退 CDN
  useEffect(() => {
    fetch('/maps/world.json')
      .then(response => response.json())
      .then(mapJson => {
        echarts.registerMap('world', mapJson)
        setIsMapLoaded(true)
      })
      .catch(e => {
        console.error('Failed to load world map, trying fallback CDN', e)
        fetch('https://cdn.jsdelivr.net/npm/echarts@4.9.0/map/json/world.json')
          .then(response => response.json())
          .then(mapJson => {
            echarts.registerMap('world', mapJson)
            setIsMapLoaded(true)
          })
          .catch(e2 => console.error('Failed to load world map from CDN', e2))
      })
  }, [])

  // 数据轮询：卸载自动清理，页面不可见时暂停
  usePolling(fetchData, { interval: pollingIntervals.dashboard })

  return (
    <Spin spinning={loading} tip={t('common.loading')}>
      <div>
        <h1 className="page-title">{t('menu.overview')}</h1>
        
        {/* 系统健康状态 */}
        <Card className="card" style={{ marginTop: 16 }}>
          <h3 style={{ marginBottom: 16 }}>{t('overview.systemHealth')}</h3>
          <Row gutter={[16, 16]}>
            <Col span={8}>
              <Card variant="outlined" bodyStyle={{ padding: '16px' }}>
                <Statistic title={t('overview.cpuUsage')} value={systemHealth.cpu} suffix="%" precision={1}
                  valueStyle={{ color: systemHealth.cpu > 90 ? '#ff4d4f' : systemHealth.cpu > 70 ? '#faad14' : '#52c41a' }} />
                <Progress percent={systemHealth.cpu} showInfo={false} strokeColor={
                  systemHealth.cpu > 90 ? '#ff4d4f' : systemHealth.cpu > 70 ? '#faad14' : '#52c41a'
                } style={{ marginTop: 8 }} />
              </Card>
            </Col>
            <Col span={8}>
              <Card variant="outlined" bodyStyle={{ padding: '16px' }}>
                <Statistic title={t('overview.memoryUsage')} value={systemHealth.memory} suffix="%" precision={1}
                  valueStyle={{ color: systemHealth.memory > 85 ? '#ff4d4f' : systemHealth.memory > 70 ? '#faad14' : '#52c41a' }} />
                <Progress percent={systemHealth.memory} showInfo={false} strokeColor={
                  systemHealth.memory > 85 ? '#ff4d4f' : systemHealth.memory > 70 ? '#faad14' : '#52c41a'
                } style={{ marginTop: 8 }} />
              </Card>
            </Col>
            <Col span={8}>
              <Card variant="outlined" bodyStyle={{ padding: '16px' }}>
                <Statistic title={t('overview.diskUsage')} value={systemHealth.disk} suffix="%" precision={1}
                  valueStyle={{ color: systemHealth.disk > 90 ? '#ff4d4f' : systemHealth.disk > 80 ? '#faad14' : '#52c41a' }} />
                <Progress percent={systemHealth.disk} showInfo={false} strokeColor={
                  systemHealth.disk > 90 ? '#ff4d4f' : systemHealth.disk > 80 ? '#faad14' : '#52c41a'
                } style={{ marginTop: 8 }} />
              </Card>
            </Col>
          </Row>
        </Card>
        
        {/* 全球访问分布 */}
        <Card className="card" style={{ marginTop: 16 }}>
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16 }}>
            <h3 style={{ margin: 0 }}>{t('overview.geoDistribution')}</h3>
            <div>
              <span style={{ marginRight: 8 }}>{t('common.view')}：</span>
              <Select
                value={mapType}
                onChange={setMapType}
                style={{ width: 120 }}
                size="small"
              >
                <Option value="2d">{t('overview.map2d')}</Option>
                <Option value="bar">{t('overview.mapBar')}</Option>
              </Select>
            </div>
          </div>

          {/* 统计数据行 */}
          <Row gutter={[16, 16]} style={{ marginBottom: 24 }}>
            <Col span={4}>
              <Card variant="outlined" bodyStyle={{ padding: '12px' }}>
                <Statistic title="PV" value={stats.accessStats?.pv || accessStats.pv} valueStyle={{ color: '#1890ff', fontSize: '18px' }} />
              </Card>
            </Col>
            <Col span={4}>
              <Card variant="outlined" bodyStyle={{ padding: '12px' }}>
                <Statistic title="UV" value={stats.accessStats?.uv || accessStats.uv} valueStyle={{ color: '#52c41a', fontSize: '18px' }} />
              </Card>
            </Col>
            <Col span={4}>
              <Card variant="outlined" bodyStyle={{ padding: '12px' }}>
                <Statistic title="IP" value={stats.accessStats?.ip || accessStats.ip} valueStyle={{ color: '#faad14', fontSize: '18px' }} />
              </Card>
            </Col>
            <Col span={4}>
              <Card variant="outlined" bodyStyle={{ padding: '12px' }}>
                <Statistic title={t('overview.totalRequests')} value={stats.totalRequests} valueStyle={{ color: '#1890ff', fontSize: '18px' }} />
              </Card>
            </Col>
            <Col span={4}>
              <Card variant="outlined" bodyStyle={{ padding: '12px' }}>
                <Statistic title={t('overview.crawlerRequests')} value={stats.crawlerRequests} valueStyle={{ color: '#52c41a', fontSize: '18px' }} />
              </Card>
            </Col>
            <Col span={4}>
              <Card variant="outlined" bodyStyle={{ padding: '12px' }}>
                <Statistic title={t('overview.blockedRequests')} value={stats.blockedRequests} valueStyle={{ color: '#ff4d4f', fontSize: '18px' }} />
              </Card>
            </Col>
          </Row>

          <Row gutter={[16, 16]}>
            <Col span={16}>
              <div style={{ height: 400 }}>
                {isMapLoaded || mapType === 'bar' ? (
                  <BaseChart option={mapType === '2d' ? mapSeriesOption : barOption} />
                ) : (
                  <div style={{ height: '100%', display: 'flex', justifyContent: 'center', alignItems: 'center' }}>
                    <Spin tip={t('common.loading')} />
                  </div>
                )}
              </div>
            </Col>
            <Col span={8}>
              <Card title={t('overview.countryRank')} variant="outlined">
                <div style={{ maxHeight: 400, overflowY: 'auto' }}>
                  {(stats.geoData?.countryData || accessStats.countryData).length > 0 ? (
                    (stats.geoData?.countryData || accessStats.countryData).map((item: any, index: number) => (
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
                    ))
                  ) : (
                    <div style={{ textAlign: 'center', padding: '20px', color: '#999' }}>
                      {t('common.loading')}
                    </div>
                  )}
                </div>
              </Card>
            </Col>
          </Row>
        </Card>

        {/* 流量趋势图 */}
        <Card className="card" style={{ marginTop: 16 }}>
          <h3 style={{ marginBottom: 16 }}>{t('overview.trafficTrend24h')}</h3>
          <BaseChart option={{
            tooltip: { trigger: 'axis' },
            legend: { data: [t('overview.totalRequestsShort'), t('overview.crawlerRequests'), t('overview.blockedRequests')] },
            xAxis: { type: 'category', data: (stats.trafficData || []).map((d: any) => d.time || '') },
            yAxis: { type: 'value' },
            series: [
              { name: t('overview.totalRequestsShort'), type: 'line', data: (stats.trafficData || []).map((d: any) => d.totalRequests || 0), smooth: true, itemStyle: { color: '#1890ff' } },
              { name: t('overview.crawlerRequests'), type: 'line', data: (stats.trafficData || []).map((d: any) => d.crawlerRequests || 0), smooth: true, itemStyle: { color: '#52c41a' } },
              { name: t('overview.blockedRequests'), type: 'line', data: (stats.trafficData || []).map((d: any) => d.blockedRequests || 0), smooth: true, itemStyle: { color: '#ff4d4f' } },
            ]
          }} style={{ height: 300 }} />
        </Card>

        {/* 渲染成功率和缓存命中率指标 */}
        <Card className="card" style={{ marginTop: 16 }}>
          <h3 style={{ marginBottom: 16 }}>{t('overview.renderCacheMetrics')}</h3>
          <Row gutter={[16, 16]}>
            <Col span={6}>
              <Card variant="outlined" bodyStyle={{ padding: '20px', textAlign: 'center' }}>
                <Statistic 
                  title={t('overview.renderSuccessRate')} 
                  value={renderStats.successRate} 
                  suffix="%" 
                  precision={1}
                  valueStyle={{ color: renderStats.successRate >= 95 ? '#52c41a' : renderStats.successRate >= 80 ? '#faad14' : '#ff4d4f', fontSize: '28px' }} 
                />
                <Progress 
                  percent={renderStats.successRate} 
                  showInfo={false} 
                  strokeColor={renderStats.successRate >= 95 ? '#52c41a' : renderStats.successRate >= 80 ? '#faad14' : '#ff4d4f'} 
                  style={{ marginTop: 12 }} 
                />
              </Card>
            </Col>
            <Col span={6}>
              <Card variant="outlined" bodyStyle={{ padding: '20px', textAlign: 'center' }}>
                <Statistic 
                  title={t('overview.cacheHitRate')} 
                  value={renderStats.cacheHitRate} 
                  suffix="%" 
                  precision={1}
                  valueStyle={{ color: renderStats.cacheHitRate >= 90 ? '#52c41a' : renderStats.cacheHitRate >= 70 ? '#faad14' : '#ff4d4f', fontSize: '28px' }} 
                />
                <Progress 
                  percent={renderStats.cacheHitRate} 
                  showInfo={false} 
                  strokeColor={renderStats.cacheHitRate >= 90 ? '#52c41a' : renderStats.cacheHitRate >= 70 ? '#faad14' : '#ff4d4f'} 
                  style={{ marginTop: 12 }} 
                />
              </Card>
            </Col>
            <Col span={6}>
              <Card variant="outlined" bodyStyle={{ padding: '20px', textAlign: 'center' }}>
                <Statistic 
                  title={t('overview.avgRenderTime')} 
                  value={renderStats.avgRenderTime} 
                  suffix="ms" 
                  precision={0}
                  valueStyle={{ color: renderStats.avgRenderTime <= 500 ? '#52c41a' : renderStats.avgRenderTime <= 1000 ? '#faad14' : '#ff4d4f', fontSize: '28px' }} 
                />
                <div style={{ marginTop: 12, color: '#666', fontSize: '12px' }}>
                  {renderStats.avgRenderTime <= 500 ? t('overview.perfExcellent') : renderStats.avgRenderTime <= 1000 ? t('overview.perfGood') : t('overview.perfOptimize')}
                </div>
              </Card>
            </Col>
            <Col span={6}>
              <Card variant="outlined" bodyStyle={{ padding: '20px', textAlign: 'center' }}>
                <Statistic 
                  title={t('overview.securityEventsTotal')} 
                  value={securityEvents.length} 
                  valueStyle={{ color: securityEvents.length === 0 ? '#52c41a' : '#ff4d4f', fontSize: '28px' }} 
                />
                <div style={{ marginTop: 12, color: '#666', fontSize: '12px' }}>
                  {securityEvents.length === 0 ? t('overview.securityGood') : t('overview.needsAttention')}
                </div>
              </Card>
            </Col>
          </Row>
        </Card>

        {/* 安全事件和渲染性能图表 */}
        <Card className="card" style={{ marginTop: 16 }}>
          <Row gutter={[16, 16]}>
            <Col span={12}>
              <Card title={t('overview.securityEventDistribution')} variant="outlined" bodyStyle={{ padding: '16px' }}>
                <div style={{ height: 300 }}>
                  <BaseChart option={securityPieOption} style={{ height: '100%' }} />
                </div>
              </Card>
            </Col>
            <Col span={12}>
              <Card title={t('overview.renderTimeDistribution')} variant="outlined" bodyStyle={{ padding: '16px' }}>
                <div style={{ height: 300 }}>
                  <BaseChart option={renderPerformanceOption} style={{ height: '100%' }} />
                </div>
              </Card>
            </Col>
          </Row>
        </Card>

        {/* 缓存命中率趋势 */}
        <Card className="card" style={{ marginTop: 16 }}>
          <h3 style={{ marginBottom: 16 }}>{t('overview.cacheHitTrend')}</h3>
          <BaseChart option={cacheHitTrendOption} style={{ height: 250 }} />
        </Card>
      </div>
    </Spin>
  )
}

export default Overview