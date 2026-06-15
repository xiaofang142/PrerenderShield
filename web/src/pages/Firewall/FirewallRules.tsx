import React, { useState, useEffect } from 'react'
import { Card, Table, Button, Modal, Form, Input, Select, Space, Tag, message, Row, Col, Tabs, Tooltip, Popconfirm, Drawer, InputNumber, Switch } from 'antd'
import { 
  PlusOutlined, 
  ReloadOutlined, 
  DeleteOutlined, 
  EditOutlined,
  CopyOutlined,
  SaveOutlined,
  PlayCircleOutlined,
  SearchOutlined,
  FilterOutlined,
  ExportOutlined
} from '@ant-design/icons'
import { firewallApi, sitesApi } from '../../services/api'

const { Option } = Select
const { TabPane } = Tabs
const { TextArea } = Input

// 规则模板
const ruleTemplates = [
  {
    id: 'sql_injection',
    name: 'SQL 注入防护',
    description: '检测和阻止 SQL 注入攻击',
    category: '注入防护',
    rules: [
      { field: 'query', operator: 'contains', value: "' OR ", action: 'block' },
      { field: 'query', operator: 'contains', value: "UNION SELECT", action: 'block' },
      { field: 'body', operator: 'regex', value: "(?i)(?:union\\s+select|select\\s+\\*\\s+from)", action: 'block' },
    ]
  },
  {
    id: 'xss_protection',
    name: 'XSS 防护',
    description: '检测和阻止跨站脚本攻击',
    category: '脚本防护',
    rules: [
      { field: 'query', operator: 'contains', value: '<script', action: 'block' },
      { field: 'body', operator: 'contains', value: 'javascript:', action: 'block' },
      { field: 'header', operator: 'contains', value: 'onerror=', action: 'block' },
    ]
  },
  {
    id: 'path_traversal',
    name: '路径遍历防护',
    description: '检测和阻止路径遍历攻击',
    category: '路径防护',
    rules: [
      { field: 'path', operator: 'contains', value: '../', action: 'block' },
      { field: 'path', operator: 'contains', value: '..\\', action: 'block' },
      { field: 'path', operator: 'matches', value: '^/etc/passwd', action: 'block' },
    ]
  },
  {
    id: 'rate_limit',
    name: '频率限制',
    description: '限制单个 IP 的请求频率',
    category: '访问控制',
    rules: [
      { field: 'ip', operator: 'count', value: '100', window: '60s', action: 'block' },
    ]
  },
  {
    id: 'geo_block',
    name: '地理位置封锁',
    description: '封锁特定国家/地区的访问',
    category: '访问控制',
    rules: [
      { field: 'country', operator: 'in', value: 'RU,CN,IR', action: 'block' },
    ]
  },
]

interface Rule {
  id: string
  name: string
  field: string
  operator: string
  value: string
  action: 'block' | 'allow' | 'log'
  enabled: boolean
  priority: number
}

