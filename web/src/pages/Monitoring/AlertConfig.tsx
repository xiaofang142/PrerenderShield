import React, { useState, useEffect } from 'react'
import { Card, Table, Button, Modal, Form, Input, Select, Space, Tag, message, Row, Col, Tabs, Tooltip, Popconfirm, Switch, InputNumber, Empty } from 'antd'
import { 
  PlusOutlined, 
  ReloadOutlined, 
  DeleteOutlined, 
  EditOutlined,
  BellOutlined,
  WarningOutlined,
  CheckCircleOutlined,
  CloseCircleOutlined,
  SaveOutlined
} from '@ant-design/icons'

const { Option } = Select
const { TabPane } = Tabs

// 告警规则模板
const alertRuleTemplates = [
  {
    id: 'cpu_high',
    name: 'CPU 使用率过高',
    metric: 'system_cpu_usage',
    operator: 'gt',
    threshold: 90,
    severity: 'warning',
    description: '当 CPU 使用率超过 90% 时触发告警',
  },
  {
    id: 'memory_high',
    name: '内存使用率过高',
    metric: 'system_memory_usage',
    operator: 'gt',
    threshold: 85,
    severity: 'warning',
    description: '当内存使用率超过 85% 时触发告警',
  },
  {
    id: 'disk_high',
    name: '磁盘使用率过高',
    metric: 'system_disk_usage',
    operator: 'gt',
    threshold: 90,
    severity: 'warning',
    description: '当磁盘使用率超过 90% 时触发告警',
  },
  {
    id: 'threat_spike',
    name: '威胁检测激增',
    metric: 'threats_per_minute',
    operator: 'gt',
    threshold: 100,
    severity: 'critical',
    description: '当每分钟检测到的威胁数超过 100 时触发告警',
  },
  {
    id: 'render_queue_backlog',
    name: '渲染队列积压',
    metric: 'render_queue_size',
    operator: 'gt',
    threshold: 50,
    severity: 'warning',
    description: '当渲染队列积压超过 50 时触发告警',
  },
  {
    id: 'ssl_expiring',
    name: 'SSL 证书即将过期',
    metric: 'ssl_cert_days_remaining',
    operator: 'lt',
    threshold: 30,
    severity: 'critical',
    description: '当 SSL 证书将在 30 天内过期时触发告警',
  },
]

