import React, { useState, useEffect } from 'react'
import { Card, Row, Col, Statistic, Spin, Select, Tag } from 'antd'
import * as echarts from 'echarts'
import BaseChart from '../../components/charts/BaseChart'
import { overviewApi } from '../../services/api'

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

const Overview: React.FC = () => {
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
  
  const [accessStats] = useState({
    pv: 0,
    uv: 0,
    ip: 0,
    countryData: [],
    mapData: []
  })
  const [loading, setLoading] = useState(true)

  const [isMapLoaded, setIsMapLoaded] = useState(false)

  // 2D地图配置
  const map2DOption: echarts.EChartsOption = {
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
        data: (stats.geoData?.mapData || accessStats.mapData).map(item => ({
          name: item.name,
          value: [item.name, item.value] // 这里需要地图的经纬度，scatter需要坐标，但这里我们用 map series 更好
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

  // 如果地图未加载完成，暂时不渲染地图相关的 Option
  // 或者修改 series type 为 'map'，这样不需要经纬度数据，直接用 name 匹配
  const mapSeriesOption: echarts.EChartsOption = {
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
        text: ['High', 'Low'],
        realtime: false,
        calculable: true,
        inRange: {
            color: ['#lightskyblue', 'yellow', 'orangered']
        }
    },
    series: [
        {
            name: '访问分布',
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
  const barOption: echarts.EChartsOption = {
    tooltip: {
      trigger: 'axis',
      axisPointer: {
        type: 'shadow'
      },
      formatter: '{b}<br/>30天内访问数量: {c}',
    },
    xAxis: {
      type: 'category',
      data: (stats.geoData?.countryData || accessStats.countryData).length > 0
        ? (stats.geoData?.countryData || accessStats.countryData).map(item => item.country)
        : ['暂无数据'], // 空数据占位
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

  // 获取概览数据
  const fetchData = async () => {
    try {
      setLoading(true)
      const overviewRes = await overviewApi.getStats()
      
      if (overviewRes.code === 200) {
        setStats(overviewRes.data)
      }
    } catch (error) {
      console.error('Failed to fetch overview data:', error)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    // 注册地图
    // 使用本地地图文件，避免CDN访问超时问题
    fetch('/maps/world.json')
      .then(response => response.json())
      .then(mapJson => {
        echarts.registerMap('world', mapJson)
        setIsMapLoaded(true)
      })
      .catch(e => {
        console.error('Failed to load world map, trying fallback CDN', e)
        // 如果本地加载失败，尝试使用CDN作为备选
        fetch('https://cdn.jsdelivr.net/npm/echarts@4.9.0/map/json/world.json')
          .then(response => response.json())
          .then(mapJson => {
            echarts.registerMap('world', mapJson)
            setIsMapLoaded(true)
          })
          .catch(e2 => console.error('Failed to load world map from CDN', e2))
      })

    fetchData()
    // 每30秒刷新一次数据
    const interval = setInterval(() => {
      fetchData()
    }, 30000)
    return () => clearInterval(interval)
  }, [])

  return (
    <Spin spinning={loading} tip="加载中...">
      <div>
        <h1 className="page-title">概览</h1>
        
        {/* 全球访问分布 */}
        <Card className="card" style={{ marginTop: 16 }}>
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16 }}>
            <h3 style={{ margin: 0 }}>全球访问分布</h3>
            <div>
              <span style={{ marginRight: 8 }}>地图类型：</span>
              <Select
                value={mapType}
                onChange={setMapType}
                style={{ width: 120 }}
                size="small"
              >
                <Option value="2d">2D地图</Option>
                <Option value="bar">柱状图</Option>
              </Select>
            </div>
          </div>

          {/* 统计数据行 */}
          <Row gutter={[16, 16]} style={{ marginBottom: 24 }}>
            <Col span={4}>
              <Card variant="outlined" bodyStyle={{ padding: '12px' }}>
                <Statistic title="PV (访问量)" value={stats.accessStats?.pv || accessStats.pv} valueStyle={{ color: '#1890ff', fontSize: '18px' }} />
              </Card>
            </Col>
            <Col span={4}>
              <Card variant="outlined" bodyStyle={{ padding: '12px' }}>
                <Statistic title="UV (独立访客)" value={stats.accessStats?.uv || accessStats.uv} valueStyle={{ color: '#52c41a', fontSize: '18px' }} />
              </Card>
            </Col>
            <Col span={4}>
              <Card variant="outlined" bodyStyle={{ padding: '12px' }}>
                <Statistic title="IP (独立IP)" value={stats.accessStats?.ip || accessStats.ip} valueStyle={{ color: '#faad14', fontSize: '18px' }} />
              </Card>
            </Col>
            <Col span={4}>
              <Card variant="outlined" bodyStyle={{ padding: '12px' }}>
                <Statistic title="请求总数" value={stats.totalRequests} valueStyle={{ color: '#1890ff', fontSize: '18px' }} />
              </Card>
            </Col>
            <Col span={4}>
              <Card variant="outlined" bodyStyle={{ padding: '12px' }}>
                <Statistic title="爬虫请求总数" value={stats.crawlerRequests} valueStyle={{ color: '#52c41a', fontSize: '18px' }} />
              </Card>
            </Col>
            <Col span={4}>
              <Card variant="outlined" bodyStyle={{ padding: '12px' }}>
                <Statistic title="攻击拦截总数" value={stats.blockedRequests} valueStyle={{ color: '#ff4d4f', fontSize: '18px' }} />
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
                    <Spin tip="正在加载地图数据..." />
                  </div>
                )}
              </div>
            </Col>
            <Col span={8}>
              <Card title="国家访问排行" variant="outlined">
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
                      暂无访问数据
                    </div>
                  )}
                </div>
              </Card>
            </Col>
          </Row>
        </Card>
      </div>
    </Spin>
  )
}

export default Overview