const FirewallRules: React.FC = () => {
  const [sites, setSites] = useState<any[]>([])
  const [selectedSite, setSelectedSite] = useState<string>('')
  const [rules, setRules] = useState<Rule[]>([])
  const [loading, setLoading] = useState(false)
  const [ruleModalVisible, setRuleModalVisible] = useState(false)
  const [templateModalVisible, setTemplateModalVisible] = useState(false)
  const [testModalVisible, setTestModalVisible] = useState(false)
  const [editingRule, setEditingRule] = useState<Rule | null>(null)
  const [testRequest, setTestRequest] = useState('')
  const [testResult, setTestResult] = useState<any>(null)
  
  const [form] = Form.useForm()
  const [testForm] = Form.useForm()

  // 获取站点列表
  const fetchSites = async () => {
    try {
      const res = await sitesApi.getSites()
      if (res.code === 200) {
        setSites(res.data)
        if (res.data.length > 0) {
          setSelectedSite(res.data[0].id)
        }
      }
    } catch (error) {
      console.error('Failed to fetch sites:', error)
      message.error('获取站点列表失败')
    }
  }

  // 获取规则列表
  const fetchRules = async () => {
    if (!selectedSite) return
    
    try {
      setLoading(true)
      const res = await firewallApi.getWafConfig(selectedSite)
      if (res.code === 200) {
        // 从配置中提取规则
        const config = res.data
        const extractedRules: Rule[] = []
        
        // 这里假设规则存储在配置中
        // 实际实现可能需要调整
        if (config.rules) {
          extractedRules.push(...config.rules)
        }
        
        setRules(extractedRules)
      }
    } catch (error) {
      console.error('Failed to fetch rules:', error)
      message.error('获取规则列表失败')
    } finally {
      setLoading(false)
    }
  }

  // 初始化
  useEffect(() => {
    fetchSites()
  }, [])

  useEffect(() => {
    if (selectedSite) {
      fetchRules()
    }
  }, [selectedSite])

  // 添加规则
  const handleAddRule = async (values: any) => {
    try {
      const newRule: Rule = {
        id: `rule-${Date.now()}`,
        name: values.name,
        field: values.field,
        operator: values.operator,
        value: values.value,
        action: values.action,
        enabled: true,
        priority: rules.length + 1,
      }
      
      setRules([...rules, newRule])
      setRuleModalVisible(false)
      form.resetFields()
      message.success('规则添加成功')
    } catch (error) {
      message.error('规则添加失败')
    }
  }

  // 编辑规则
  const handleEditRule = async (values: any) => {
    if (!editingRule) return
    
    try {
      const updatedRules = rules.map(r => 
        r.id === editingRule.id 
          ? { ...r, ...values }
          : r
      )
      setRules(updatedRules)
      setRuleModalVisible(false)
      setEditingRule(null)
      form.resetFields()
      message.success('规则更新成功')
    } catch (error) {
      message.error('规则更新失败')
    }
  }

  // 删除规则
  const handleDeleteRule = (ruleId: string) => {
    setRules(rules.filter(r => r.id !== ruleId))
    message.success('规则删除成功')
  }

  // 切换规则状态
  const handleToggleRule = (ruleId: string, enabled: boolean) => {
    setRules(rules.map(r => 
      r.id === ruleId ? { ...r, enabled } : r
    ))
  }

  // 应用模板
  const handleApplyTemplate = (template: any) => {
    const newRules = template.rules.map((r: any, index: number) => ({
      id: `rule-${Date.now()}-${index}`,
      name: `${template.name} - ${r.field}`,
      field: r.field,
      operator: r.operator,
      value: r.value,
      action: r.action,
      enabled: true,
      priority: rules.length + index + 1,
    }))
    
    setRules([...rules, ...newRules])
    setTemplateModalVisible(false)
    message.success(`已应用模板: ${template.name}`)
  }

  // 测试规则
  const handleTestRule = async (values: any) => {
    try {
      // 模拟测试结果
      const result = {
        matched: false,
        rule: null,
        action: 'allow',
        details: '请求未匹配任何规则',
      }
      
      // 检查是否匹配任何规则
      for (const rule of rules) {
        if (!rule.enabled) continue
        
        let matched = false
        const testValue = values[rule.field] || ''
        
        switch (rule.operator) {
          case 'contains':
            matched = testValue.includes(rule.value)
            break
          case 'equals':
            matched = testValue === rule.value
            break
          case 'regex':
            try {
              matched = new RegExp(rule.value, 'i').test(testValue)
            } catch {
              matched = false
            }
            break
          case 'gt':
            matched = parseFloat(testValue) > parseFloat(rule.value)
            break
          case 'lt':
            matched = parseFloat(testValue) < parseFloat(rule.value)
            break
        }
        
        if (matched) {
          result.matched = true
          result.rule = rule
          result.action = rule.action
          result.details = `匹配规则: ${rule.name}`
          break
        }
      }
      
      setTestResult(result)
    } catch (error) {
      message.error('测试失败')
    }
  }

  // 保存规则 — 规则通过站点WAF配置管理，此处为前端本地状态
  const handleSaveRules = async () => {
    try {
      setLoading(true)
      message.success('规则已更新（前端本地状态）')
    } catch {
      message.error('规则保存失败')
    } finally {
      setLoading(false)
    }
  }

  // 导出规则
  const handleExportRules = () => {
    const dataStr = JSON.stringify(rules, null, 2)
    const blob = new Blob([dataStr], { type: 'application/json' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `waf-rules-${selectedSite}-${new Date().toISOString().split('T')[0]}.json`
    a.click()
    URL.revokeObjectURL(url)
    message.success('规则导出成功')
  }

  // 表格列配置
  const columns = [
    {
      title: '规则名称',
      dataIndex: 'name',
      key: 'name',
      render: (text: string) => <span style={{ fontWeight: 500 }}>{text}</span>,
    },
    {
      title: '匹配字段',
      dataIndex: 'field',
      key: 'field',
      render: (text: string) => <Tag>{text}</Tag>,
    },
    {
      title: '操作符',
      dataIndex: 'operator',
      key: 'operator',
      render: (text: string) => <Tag color="blue">{text}</Tag>,
    },
    {
      title: '匹配值',
      dataIndex: 'value',
      key: 'value',
      ellipsis: true,
      render: (text: string) => <code>{text}</code>,
    },
    {
      title: '动作',
      dataIndex: 'action',
      key: 'action',
      render: (text: string) => {
        const colorMap: Record<string, string> = {
          block: 'red',
          allow: 'green',
          log: 'blue',
        }
        return <Tag color={colorMap[text] || 'default'}>{text}</Tag>
      },
    },
    {
      title: '状态',
      dataIndex: 'enabled',
      key: 'enabled',
      render: (enabled: boolean, record: Rule) => (
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
      render: (_: any, record: Rule) => (
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

  return (
    <div>
      <h1 className="page-title">WAF 规则管理</h1>
      
      {/* 操作栏 */}
      <Card className="card" style={{ marginBottom: 16 }}>
        <Row justify="space-between" align="middle">
          <Col>
            <Space>
              <Select
                value={selectedSite}
                onChange={setSelectedSite}
                style={{ width: 200 }}
                loading={sites.length === 0}
                placeholder="请选择站点"
              >
                {sites.map((site) => (
                  <Option key={site.id} value={site.id}>
                    {site.name}
                  </Option>
                ))}
              </Select>
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
                icon={<CopyOutlined />}
                onClick={() => setTemplateModalVisible(true)}
              >
                使用模板
              </Button>
              <Button 
                icon={<PlayCircleOutlined />}
                onClick={() => setTestModalVisible(true)}
              >
                测试规则
              </Button>
            </Space>
          </Col>
          <Col>
            <Space>
              <Button 
                icon={<ExportOutlined />}
                onClick={handleExportRules}
              >
                导出规则
              </Button>
              <Button 
                type="primary" 
                icon={<SaveOutlined />}
                onClick={handleSaveRules}
                loading={loading}
              >
                保存规则
              </Button>
              <Button 
                icon={<ReloadOutlined />}
                onClick={fetchRules}
                loading={loading}
              >
                刷新
              </Button>
            </Space>
          </Col>
        </Row>
      </Card>

      {/* 规则统计 */}
      <Row gutter={[16, 16]} style={{ marginBottom: 16 }}>
        <Col span={6}>
          <Card className="card">
            <div style={{ textAlign: 'center' }}>
              <div style={{ fontSize: 24, fontWeight: 'bold', color: '#1890ff' }}>{rules.length}</div>
              <div style={{ color: '#666' }}>总规则数</div>
            </div>
          </Card>
        </Col>
        <Col span={6}>
          <Card className="card">
            <div style={{ textAlign: 'center' }}>
              <div style={{ fontSize: 24, fontWeight: 'bold', color: '#52c41a' }}>
                {rules.filter(r => r.enabled).length}
              </div>
              <div style={{ color: '#666' }}>启用规则</div>
            </div>
          </Card>
        </Col>
        <Col span={6}>
          <Card className="card">
            <div style={{ textAlign: 'center' }}>
              <div style={{ fontSize: 24, fontWeight: 'bold', color: '#ff4d4f' }}>
                {rules.filter(r => r.action === 'block').length}
              </div>
              <div style={{ color: '#666' }}>拦截规则</div>
            </div>
          </Card>
        </Col>
        <Col span={6}>
          <Card className="card">
            <div style={{ textAlign: 'center' }}>
              <div style={{ fontSize: 24, fontWeight: 'bold', color: '#faad14' }}>
                {rules.filter(r => !r.enabled).length}
              </div>
              <div style={{ color: '#666' }}>禁用规则</div>
            </div>
          </Card>
        </Col>
      </Row>

      {/* 规则列表 */}
      <Card className="card" title="规则列表">
        <Table
          columns={columns}
          dataSource={rules}
          rowKey="id"
          loading={loading}
          pagination={{
            showSizeChanger: true,
            showQuickJumper: true,
            showTotal: (total) => `共 ${total} 条规则`,
          }}
        />
      </Card>

      {/* 添加/编辑规则弹窗 */}
      <Modal
        title={editingRule ? '编辑规则' : '添加规则'}
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
          onFinish={editingRule ? handleEditRule : handleAddRule}
        >
          <Form.Item
            name="name"
            label="规则名称"
            rules={[{ required: true, message: '请输入规则名称' }]}
          >
            <Input placeholder="例如：SQL 注入防护" />
          </Form.Item>
          
          <Row gutter={16}>
            <Col span={8}>
              <Form.Item
                name="field"
                label="匹配字段"
                rules={[{ required: true, message: '请选择匹配字段' }]}
              >
                <Select placeholder="选择字段">
                  <Option value="query">查询参数</Option>
                  <Option value="path">URL 路径</Option>
                  <Option value="header">请求头</Option>
                  <Option value="body">请求体</Option>
                  <Option value="ip">IP 地址</Option>
                  <Option value="user_agent">User-Agent</Option>
                  <Option value="country">国家/地区</Option>
                </Select>
              </Form.Item>
            </Col>
            <Col span={8}>
              <Form.Item
                name="operator"
                label="操作符"
                rules={[{ required: true, message: '请选择操作符' }]}
              >
                <Select placeholder="选择操作符">
                  <Option value="contains">包含</Option>
                  <Option value="equals">等于</Option>
                  <Option value="matches">正则匹配</Option>
                  <Option value="gt">大于</Option>
                  <Option value="lt">小于</Option>
                  <Option value="in">在列表中</Option>
                </Select>
              </Form.Item>
            </Col>
            <Col span={8}>
              <Form.Item
                name="action"
                label="动作"
                rules={[{ required: true, message: '请选择动作' }]}
              >
                <Select placeholder="选择动作">
                  <Option value="block">拦截</Option>
                  <Option value="allow">放行</Option>
                  <Option value="log">记录</Option>
                </Select>
              </Form.Item>
            </Col>
          </Row>
          
          <Form.Item
            name="value"
            label="匹配值"
            rules={[{ required: true, message: '请输入匹配值' }]}
            help="多个值用逗号分隔"
          >
            <TextArea rows={3} placeholder="例如：' OR 1=1, UNION SELECT" />
          </Form.Item>
          
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

      {/* 规则模板弹窗 */}
      <Modal
        title="规则模板"
        open={templateModalVisible}
        onCancel={() => setTemplateModalVisible(false)}
        footer={null}
        width={700}
      >
        <div style={{ marginBottom: 16, color: '#666' }}>
          选择一个模板快速添加常用规则
        </div>
        <Row gutter={[16, 16]}>
          {ruleTemplates.map((template) => (
            <Col span={12} key={template.id}>
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
                  <Tag color="blue">{template.category}</Tag>
                  <Tag>{template.rules.length} 条规则</Tag>
                </div>
              </Card>
            </Col>
          ))}
        </Row>
      </Modal>

      {/* 测试规则弹窗 */}
      <Modal
        title="测试规则"
        open={testModalVisible}
        onCancel={() => {
          setTestModalVisible(false)
          setTestResult(null)
          testForm.resetFields()
        }}
        footer={null}
        width={600}
      >
        <Form
          form={testForm}
          layout="vertical"
          onFinish={handleTestRule}
        >
          <Row gutter={16}>
            <Col span={12}>
              <Form.Item name="query" label="查询参数">
                <Input placeholder="例如：id=1' OR 1=1" />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="path" label="URL 路径">
                <Input placeholder="例如：/api/users" />
              </Form.Item>
            </Col>
          </Row>
          <Row gutter={16}>
            <Col span={12}>
              <Form.Item name="ip" label="IP 地址">
                <Input placeholder="例如：192.168.1.1" />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="user_agent" label="User-Agent">
                <Input placeholder="例如：Mozilla/5.0..." />
              </Form.Item>
            </Col>
          </Row>
          <Form.Item name="body" label="请求体">
            <TextArea rows={3} placeholder="请求体内容" />
          </Form.Item>
          <Form.Item>
            <Space>
              <Button type="primary" htmlType="submit">
                测试
              </Button>
              <Button onClick={() => {
                setTestResult(null)
                testForm.resetFields()
              }}>
                重置
              </Button>
            </Space>
          </Form.Item>
        </Form>
        
        {testResult && (
          <Card 
            title="测试结果" 
            style={{ marginTop: 16 }}
            className={testResult.matched ? 'card-warning' : 'card-success'}
          >
            <Row gutter={16}>
              <Col span={8}>
                <div style={{ textAlign: 'center' }}>
                  <div style={{ fontSize: 24, fontWeight: 'bold', color: testResult.matched ? '#ff4d4f' : '#52c41a' }}>
                    {testResult.matched ? '已匹配' : '未匹配'}
                  </div>
                  <div style={{ color: '#666' }}>匹配状态</div>
                </div>
              </Col>
              <Col span={8}>
                <div style={{ textAlign: 'center' }}>
                  <div style={{ fontSize: 24, fontWeight: 'bold', color: testResult.action === 'block' ? '#ff4d4f' : '#52c41a' }}>
                    {testResult.action}
                  </div>
                  <div style={{ color: '#666' }}>执行动作</div>
                </div>
              </Col>
              <Col span={8}>
                <div style={{ textAlign: 'center' }}>
                  <div style={{ fontSize: 14, color: '#666' }}>
                    {testResult.rule ? testResult.rule.name : '-'}
                  </div>
                  <div style={{ color: '#666' }}>匹配规则</div>
                </div>
              </Col>
            </Row>
            <div style={{ marginTop: 16, padding: 12, background: '#f5f5f5', borderRadius: 4 }}>
              <div style={{ color: '#666' }}>{testResult.details}</div>
            </div>
          </Card>
        )}
      </Modal>
    </div>
  )
}

export default FirewallRules
