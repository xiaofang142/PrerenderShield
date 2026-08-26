import React, { useState } from 'react';
import { Card, Row, Col, Statistic, Spin, message } from 'antd';
import { SafetyCertificateOutlined, GlobalOutlined, ThunderboltOutlined, BugOutlined } from '@ant-design/icons';
import BaseChart from '../components/charts/BaseChart';
import type { EChartsOption } from 'echarts';
import { overviewApi } from '../services/api';
import { usePolling } from '@prerender/utils';
import { pollingIntervals } from '@prerender/design-tokens';
import { useRealtime, type RealtimeMessage } from '../hooks/useRealtime';
import { useTranslation } from 'react-i18next';

interface DashboardStats {
  totalRequests: number;
  crawlerRequests: number;
  blockedRequests: number;
  cacheHitRate: number;
  activeBrowsers: number;
  activeSites: number;
  sslCertificates: number;
  firewallEnabled: boolean;
  prerenderEnabled: boolean;
  trafficData: Array<{
    time: string;
    totalRequests: number;
    crawlerRequests: number;
    blockedRequests: number;
  }>;
  accessStats: {
    pv: number;
    uv: number;
    ip: number;
  };
}

const Dashboard: React.FC = () => {
  const { t } = useTranslation();
  const [loading, setLoading] = useState(true);
  const [stats, setStats] = useState<DashboardStats | null>(null);

  // usePolling：卸载自动清理 + 页面不可见暂停
  usePolling(
    async () => {
      try {
        const response = await overviewApi.getStats();
        if (response.code === 200) {
          setStats(response.data);
        } else {
          message.error(response.message || t('dashboard.fetchFailed'));
        }
      } catch (error) {
        console.error('Failed to fetch dashboard stats:', error);
      } finally {
        setLoading(false);
      }
    },
    { interval: pollingIntervals.dashboard }
  );

  // WebSocket 实时推送：监控指标到达时立即刷新，告警弹出提示
  useRealtime((msg: RealtimeMessage) => {
    if (msg.type === 'monitor' && msg.data) {
      setStats((prev) => ({ ...(prev ?? ({} as DashboardStats)), ...(msg.data as Partial<DashboardStats>) }));
    } else if (msg.type === 'alert') {
      message.warning(t('dashboard.newAlert'));
    }
  });

  if (loading && !stats) {
    return (
      <div style={{ display: 'flex', justifyContent: 'center', alignItems: 'center', height: '100vh' }}>
        <Spin size="large" />
      </div>
    );
  }

  // 流量趋势折线图配置（echarts）
  const trafficOption: EChartsOption = {
    tooltip: { trigger: 'axis' },
    legend: { data: [t('dashboard.series.total'), t('dashboard.series.crawler'), t('dashboard.series.blocked')] },
    grid: { left: '3%', right: '4%', bottom: '3%', containLabel: true },
    xAxis: {
      type: 'category',
      boundaryGap: false,
      data: (stats?.trafficData || []).map(item => item.time),
    },
    yAxis: { type: 'value' },
    series: [
      { name: t('dashboard.series.total'), type: 'line', smooth: true, itemStyle: { color: '#1890ff' }, data: (stats?.trafficData || []).map(item => item.totalRequests) },
      { name: t('dashboard.series.crawler'), type: 'line', smooth: true, itemStyle: { color: '#52c41a' }, data: (stats?.trafficData || []).map(item => item.crawlerRequests) },
      { name: t('dashboard.series.blocked'), type: 'line', smooth: true, itemStyle: { color: '#cf1322' }, data: (stats?.trafficData || []).map(item => item.blockedRequests) },
    ],
  };

  return (
    <div style={{ padding: '24px' }}>
      <h1 className="page-title">{t('dashboard.title')}</h1>

      <Row gutter={16}>
        <Col span={6}>
          <Card hoverable>
            <Statistic
              title={t('dashboard.statTotalRequests')}
              value={stats?.totalRequests || 0}
              prefix={<GlobalOutlined />}
              valueStyle={{ color: '#1890ff' }}
            />
          </Card>
        </Col>
        <Col span={6}>
          <Card hoverable>
            <Statistic
              title={t('dashboard.statBlocked')}
              value={stats?.blockedRequests || 0}
              prefix={<SafetyCertificateOutlined />}
              valueStyle={{ color: '#cf1322' }}
            />
          </Card>
        </Col>
        <Col span={6}>
          <Card hoverable>
            <Statistic
              title={t('dashboard.statCrawler')}
              value={stats?.crawlerRequests || 0}
              prefix={<BugOutlined />}
              valueStyle={{ color: '#52c41a' }}
            />
          </Card>
        </Col>
        <Col span={6}>
          <Card hoverable>
            <Statistic
              title={t('dashboard.statActiveSites')}
              value={stats?.activeSites || 0}
              prefix={<ThunderboltOutlined />}
              valueStyle={{ color: '#faad14' }}
            />
          </Card>
        </Col>
      </Row>

      <div style={{ marginTop: 24 }}>
        <Row gutter={24}>
          <Col span={24}>
            <Card title={t('dashboard.trafficTrend')}>
              {stats?.trafficData && stats.trafficData.length > 0 ? (
                <BaseChart option={trafficOption} height={300} />
              ) : (
                <div style={{ textAlign: 'center', padding: '40px' }}>{t('dashboard.noData')}</div>
              )}
            </Card>
          </Col>
        </Row>
      </div>

      <div style={{ marginTop: 24 }}>
        <Row gutter={16}>
          <Col span={8}>
            <Card title={t('dashboard.accessStats')}>
              <div style={{ display: 'flex', justifyContent: 'space-around', textAlign: 'center' }}>
                <div>
                  <div style={{ color: '#8c8c8c' }}>PV</div>
                  <div style={{ fontSize: '24px', fontWeight: 'bold' }}>{stats?.accessStats?.pv || 0}</div>
                </div>
                <div>
                  <div style={{ color: '#8c8c8c' }}>UV</div>
                  <div style={{ fontSize: '24px', fontWeight: 'bold' }}>{stats?.accessStats?.uv || 0}</div>
                </div>
                <div>
                  <div style={{ color: '#8c8c8c' }}>IP</div>
                  <div style={{ fontSize: '24px', fontWeight: 'bold' }}>{stats?.accessStats?.ip || 0}</div>
                </div>
              </div>
            </Card>
          </Col>
          <Col span={16}>
            <Card title={t('dashboard.systemStatus')}>
              <Row gutter={16}>
                <Col span={12}>
                  <Statistic
                    title={t('dashboard.wafFirewall')}
                    value={stats?.firewallEnabled ? t('dashboard.enabled') : t('dashboard.disabled')}
                    valueStyle={{ color: stats?.firewallEnabled ? '#52c41a' : '#bfbfbf' }}
                  />
                </Col>
                <Col span={12}>
                  <Statistic
                    title={t('dashboard.prerenderEngine')}
                    value={stats?.prerenderEnabled ? t('dashboard.enabled') : t('dashboard.disabled')}
                    valueStyle={{ color: stats?.prerenderEnabled ? '#52c41a' : '#bfbfbf' }}
                  />
                </Col>
              </Row>
            </Card>
          </Col>
        </Row>
      </div>
    </div>
  );
};

export default Dashboard;
