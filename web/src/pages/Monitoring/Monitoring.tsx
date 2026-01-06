import React, { useState, useEffect } from 'react'
import { Card, Row, Col, Statistic, message, Progress } from 'antd'
import { monitoringApi } from '../../services/api'
import BaseChart from '../../components/charts/BaseChart'

// 格式化字节数
const formatBytes = (bytes: number, decimals = 2): string => {
  if (bytes === 0) return '0 Bytes'
  const k = 1024
  const dm = decimals < 0 ? 0 : decimals
  const sizes = ['Bytes', 'KB', 'MB', 'GB', 'TB', 'PB', 'EB', 'ZB', 'YB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(dm)) + ' ' + sizes[i]
}

const Monitoring: React.FC = () => {
  const [stats, setStats] = useState({
    requestsPerSecond: 12.5,
    cpuUsage: 25.3,
    memoryUsage: 67.8,
    memoryTotal: 0,
    memoryUsed: 0,
    memoryFree: 0,
    diskUsage: 45.2,
    diskTotal: 0,
    diskUsed: 0,
    diskFree: 0,
    networkSent: 0,
    networkRecv: 0,
    networkPacketsSent: 0,
    networkPacketsRecv: 0,
  })

  // 图表配置
  const cpuChartOption: echarts.EChartsOption = {
    tooltip: {
      trigger: 'item' as const,
    },
    series: [
      {
        name: 'CPU使用率',
        type: 'gauge' as const,
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

  const memoryChartOption: echarts.EChartsOption = {
    tooltip: {
      trigger: 'item' as const,
    },
    series: [
      {
        name: '内存使用率',
        type: 'gauge' as const,
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

  const diskChartOption: echarts.EChartsOption = {
    tooltip: {
      trigger: 'item' as const,
    },
    series: [
      {
        name: '磁盘使用率',
        type: 'gauge' as const,
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
      <h1 className="page-title">监控警告</h1>
      
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

      {/* 系统指标概览 */}
      <Card className="card" title="系统指标概览">
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

      {/* 详细资源使用情况 */}
      <Row gutter={[16, 16]} style={{ marginTop: 16 }}>
        {/* 内存详情 */}
        <Col span={12}>
          <Card className="card" title="内存详情">
            <div style={{ marginBottom: 16 }}>
              <Progress percent={stats.memoryUsage} strokeColor={{'0%': '#108ee9', '100%': '#87d068'}} />
            </div>
            <Row gutter={[16, 8]}>
              <Col span={12}>
                <Statistic
                  title="总内存"
                  value={formatBytes(stats.memoryTotal)}
                  valueStyle={{ color: '#1890ff' }}
                />
              </Col>
              <Col span={12}>
                <Statistic
                  title="已用内存"
                  value={formatBytes(stats.memoryUsed)}
                  valueStyle={{ color: '#f5222d' }}
                />
              </Col>
              <Col span={12}>
                <Statistic
                  title="可用内存"
                  value={formatBytes(stats.memoryFree)}
                  valueStyle={{ color: '#52c41a' }}
                />
              </Col>
              <Col span={12}>
                <Statistic
                  title="使用率"
                  value={stats.memoryUsage}
                  suffix="%"
                  valueStyle={{ color: stats.memoryUsage > 80 ? '#f5222d' : '#52c41a' }}
                />
              </Col>
            </Row>
          </Card>
        </Col>

        {/* 磁盘详情 */}
        <Col span={12}>
          <Card className="card" title="磁盘详情">
            <div style={{ marginBottom: 16 }}>
              <Progress percent={stats.diskUsage} strokeColor={{'0%': '#108ee9', '100%': '#87d068'}} />
            </div>
            <Row gutter={[16, 8]}>
              <Col span={12}>
                <Statistic
                  title="总容量"
                  value={formatBytes(stats.diskTotal)}
                  valueStyle={{ color: '#1890ff' }}
                />
              </Col>
              <Col span={12}>
                <Statistic
                  title="已用容量"
                  value={formatBytes(stats.diskUsed)}
                  valueStyle={{ color: '#f5222d' }}
                />
              </Col>
              <Col span={12}>
                <Statistic
                  title="可用容量"
                  value={formatBytes(stats.diskFree)}
                  valueStyle={{ color: '#52c41a' }}
                />
              </Col>
              <Col span={12}>
                <Statistic
                  title="使用率"
                  value={stats.diskUsage}
                  suffix="%"
                  valueStyle={{ color: stats.diskUsage > 80 ? '#f5222d' : '#52c41a' }}
                />
              </Col>
            </Row>
          </Card>
        </Col>
      </Row>

      {/* 网络详情 */}
      <Card className="card" title="网络流量" style={{ marginTop: 16 }}>
        <Row gutter={[16, 16]}>
          <Col span={6}>
            <Statistic
              title="发送字节"
              value={formatBytes(stats.networkSent)}
              valueStyle={{ color: '#1890ff' }}
            />
          </Col>
          <Col span={6}>
            <Statistic
              title="接收字节"
              value={formatBytes(stats.networkRecv)}
              valueStyle={{ color: '#52c41a' }}
            />
          </Col>
          <Col span={6}>
            <Statistic
              title="发送包数"
              value={stats.networkPacketsSent}
              valueStyle={{ color: '#faad14' }}
            />
          </Col>
          <Col span={6}>
            <Statistic
              title="接收包数"
              value={stats.networkPacketsRecv}
              valueStyle={{ color: '#722ed1' }}
            />
          </Col>
        </Row>
      </Card>
    </div>
  )
}

export default Monitoring