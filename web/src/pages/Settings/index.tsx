import React, { useState, useEffect } from 'react'
import { Card, Form, InputNumber, Button, Space, message, Row, Col, Divider, Tabs, Tag, Statistic, Popconfirm } from 'antd'
import { 
  SaveOutlined, 
  ReloadOutlined, 
  SettingOutlined,
  CloudServerOutlined,
  ToolOutlined,
} from '@ant-design/icons'
import { systemApi } from '../../services/api'

const SettingsPage: React.FC = () => {
  const [form] = Form.useForm()
  const [loading, setLoading] = useState(false)
  const [saving, setSaving] = useState(false)
  const [healthData, setHealthData] = useState<any>(null)
  const [versionData, setVersionData] = useState<any>(null)
  const [activeTab, setActiveTab] = useState('config')
  // Backup state
  const [backups, setBackups] = useState<any[]>([])
  const [backupsLoading, setBackupsLoading] = useState(false)
  const [backupLoading, setBackupLoading] = useState(false)

  const fetchBackups = async () => {
    try {
      setBackupsLoading(true)
      const res = await systemApi.listBackups()
      if (res.code === 200) {
        setBackups(res.data || [])
      }
    } catch (error) {
      console.error('Failed to fetch backups:', error)
    } finally {
      setBackupsLoading(false)
    }
  }

  const handleBackup = async () => {
    try {
      setBackupLoading(true)
      const res = await systemApi.backup()
      if (res.code === 200) {
        message.success('备份创建成功')
        fetchBackups()
      } else {
        message.error(res.message || '备份失败')
      }
    } catch (error: any) {
      message.error(error.response?.data?.message || '备份失败')
    } finally {
      setBackupLoading(false)
    }
  }

  const handleRestore = async (backupKey: string) => {
    try {
      const res = await systemApi.restore(backupKey)
      if (res.code === 200) {
        message.success('配置恢复成功')
        fetchConfig()
      } else {
        message.error(res.message || '恢复失败')
      }
    } catch (error: any) {
      message.error(error.response?.data?.message || '恢复失败')
    }
  }

  // 获取系统配置
  const fetchConfig = async () => {
    try {
      setLoading(true)
      const res = await systemApi.getConfig()
      if (res.code === 200) {
        form.setFieldsValue(res.data)
      }
    } catch (error) {
      console.error('Failed to fetch config:', error)
      message.error('获取配置失败')
    } finally {
      setLoading(false)
    }
  }

  // 获取健康状态
  const fetchHealth = async () => {
    try {
      const res = await systemApi.health()
      if (res.code === 200) {
        setHealthData(res.data)
      }
    } catch (error) {
      console.error('Failed to fetch health:', error)
    }
  }

  // 获取版本信息
  const fetchVersion = async () => {
    try {
      const res = await systemApi.version()
      if (res.code === 200) {
        setVersionData(res.data)
      }
    } catch (error) {
      console.error('Failed to fetch version:', error)
    }
  }

  // 初始化
  useEffect(() => {
    fetchConfig()
    fetchHealth()
    fetchVersion()
    fetchBackups()
  }, [])

  // 保存配置
  const handleSave = async (values: any) => {
    try {
      setSaving(true)
      const res = await systemApi.updateConfig(values)
      if (res.code === 200) {
        message.success('配置保存成功')
      } else {
        message.error(res.message || '保存失败')
      }
    } catch (error) {
      console.error('Failed to save config:', error)
      message.error('保存失败')
    } finally {
      setSaving(false)
    }
  }

  // 格式化字节数
  const formatBytes = (bytes: number) => {
    if (!bytes) return '0 Bytes'
    const k = 1024
    const sizes = ['Bytes', 'KB', 'MB', 'GB', 'TB']
    const i = Math.floor(Math.log(bytes) / Math.log(k))
    return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i]
  }

  return (
    <div>
      <h1 className="page-title">系统设置</h1>
      
      <Tabs activeKey={activeTab} onChange={setActiveTab} items={[
        {
          key: 'config',
          label: <Space><SettingOutlined /><span>系统配置</span></Space>,
          children: (
            <Card className="card" loading={loading}>
              <Form
                form={form}
                layout="vertical"
                onFinish={handleSave}
                initialValues={{
                  access_log_retention_days: 7,
                  access_log_max_size: 128,
                  crawler_log_retention_days: 7,
                  crawler_log_max_size: 128,
                }}
              >
                <Divider orientation="left">访问日志配置</Divider>
                <Row gutter={24}>
                  <Col span={12}>
                    <Form.Item name="access_log_retention_days" label="日志保留天数" help="访问日志保留的天数，超过后自动清理">
                      <InputNumber min={1} max={365} addonAfter="天" style={{ width: '100%' }} />
                    </Form.Item>
                  </Col>
                  <Col span={12}>
                    <Form.Item name="access_log_max_size" label="日志文件大小限制" help="单个日志文件的最大大小(MB)">
                      <InputNumber min={1} max={1024} addonAfter="MB" style={{ width: '100%' }} />
                    </Form.Item>
                  </Col>
                </Row>
                <Divider orientation="left">爬虫日志配置</Divider>
                <Row gutter={24}>
                  <Col span={12}>
                    <Form.Item name="crawler_log_retention_days" label="爬虫日志保留天数" help="爬虫日志保留的天数，超过后自动清理">
                      <InputNumber min={1} max={365} addonAfter="天" style={{ width: '100%' }} />
                    </Form.Item>
                  </Col>
                  <Col span={12}>
                    <Form.Item name="crawler_log_max_size" label="爬虫日志大小限制" help="单个爬虫日志文件的最大大小(MB)">
                      <InputNumber min={1} max={1024} addonAfter="MB" style={{ width: '100%' }} />
                    </Form.Item>
                  </Col>
                </Row>
                <Form.Item>
                  <Space>
                    <Button type="primary" htmlType="submit" icon={<SaveOutlined />} loading={saving}>保存配置</Button>
                    <Button icon={<ReloadOutlined />} onClick={fetchConfig} loading={loading}>重置</Button>
                  </Space>
                </Form.Item>
              </Form>
            </Card>
          ),
        },
        {
          key: 'status',
          label: <Space><CloudServerOutlined /><span>系统状态</span></Space>,
          children: (
            <>
              <Card className="card" title="系统健康状态">
                {healthData ? (
                  <Row gutter={[24, 24]}>
                    <Col span={8}><Card size="small"><Statistic title="服务状态" value={healthData.status === 'running' ? '运行中' : healthData.status} valueStyle={{ color: healthData.status === 'running' ? '#52c41a' : '#faad14' }} /></Card></Col>
                    <Col span={8}><Card size="small"><Statistic title="Redis 状态" value={healthData.redis_status === 'connected' ? '已连接' : '未连接'} valueStyle={{ color: healthData.redis_status === 'connected' ? '#52c41a' : '#ff4d4f' }} /></Card></Col>
                    <Col span={8}><Card size="small"><Statistic title="SSL 状态" value={healthData.ssl_status === 'active' ? '活跃' : healthData.ssl_status} valueStyle={{ color: healthData.ssl_status === 'active' ? '#52c41a' : '#1890ff' }} /></Card></Col>
                    {healthData.health_details && (
                      <>
                        <Col span={6}><Card size="small"><Statistic title="内存使用" value={formatBytes(healthData.health_details.memory_allocated)} valueStyle={{ color: '#1890ff' }} /></Card></Col>
                        <Col span={6}><Card size="small"><Statistic title="系统内存" value={formatBytes(healthData.health_details.memory_sys)} valueStyle={{ color: '#1890ff' }} /></Card></Col>
                        <Col span={6}><Card size="small"><Statistic title="Goroutine 数量" value={healthData.health_details.num_goroutines} valueStyle={{ color: '#1890ff' }} /></Card></Col>
                        <Col span={6}><Card size="small"><Statistic title="GC 次数" value={healthData.health_details.gc_cycles} valueStyle={{ color: '#1890ff' }} /></Card></Col>
                      </>
                    )}
                  </Row>
                ) : (
                  <div style={{ textAlign: 'center', padding: '40px' }}><Tag color="processing">加载中...</Tag></div>
                )}
              </Card>
              <Card className="card" title="版本信息" style={{ marginTop: 16 }}>
                {versionData ? (
                  <Row gutter={[24, 24]}>
                    <Col span={8}><Card size="small"><Statistic title="当前版本" value={versionData.version || 'unknown'} valueStyle={{ color: '#52c41a' }} /></Card></Col>
                    <Col span={8}><Card size="small"><Statistic title="服务名称" value={versionData.name || 'prerender-shield'} valueStyle={{ color: '#1890ff' }} /></Card></Col>
                    <Col span={8}><Card size="small"><Statistic title="官方网址" value={versionData.official_url || 'N/A'} valueStyle={{ color: '#1890ff', fontSize: 14 }} /></Card></Col>
                  </Row>
                ) : (
                  <div style={{ textAlign: 'center', padding: '40px' }}><Tag color="processing">加载中...</Tag></div>
                )}
              </Card>
            </>
          ),
        },
        {
          key: 'backup',
          label: <Space><CloudServerOutlined /><span>备份恢复</span></Space>,
          children: (
            <Card className="card" title="配置备份" extra={<Button type="primary" icon={<SaveOutlined />} onClick={handleBackup} loading={backupLoading}>创建备份</Button>}>
              <Table
                dataSource={backups}
                rowKey="key"
                loading={backupsLoading}
                columns={[
                  { title: '备份时间', dataIndex: 'timestamp', key: 'timestamp' },
                  { title: '备份键', dataIndex: 'key', key: 'key', ellipsis: true },
                  {
                    title: '操作', key: 'action',
                    render: (_: any, record: any) => (
                      <Popconfirm title="确定要恢复此备份吗？当前配置将被覆盖。" onConfirm={() => handleRestore(record.key)} okText="确定" cancelText="取消">
                        <Button type="link" size="small">恢复</Button>
                      </Popconfirm>
                    ),
                  },
                ]}
                pagination={false}
                locale={{ emptyText: '暂无备份' }}
              />
            </Card>
          ),
        },
        {
          key: 'about',
          label: <Space><ToolOutlined /><span>关于</span></Space>,
          children: (
            <Card className="card">
              <div style={{ textAlign: 'center', padding: '40px' }}>
                <h2 style={{ marginBottom: 16 }}>Prerender Shield</h2>
                <p style={{ color: '#666', marginBottom: 24 }}>企业级 Web 应用中间件，集成 OWASP Top 10 安全防护与智能渲染预热功能</p>
                <Row gutter={[24, 24]} justify="center">
                  <Col span={8}><Card size="small"><Statistic title="版本" value={versionData?.version || 'v1.0.1'} valueStyle={{ color: '#52c41a' }} /></Card></Col>
                  <Col span={8}><Card size="small"><Statistic title="许可证" value="MIT License" valueStyle={{ color: '#1890ff' }} /></Card></Col>
                  <Col span={8}><Card size="small"><Statistic title="技术栈" value="Go + React" valueStyle={{ color: '#722ed1' }} /></Card></Col>
                </Row>
                <div style={{ marginTop: 24 }}>
                  <Space>
                    <Button type="link" href="https://github.com/xiaofang142/PrerenderShield" target="_blank">GitHub</Button>
                    <Button type="link" href="https://gitee.com/xhpmayun/prerender-shield" target="_blank">Gitee</Button>
                    <Button type="link" href="https://prerender.websitetool.cn" target="_blank">官方文档</Button>
                  </Space>
                </div>
              </div>
            </Card>
          ),
        },
      ]} />
    </div>
  )
}

export default SettingsPage
