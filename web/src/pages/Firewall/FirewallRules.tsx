import React, { useState, useEffect, useRef } from 'react'
import { useTranslation } from 'react-i18next'
import { Card, Table, Button, Modal, Form, Input, Select, Space, Tag, message, Row, Col, Tooltip, Popconfirm, Switch } from 'antd'
import { 
  PlusOutlined, 
  ReloadOutlined, 
  DeleteOutlined, 
  EditOutlined,
  CopyOutlined,
  SaveOutlined,
  PlayCircleOutlined,
  ExportOutlined
} from '@ant-design/icons'
import { firewallApi } from '../../services/api'
import { useSites } from '../../hooks/useSites'

const { Option } = Select
const { TextArea } = Input

// 规则模板（名称/描述/分类文案经 i18n 解析）
const ruleTemplates = [
  {
    id: 'sql_injection',
    category: 'injection',
    rules: [
      { field: 'query', operator: 'contains', value: "' OR ", action: 'block' },
      { field: 'query', operator: 'contains', value: "UNION SELECT", action: 'block' },
      { field: 'body', operator: 'regex', value: "(?i)(?:union\\s+select|select\\s+\\*\\s+from)", action: 'block' },
    ]
  },
  {
    id: 'xss_protection',
    category: 'script',
    rules: [
      { field: 'query', operator: 'contains', value: '<script', action: 'block' },
      { field: 'body', operator: 'contains', value: 'javascript:', action: 'block' },
      { field: 'header', operator: 'contains', value: 'onerror=', action: 'block' },
    ]
  },
  {
    id: 'path_traversal',
    category: 'path',
    rules: [
      { field: 'path', operator: 'contains', value: '../', action: 'block' },
      { field: 'path', operator: 'contains', value: '..\\', action: 'block' },
      { field: 'path', operator: 'matches', value: '^/etc/passwd', action: 'block' },
    ]
  },
  {
    id: 'rate_limit',
    category: 'access',
    rules: [
      { field: 'ip', operator: 'count', value: '100', window: '60s', action: 'block' },
    ]
  },
  {
    id: 'geo_block',
    category: 'access',
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
  const { t } = useTranslation()
  const { sites, selectedSiteId: selectedSite, setSelectedSiteId: setSelectedSite } = useSites({
    autoSelectFirst: true,
    onFetchError: (msg) => message.error(msg),
  })
  const [rules, setRules] = useState<Rule[]>([])
  const [loading, setLoading] = useState(false)
  const [ruleModalVisible, setRuleModalVisible] = useState(false)
  const [templateModalVisible, setTemplateModalVisible] = useState(false)
  const [testModalVisible, setTestModalVisible] = useState(false)
  const [editingRule, setEditingRule] = useState<Rule | null>(null)
  const [testResult, setTestResult] = useState<any>(null)
  // 竞态防护：站点快速切换时，旧请求的响应不再写入 state
  const requestVersionRef = useRef(0)
  
  const [form] = Form.useForm()
  const [testForm] = Form.useForm()

  // 获取规则列表
  const fetchRules = async () => {
    if (!selectedSite) return

    const version = ++requestVersionRef.current
    try {
      setLoading(true)
      const res = await firewallApi.getFirewallRules(selectedSite)
      if (version !== requestVersionRef.current) return
      if (res.code === 200 && res.data) {
        setRules(res.data.rules || [])
      }
    } catch (error) {
      console.error('Failed to fetch rules:', error)
      message.error(t('firewallRules.messages.fetchFail'))
    } finally {
      setLoading(false)
    }
  }

  // 初始化
  useEffect(() => {
    // 站点列表由 useSites 自动加载
  }, [])

  useEffect(() => {
    if (selectedSite) {
      fetchRules()
    }
  }, [selectedSite])

  // 添加/编辑规则统一处理（原两份同构函数收敛）
  const handleSaveRule = async (values: any) => {
    const isEdit = Boolean(editingRule)
    try {
      // 编辑提交前校验目标规则仍在当前站点列表中（防切换站点后的静默无效写回）
      if (isEdit && !rules.some(r => r.id === editingRule!.id)) {
        message.warning(t('firewallRules.messages.ruleGone'))
        setRuleModalVisible(false)
        setEditingRule(null)
        return
      }
      const updatedRules: Rule[] = isEdit
        ? rules.map(r => (r.id === editingRule!.id ? { ...r, ...values } : r))
        : [
            ...rules,
            {
              id: `rule-${Date.now()}`,
              name: values.name,
              field: values.field,
              operator: values.operator,
              value: values.value,
              action: values.action,
              enabled: true,
              priority: rules.length + 1,
            },
          ]

      const res = await firewallApi.saveFirewallRules(selectedSite, updatedRules)
      if (res.code === 200) {
        setRules(updatedRules)
        setRuleModalVisible(false)
        setEditingRule(null)
        form.resetFields()
        message.success(t(isEdit ? 'firewallRules.messages.updateSuccess' : 'firewallRules.messages.addSuccess'))
      } else {
        message.error(res.message || t(isEdit ? 'firewallRules.messages.updateFail' : 'firewallRules.messages.addFail'))
      }
    } catch (error) {
      message.error(t(isEdit ? 'firewallRules.messages.updateFail' : 'firewallRules.messages.addFail'))
    }
  }

  // 删除规则
  const handleDeleteRule = async (ruleId: string) => {
    try {
      const res = await firewallApi.deleteFirewallRule(selectedSite, ruleId)
      if (res.code === 200) {
        setRules(rules.filter(r => r.id !== ruleId))
        message.success(t('firewallRules.messages.deleteSuccess'))
      } else {
        message.error(res.message || t('firewallRules.messages.deleteFail'))
      }
    } catch (error) {
      message.error(t('firewallRules.messages.deleteFail'))
    }
  }

  // 切换规则状态
  const handleToggleRule = async (ruleId: string, enabled: boolean) => {
    try {
      const updatedRules = rules.map(r => 
        r.id === ruleId ? { ...r, enabled } : r
      )
      const res = await firewallApi.saveFirewallRules(selectedSite, updatedRules)
      if (res.code === 200) {
        setRules(updatedRules)
      } else {
        message.error(t('firewallRules.messages.statusUpdateFail'))
      }
    } catch (error) {
      message.error(t('firewallRules.messages.statusUpdateFail'))
    }
  }

  // 应用模板
  const handleApplyTemplate = async (template: any) => {
    const templateName = t(`firewallRules.templates.items.${template.id}.name`)
    const newRules = template.rules.map((r: any, index: number) => ({
      id: `rule-${Date.now()}-${index}`,
      name: `${templateName} - ${r.field}`,
      field: r.field,
      operator: r.operator,
      value: r.value,
      action: r.action,
      enabled: true,
      priority: rules.length + index + 1,
    }))
    
    try {
      const updatedRules = [...rules, ...newRules]
      const res = await firewallApi.saveFirewallRules(selectedSite, updatedRules)
      if (res.code === 200) {
        setRules(updatedRules)
        setTemplateModalVisible(false)
        message.success(t('firewallRules.messages.applyTemplateSuccess', { name: templateName }))
      } else {
        message.error(res.message || t('firewallRules.messages.applyTemplateFail'))
      }
    } catch (error) {
      message.error(t('firewallRules.messages.applyTemplateFail'))
    }
  }

  // 测试规则
  const handleTestRule = async (values: any) => {
    try {
      // 模拟测试结果
      const result: {
        matched: boolean
        rule: Rule | null
        action: string
        details: string
      } = {
        matched: false,
        rule: null,
        action: 'allow',
        details: t('firewallRules.test.result.noMatch'),
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
          result.details = t('firewallRules.test.result.detail', { name: rule.name })
          break
        }
      }
      
      setTestResult(result)
    } catch (error) {
      message.error(t('firewallRules.messages.testFail'))
    }
  }

  // 保存规则
  const handleSaveRules = async () => {
    try {
      setLoading(true)
      const res = await firewallApi.saveFirewallRules(selectedSite, rules)
      if (res.code === 200) {
        message.success(t('firewallRules.messages.saveSuccess'))
      } else {
        message.error(res.message || t('firewallRules.messages.saveFail'))
      }
    } catch {
      message.error(t('firewallRules.messages.saveFail'))
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
    message.success(t('firewallRules.messages.exportSuccess'))
  }

  // 表格列配置
  const columns = [
    {
      title: t('firewallRules.columns.name'),
      dataIndex: 'name',
      key: 'name',
      render: (text: string) => <span style={{ fontWeight: 500 }}>{text}</span>,
    },
    {
      title: t('firewallRules.columns.field'),
      dataIndex: 'field',
      key: 'field',
      render: (text: string) => <Tag>{text}</Tag>,
    },
    {
      title: t('firewallRules.columns.operator'),
      dataIndex: 'operator',
      key: 'operator',
      render: (text: string) => <Tag color="blue">{text}</Tag>,
    },
    {
      title: t('firewallRules.columns.value'),
      dataIndex: 'value',
      key: 'value',
      ellipsis: true,
      render: (text: string) => <code>{text}</code>,
    },
    {
      title: t('firewallRules.columns.action'),
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
      title: t('firewallRules.columns.status'),
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
      title: t('firewallRules.columns.actions'),
      key: 'action',
      render: (_: any, record: Rule) => (
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
            title={t('firewallRules.deleteConfirm')}
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

  return (
    <div>
      <h1 className="page-title">{t('firewallRules.title')}</h1>
      
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
                placeholder={t('firewallRules.selectSite')}
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
                {t('firewallRules.toolbar.addRule')}
              </Button>
              <Button 
                icon={<CopyOutlined />}
                onClick={() => setTemplateModalVisible(true)}
              >
                {t('firewallRules.toolbar.useTemplate')}
              </Button>
              <Button 
                icon={<PlayCircleOutlined />}
                onClick={() => setTestModalVisible(true)}
              >
                {t('firewallRules.toolbar.testRule')}
              </Button>
            </Space>
          </Col>
          <Col>
            <Space>
              <Button 
                icon={<ExportOutlined />}
                onClick={handleExportRules}
              >
                {t('firewallRules.toolbar.exportRules')}
              </Button>
              <Button 
                type="primary" 
                icon={<SaveOutlined />}
                onClick={handleSaveRules}
                loading={loading}
              >
                {t('firewallRules.toolbar.saveRules')}
              </Button>
              <Button 
                icon={<ReloadOutlined />}
                onClick={fetchRules}
                loading={loading}
              >
                {t('firewallRules.toolbar.refresh')}
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
              <div style={{ color: '#666' }}>{t('firewallRules.stats.total')}</div>
            </div>
          </Card>
        </Col>
        <Col span={6}>
          <Card className="card">
            <div style={{ textAlign: 'center' }}>
              <div style={{ fontSize: 24, fontWeight: 'bold', color: '#52c41a' }}>
                {rules.filter(r => r.enabled).length}
              </div>
              <div style={{ color: '#666' }}>{t('firewallRules.stats.enabled')}</div>
            </div>
          </Card>
        </Col>
        <Col span={6}>
          <Card className="card">
            <div style={{ textAlign: 'center' }}>
              <div style={{ fontSize: 24, fontWeight: 'bold', color: '#ff4d4f' }}>
                {rules.filter(r => r.action === 'block').length}
              </div>
              <div style={{ color: '#666' }}>{t('firewallRules.stats.blocked')}</div>
            </div>
          </Card>
        </Col>
        <Col span={6}>
          <Card className="card">
            <div style={{ textAlign: 'center' }}>
              <div style={{ fontSize: 24, fontWeight: 'bold', color: '#faad14' }}>
                {rules.filter(r => !r.enabled).length}
              </div>
              <div style={{ color: '#666' }}>{t('firewallRules.stats.disabled')}</div>
            </div>
          </Card>
        </Col>
      </Row>

      {/* 规则列表 */}
      <Card className="card" title={t('firewallRules.list.title')}>
        <Table
          columns={columns}
          dataSource={rules}
          rowKey="id"
          loading={loading}
          pagination={{
            showSizeChanger: true,
            showQuickJumper: true,
            showTotal: (total) => t('firewallRules.list.total', { total }),
          }}
        />
      </Card>

      {/* 添加/编辑规则弹窗 */}
      <Modal
        title={editingRule ? t('firewallRules.form.editTitle') : t('firewallRules.form.addTitle')}
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
        >
          <Form.Item
            name="name"
            label={t('firewallRules.form.ruleName')}
            rules={[{ required: true, message: t('firewallRules.form.ruleNameRequired') }]}
          >
            <Input placeholder={t('firewallRules.form.namePlaceholder')} />
          </Form.Item>
          
          <Row gutter={16}>
            <Col span={8}>
              <Form.Item
                name="field"
                label={t('firewallRules.columns.field')}
                rules={[{ required: true, message: t('firewallRules.form.fieldRequired') }]}
              >
                <Select placeholder={t('firewallRules.form.selectField')}>
                  <Option value="query">{t('firewallRules.form.fieldOptions.query')}</Option>
                  <Option value="path">{t('firewallRules.form.fieldOptions.path')}</Option>
                  <Option value="header">{t('firewallRules.form.fieldOptions.header')}</Option>
                  <Option value="body">{t('firewallRules.form.fieldOptions.body')}</Option>
                  <Option value="ip">{t('firewallRules.form.fieldOptions.ip')}</Option>
                  <Option value="user_agent">{t('firewallRules.form.fieldOptions.user_agent')}</Option>
                  <Option value="country">{t('firewallRules.form.fieldOptions.country')}</Option>
                </Select>
              </Form.Item>
            </Col>
            <Col span={8}>
              <Form.Item
                name="operator"
                label={t('firewallRules.columns.operator')}
                rules={[{ required: true, message: t('firewallRules.form.operatorRequired') }]}
              >
                <Select placeholder={t('firewallRules.form.selectOperator')}>
                  <Option value="contains">{t('firewallRules.form.operatorOptions.contains')}</Option>
                  <Option value="equals">{t('firewallRules.form.operatorOptions.equals')}</Option>
                  <Option value="matches">{t('firewallRules.form.operatorOptions.matches')}</Option>
                  <Option value="gt">{t('firewallRules.form.operatorOptions.gt')}</Option>
                  <Option value="lt">{t('firewallRules.form.operatorOptions.lt')}</Option>
                  <Option value="in">{t('firewallRules.form.operatorOptions.in')}</Option>
                </Select>
              </Form.Item>
            </Col>
            <Col span={8}>
              <Form.Item
                name="action"
                label={t('firewallRules.columns.action')}
                rules={[{ required: true, message: t('firewallRules.form.actionRequired') }]}
              >
                <Select placeholder={t('firewallRules.form.selectAction')}>
                  <Option value="block">{t('firewallRules.form.actionOptions.block')}</Option>
                  <Option value="allow">{t('firewallRules.form.actionOptions.allow')}</Option>
                  <Option value="log">{t('firewallRules.form.actionOptions.log')}</Option>
                </Select>
              </Form.Item>
            </Col>
          </Row>
          
          <Form.Item
            name="value"
            label={t('firewallRules.columns.value')}
            rules={[{ required: true, message: t('firewallRules.form.valueRequired') }]}
            help={t('firewallRules.form.valueHelp')}
          >
            <TextArea rows={3} placeholder={t('firewallRules.form.valuePlaceholder')} />
          </Form.Item>
          
          <Form.Item>
            <Space>
              <Button type="primary" htmlType="submit">
                {editingRule ? t('firewallRules.form.updateRule') : t('firewallRules.toolbar.addRule')}
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

      {/* 规则模板弹窗 */}
      <Modal
        title={t('firewallRules.templates.title')}
        open={templateModalVisible}
        onCancel={() => setTemplateModalVisible(false)}
        footer={null}
        width={700}
      >
        <div style={{ marginBottom: 16, color: '#666' }}>
          {t('firewallRules.templates.description')}
        </div>
        <Row gutter={[16, 16]}>
          {ruleTemplates.map((template) => (
            <Col span={12} key={template.id}>
              <Card 
                hoverable
                onClick={() => handleApplyTemplate(template)}
                style={{ cursor: 'pointer' }}
              >
                <div style={{ fontWeight: 'bold', marginBottom: 8 }}>
                  {t(`firewallRules.templates.items.${template.id}.name`)}
                </div>
                <div style={{ color: '#666', fontSize: 12, marginBottom: 8 }}>
                  {t(`firewallRules.templates.items.${template.id}.description`)}
                </div>
                <div>
                  <Tag color="blue">{t(`firewallRules.templates.categories.${template.category}`)}</Tag>
                  <Tag>{t('firewallRules.templates.ruleCount', { count: template.rules.length })}</Tag>
                </div>
              </Card>
            </Col>
          ))}
        </Row>
      </Modal>

      {/* 测试规则弹窗 */}
      <Modal
        title={t('firewallRules.toolbar.testRule')}
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
              <Form.Item name="query" label={t('firewallRules.test.query')}>
                <Input placeholder={t('firewallRules.test.phQuery')} />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="path" label={t('firewallRules.test.path')}>
                <Input placeholder={t('firewallRules.test.phPath')} />
              </Form.Item>
            </Col>
          </Row>
          <Row gutter={16}>
            <Col span={12}>
              <Form.Item name="ip" label={t('firewallRules.test.ip')}>
                <Input placeholder={t('firewallRules.test.phIp')} />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="user_agent" label={t('firewallRules.test.userAgent')}>
                <Input placeholder={t('firewallRules.test.phUserAgent')} />
              </Form.Item>
            </Col>
          </Row>
          <Form.Item name="body" label={t('firewallRules.test.body')}>
            <TextArea rows={3} placeholder={t('firewallRules.test.phBody')} />
          </Form.Item>
          <Form.Item>
            <Space>
              <Button type="primary" htmlType="submit">
                {t('common.ok')}
              </Button>
              <Button onClick={() => {
                setTestResult(null)
                testForm.resetFields()
              }}>
                {t('common.reset')}
              </Button>
            </Space>
          </Form.Item>
        </Form>
        
        {testResult && (
          <Card 
            title={t('firewallRules.test.result.title')} 
            style={{ marginTop: 16 }}
            className={testResult.matched ? 'card-warning' : 'card-success'}
          >
            <Row gutter={16}>
              <Col span={8}>
                <div style={{ textAlign: 'center' }}>
                  <div style={{ fontSize: 24, fontWeight: 'bold', color: testResult.matched ? '#ff4d4f' : '#52c41a' }}>
                    {testResult.matched ? t('firewallRules.test.result.matched') : t('firewallRules.test.result.unmatched')}
                  </div>
                  <div style={{ color: '#666' }}>{t('firewallRules.test.result.matchStatus')}</div>
                </div>
              </Col>
              <Col span={8}>
                <div style={{ textAlign: 'center' }}>
                  <div style={{ fontSize: 24, fontWeight: 'bold', color: testResult.action === 'block' ? '#ff4d4f' : '#52c41a' }}>
                    {testResult.action}
                  </div>
                  <div style={{ color: '#666' }}>{t('firewallRules.test.result.execAction')}</div>
                </div>
              </Col>
              <Col span={8}>
                <div style={{ textAlign: 'center' }}>
                  <div style={{ fontSize: 14, color: '#666' }}>
                    {testResult.rule ? testResult.rule.name : '-'}
                  </div>
                  <div style={{ color: '#666' }}>{t('firewallRules.test.result.matchedRule')}</div>
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
