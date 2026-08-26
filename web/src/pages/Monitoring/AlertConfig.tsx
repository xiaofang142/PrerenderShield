import React, { useState, useEffect } from 'react'
import { useTranslation } from 'react-i18next'
import { Card, Table, Button, Modal, Form, Input, Select, Space, Tag, message, Row, Col, Tabs, Tooltip, Popconfirm, Switch, InputNumber } from 'antd'
import { 
  PlusOutlined, 
  ReloadOutlined, 
  DeleteOutlined, 
  EditOutlined,
  BellOutlined,
  WarningOutlined,
  CheckCircleOutlined
} from '@ant-design/icons'
import { monitoringApi } from '../../services/api'

const { Option } = Select
const { TabPane } = Tabs

// 告警规则模板（名称/描述文案经 i18n 解析）
const alertRuleTemplates = [
  {
    id: 'cpu_high',
    metric: 'system_cpu_usage',
    operator: 'gt',
    threshold: 90,
    severity: 'warning',
  },
  {
    id: 'memory_high',
    metric: 'system_memory_usage',
    operator: 'gt',
    threshold: 85,
    severity: 'warning',
  },
  {
    id: 'disk_high',
    metric: 'system_disk_usage',
    operator: 'gt',
    threshold: 90,
    severity: 'warning',
  },
  {
    id: 'threat_spike',
    metric: 'threats_per_minute',
    operator: 'gt',
    threshold: 100,
    severity: 'critical',
  },
  {
    id: 'render_queue_backlog',
    metric: 'render_queue_size',
    operator: 'gt',
    threshold: 50,
    severity: 'warning',
  },
  {
    id: 'ssl_expiring',
    metric: 'ssl_cert_days_remaining',
    operator: 'lt',
    threshold: 30,
    severity: 'critical',
  },
]

// 通知渠道（名称文案经 i18n 解析）
const notificationChannels = [
  { id: 'webhook', enabled: true, icon: '🔗' },
  { id: 'email', enabled: false, icon: '📧' },
  { id: 'slack', enabled: false, icon: '💬' },
  { id: 'dingtalk', enabled: false, icon: '🔔' },
]

interface AlertRule {
  id: string
  name: string
  metric: string
  operator: string
  threshold: number
  severity: string
  enabled: boolean
  cooldown: number
  description: string
}

interface AlertRecord {
  id: string
  ruleId: string
  ruleName: string
  severity: string
  message: string
  timestamp: string
  value: number
  status: 'active' | 'resolved'
}

