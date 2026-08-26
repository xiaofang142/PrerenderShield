import React, { useState } from 'react'
import { Card, Row, Col, Statistic, message, Progress } from 'antd'
import type { EChartsOption } from 'echarts'
import { monitoringApi } from '../../services/api'
import BaseChart from '../../components/charts/BaseChart'
import { usePolling } from '@prerender/utils'
import { formatBytes, formatPercent } from '@prerender/utils'
import { pollingIntervals } from '@prerender/design-tokens'
import { useTranslation } from 'react-i18next'

interface SystemStats {
  requestsPerSecond: number
  cpuUsage: number
  memoryUsage: number
  memoryTotal: number
  memoryUsed: number
  memoryFree: number
  diskUsage: number
  diskTotal: number
  diskUsed: number
  diskFree: number
  networkSent: number
  networkRecv: number
  networkPacketsSent: number
  networkPacketsRecv: number
}

const initialStats: SystemStats = {
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
}

// gauge 图表配置工厂：三份重复配置收敛为一个函数（name 为已翻译的完整显示名）
function buildGaugeOption(name: string, value: number): EChartsOption {
  return {
    tooltip: { trigger: 'item' as const },
    series: [
      {
        name,
        type: 'gauge' as const,
        detail: { formatter: '{value}%' },
        data: [{ value, name }],
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
}

const usageColor = (v: number) => (v > 80 ? '#f5222d' : '#52c41a')

const Monitoring: React.FC = () => {
  const { t } = useTranslation()
  const [stats, setStats] = useState<SystemStats>(initialStats)

  // usePolling 统一管理轮询生命周期（卸载清理 + 页面不可见自动暂停）
  usePolling(
    async () => {
      try {
        const statsRes = await monitoringApi.getStats()
        if (statsRes.code === 200) {
          setStats(statsRes.data)
        }
      } catch (error) {
        console.error('Failed to fetch monitoring data:', error)
        message.error(t('monitoringPage.fetchFailed'))
      }
    },
    { interval: pollingIntervals.realtime }
  )

  return (
    <div>
      <h1 className="page-title">{t('monitoringPage.title')}</h1>

      {/* 实时统计卡片 */}
      <Row gutter={[16, 16]}>
        <Col span={8}>
          <Card className="card">
            <h3 style={{ marginBottom: 16 }}>{t('monitoringPage.cpuUsage')}</h3>
            <div style={{ height: 200 }}>
              <BaseChart option={buildGaugeOption(t('monitoringPage.cpuUsage'), stats.cpuUsage)} />
            </div>
          </Card>
        </Col>
        <Col span={8}>
          <Card className="card">
            <h3 style={{ marginBottom: 16 }}>{t('monitoringPage.memoryUsage')}</h3>
            <div style={{ height: 200 }}>
              <BaseChart option={buildGaugeOption(t('monitoringPage.memoryUsage'), stats.memoryUsage)} />
            </div>
          </Card>
        </Col>
        <Col span={8}>
          <Card className="card">
            <h3 style={{ marginBottom: 16 }}>{t('monitoringPage.diskUsage')}</h3>
            <div style={{ height: 200 }}>
              <BaseChart option={buildGaugeOption(t('monitoringPage.diskUsage'), stats.diskUsage)} />
            </div>
          </Card>
        </Col>
      </Row>

      {/* 系统指标概览 */}
      <Card className="card" title={t('monitoringPage.metricsOverview')}>
        <Row gutter={[16, 16]}>
          <Col span={6}>
            <Statistic
              title={t('monitoringPage.requestsPerSecond')}
              value={stats.requestsPerSecond}
              valueStyle={{ color: '#1890ff' }}
            />
          </Col>
          <Col span={6}>
            <Statistic
              title={t('monitoringPage.cpuUsage')}
              value={formatPercent(stats.cpuUsage)}
              valueStyle={{ color: usageColor(stats.cpuUsage) }}
            />
          </Col>
          <Col span={6}>
            <Statistic
              title={t('monitoringPage.memoryUsage')}
              value={formatPercent(stats.memoryUsage)}
              valueStyle={{ color: usageColor(stats.memoryUsage) }}
            />
          </Col>
          <Col span={6}>
            <Statistic
              title={t('monitoringPage.diskUsage')}
              value={formatPercent(stats.diskUsage)}
              valueStyle={{ color: usageColor(stats.diskUsage) }}
            />
          </Col>
        </Row>
      </Card>

      {/* 详细资源使用情况 */}
      <Row gutter={[16, 16]} style={{ marginTop: 16 }}>
        {/* 内存详情 */}
        <Col span={12}>
          <Card className="card" title={t('monitoringPage.memoryDetail')}>
            <div style={{ marginBottom: 16 }}>
              <Progress percent={stats.memoryUsage} strokeColor={{'0%': '#108ee9', '100%': '#87d068'}} />
            </div>
            <Row gutter={[16, 8]}>
              <Col span={12}>
                <Statistic
                  title={t('monitoringPage.totalMemory')}
                  value={formatBytes(stats.memoryTotal)}
                  valueStyle={{ color: '#1890ff' }}
                />
              </Col>
              <Col span={12}>
                <Statistic
                  title={t('monitoringPage.usedMemory')}
                  value={formatBytes(stats.memoryUsed)}
                  valueStyle={{ color: '#f5222d' }}
                />
              </Col>
              <Col span={12}>
                <Statistic
                  title={t('monitoringPage.freeMemory')}
                  value={formatBytes(stats.memoryFree)}
                  valueStyle={{ color: '#52c41a' }}
                />
              </Col>
              <Col span={12}>
                <Statistic
                  title={t('monitoringPage.usageRate')}
                  value={stats.memoryUsage}
                  suffix="%"
                  valueStyle={{ color: usageColor(stats.memoryUsage) }}
                />
              </Col>
            </Row>
          </Card>
        </Col>

        {/* 磁盘详情 */}
        <Col span={12}>
          <Card className="card" title={t('monitoringPage.diskDetail')}>
            <div style={{ marginBottom: 16 }}>
              <Progress percent={stats.diskUsage} strokeColor={{'0%': '#108ee9', '100%': '#87d068'}} />
            </div>
            <Row gutter={[16, 8]}>
              <Col span={12}>
                <Statistic
                  title={t('monitoringPage.totalCapacity')}
                  value={formatBytes(stats.diskTotal)}
                  valueStyle={{ color: '#1890ff' }}
                />
              </Col>
              <Col span={12}>
                <Statistic
                  title={t('monitoringPage.usedCapacity')}
                  value={formatBytes(stats.diskUsed)}
                  valueStyle={{ color: '#f5222d' }}
                />
              </Col>
              <Col span={12}>
                <Statistic
                  title={t('monitoringPage.freeCapacity')}
                  value={formatBytes(stats.diskFree)}
                  valueStyle={{ color: '#52c41a' }}
                />
              </Col>
              <Col span={12}>
                <Statistic
                  title={t('monitoringPage.usageRate')}
                  value={stats.diskUsage}
                  suffix="%"
                  valueStyle={{ color: usageColor(stats.diskUsage) }}
                />
              </Col>
            </Row>
          </Card>
        </Col>
      </Row>

      {/* 网络详情 */}
      <Card className="card" title={t('monitoringPage.networkTraffic')} style={{ marginTop: 16 }}>
        <Row gutter={[16, 16]}>
          <Col span={6}>
            <Statistic
              title={t('monitoringPage.sentBytes')}
              value={formatBytes(stats.networkSent)}
              valueStyle={{ color: '#1890ff' }}
            />
          </Col>
          <Col span={6}>
            <Statistic
              title={t('monitoringPage.recvBytes')}
              value={formatBytes(stats.networkRecv)}
              valueStyle={{ color: '#52c41a' }}
            />
          </Col>
          <Col span={6}>
            <Statistic
              title={t('monitoringPage.sentPackets')}
              value={stats.networkPacketsSent}
              valueStyle={{ color: '#faad14' }}
            />
          </Col>
          <Col span={6}>
            <Statistic
              title={t('monitoringPage.recvPackets')}
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
