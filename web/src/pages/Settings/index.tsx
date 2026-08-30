import React, { useState, useEffect, useRef } from 'react'
import { Card, Form, InputNumber, Button, Space, message, Row, Col, Divider, Tabs, Tag, Statistic, Popconfirm, Table, Modal, Typography, List } from 'antd'
import {
  SaveOutlined,
  ReloadOutlined,
  SettingOutlined,
  CloudServerOutlined,
  ToolOutlined,
  KeyOutlined,
  PlusOutlined,
  DeleteOutlined,
  CopyOutlined,
} from '@ant-design/icons'
import { systemApi } from '../../services/api'
import { useTranslation } from 'react-i18next'

// 后端只读保护键：整段提交时必须剥离，否则 SaveSystemConfig 直接拒绝
const BLOCKED_CONFIG_KEYS = ['jwt_secret', 'redis_url', 'admin_password']

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
  // API Token 管理（sha256 哈希存储；原文仅生成时展示一次）
  const [apiTokens, setApiTokens] = useState<string[]>([])
  const [tokenSaving, setTokenSaving] = useState(false)
  const [generatedRaw, setGeneratedRaw] = useState('')
  // 最近一次 GET 的完整 system:config（保存时整段合并提交，避免整段替换丢字段）
  const configRef = useRef<Record<string, any>>({})

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
        configRef.current = { ...(res.data || {}) }
        form.setFieldsValue(res.data)
        // api_tokens 存储为 JSON 数组字符串
        try {
          const raw = (res.data || {}).api_tokens
          setApiTokens(raw ? JSON.parse(raw) : [])
        } catch {
          setApiTokens([])
        }
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

  // 保存配置（整段合并：system:config 为全量替换语义，只传表单字段会清掉其余键）
  const handleSave = async (values: any) => {
    try {
      setSaving(true)
      const merged: Record<string, any> = { ...configRef.current, ...values }
      for (const key of BLOCKED_CONFIG_KEYS) {
        delete merged[key]
      }
      const res = await systemApi.updateConfig(merged)
      if (res.code === 200) {
        configRef.current = merged
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

  // ─── API Token 管理（仅 /preheat/* 端点可用；sha256 哈希存储，原文不落盘）───
  const saveTokens = async (tokens: string[]) => {
    try {
      setTokenSaving(true)
      const merged: Record<string, any> = { ...configRef.current }
      for (const key of BLOCKED_CONFIG_KEYS) {
        delete merged[key]
      }
      merged.api_tokens = JSON.stringify(tokens)
      const res = await systemApi.updateConfig(merged)
      if (res.code === 200) {
        configRef.current = merged
        setApiTokens(tokens)
        return true
      }
      message.error(res.message || t('settings.token.saveFailed'))
      return false
    } catch (error) {
      console.error('Failed to save api tokens:', error)
      message.error(t('settings.token.saveFailed'))
      return false
    } finally {
      setTokenSaving(false)
    }
  }

  const sha256Hex = async (text: string): Promise<string> => {
    const digest = await crypto.subtle.digest('SHA-256', new TextEncoder().encode(text))
    return Array.from(new Uint8Array(digest)).map((b) => b.toString(16).padStart(2, '0')).join('')
  }

  const handleGenerateToken = async () => {
    // 原文仅存于浏览器变量：pst_ + 32 字节随机 hex
    const bytes = new Uint8Array(32)
    crypto.getRandomValues(bytes)
    const raw = 'pst_' + Array.from(bytes).map((b) => b.toString(16).padStart(2, '0')).join('')
    const hash = await sha256Hex(raw)
    const ok = await saveTokens([...apiTokens, hash])
    if (ok) {
      setGeneratedRaw(raw)
    }
  }

  const handleRevokeToken = async (hash: string) => {
    const ok = await saveTokens(apiTokens.filter((h) => h !== hash))
    if (ok) {
      message.success(t('settings.token.revokeSuccess'))
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

                <Divider orientation="left">{t('settings.token.title')}</Divider>
                <p style={{ color: '#666', marginTop: 0 }}>{t('settings.token.help')}</p>
                <Space style={{ marginBottom: 16 }}>
                  <Button type="primary" icon={<PlusOutlined />} onClick={handleGenerateToken} loading={tokenSaving}>
                    {t('settings.token.generate')}
                  </Button>
                </Space>
                <List
                  size="small"
                  bordered
                  dataSource={apiTokens}
                  locale={{ emptyText: t('settings.token.empty') }}
                  renderItem={(hash: string) => (
                    <List.Item
                      actions={[
                        <Popconfirm key="revoke" title={t('settings.token.revokeConfirm')} onConfirm={() => handleRevokeToken(hash)} okText={t('common.ok')} cancelText={t('common.cancel')}>
                          <Button type="link" size="small" danger icon={<DeleteOutlined />}>{t('settings.token.revoke')}</Button>
                        </Popconfirm>,
                      ]}
                    >
                      <Space>
                        <KeyOutlined />
                        <Typography.Text code copyable={{ text: hash }}>{hash.slice(0, 16)}…{hash.slice(-8)}</Typography.Text>
                      </Space>
                    </List.Item>
                  )}
                />
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

      {/* Token 原文一次性展示（关闭后无法再查看，仅哈希已保存） */}
      <Modal
        title={t('settings.token.generatedTitle')}
        open={!!generatedRaw}
        onCancel={() => setGeneratedRaw('')}
        footer={[
          <Button key="copy" icon={<CopyOutlined />} onClick={() => { navigator.clipboard?.writeText(generatedRaw); message.success(t('common.copied')) }}>
            {t('settings.token.copy')}
          </Button>,
          <Button key="ok" type="primary" onClick={() => setGeneratedRaw('')}>{t('common.ok')}</Button>,
        ]}
      >
        <p>{t('settings.token.generatedWarning')}</p>
        <Typography.Paragraph code copyable={{ text: generatedRaw }} style={{ wordBreak: 'break-all' }}>
          {generatedRaw}
        </Typography.Paragraph>
        <p style={{ color: '#666', fontSize: 12 }}>{t('settings.token.usage')}</p>
      </Modal>
    </div>
  )
}

export default SettingsPage
