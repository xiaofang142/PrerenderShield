import React, { useState, useEffect } from 'react'
import { Card, Form, InputNumber, Button, message, Spin, Typography, Divider, Row, Col } from 'antd'
import { SaveOutlined, SettingOutlined } from '@ant-design/icons'
import { systemApi } from '../../services/api'
import { useTranslation } from 'react-i18next'

const { Title, Text } = Typography

const SystemConfig: React.FC = () => {
  const { t } = useTranslation()
  const [loading, setLoading] = useState(false)
  const [saving, setSaving] = useState(false)
  const [form] = Form.useForm()

  useEffect(() => {
    fetchConfig()
  }, [])

  const fetchConfig = async () => {
    setLoading(true)
    try {
      const response = await systemApi.getConfig()
      if (response.code === 200) {
        const config = response.data
        form.setFieldsValue({
          access_log_retention_days: parseInt(config.access_log_retention_days || '7'),
          access_log_max_size: parseInt(config.access_log_max_size || '128'),
          crawler_log_retention_days: parseInt(config.crawler_log_retention_days || '7'),
          crawler_log_max_size: parseInt(config.crawler_log_max_size || '128'),
        })
      }
    } catch (error) {
      console.error('Failed to fetch system config:', error)
      message.error(t('system.messages.fetchFailed'))
    } finally {
      setLoading(false)
    }
  }

  const handleSave = async (values: any) => {
    setSaving(true)
    try {
      const config = {
        access_log_retention_days: values.access_log_retention_days.toString(),
        access_log_max_size: values.access_log_max_size.toString(),
        crawler_log_retention_days: values.crawler_log_retention_days.toString(),
        crawler_log_max_size: values.crawler_log_max_size.toString(),
      }

      const response = await systemApi.updateConfig(config)
      if (response.code === 200) {
        message.success(t('system.messages.updateSuccess'))
      } else {
        message.error(response.message || t('system.messages.updateFailed'))
      }
    } catch (error) {
      console.error('Failed to update system config:', error)
      message.error(t('system.messages.updateRequestFailed'))
    } finally {
      setSaving(false)
    }
  }

  return (
    <div className="system-config-container">
      <div style={{ marginBottom: 24, display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
        <div>
          <Title level={2} style={{ margin: 0, color: '#2f855a' }}>{t('system.title')}</Title>
          <Text type="secondary">{t('system.subtitle')}</Text>
        </div>
        <Button 
          type="primary" 
          icon={<SaveOutlined />} 
          onClick={() => form.submit()} 
          loading={saving}
          style={{ background: '#2f855a', borderColor: '#2f855a' }}
        >
          {t('system.saveConfig')}
        </Button>
      </div>

      <Spin spinning={loading}>
        <Card bordered={false} style={{ boxShadow: '0 2px 8px rgba(0,0,0,0.08)' }}>
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
            <div style={{ display: 'flex', alignItems: 'center', marginBottom: 16 }}>
              <SettingOutlined style={{ fontSize: 20, color: '#2f855a', marginRight: 8 }} />
              <Title level={4} style={{ margin: 0 }}>{t('system.logPolicyTitle')}</Title>
            </div>
            <Divider style={{ margin: '12px 0 24px' }} />

            <Row gutter={24}>
              <Col span={12}>
                <Card title={t('system.accessLogCard')} bordered={true} size="small">
                  <Form.Item
                    name="access_log_retention_days"
                    label={t('system.retentionDays')}
                    help={t('system.accessRetentionHelp')}
                  >
                    <InputNumber min={1} max={365} addonAfter={t('system.dayUnit')} style={{ width: '100%' }} />
                  </Form.Item>

                  <Form.Item
                    name="access_log_max_size"
                    label={t('system.maxSize')}
                    help={t('system.maxSizeHelp')}
                  >
                    <InputNumber min={1} max={10240} addonAfter="MB" style={{ width: '100%' }} />
                  </Form.Item>
                </Card>
              </Col>
              
              <Col span={12}>
                <Card title={t('system.crawlerLogCard')} bordered={true} size="small">
                  <Form.Item
                    name="crawler_log_retention_days"
                    label={t('system.retentionDays')}
                    help={t('system.crawlerRetentionHelp')}
                  >
                    <InputNumber min={1} max={365} addonAfter={t('system.dayUnit')} style={{ width: '100%' }} />
                  </Form.Item>

                  <Form.Item
                    name="crawler_log_max_size"
                    label={t('system.maxSize')}
                    help={t('system.maxSizeHelp')}
                  >
                    <InputNumber min={1} max={10240} addonAfter="MB" style={{ width: '100%' }} />
                  </Form.Item>
                </Card>
              </Col>
            </Row>

          </Form>
        </Card>
      </Spin>
    </div>
  )
}

export default SystemConfig
