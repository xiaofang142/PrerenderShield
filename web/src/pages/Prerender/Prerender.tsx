import React, { useState, useEffect, useRef } from 'react'
import { Card, Row, Col, Statistic, Button, Modal, Input, message, Table, Select, Form, InputNumber, Switch, Divider, Space } from 'antd'
import { CodeOutlined, PlayCircleOutlined, FireOutlined, ReloadOutlined, SaveOutlined, SettingOutlined, PlusOutlined, DeleteOutlined } from '@ant-design/icons'
import { prerenderApi, sitesApi, extractErrorMessage } from '../../services/api'
import { useSites } from '../../hooks/useSites'
import { useTranslation } from 'react-i18next'

const { Search } = Input
const { Option } = Select

// 爬虫分类与其渲染策略选项（空=站点默认 render）
const CATEGORY_KEYS = ['search', 'social', 'ai', 'generic'] as const
const CATEGORY_POLICY_OPTIONS = ['render', 'cache_only', 'passthrough'] as const

const Prerender: React.FC = () => {
  const { t } = useTranslation()
  const { sites, selectedSiteId: selectedSite, setSelectedSiteId: setSelectedSite } = useSites({
    autoSelectFirst: true,
    onFetchError: (msg) => message.error(msg),
  })
  const [status, setStatus] = useState({
    enabled: false,
    poolSize: 5,
    timeout: 30,
    cacheTTL: 3600,
    preheat: {
      enabled: false,
      sitemapURL: '',
      schedule: '0 0 * * *',
    },
  })
  const [loading, setLoading] = useState(true)
  const [renderModalVisible, setRenderModalVisible] = useState(false)
  const [preheatModalVisible, setPreheatModalVisible] = useState(false)
  const [renderUrl, setRenderUrl] = useState('')
  const [renderLoading, setRenderLoading] = useState(false)
  const [preheatLoading, setPreheatLoading] = useState(false)
  const [renderHistory, setRenderHistory] = useState<any[]>([])
  // 渲染策略设置表单
  const [policyForm] = Form.useForm()
  const [sitePrerenderConfig, setSitePrerenderConfig] = useState<any>(null)
  const [configLoading, setConfigLoading] = useState(false)
  const [configSaving, setConfigSaving] = useState(false)
  // 竞态防护：站点快速切换时，旧请求的响应不再写入 state
  const requestVersionRef = useRef(0)
  const configVersionRef = useRef(0)

  // 表格列配置
  const columns = [
    {
      title: 'URL',
      dataIndex: 'url',
      key: 'url',
      ellipsis: true,
    },
    {
      title: t('prerender.columns.site'),
      dataIndex: 'site',
      key: 'site',
    },
    {
      title: t('prerender.columns.status'),
      dataIndex: 'status',
      key: 'status',
      render: (text: string) => {
        const color = text === 'success' ? '#52c41a' : '#f5222d'
        return <span style={{ color }}>{text}</span>
      },
    },
    {
      title: t('prerender.columns.duration'),
      dataIndex: 'duration',
      key: 'duration',
    },
    {
      title: t('prerender.columns.time'),
      dataIndex: 'time',
      key: 'time',
    },
  ]

  // 获取渲染预热状态
  const fetchStatus = async () => {
    if (!selectedSite) return

    const version = ++requestVersionRef.current
    try {
      setLoading(true)
      const res = await prerenderApi.getStatus(selectedSite)
      if (version !== requestVersionRef.current) return
      if (res.code === 200) {
        // 处理单站点数据结构
        const statusData = typeof res.data === 'object' && res.data.enabled !== undefined ? res.data : res.data[selectedSite]
        setStatus(statusData)
      }
    } catch (error) {
      console.error('Failed to fetch prerender status:', error)
      message.error(t('prerender.fetchStatusFailed'))
    } finally {
      setLoading(false)
    }
  }

  // 初始化数据
  useEffect(() => {
    // 站点列表由 useSites 自动加载
  }, [])

  // 当选择的站点变化时，重新获取状态
  useEffect(() => {
    if (selectedSite) {
      fetchStatus()
      fetchSiteConfig()
    }
  }, [selectedSite])

  // 渲染策略配置：读取站点完整 prerender 配置（PUT 为整段提交，需保留未编辑字段）
  const fetchSiteConfig = async () => {
    if (!selectedSite) return
    const version = ++configVersionRef.current
    try {
      setConfigLoading(true)
      const res = await sitesApi.getSiteConfig(selectedSite, 'prerender')
      if (version !== configVersionRef.current) return
      if (res.code === 200 && res.data) {
        const cfg = res.data
        setSitePrerenderConfig(cfg)
        const policy = cfg.category_policy || {}
        policyForm.setFieldsValue({
          cache_ttl: cfg.cache_ttl,
          timeout: cfg.timeout,
          max_concurrency: cfg.max_concurrency || 0,
          include_patterns: cfg.include_patterns || [],
          exclude_patterns: cfg.exclude_patterns || [],
          stale_while_revalidate: cfg.stale_while_revalidate === undefined ? true : !!cfg.stale_while_revalidate,
          ttl_rules: cfg.ttl_rules || [],
          policy_search: policy.search || '',
          policy_social: policy.social || '',
          policy_ai: policy.ai || '',
          policy_generic: policy.generic || '',
        })
      }
    } catch (error) {
      console.error('Failed to fetch site prerender config:', error)
      message.error(extractErrorMessage(error))
    } finally {
      if (version === configVersionRef.current) setConfigLoading(false)
    }
  }

  // 保存渲染策略（整段提交完整 PrerenderConfig，避免后端整段绑定清零未编辑字段）
  const handleSaveConfig = async (values: any) => {
    if (!selectedSite || !sitePrerenderConfig) return
    const merged: any = { ...sitePrerenderConfig }
    merged.cache_ttl = values.cache_ttl
    merged.timeout = values.timeout
    merged.max_concurrency = values.max_concurrency || 0
    merged.include_patterns = values.include_patterns || []
    merged.exclude_patterns = values.exclude_patterns || []
    merged.stale_while_revalidate = values.stale_while_revalidate
    merged.ttl_rules = (values.ttl_rules || []).filter((r: any) => r && r.pattern && r.ttl_seconds)
    const policy: Record<string, string> = {}
    for (const key of CATEGORY_KEYS) {
      const v = values[`policy_${key}`]
      if (v) policy[key] = v
    }
    merged.category_policy = policy

    try {
      setConfigSaving(true)
      const res = await sitesApi.updatePrerenderConfig(selectedSite, merged)
      if (res.code === 200) {
        message.success(t('prerender.configSaveSuccess'))
        fetchSiteConfig()
        fetchStatus()
      } else {
        message.error(res.message || t('prerender.configSaveFailed'))
      }
    } catch (error) {
      console.error('Failed to save prerender config:', error)
      message.error(extractErrorMessage(error))
    } finally {
      setConfigSaving(false)
    }
  }

  // 手动触发渲染
  const handleRender = async () => {
    if (!selectedSite) {
      message.warning(t('prerender.selectSiteWarning'))
      return
    }
    
    if (!renderUrl) {
      message.warning(t('prerender.inputUrlWarning'))
      return
    }

    try {
      setRenderLoading(true)
      const startTime = Date.now()
      const res = await prerenderApi.triggerPreheat(selectedSite)
      const duration = Date.now() - startTime
      
      if (res.code === 200) {
        message.success(t('prerender.renderSubmitted'))
        setRenderHistory(prev => [
          {
            site: selectedSite,
            url: renderUrl,
            status: 'success',
            duration,
            time: new Date().toLocaleString(),
          },
          ...prev.slice(0, 9),
        ])
        setRenderModalVisible(false)
        setRenderUrl('')
      } else {
        message.error(t('prerender.renderSubmitFailed'))
      }
    } catch (error) {
      console.error('Failed to render:', error)
      message.error(t('prerender.renderFailed'))
    } finally {
      setRenderLoading(false)
    }
  }

  // 触发缓存预热
  const handlePreheat = async () => {
    if (!selectedSite) {
      message.warning(t('prerender.selectSiteWarning'))
      return
    }

    try {
      setPreheatLoading(true)
      const res = await prerenderApi.triggerPreheat(selectedSite)
      if (res.code === 200) {
        message.success(t('prerender.preheatTriggered'))
        setPreheatModalVisible(false)
      } else {
        message.error(t('prerender.preheatTriggerFailed'))
      }
    } catch (error) {
      console.error('Failed to trigger preheat:', error)
      message.error(t('prerender.preheatTriggerFailed'))
    } finally {
      setPreheatLoading(false)
    }
  }

  return (
    <div>
      <h1 className="page-title">{t('prerender.title')}</h1>
      
      {/* 站点选择器 */}
      <Card className="card" style={{ marginBottom: 16 }}>
        <Row align="middle">
          <Col span={8}>
            <label style={{ marginRight: 8 }}>{t('prerender.selectSite')}</label>
            <Select
              value={selectedSite}
              onChange={setSelectedSite}
              style={{ width: 200 }}
              loading={sites.length === 0}
            >
              {sites.map((site) => (
                <Option key={site.id} value={site.id}>
                  {site.name} ({site.domains?.[0] || site.id})
                </Option>
              ))}
            </Select>
          </Col>
          <Col span={8}>
            <Button type="primary" icon={<ReloadOutlined />} onClick={fetchStatus} loading={loading}>
              {t('prerender.refreshStatus')}
            </Button>
          </Col>
        </Row>
      </Card>
      
      {/* 渲染预热状态卡片 */}
      <Card className="card">
        <Row gutter={[16, 16]}>
          <Col span={6}>
            <Statistic
              title={t('prerender.statusTitle')}
              value={status.enabled ? t('prerender.enabled') : t('prerender.disabled')}
              prefix={<CodeOutlined />}
              valueStyle={{ color: status.enabled ? '#52c41a' : '#faad14' }}
            />
          </Col>
          <Col span={6}>
            <Statistic
              title={t('prerender.browserPoolSize')}
              value={status.poolSize}
              valueStyle={{ color: '#1890ff' }}
            />
          </Col>
          <Col span={6}>
            <Statistic
              title={t('prerender.renderTimeout')}
              value={status.timeout}
              valueStyle={{ color: '#faad14' }}
            />
          </Col>
          <Col span={6}>
            <Statistic
              title={t('prerender.cacheTTL')}
              value={status.cacheTTL}
              valueStyle={{ color: '#52c41a' }}
            />
          </Col>
        </Row>
      </Card>

      {/* 渲染策略设置 */}
      <Card
        className="card"
        title={<Space><SettingOutlined />{t('prerender.policyTitle')}</Space>}
        style={{ marginBottom: 16 }}
      >
        <Form
          form={policyForm}
          layout="vertical"
          onFinish={handleSaveConfig}
          disabled={configLoading}
        >
          <Divider orientation="left" plain>{t('prerender.policyBasic')}</Divider>
          <Row gutter={24}>
            <Col span={6}>
              <Form.Item name="cache_ttl" label={t('prerender.policyCacheTTL')} help={t('prerender.policyCacheTTLHelp')} rules={[{ required: true, message: t('prerender.policyRequired') }]}>
                <InputNumber min={0} max={604800} addonAfter="s" style={{ width: '100%' }} />
              </Form.Item>
            </Col>
            <Col span={6}>
              <Form.Item name="timeout" label={t('prerender.policyTimeout')} help={t('prerender.policyTimeoutHelp')} rules={[{ required: true, message: t('prerender.policyRequired') }]}>
                <InputNumber min={1} max={300} addonAfter="s" style={{ width: '100%' }} />
              </Form.Item>
            </Col>
            <Col span={6}>
              <Form.Item name="max_concurrency" label={t('prerender.policyMaxConcurrency')} help={t('prerender.policyMaxConcurrencyHelp')}>
                <InputNumber min={0} max={100} style={{ width: '100%' }} />
              </Form.Item>
            </Col>
            <Col span={6}>
              <Form.Item name="stale_while_revalidate" label={t('prerender.policyStale')} valuePropName="checked" help={t('prerender.policyStaleHelp')}>
                <Switch />
              </Form.Item>
            </Col>
          </Row>

          <Divider orientation="left" plain>{t('prerender.policyPatterns')}</Divider>
          <Row gutter={24}>
            <Col span={12}>
              <Form.Item name="include_patterns" label={t('prerender.policyInclude')} help={t('prerender.policyIncludeHelp')}>
                <Select mode="tags" open={false} tokenSeparators={[',']} placeholder={t('prerender.policyPatternsPlaceholder')} />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="exclude_patterns" label={t('prerender.policyExclude')} help={t('prerender.policyExcludeHelp')}>
                <Select mode="tags" open={false} tokenSeparators={[',']} placeholder={t('prerender.policyPatternsPlaceholder')} />
              </Form.Item>
            </Col>
          </Row>

          <Divider orientation="left" plain>{t('prerender.policyTtlRules')}</Divider>
          <Form.List name="ttl_rules">
            {(fields, { add, remove }) => (
              <>
                {fields.map(({ key, name, ...restField }) => (
                  <Row gutter={16} key={key} align="middle">
                    <Col span={14}>
                      <Form.Item
                        {...restField}
                        name={[name, 'pattern']}
                        label={name === 0 ? t('prerender.policyTtlPattern') : ''}
                        rules={[{ required: true, message: t('prerender.policyTtlPatternRequired') }]}
                      >
                        <Input placeholder={t('prerender.policyTtlPatternPlaceholder')} maxLength={200} />
                      </Form.Item>
                    </Col>
                    <Col span={7}>
                      <Form.Item
                        {...restField}
                        name={[name, 'ttl_seconds']}
                        label={name === 0 ? t('prerender.policyTtlSeconds') : ''}
                        rules={[{ required: true, message: t('prerender.policyTtlSecondsRequired') }]}
                      >
                        <InputNumber min={60} max={2592000} addonAfter="s" style={{ width: '100%' }} />
                      </Form.Item>
                    </Col>
                    <Col span={3}>
                      <Button type="text" danger icon={<DeleteOutlined />} onClick={() => remove(name)} />
                    </Col>
                  </Row>
                ))}
                <Form.Item>
                  <Button type="dashed" icon={<PlusOutlined />} onClick={() => add({ pattern: '', ttl_seconds: 3600 })}>
                    {t('prerender.policyTtlAdd')}
                  </Button>
                  <span style={{ marginLeft: 12, color: 'var(--text-tertiary, #999)', fontSize: 12 }}>{t('prerender.policyTtlHelp')}</span>
                </Form.Item>
              </>
            )}
          </Form.List>

          <Divider orientation="left" plain>{t('prerender.policyCategory')}</Divider>
          <Row gutter={24}>
            {CATEGORY_KEYS.map((key) => (
              <Col span={6} key={key}>
                <Form.Item name={`policy_${key}`} label={t(`prerender.category_${key}`)}>
                  <Select allowClear placeholder={t('prerender.policyCategoryDefault')}>
                    {CATEGORY_POLICY_OPTIONS.map((opt) => (
                      <Option key={opt} value={opt}>{t(`prerender.policy_${opt}`)}</Option>
                    ))}
                  </Select>
                </Form.Item>
              </Col>
            ))}
          </Row>

          <Form.Item>
            <Button type="primary" htmlType="submit" icon={<SaveOutlined />} loading={configSaving}>
              {t('prerender.policySave')}
            </Button>
          </Form.Item>
        </Form>
      </Card>

      {/* 操作按钮 */}
      <Row gutter={[16, 16]} style={{ marginBottom: 16 }}>
        <Col span={12}>
          <Card className="card" title={t('prerender.manualOps')}>
            <Row gutter={[16, 16]}>
              <Col span={12}>
                <Button
                  type="primary"
                  icon={<PlayCircleOutlined />}
                  onClick={() => setRenderModalVisible(true)}
                  block
                >
                  {t('prerender.manualRender')}
                </Button>
              </Col>
              <Col span={12}>
                <Button
                  type="default"
                  icon={<FireOutlined />}
                  onClick={() => setPreheatModalVisible(true)}
                  block
                >
                  {t('prerender.cachePreheat')}
                </Button>
              </Col>
            </Row>
          </Card>
        </Col>
        <Col span={12}>
          <Card className="card" title={t('prerender.preheatConfig')}>
            <Row gutter={[16, 16]}>
              <Col span={24}>
                <Statistic
                  title={t('prerender.preheatStatus')}
                  value={status.preheat.enabled ? t('prerender.enabled') : t('prerender.disabled')}
                  valueStyle={{ color: status.preheat.enabled ? '#52c41a' : '#faad14' }}
                />
              </Col>
              <Col span={24}>
                <Statistic
                  title={t('prerender.preheatSchedule')}
                  value={status.preheat.schedule}
                  valueStyle={{ color: '#1890ff' }}
                />
              </Col>
            </Row>
          </Card>
        </Col>
      </Row>

      {/* 渲染历史 */}
      <Card className="card" title={t('prerender.renderHistory')}>
        <Table
          columns={columns}
          dataSource={renderHistory}
          rowKey={(_record, index) => (index || 0).toString()}
          pagination={{ pageSize: 10 }}
        />
      </Card>

      {/* 手动渲染模态框 */}
      <Modal
        title={t('prerender.manualRender')}
        open={renderModalVisible}
        onOk={handleRender}
        onCancel={() => setRenderModalVisible(false)}
        confirmLoading={renderLoading}
        okText={t('prerender.startRender')}
        cancelText={t('common.cancel')}
      >
        <div style={{ marginBottom: 16 }}>
          <label style={{ display: 'block', marginBottom: 8 }}>{t('prerender.urlLabel')}</label>
          <Search
            placeholder={t('prerender.urlPlaceholder')}
            allowClear
            value={renderUrl}
            onChange={(e) => setRenderUrl(e.target.value)}
            onPressEnter={handleRender}
            style={{ width: '100%' }}
          />
        </div>
        <p style={{ color: '#666', fontSize: 12 }}>
          {t('prerender.renderNote')}
        </p>
      </Modal>

      {/* 缓存预热模态框 */}
      <Modal
        title={t('prerender.cachePreheat')}
        open={preheatModalVisible}
        onOk={handlePreheat}
        onCancel={() => setPreheatModalVisible(false)}
        confirmLoading={preheatLoading}
        okText={t('prerender.startPreheat')}
        cancelText={t('common.cancel')}
      >
        <p>{t('prerender.preheatConfirm')}</p>
        <p style={{ color: '#666', fontSize: 12, marginTop: 16 }}>
          {t('prerender.preheatNote')}
        </p>
      </Modal>
    </div>
  )
}

export default Prerender