import React, { useEffect, useState } from 'react';
import { Card, Row, Col, Statistic, Spin, message } from 'antd';
import { SafetyCertificateOutlined, GlobalOutlined, ThunderboltOutlined, BugOutlined } from '@ant-design/icons';
import { Line } from '@ant-design/charts';
import { overviewApi } from '../services/api';

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
  const [loading, setLoading] = useState(true);
  const [stats, setStats] = useState<DashboardStats | null>(null);

  const fetchStats = async () => {
    try {
      const response = await overviewApi.getStats();
      if (response.code === 200) {
        setStats(response.data);
      } else {
        message.error(response.message || '获取数据失败');
      }
    } catch (error) {
      console.error('Failed to fetch dashboard stats:', error);
      // message.error('获取数据失败'); // Prevent spamming error on first load if auth fails
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchStats();
    // Refresh every 30 seconds
    const interval = setInterval(fetchStats, 30000);
    return () => clearInterval(interval);
  }, []);

  if (loading && !stats) {
    return (
      <div style={{ display: 'flex', justifyContent: 'center', alignItems: 'center', height: '100vh' }}>
        <Spin size="large" />
      </div>
    );
  }

  // Transformation for Line chart (wide to long)
  const chartData: any[] = [];
  if (stats?.trafficData) {
    stats.trafficData.forEach(item => {
      chartData.push({ time: item.time, value: item.totalRequests, type: '总请求' });
      chartData.push({ time: item.time, value: item.crawlerRequests, type: '爬虫请求' });
      chartData.push({ time: item.time, value: item.blockedRequests, type: '拦截请求' });
    });
  }

  const lineConfig = {
    data: chartData,
    xField: 'time',
    yField: 'value',
    seriesField: 'type',
    smooth: true,
    color: ['#1890ff', '#52c41a', '#cf1322'],
  };

  return (
    <div style={{ padding: '24px' }}>
      <h1 className="page-title">控制台首页</h1>
      
      <Row gutter={16}>
        <Col span={6}>
          <Card hoverable>
            <Statistic
              title="总请求量"
              value={stats?.totalRequests || 0}
              prefix={<GlobalOutlined />}
              valueStyle={{ color: '#1890ff' }}
            />
          </Card>
        </Col>
        <Col span={6}>
          <Card hoverable>
            <Statistic
              title="拦截攻击"
              value={stats?.blockedRequests || 0}
              prefix={<SafetyCertificateOutlined />}
              valueStyle={{ color: '#cf1322' }}
            />
          </Card>
        </Col>
        <Col span={6}>
          <Card hoverable>
            <Statistic
              title="爬虫请求"
              value={stats?.crawlerRequests || 0}
              prefix={<BugOutlined />}
              valueStyle={{ color: '#52c41a' }}
            />
          </Card>
        </Col>
        <Col span={6}>
          <Card hoverable>
            <Statistic
              title="活跃站点"
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
            <Card title="流量趋势 (24小时)">
              {chartData.length > 0 ? (
                <Line {...lineConfig} />
              ) : (
                <div style={{ textAlign: 'center', padding: '40px' }}>暂无数据</div>
              )}
            </Card>
          </Col>
        </Row>
      </div>

      <div style={{ marginTop: 24 }}>
        <Row gutter={16}>
          <Col span={8}>
            <Card title="访问统计">
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
            <Card title="系统状态">
              <Row gutter={16}>
                <Col span={12}>
                  <Statistic 
                    title="WAF 防火墙" 
                    value={stats?.firewallEnabled ? '开启' : '关闭'} 
                    valueStyle={{ color: stats?.firewallEnabled ? '#52c41a' : '#bfbfbf' }}
                  />
                </Col>
                <Col span={12}>
                  <Statistic 
                    title="预渲染引擎" 
                    value={stats?.prerenderEnabled ? '开启' : '关闭'} 
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
