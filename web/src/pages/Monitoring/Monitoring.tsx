import React, { useState, useEffect } from 'react'
import { Card, Row, Col, Statistic, message } from 'antd'
import { monitoringApi } from '../../services/api'
import BaseChart from '../../components/charts/BaseChart'

const Monitoring: React.FC = () => {
  const [stats, setStats] = useState({
    requestsPerSecond: 12.5,
    cpuUsage: 25.3,
    memoryUsage: 67.8,
    diskUsage: 45.2,
  })

  // 图表配置
  const cpuChartOption = {
    tooltip: {
      trigger: 'item' as const,
    },
    series: [
      {
        name: 'CPU使用率',
        type: 'gauge',
        detail: { formatter: '{value}%' },
        data: [{ value: stats.cpuUsage, name: 'CPU' }],
        axisLine: {
          lineStyle: {
            color: [
              [0.3, '#67e0e3'],
              [0.7, '#37a2da'],
              [1, '#fd666d'],
            ],
          },
        },
      },
    ],
  }

  const memoryChartOption = {
    tooltip: {
      trigger: 'item' as const,
    },
    series: [
      {
        name: '内存使用率',
        type: 'gauge',
        detail: { formatter: '{value}%' },
        data: [{ value: stats.memoryUsage, name: '内存' }],
        axisLine: {
          lineStyle: {
            color: [
              [0.3, '#67e0e3'],
              [0.7, '#37a2da'],
              [1, '#fd666d'],
            ],
          },
        },
      },
    ],
  }

  const diskChartOption = {
    tooltip: {
      trigger: 'item' as const,
    },
    series: [
      {
        name: '磁盘使用率',
        type: 'gauge',
        detail: { formatter: '{value}%' },
        data: [{ value: stats.diskUsage, name: '磁盘' }],
        axisLine: {
          lineStyle: {
            color: [
              [0.3, '#67e0e3'],
              [0.7, '#37a2da'],
              [1, '#fd666d'],
            ],
          },
        },
      },
    ],
  }

  // 获取监控数据
  useEffect(() => {
    const fetchData = async () => {
      try {
        const statsRes = await monitoringApi.getStats()
        
        if (statsRes.code === 200) {
          setStats(statsRes.data)
        }
      } catch (error) {
        console.error('Failed to fetch monitoring data:', error)
        message.error('获取监控数据失败')
      }
    }

    fetchData()
    // 每10秒刷新一次数据
    const interval = setInterval(fetchData, 10000)
    return () => clearInterval(interval)
  }, [])

  return (
    <div>
      <h1 className="page-title">监控告警</h1>
      
      {/* 实时统计卡片 */}
      <Row gutter={[16, 16]}>
        <Col span={8}>
          <Card className="card">
            <h3 style={{ marginBottom: 16 }}>CPU使用率</h3>
            <div style={{ height: 200 }}>
              <BaseChart option={cpuChartOption} />
            </div>
          </Card>
        </Col>
        <Col span={8}>
          <Card className="card">
            <h3 style={{ marginBottom: 16 }}>内存使用率</h3>
            <div style={{ height: 200 }}>
              <BaseChart option={memoryChartOption} />
            </div>
          </Card>
        </Col>
        <Col span={8}>
          <Card className="card">
            <h3 style={{ marginBottom: 16 }}>磁盘使用率</h3>
            <div style={{ height: 200 }}>
              <BaseChart option={diskChartOption} />
            </div>
          </Card>
        </Col>
      </Row>

      {/* 系统指标 */}
      <Card className="card" title="系统指标">
        <Row gutter={[16, 16]}>
          <Col span={6}>
            <Statistic
              title="请求/秒"
              value={stats.requestsPerSecond}
              valueStyle={{ color: '#1890ff' }}
            />
          </Col>
          <Col span={6}>
            <Statistic
              title="CPU使用率"
              value={stats.cpuUsage}
              suffix="%"
              valueStyle={{ color: stats.cpuUsage > 80 ? '#f5222d' : '#52c41a' }}
            />
          </Col>
          <Col span={6}>
            <Statistic
              title="内存使用率"
              value={stats.memoryUsage}
              suffix="%"
              valueStyle={{ color: stats.memoryUsage > 80 ? '#f5222d' : '#52c41a' }}
            />
          </Col>
          <Col span={6}>
            <Statistic
              title="磁盘使用率"
              value={stats.diskUsage}
              suffix="%"
              valueStyle={{ color: stats.diskUsage > 80 ? '#f5222d' : '#52c41a' }}
            />
          </Col>
        </Row>
      </Card>


    </div>
  )
}

export default Monitoring