// 通知渠道
const notificationChannels = [
  { id: 'webhook', name: 'Webhook', enabled: true, icon: '🔗' },
  { id: 'email', name: '邮件', enabled: false, icon: '📧' },
  { id: 'slack', name: 'Slack', enabled: false, icon: '💬' },
  { id: 'dingtalk', name: '钉钉', enabled: false, icon: '🔔' },
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
  const [alertRules, setAlertRules] = useState<AlertRule[]>([])
  const [alertRecords, setAlertRecords] = useState<AlertRecord[]>([])
  const [loading, setLoading] = useState(false)
  const [ruleModalVisible, setRuleModalVisible] = useState(false)
  const [editingRule, setEditingRule] = useState<AlertRule | null>(null)
  const [channels, setChannels] = useState(notificationChannels)
  
  const [form] = Form.useForm()

  // 初始化数据
  useEffect(() => {
    // 模拟加载告警规则
    setAlertRules([
      {
        id: 'cpu_high',
        name: 'CPU 使用率过高',
        metric: 'system_cpu_usage',
        operator: 'gt',
        threshold: 90,
        severity: 'warning',
        enabled: true,
        cooldown: 300,
        description: '当 CPU 使用率超过 90% 时触发告警',
      },
      {
        id: 'memory_high',
        name: '内存使用率过高',
        metric: 'system_memory_usage',
        operator: 'gt',
        threshold: 85,
        severity: 'warning',
        enabled: true,
        cooldown: 300,
        description: '当内存使用率超过 85% 时触发告警',
      },
      {
        id: 'threat_spike',
        name: '威胁检测激增',
        metric: 'threats_per_minute',
        operator: 'gt',
        threshold: 100,
        severity: 'critical',
        enabled: true,
        cooldown: 600,
        description: '当每分钟检测到的威胁数超过 100 时触发告警',
      },
    ])
    
    // 模拟加载告警记录
    setAlertRecords([
      {
        id: 'alert-1',
        ruleId: 'cpu_high',
        ruleName: 'CPU 使用率过高',
        severity: 'warning',
        message: 'CPU 使用率: 92.5%',
        timestamp: new Date(Date.now() - 3600000).toISOString(),
        value: 92.5,
        status: 'resolved',
      },
      {
        id: 'alert-2',
        ruleId: 'memory_high',
        ruleName: '内存使用率过高',
        severity: 'warning',
        message: '内存使用率: 87.3%',
        timestamp: new Date(Date.now() - 7200000).toISOString(),
        value: 87.3,
        status: 'resolved',
      },
      {
        id: 'alert-3',
        ruleId: 'threat_spike',
        ruleName: '威胁检测激增',
        severity: 'critical',
        message: '威胁检测: 156 次/分钟',
        timestamp: new Date(Date.now() - 1800000).toISOString(),
        value: 156,
        status: 'active',
      },
    ])
  }, [])

  // 添加/编辑规则
  const handleSaveRule = async (values: any) => {
    try {
      if (editingRule) {
        // 编辑规则
        setAlertRules(alertRules.map(r => 
          r.id === editingRule.id ? { ...r, ...values } : r
        ))
        message.success('规则更新成功')
      } else {
        // 添加规则
        const newRule: AlertRule = {
          id: `rule-${Date.now()}`,
          ...values,
          enabled: true,
        }
        setAlertRules([...alertRules, newRule])
        message.success('规则添加成功')
      }
      setRuleModalVisible(false)
      setEditingRule(null)
      form.resetFields()
    } catch (error) {
      message.error('操作失败')
    }
  }

  // 删除规则
  const handleDeleteRule = (ruleId: string) => {
    setAlertRules(alertRules.filter(r => r.id !== ruleId))
    message.success('规则删除成功')
  }

  // 切换规则状态
  const handleToggleRule = (ruleId: string, enabled: boolean) => {
    setAlertRules(alertRules.map(r => 
      r.id === ruleId ? { ...r, enabled } : r
    ))
  }

  // 切换通知渠道
  const handleToggleChannel = (channelId: string, enabled: boolean) => {
    setChannels(channels.map(c => 
      c.id === channelId ? { ...c, enabled } : c
    ))
    message.success(`${channels.find(c => c.id === channelId)?.name} 已${enabled ? '启用' : '禁用'}`)
  }

  // 应用模板
  const handleApplyTemplate = (template: any) => {
    const newRule: AlertRule = {
      id: `rule-${Date.now()}`,
      name: template.name,
      metric: template.metric,
      operator: template.operator,
      threshold: template.threshold,
      severity: template.severity,
      enabled: true,
      cooldown: 300,
      description: template.description,
    }
    setAlertRules([...alertRules, newRule])
    message.success(`已应用模板: ${template.name}`)
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
      return <Tag icon={<WarningOutlined />} color="error">告警中</Tag>
    }
    return <Tag icon={<CheckCircleOutlined />} color="success">已恢复</Tag>
  }

  // 规则表格列
  const ruleColumns = [
    {
      title: '规则名称',
      dataIndex: 'name',
      key: 'name',
      render: (text: string) => <span style={{ fontWeight: 500 }}>{text}</span>,
    },
    {
      title: '监控指标',
      dataIndex: 'metric',
      key: 'metric',
      render: (text: string) => <Tag>{text}</Tag>,
    },
    {
      title: '条件',
      key: 'condition',
      render: (_: any, record: AlertRule) => (
        <span>
          {record.operator === 'gt' ? '>' : record.operator === 'lt' ? '<' : '='} {record.threshold}
        </span>
      ),
    },
    {
      title: '严重程度',
      dataIndex: 'severity',
      key: 'severity',
      render: (text: string) => <Tag color={getSeverityColor(text)}>{text}</Tag>,
    },
    {
      title: '冷却时间',
      dataIndex: 'cooldown',
      key: 'cooldown',
      render: (text: number) => `${text / 60} 分钟`,
    },
    {
      title: '状态',
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
      title: '操作',
      key: 'action',
      render: (_: any, record: AlertRule) => (
        <Space size="small">
          <Tooltip title="编辑">
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
            title="确定要删除这个规则吗？"
            onConfirm={() => handleDeleteRule(record.id)}
            okText="确定"
            cancelText="取消"
          >
            <Tooltip title="删除">
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
      title: '告警规则',
      dataIndex: 'ruleName',
      key: 'ruleName',
      render: (text: string) => <span style={{ fontWeight: 500 }}>{text}</span>,
    },
    {
      title: '严重程度',
      dataIndex: 'severity',
      key: 'severity',
      render: (text: string) => <Tag color={getSeverityColor(text)}>{text}</Tag>,
    },
    {
      title: '告警信息',
      dataIndex: 'message',
      key: 'message',
      ellipsis: true,
    },
    {
      title: '触发值',
      dataIndex: 'value',
      key: 'value',
      render: (text: number) => text.toFixed(1),
    },
    {
      title: '触发时间',
      dataIndex: 'timestamp',
      key: 'timestamp',
      render: (text: string) => new Date(text).toLocaleString(),
    },
    {
      title: '状态',
      dataIndex: 'status',
      key: 'status',
      render: (text: string) => getStatusTag(text),
    },
  ]

  return (
    <div>
      <h1 className="page-title">告警配置</h1>
      
      <Tabs defaultActiveKey="rules">
        {/* 告警规则 */}
        <TabPane 
          tab={
            <Space>
              <BellOutlined />
              <span>告警规则</span>
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
                    添加规则
                  </Button>
                  <Button 
                    icon={<ReloadOutlined />}
                    onClick={() => message.success('规则已刷新')}
                  >
                    刷新
                  </Button>
                </Space>
              </Col>
              <Col>
                <Space>
                  <Tag color="blue">共 {alertRules.length} 条规则</Tag>
                  <Tag color="green">{alertRules.filter(r => r.enabled).length} 条启用</Tag>
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
              <span>告警记录</span>
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
                    <div style={{ color: '#666' }}>活跃告警</div>
                  </div>
                </Card>
              </Col>
              <Col span={6}>
                <Card size="small">
                  <div style={{ textAlign: 'center' }}>
                    <div style={{ fontSize: 24, fontWeight: 'bold', color: '#52c41a' }}>
                      {alertRecords.filter(r => r.status === 'resolved').length}
                    </div>
                    <div style={{ color: '#666' }}>已恢复</div>
                  </div>
                </Card>
              </Col>
              <Col span={6}>
                <Card size="small">
                  <div style={{ textAlign: 'center' }}>
                    <div style={{ fontSize: 24, fontWeight: 'bold', color: '#ff4d4f' }}>
                      {alertRecords.filter(r => r.severity === 'critical').length}
                    </div>
                    <div style={{ color: '#666' }}>严重告警</div>
                  </div>
                </Card>
              </Col>
              <Col span={6}>
                <Card size="small">
                  <div style={{ textAlign: 'center' }}>
                    <div style={{ fontSize: 24, fontWeight: 'bold', color: '#faad14' }}>
                      {alertRecords.filter(r => r.severity === 'warning').length}
                    </div>
                    <div style={{ color: '#666' }}>警告</div>
                  </div>
                </Card>
              </Col>
            </Row>
            
            <Table
              columns={recordColumns}
              dataSource={alertRecords}
              rowKey="id"
              pagination={{
                showSizeChanger: true,
                showQuickJumper: true,
                showTotal: (total) => `共 ${total} 条记录`,
              }}
            />
          </Card>
        </TabPane>

        {/* 通知渠道 */}
        <TabPane 
          tab={
            <Space>
              <CheckCircleOutlined />
              <span>通知渠道</span>
            </Space>
          } 
          key="channels"
        >
          <Card className="card">
            <div style={{ marginBottom: 16, color: '#666' }}>
              配置告警通知渠道，当告警触发时将通过启用的渠道发送通知
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
                      <div style={{ fontWeight: 'bold', marginBottom: 8 }}>{channel.name}</div>
                      <Switch 
                        checked={channel.enabled}
                        onChange={(checked) => handleToggleChannel(channel.id, checked)}
                        checkedChildren="启用"
                        unCheckedChildren="禁用"
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
              <span>规则模板</span>
            </Space>
          } 
          key="templates"
        >
          <Card className="card">
            <div style={{ marginBottom: 16, color: '#666' }}>
              选择一个模板快速添加常用告警规则
            </div>
            
            <Row gutter={[16, 16]}>
              {alertRuleTemplates.map((template) => (
                <Col span={8} key={template.id}>
                  <Card 
                    hoverable
                    onClick={() => handleApplyTemplate(template)}
                    style={{ cursor: 'pointer' }}
                  >
                    <div style={{ fontWeight: 'bold', marginBottom: 8 }}>{template.name}</div>
                    <div style={{ color: '#666', fontSize: 12, marginBottom: 8 }}>
                      {template.description}
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
        title={editingRule ? '编辑告警规则' : '添加告警规则'}
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
            label="规则名称"
            rules={[{ required: true, message: '请输入规则名称' }]}
          >
            <Input placeholder="例如：CPU 使用率过高" />
          </Form.Item>
          
          <Form.Item
            name="description"
            label="规则描述"
          >
            <Input.TextArea rows={2} placeholder="规则描述" />
          </Form.Item>
          
          <Row gutter={16}>
            <Col span={12}>
              <Form.Item
                name="metric"
                label="监控指标"
                rules={[{ required: true, message: '请选择监控指标' }]}
              >
                <Select placeholder="选择指标">
                  <Option value="system_cpu_usage">CPU 使用率</Option>
                  <Option value="system_memory_usage">内存使用率</Option>
                  <Option value="system_disk_usage">磁盘使用率</Option>
                  <Option value="threats_per_minute">威胁检测/分钟</Option>
                  <Option value="render_queue_size">渲染队列大小</Option>
                  <Option value="ssl_cert_days_remaining">SSL 证书剩余天数</Option>
                </Select>
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item
                name="severity"
                label="严重程度"
                rules={[{ required: true, message: '请选择严重程度' }]}
              >
                <Select placeholder="选择严重程度">
                  <Option value="critical">严重</Option>
                  <Option value="warning">警告</Option>
                  <Option value="info">信息</Option>
                </Select>
              </Form.Item>
            </Col>
          </Row>
          
          <Row gutter={16}>
            <Col span={8}>
              <Form.Item
                name="operator"
                label="操作符"
                rules={[{ required: true, message: '请选择操作符' }]}
              >
                  <Select placeholder="选择操作符">
                    <Option value="gt">大于 (&gt;)</Option>
                    <Option value="lt">小于 (&lt;)</Option>
                    <Option value="eq">等于 (=)</Option>
                    <Option value="ge">大于等于 (&gt;=)</Option>
                    <Option value="le">小于等于 (&lt;=)</Option>
                  </Select>
              </Form.Item>
            </Col>
            <Col span={8}>
              <Form.Item
                name="threshold"
                label="阈值"
                rules={[{ required: true, message: '请输入阈值' }]}
              >
                <InputNumber style={{ width: '100%' }} placeholder="阈值" />
              </Form.Item>
            </Col>
            <Col span={8}>
              <Form.Item
                name="cooldown"
                label="冷却时间(秒)"
                rules={[{ required: true, message: '请输入冷却时间' }]}
              >
                <InputNumber style={{ width: '100%' }} min={60} max={3600} placeholder="秒" />
              </Form.Item>
            </Col>
          </Row>
          
          <Form.Item>
            <Space>
              <Button type="primary" htmlType="submit">
                {editingRule ? '更新规则' : '添加规则'}
              </Button>
              <Button onClick={() => {
                setRuleModalVisible(false)
                setEditingRule(null)
                form.resetFields()
              }}>
                取消
              </Button>
            </Space>
          </Form.Item>
        </Form>
      </Modal>
    </div>
  )
}

export default AlertConfig