const AlertConfig: React.FC = () => {
  const { t } = useTranslation()
  const [alertRules, setAlertRules] = useState<AlertRule[]>([])
  const [alertRecords, setAlertRecords] = useState<AlertRecord[]>([])
  const [loading, setLoading] = useState(false)
  const [ruleModalVisible, setRuleModalVisible] = useState(false)
  const [editingRule, setEditingRule] = useState<AlertRule | null>(null)
  const [channels, setChannels] = useState(notificationChannels)
  
  const [form] = Form.useForm()

  // 初始化数据
  useEffect(() => {
    fetchAlertRules()
    fetchAlertData()
  }, [])

  const fetchAlertRules = async () => {
    try {
      const res = await monitoringApi.getAlertRules()
      if (res.code === 200 && res.data) {
        setAlertRules(res.data || [])
      }
    } catch {
      // Alert rules may not be available, use empty state
    }
  }

  const fetchAlertData = async () => {
    setLoading(true)
    try {
      const res = await monitoringApi.getAlertHistory(50)
      if (res.code === 200 && res.data) {
        setAlertRecords(res.data || [])
      }
    } catch {
      // Alert history may not be available, use empty state
    } finally {
      setLoading(false)
    }
  }

  // 添加/编辑规则
  const handleSaveRule = async (values: any) => {
    try {
      let updatedRules: AlertRule[]
      if (editingRule) {
        // 编辑规则
        updatedRules = alertRules.map(r => 
          r.id === editingRule.id ? { ...r, ...values } : r
        )
        message.success(t('alertConfig.messages.updateSuccess'))
      } else {
        // 添加规则
        const newRule: AlertRule = {
          id: `rule-${Date.now()}`,
          ...values,
          enabled: true,
          cooldown: values.cooldown || 300,
        }
        updatedRules = [...alertRules, newRule]
        message.success(t('alertConfig.messages.addSuccess'))
      }
      setAlertRules(updatedRules)

      // 持久化到后端（逐条保存，与后端单条规则 API 对齐）
      try {
        for (const rule of updatedRules) {
          await monitoringApi.saveAlertRule(rule)
        }
      } catch {
        message.warning(t('alertConfig.messages.localSaveWarning'))
      }

      setRuleModalVisible(false)
      setEditingRule(null)
      form.resetFields()
    } catch (error) {
      message.error(t('common.error'))
    }
  }

  // 删除规则
  const handleDeleteRule = async (ruleId: string) => {
    try {
      const res = await monitoringApi.deleteAlertRule(ruleId)
      if (res.code === 200) {
        setAlertRules(alertRules.filter(r => r.id !== ruleId))
        message.success(t('alertConfig.messages.deleteSuccess'))
      } else {
        message.error(res.message || t('alertConfig.messages.deleteFail'))
      }
    } catch (error) {
      message.error(t('alertConfig.messages.deleteFail'))
    }
  }

  // 切换规则状态
  const handleToggleRule = async (ruleId: string, enabled: boolean) => {
    try {
      const rule = alertRules.find(r => r.id === ruleId)
      if (rule) {
        await monitoringApi.saveAlertRule({ ...rule, enabled })
        setAlertRules(alertRules.map(r => 
          r.id === ruleId ? { ...r, enabled } : r
        ))
      }
    } catch (error) {
      message.error(t('alertConfig.messages.statusUpdateFail'))
    }
  }

  // 切换通知渠道
  const handleToggleChannel = async (channelId: string, enabled: boolean) => {
    const updatedChannels = channels.map(c => 
      c.id === channelId ? { ...c, enabled } : c
    )
    setChannels(updatedChannels)
    message.success(t(enabled ? 'alertConfig.messages.channelEnabled' : 'alertConfig.messages.channelDisabled', {
      name: t(`alertConfig.channels.names.${channelId}`),
    }))
    try {
      await monitoringApi.saveNotificationChannels(updatedChannels)
    } catch {
      message.warning(t('alertConfig.messages.channelSyncFail'))
    }
  }

  // 应用模板
  const handleApplyTemplate = async (template: any) => {
    const templateName = t(`alertConfig.templates.items.${template.id}.name`)
    const newRule: AlertRule = {
      id: `rule-${Date.now()}`,
      name: templateName,
      metric: template.metric,
      operator: template.operator,
      threshold: template.threshold,
      severity: template.severity,
      enabled: true,
      cooldown: 300,
      description: t(`alertConfig.templates.items.${template.id}.description`),
    }
    try {
      const res = await monitoringApi.saveAlertRule(newRule)
      if (res.code === 200) {
        message.success(t('alertConfig.messages.applyTemplateSuccess', { name: templateName }))
        fetchAlertRules()
      } else {
        message.error(res.message || t('alertConfig.messages.applyTemplateFail'))
      }
    } catch (error) {
      message.error(t('alertConfig.messages.applyTemplateFail'))
    }
  }

  // 严重程度标签颜色
  const getSeverityColor = (severity: string) => {
    switch (severity) {
      case 'critical': return 'red'
      case 'warning': return 'orange'
      case 'info': return 'blue'
      default: return 'default'
    }
  }

  // 状态标签
  const getStatusTag = (status: string) => {
    if (status === 'active') {
      return <Tag icon={<WarningOutlined />} color="error">{t('alertConfig.statusTag.active')}</Tag>
    }
    return <Tag icon={<CheckCircleOutlined />} color="success">{t('alertConfig.statusTag.resolved')}</Tag>
  }

  // 规则表格列
  const ruleColumns = [
    {
      title: t('alertConfig.ruleColumns.name'),
      dataIndex: 'name',
      key: 'name',
      render: (text: string) => <span style={{ fontWeight: 500 }}>{text}</span>,
    },
    {
      title: t('alertConfig.ruleColumns.metric'),
      dataIndex: 'metric',
      key: 'metric',
      render: (text: string) => <Tag>{text}</Tag>,
    },
    {
      title: t('alertConfig.ruleColumns.condition'),
      key: 'condition',
      render: (_: any, record: AlertRule) => (
        <span>
          {record.operator === 'gt' ? '>' : record.operator === 'lt' ? '<' : '='} {record.threshold}
        </span>
      ),
    },
    {
      title: t('alertConfig.ruleColumns.severity'),
      dataIndex: 'severity',
      key: 'severity',
      render: (text: string) => <Tag color={getSeverityColor(text)}>{text}</Tag>,
    },
    {
      title: t('alertConfig.ruleColumns.cooldown'),
      dataIndex: 'cooldown',
      key: 'cooldown',
      render: (text: number) => t('alertConfig.ruleColumns.cooldownMinutes', { minutes: text / 60 }),
    },
    {
      title: t('alertConfig.ruleColumns.status'),
      dataIndex: 'enabled',
      key: 'enabled',
      render: (enabled: boolean, record: AlertRule) => (
        <Switch 
          checked={enabled} 
          onChange={(checked) => handleToggleRule(record.id, checked)}
          size="small"
        />
      ),
    },
    {
      title: t('alertConfig.ruleColumns.actions'),
      key: 'action',
      render: (_: any, record: AlertRule) => (
        <Space size="small">
          <Tooltip title={t('common.edit')}>
            <Button 
              type="link" 
              size="small" 
              icon={<EditOutlined />}
              onClick={() => {
                setEditingRule(record)
                form.setFieldsValue(record)
                setRuleModalVisible(true)
              }}
            />
          </Tooltip>
          <Popconfirm
            title={t('alertConfig.deleteConfirm')}
            onConfirm={() => handleDeleteRule(record.id)}
            okText={t('common.ok')}
            cancelText={t('common.cancel')}
          >
            <Tooltip title={t('common.delete')}>
              <Button type="link" size="small" danger icon={<DeleteOutlined />} />
            </Tooltip>
          </Popconfirm>
        </Space>
      ),
    },
  ]

  // 告警记录表格列
  const recordColumns = [
    {
      title: t('alertConfig.recordColumns.ruleName'),
      dataIndex: 'ruleName',
      key: 'ruleName',
      render: (text: string) => <span style={{ fontWeight: 500 }}>{text}</span>,
    },
    {
      title: t('alertConfig.recordColumns.severity'),
      dataIndex: 'severity',
      key: 'severity',
      render: (text: string) => <Tag color={getSeverityColor(text)}>{text}</Tag>,
    },
    {
      title: t('alertConfig.recordColumns.message'),
      dataIndex: 'message',
      key: 'message',
      ellipsis: true,
    },
    {
      title: t('alertConfig.recordColumns.triggerValue'),
      dataIndex: 'value',
      key: 'value',
      render: (text: number) => text.toFixed(1),
    },
    {
      title: t('alertConfig.recordColumns.triggerTime'),
      dataIndex: 'timestamp',
      key: 'timestamp',
      render: (text: string) => new Date(text).toLocaleString(),
    },
    {
      title: t('alertConfig.recordColumns.status'),
      dataIndex: 'status',
      key: 'status',
      render: (text: string) => getStatusTag(text),
    },
  ]

  return (
    <div>
      <h1 className="page-title">{t('alertConfig.title')}</h1>
      
      <Tabs defaultActiveKey="rules">
        {/* 告警规则 */}
        <TabPane 
          tab={
            <Space>
              <BellOutlined />
              <span>{t('alertConfig.tabs.rules')}</span>
            </Space>
          } 
          key="rules"
        >
          {/* 操作栏 */}
          <Card className="card" style={{ marginBottom: 16 }}>
            <Row justify="space-between" align="middle">
              <Col>
                <Space>
                  <Button 
                    type="primary" 
                    icon={<PlusOutlined />}
                    onClick={() => {
                      setEditingRule(null)
                      form.resetFields()
                      setRuleModalVisible(true)
                    }}
                  >
                    {t('alertConfig.toolbar.addRule')}
                  </Button>
                  <Button 
                    icon={<ReloadOutlined />}
                    onClick={() => message.success(t('alertConfig.toolbar.refreshed'))}
                  >
                    {t('alertConfig.toolbar.refresh')}
                  </Button>
                </Space>
              </Col>
              <Col>
                <Space>
                  <Tag color="blue">{t('alertConfig.toolbar.rulesTotal', { count: alertRules.length })}</Tag>
                  <Tag color="green">{t('alertConfig.toolbar.rulesEnabled', { count: alertRules.filter(r => r.enabled).length })}</Tag>
                </Space>
              </Col>
            </Row>
          </Card>

          {/* 规则列表 */}
          <Card className="card">
            <Table
              columns={ruleColumns}
              dataSource={alertRules}
              rowKey="id"
              pagination={false}
            />
          </Card>
        </TabPane>

        {/* 告警记录 */}
        <TabPane 
          tab={
            <Space>
              <WarningOutlined />
              <span>{t('alertConfig.tabs.records')}</span>
            </Space>
          } 
          key="records"
        >
          <Card className="card">
            <Row gutter={[16, 16]} style={{ marginBottom: 16 }}>
              <Col span={6}>
                <Card size="small">
                  <div style={{ textAlign: 'center' }}>
                    <div style={{ fontSize: 24, fontWeight: 'bold', color: '#ff4d4f' }}>
                      {alertRecords.filter(r => r.status === 'active').length}
                    </div>
                    <div style={{ color: '#666' }}>{t('alertConfig.stats.active')}</div>
                  </div>
                </Card>
              </Col>
              <Col span={6}>
                <Card size="small">
                  <div style={{ textAlign: 'center' }}>
                    <div style={{ fontSize: 24, fontWeight: 'bold', color: '#52c41a' }}>
                      {alertRecords.filter(r => r.status === 'resolved').length}
                    </div>
                    <div style={{ color: '#666' }}>{t('alertConfig.stats.resolved')}</div>
                  </div>
                </Card>
              </Col>
              <Col span={6}>
                <Card size="small">
                  <div style={{ textAlign: 'center' }}>
                    <div style={{ fontSize: 24, fontWeight: 'bold', color: '#ff4d4f' }}>
                      {alertRecords.filter(r => r.severity === 'critical').length}
                    </div>
                    <div style={{ color: '#666' }}>{t('alertConfig.stats.critical')}</div>
                  </div>
                </Card>
              </Col>
              <Col span={6}>
                <Card size="small">
                  <div style={{ textAlign: 'center' }}>
                    <div style={{ fontSize: 24, fontWeight: 'bold', color: '#faad14' }}>
                      {alertRecords.filter(r => r.severity === 'warning').length}
                    </div>
                    <div style={{ color: '#666' }}>{t('alertConfig.stats.warning')}</div>
                  </div>
                </Card>
              </Col>
            </Row>
            
            <Table
              columns={recordColumns}
              dataSource={alertRecords}
              rowKey="id"
              loading={loading}
              pagination={{
                showSizeChanger: true,
                showQuickJumper: true,
                showTotal: (total) => t('alertConfig.recordsTotal', { total }),
              }}
            />
          </Card>
        </TabPane>

        {/* 通知渠道 */}
        <TabPane 
          tab={
            <Space>
              <CheckCircleOutlined />
              <span>{t('alertConfig.tabs.channels')}</span>
            </Space>
          } 
          key="channels"
        >
          <Card className="card">
            <div style={{ marginBottom: 16, color: '#666' }}>
              {t('alertConfig.channels.description')}
            </div>
            
            <Row gutter={[16, 16]}>
              {channels.map((channel) => (
                <Col span={6} key={channel.id}>
                  <Card 
                    hoverable
                    style={{ 
                      borderColor: channel.enabled ? '#52c41a' : '#e8e8e8',
                    }}
                  >
                    <div style={{ textAlign: 'center' }}>
                      <div style={{ fontSize: 32, marginBottom: 8 }}>{channel.icon}</div>
                      <div style={{ fontWeight: 'bold', marginBottom: 8 }}>{t(`alertConfig.channels.names.${channel.id}`)}</div>
                      <Switch 
                        checked={channel.enabled}
                        onChange={(checked) => handleToggleChannel(channel.id, checked)}
                        checkedChildren={t('alertConfig.channels.on')}
                        unCheckedChildren={t('alertConfig.channels.off')}
                      />
                    </div>
                  </Card>
                </Col>
              ))}
            </Row>
          </Card>
        </TabPane>

        {/* 规则模板 */}
        <TabPane 
          tab={
            <Space>
              <EditOutlined />
              <span>{t('alertConfig.tabs.templates')}</span>
            </Space>
          } 
          key="templates"
        >
          <Card className="card">
            <div style={{ marginBottom: 16, color: '#666' }}>
              {t('alertConfig.templates.description')}
            </div>
            
            <Row gutter={[16, 16]}>
              {alertRuleTemplates.map((template) => (
                <Col span={8} key={template.id}>
                  <Card 
                    hoverable
                    onClick={() => handleApplyTemplate(template)}
                    style={{ cursor: 'pointer' }}
                  >
                    <div style={{ fontWeight: 'bold', marginBottom: 8 }}>
                      {t(`alertConfig.templates.items.${template.id}.name`)}
                    </div>
                    <div style={{ color: '#666', fontSize: 12, marginBottom: 8 }}>
                      {t(`alertConfig.templates.items.${template.id}.description`)}
                    </div>
                    <div>
                      <Tag color={getSeverityColor(template.severity)}>{template.severity}</Tag>
                      <Tag>{template.metric}</Tag>
                    </div>
                  </Card>
                </Col>
              ))}
            </Row>
          </Card>
        </TabPane>
      </Tabs>

      {/* 添加/编辑规则弹窗 */}
      <Modal
        title={editingRule ? t('alertConfig.form.editTitle') : t('alertConfig.form.addTitle')}
        open={ruleModalVisible}
        onCancel={() => {
          setRuleModalVisible(false)
          setEditingRule(null)
          form.resetFields()
        }}
        footer={null}
        width={600}
      >
        <Form
          form={form}
          layout="vertical"
          onFinish={handleSaveRule}
          initialValues={{
            severity: 'warning',
            operator: 'gt',
            cooldown: 300,
          }}
        >
          <Form.Item
            name="name"
            label={t('alertConfig.form.name')}
            rules={[{ required: true, message: t('alertConfig.form.nameRequired') }]}
          >
            <Input placeholder={t('alertConfig.form.namePlaceholder')} />
          </Form.Item>
          
          <Form.Item
            name="description"
            label={t('alertConfig.form.description')}
          >
            <Input.TextArea rows={2} placeholder={t('alertConfig.form.descriptionPlaceholder')} />
          </Form.Item>
          
          <Row gutter={16}>
            <Col span={12}>
              <Form.Item
                name="metric"
                label={t('alertConfig.form.metric')}
                rules={[{ required: true, message: t('alertConfig.form.metricRequired') }]}
              >
                <Select placeholder={t('alertConfig.form.selectMetric')}>
                  <Option value="system_cpu_usage">{t('alertConfig.form.metricOptions.system_cpu_usage')}</Option>
                  <Option value="system_memory_usage">{t('alertConfig.form.metricOptions.system_memory_usage')}</Option>
                  <Option value="system_disk_usage">{t('alertConfig.form.metricOptions.system_disk_usage')}</Option>
                  <Option value="threats_per_minute">{t('alertConfig.form.metricOptions.threats_per_minute')}</Option>
                  <Option value="render_queue_size">{t('alertConfig.form.metricOptions.render_queue_size')}</Option>
                  <Option value="ssl_cert_days_remaining">{t('alertConfig.form.metricOptions.ssl_cert_days_remaining')}</Option>
                </Select>
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item
                name="severity"
                label={t('alertConfig.form.severity')}
                rules={[{ required: true, message: t('alertConfig.form.severityRequired') }]}
              >
                <Select placeholder={t('alertConfig.form.selectSeverity')}>
                  <Option value="critical">{t('alertConfig.form.severityOptions.critical')}</Option>
                  <Option value="warning">{t('alertConfig.form.severityOptions.warning')}</Option>
                  <Option value="info">{t('alertConfig.form.severityOptions.info')}</Option>
                </Select>
              </Form.Item>
            </Col>
          </Row>
          
          <Row gutter={16}>
            <Col span={8}>
              <Form.Item
                name="operator"
                label={t('alertConfig.form.operator')}
                rules={[{ required: true, message: t('alertConfig.form.operatorRequired') }]}
              >
                  <Select placeholder={t('alertConfig.form.selectOperator')}>
                    <Option value="gt">{t('alertConfig.form.operatorOptions.gt')}</Option>
                    <Option value="lt">{t('alertConfig.form.operatorOptions.lt')}</Option>
                    <Option value="eq">{t('alertConfig.form.operatorOptions.eq')}</Option>
                    <Option value="ge">{t('alertConfig.form.operatorOptions.ge')}</Option>
                    <Option value="le">{t('alertConfig.form.operatorOptions.le')}</Option>
                  </Select>
              </Form.Item>
            </Col>
            <Col span={8}>
              <Form.Item
                name="threshold"
                label={t('alertConfig.form.threshold')}
                rules={[{ required: true, message: t('alertConfig.form.thresholdRequired') }]}
              >
                <InputNumber style={{ width: '100%' }} placeholder={t('alertConfig.form.thresholdPlaceholder')} />
              </Form.Item>
            </Col>
            <Col span={8}>
              <Form.Item
                name="cooldown"
                label={t('alertConfig.form.cooldown')}
                rules={[{ required: true, message: t('alertConfig.form.cooldownRequired') }]}
              >
                <InputNumber style={{ width: '100%' }} min={60} max={3600} placeholder={t('alertConfig.form.cooldownPlaceholder')} />
              </Form.Item>
            </Col>
          </Row>
          
          <Form.Item>
            <Space>
              <Button type="primary" htmlType="submit">
                {editingRule ? t('alertConfig.form.updateRule') : t('alertConfig.toolbar.addRule')}
              </Button>
              <Button onClick={() => {
                setRuleModalVisible(false)
                setEditingRule(null)
                form.resetFields()
              }}>
                {t('common.cancel')}
              </Button>
            </Space>
          </Form.Item>
        </Form>
      </Modal>
    </div>
  )
}

export default AlertConfig
