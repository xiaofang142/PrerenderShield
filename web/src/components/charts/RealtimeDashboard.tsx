import React, { useState, useEffect, useRef } from 'react'
import { Card, Row, Col, Statistic, Tag, Space, Button, Switch } from 'antd'
import { 
  ReloadOutlined, 
  WifiOutlined, 
  DisconnectOutlined,
  DashboardOutlined,
  SafetyCertificateOutlined,
  CodeOutlined,
  GlobalOutlined
} from '@ant-design/icons'
import * as echarts from 'echarts'
import BaseChart from '../../components/charts/BaseChart'

interface RealtimeData {
  timestamp: number
  requests: number
  crawlers: number
  blocked: number
  cacheHits: number
  cacheMisses: number
}

interface RealtimeDashboardProps {
  wsUrl?: string
}

const RealtimeDashboard: React.FC<RealtimeDashboardProps> = ({ 
  wsUrl = 'ws://localhost:9598/ws' 
}) => {
  const [isConnected, setIsConnected] = useState(false)
  const [realtimeData, setRealtimeData] = useState<RealtimeData[]>([])
  const [currentStats, setCurrentStats] = useState({
    requests: 0,
    crawlers: 0,
    blocked: 0,
    cacheHitRate: 0,
  })
  const wsRef = useRef<WebSocket | null>(null)
  const chartRef = useRef<echarts.ECharts | null>(null)

  // WebSocket 连接
  useEffect(() => {
    const connectWebSocket = () => {
      try {
        const ws = new WebSocket(`${wsUrl}?id=dashboard-${Date.now()}`)
        
        ws.onopen = () => {
          setIsConnected(true)
          console.log('WebSocket connected')
        }
        
        ws.onmessage = (event) => {
          try {
            const message = JSON.parse(event.data)
            handleWebSocketMessage(message)
          } catch (e) {
            console.error('Failed to parse message:', e)
          }
        }
        
        ws.onclose = () => {
          setIsConnected(false)
          console.log('WebSocket disconnected')
          // 自动重连
          setTimeout(connectWebSocket, 3000)
        }
        
        ws.onerror = (error) => {
          console.error('WebSocket error:', error)
        }
        
        wsRef.current = ws
      } catch (e) {
        console.error('Failed to connect WebSocket:', e)
      }
    }
    
    connectWebSocket()
    
    return () => {
      if (wsRef.current) {
        wsRef.current.close()
      }
    }
  }, [wsUrl])

  // 处理 WebSocket 消息
  const handleWebSocketMessage = (message: any) => {
    if (message.type === 'dashboard') {
      const data = message.payload
      setCurrentStats({
        requests: data.total_requests || 0,
        crawlers: data.crawler_requests || 0,
        blocked: data.blocked_requests || 0,
        cacheHitRate: data.cache_hit_rate || 0,
      })
      
      // 添加到历史数据
      setRealtimeData(prev => {
        const newData = [...prev, {
          timestamp: Date.now(),
          requests: data.total_requests || 0,
          crawlers: data.crawler_requests || 0,
          blocked: data.blocked_requests || 0,
          cacheHits: 0,
          cacheMisses: 0,
        }]
        // 保留最近60个数据点
        return newData.slice(-60)
      })
    }
  }

  // 图表配置
  const chartOption: echarts.EChartsOption = {
    tooltip: {
      trigger: 'axis',
      axisPointer: {
        type: 'cross',
      },
    },
    legend: {
      data: ['请求数', '爬虫请求', '拦截请求'],
    },
    grid: {
      left: '3%',
      right: '4%',
      bottom: '3%',
      containLabel: true,
    },
    xAxis: {
      type: 'category',
      boundaryGap: false,
      data: realtimeData.map(d => new Date(d.timestamp).toLocaleTimeString()),
    },
    yAxis: {
      type: 'value',
    },
    series: [
      {
        name: '请求数',
        type: 'line',
        smooth: true,
        data: realtimeData.map(d => d.requests),
        itemStyle: { color: '#1890ff' },
      },
      {
        name: '爬虫请求',
        type: 'line',
        smooth: true,
        data: realtimeData.map(d => d.crawlers),
        itemStyle: { color: '#52c41a' },
      },
      {
        name: '拦截请求',
        type: 'line',
        smooth: true,
        data: realtimeData.map(d => d.blocked),
        itemStyle: { color: '#ff4d4f' },
      },
    ],
  }

  // 发送消息
  const sendMessage = (type: string, payload: any) => {
    if (wsRef.current && wsRef.current.readyState === WebSocket.OPEN) {
      wsRef.current.send(JSON.stringify({ type, payload }))
    }
  }

  return (
    <div>
      <Row gutter={[16, 16]} style={{ marginBottom: 16 }}>
        <Col span={6}>
          <Card className="card">
            <Space>
              {isConnected ? (
                <WifiOutlined style={{ color: '#52c41a', fontSize: 16 }} />
              ) : (
                <DisconnectOutlined style={{ color: '#ff4d4f', fontSize: 16 }} />
              )}
              <span>{isConnected ? '已连接' : '未连接'}</span>
            </Space>
            <Button 
              size="small" 
              icon={<ReloadOutlined />}
              onClick={() => sendMessage('ping', {})}
              style={{ marginLeft: 8 }}
            >
              刷新
            </Button>
          </Card>
        </Col>
        <Col span={6}>
          <Card className="card">
            <Statistic
              title="实时请求数"
              value={currentStats.requests}
              prefix={<DashboardOutlined />}
              valueStyle={{ color: '#1890ff' }}
            />
          </Card>
        </Col>
        <Col span={6}>
          <Card className="card">
            <Statistic
              title="爬虫请求"
              value={currentStats.crawlers}
              prefix={<GlobalOutlined />}
              valueStyle={{ color: '#52c41a' }}
            />
          </Card>
        </Col>
        <Col span={6}>
          <Card className="card">
            <Statistic
              title="拦截请求"
              value={currentStats.blocked}
              prefix={<SafetyCertificateOutlined />}
              valueStyle={{ color: '#ff4d4f' }}
            />
          </Card>
        </Col>
      </Row>

      <Card className="card" title="实时流量趋势">
        {realtimeData.length > 0 ? (
          <BaseChart option={chartOption} height={400} />
        ) : (
          <div style={{ textAlign: 'center', padding: '100px 0', color: '#999' }}>
            等待数据...
          </div>
        )}
      </Card>
    </div>
  )
}

export default RealtimeDashboard
