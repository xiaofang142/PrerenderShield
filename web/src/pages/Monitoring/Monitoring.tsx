import React, { useState, useEffect } from 'react'
import { Card, Row, Col, Statistic, Button, Table, Modal, Input, message, Switch } from 'antd'
import { BarChartOutlined, BellOutlined, ReloadOutlined, PlusOutlined, DeleteOutlined } from '@ant-design/icons'
import { monitoringApi } from '../../services/api'
import BaseChart from '../../components/charts/BaseChart'

const { Search } = Input

const Monitoring: React.FC = () => {
  const [stats, setStats] = useState({
    requestsPerSecond: 12.5,
    cpuUsage: 25.3,
    memoryUsage: 67.8,
    diskUsage: 45.2,
  })
  const [loading, setLoading] = useState(true)
  const [logs, setLogs] = useState<any[]>([])

  // 图表配置
  const cpuChartOption = {
    tooltip: {
      trigger: 'axis',
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
      trigger: 'axis',
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
      trigger: 'axis',
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

  // 表格列配置
  const logColumns = [
    {
      title: '时间',
      dataIndex: 'time',
      key: 'time',
    },
    {
      title: '类型',
      dataIndex: 'type',
      key: 'type',
      render: (text: string) => {
        const color = text === 'error' ? '#f5222d' : text === 'warning' ? '#faad14' : '#1890ff'
        return <span style={{ color }}>{text}</span>
      },
    },
    {
      title: '消息',
      dataIndex: 'message',
      key: 'message',
      ellipsis: true,
    },
    {
      title: '详情',
      dataIndex: 'detail',
      key: 'detail',
      ellipsis: true,
    },
  ]

  // 获取监控数据
  useEffect(() => {
    const fetchData = async () => {
      try {
        setLoading(true)
        const [statsRes, logsRes] = await Promise.all([
          monitoringApi.getStats(),
          monitoringApi.getLogs(),
        ])
        
        if (statsRes.code === 200) {
          setStats(statsRes.data)
        }
        
        if (logsRes.code === 200) {
          setLogs(logsRes.data)
        }
      } catch (error) {
        console.error('Failed to fetch monitoring data:', error)
        message.error('获取监控数据失败')
      } finally {
        setLoading(false)
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

      {/* 日志列表 */}
      <Card className="card" title="系统日志">
        <Table
          columns={logColumns}
          dataSource={logs}
          rowKey="time"
          loading={loading}
          pagination={{ pageSize: 10 }}
          scroll={{ x: 800 }}
          emptyText="暂无日志"
        />
      </Card>
    </div>
  )
}

export default Monitoring