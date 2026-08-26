import React, { useState, useEffect } from 'react'
import { Card, Form, InputNumber, Button, Space, message, Row, Col, Divider, Tabs, Tag, Statistic, Popconfirm, Table } from 'antd'
import { 
  SaveOutlined, 
  ReloadOutlined, 
  SettingOutlined,
  CloudServerOutlined,
  ToolOutlined,
} from '@ant-design/icons'
import { systemApi } from '../../services/api'
import { useTranslation } from 'react-i18next'

const SettingsPage: React.FC = () => {
  const { t } = useTranslation()
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
        message.success(t('settings.messages.backupSuccess'))
        fetchBackups()
      } else {
        message.error(res.message || t('settings.messages.backupFailed'))
      }
    } catch (error: any) {
      message.error(error.response?.data?.message || t('settings.messages.backupFailed'))
    } finally {
      setBackupLoading(false)
    }
  }

  const handleRestore = async (backupKey: string) => {
    try {
      const res = await systemApi.restore(backupKey)
      if (res.code === 200) {
        message.success(t('settings.messages.restoreSuccess'))
        fetchConfig()
      } else {
        message.error(res.message || t('settings.messages.restoreFailed'))
      }
    } catch (error: any) {
      message.error(error.response?.data?.message || t('settings.messages.restoreFailed'))
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
      message.error(t('settings.messages.fetchConfigFailed'))
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
        message.success(t('settings.messages.saveSuccess'))
      } else {
        message.error(res.message || t('settings.messages.saveFailed'))
      }
    } catch (error) {
      console.error('Failed to save config:', error)
      message.error(t('settings.messages.saveFailed'))
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
      <h1 className="page-title">{t('settings.title')}</h1>
      
      <Tabs activeKey={activeTab} onChange={setActiveTab} items={[
        {
          key: 'config',
          label: <Space><SettingOutlined /><span>{t('settings.tabs.config')}</span></Space>,
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
                <Divider orientation="left">{t('settings.config.accessLogDivider')}</Divider>
                <Row gutter={24}>
                  <Col span={12}>
                    <Form.Item name="access_log_retention_days" label={t('settings.config.accessLogRetention')} help={t('settings.config.accessLogRetentionHelp')}>
                      <InputNumber min={1} max={365} addonAfter={t('settings.config.dayUnit')} style={{ width: '100%' }} />
                    </Form.Item>
                  </Col>
                  <Col span={12}>
                    <Form.Item name="access_log_max_size" label={t('settings.config.accessLogMaxSize')} help={t('settings.config.accessLogMaxSizeHelp')}>
                      <InputNumber min={1} max={1024} addonAfter="MB" style={{ width: '100%' }} />
                    </Form.Item>
                  </Col>
                </Row>
                <Divider orientation="left">{t('settings.config.crawlerLogDivider')}</Divider>
                <Row gutter={24}>
                  <Col span={12}>
                    <Form.Item name="crawler_log_retention_days" label={t('settings.config.crawlerLogRetention')} help={t('settings.config.crawlerLogRetentionHelp')}>
                      <InputNumber min={1} max={365} addonAfter={t('settings.config.dayUnit')} style={{ width: '100%' }} />
                    </Form.Item>
                  </Col>
                  <Col span={12}>
                    <Form.Item name="crawler_log_max_size" label={t('settings.config.crawlerLogMaxSize')} help={t('settings.config.crawlerLogMaxSizeHelp')}>
                      <InputNumber min={1} max={1024} addonAfter="MB" style={{ width: '100%' }} />
                    </Form.Item>
                  </Col>
                </Row>
                <Form.Item>
                  <Space>
                    <Button type="primary" htmlType="submit" icon={<SaveOutlined />} loading={saving}>{t('settings.config.saveConfig')}</Button>
                    <Button icon={<ReloadOutlined />} onClick={fetchConfig} loading={loading}>{t('common.reset')}</Button>
                  </Space>
                </Form.Item>
              </Form>
            </Card>
          ),
        },
        {
          key: 'status',
          label: <Space><CloudServerOutlined /><span>{t('settings.tabs.status')}</span></Space>,
          children: (
            <>
              <Card className="card" title={t('settings.status.healthTitle')}>
                {healthData ? (
                  <Row gutter={[24, 24]}>
                    <Col span={8}><Card size="small"><Statistic title={t('settings.status.serviceStatus')} value={healthData.status === 'running' ? t('settings.status.running') : healthData.status} valueStyle={{ color: healthData.status === 'running' ? '#52c41a' : '#faad14' }} /></Card></Col>
                    <Col span={8}><Card size="small"><Statistic title={t('settings.status.redisStatus')} value={healthData.redis_status === 'connected' ? t('settings.status.connected') : t('settings.status.disconnected')} valueStyle={{ color: healthData.redis_status === 'connected' ? '#52c41a' : '#ff4d4f' }} /></Card></Col>
                    <Col span={8}><Card size="small"><Statistic title={t('settings.status.sslStatus')} value={healthData.ssl_status === 'active' ? t('settings.status.active') : healthData.ssl_status} valueStyle={{ color: healthData.ssl_status === 'active' ? '#52c41a' : '#1890ff' }} /></Card></Col>
                    {healthData.health_details && (
                      <>
                        <Col span={6}><Card size="small"><Statistic title={t('settings.status.memoryUsage')} value={formatBytes(healthData.health_details.memory_allocated)} valueStyle={{ color: '#1890ff' }} /></Card></Col>
                        <Col span={6}><Card size="small"><Statistic title={t('settings.status.sysMemory')} value={formatBytes(healthData.health_details.memory_sys)} valueStyle={{ color: '#1890ff' }} /></Card></Col>
                        <Col span={6}><Card size="small"><Statistic title={t('settings.status.goroutines')} value={healthData.health_details.num_goroutines} valueStyle={{ color: '#1890ff' }} /></Card></Col>
                        <Col span={6}><Card size="small"><Statistic title={t('settings.status.gcCycles')} value={healthData.health_details.gc_cycles} valueStyle={{ color: '#1890ff' }} /></Card></Col>
                      </>
                    )}
                  </Row>
                ) : (
                  <div style={{ textAlign: 'center', padding: '40px' }}><Tag color="processing">{t('common.loading')}</Tag></div>
                )}
              </Card>
              <Card className="card" title={t('settings.status.versionTitle')} style={{ marginTop: 16 }}>
                {versionData ? (
                  <Row gutter={[24, 24]}>
                    <Col span={8}><Card size="small"><Statistic title={t('settings.status.currentVersion')} value={versionData.version || 'unknown'} valueStyle={{ color: '#52c41a' }} /></Card></Col>
                    <Col span={8}><Card size="small"><Statistic title={t('settings.status.serviceName')} value={versionData.name || 'prerender-shield'} valueStyle={{ color: '#1890ff' }} /></Card></Col>
                    <Col span={8}><Card size="small"><Statistic title={t('settings.status.officialUrl')} value={versionData.official_url || 'N/A'} valueStyle={{ color: '#1890ff', fontSize: 14 }} /></Card></Col>
                  </Row>
                ) : (
                  <div style={{ textAlign: 'center', padding: '40px' }}><Tag color="processing">{t('common.loading')}</Tag></div>
                )}
              </Card>
            </>
          ),
        },
        {
          key: 'backup',
          label: <Space><CloudServerOutlined /><span>{t('settings.tabs.backup')}</span></Space>,
          children: (
            <Card className="card" title={t('settings.backup.configTitle')} extra={<Button type="primary" icon={<SaveOutlined />} onClick={handleBackup} loading={backupLoading}>{t('settings.backup.createBackup')}</Button>}>
              <Table
                dataSource={backups}
                rowKey="key"
                loading={backupsLoading}
                columns={[
                  { title: t('settings.backup.backupTime'), dataIndex: 'timestamp', key: 'timestamp' },
                  { title: t('settings.backup.backupKey'), dataIndex: 'key', key: 'key', ellipsis: true },
                  {
                    title: t('common.actions'), key: 'action',
                    render: (_: any, record: any) => (
                      <Popconfirm title={t('settings.backup.restoreConfirm')} onConfirm={() => handleRestore(record.key)} okText={t('common.ok')} cancelText={t('common.cancel')}>
                        <Button type="link" size="small">{t('settings.backup.restore')}</Button>
                      </Popconfirm>
                    ),
                  },
                ]}
                pagination={false}
                locale={{ emptyText: t('settings.backup.empty') }}
              />
            </Card>
          ),
        },
        {
          key: 'about',
          label: <Space><ToolOutlined /><span>{t('settings.tabs.about')}</span></Space>,
          children: (
            <Card className="card">
              <div style={{ textAlign: 'center', padding: '40px' }}>
                <h2 style={{ marginBottom: 16 }}>Prerender Shield</h2>
                <p style={{ color: '#666', marginBottom: 24 }}>{t('settings.about.description')}</p>
                <Row gutter={[24, 24]} justify="center">
                  <Col span={8}><Card size="small"><Statistic title={t('settings.about.version')} value={versionData?.version || 'v1.0.1'} valueStyle={{ color: '#52c41a' }} /></Card></Col>
                  <Col span={8}><Card size="small"><Statistic title={t('settings.about.license')} value="MIT License" valueStyle={{ color: '#1890ff' }} /></Card></Col>
                  <Col span={8}><Card size="small"><Statistic title={t('settings.about.techStack')} value="Go + React" valueStyle={{ color: '#722ed1' }} /></Card></Col>
                </Row>
                <div style={{ marginTop: 24 }}>
                  <Space>
                    <Button type="link" href="https://github.com/xiaofang142/PrerenderShield" target="_blank">GitHub</Button>
                    <Button type="link" href="https://gitee.com/xhpmayun/prerender-shield" target="_blank">Gitee</Button>
                    <Button type="link" href="https://prerender.websitetool.cn" target="_blank">{t('settings.about.docs')}</Button>
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
