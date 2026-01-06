import React, { useState, useEffect } from 'react'
import { Card, Form, InputNumber, Button, message, Spin, Typography, Divider, Row, Col } from 'antd'
import { SaveOutlined, SettingOutlined } from '@ant-design/icons'
import { systemApi } from '../../services/api'

const { Title, Text } = Typography

const SystemConfig: React.FC = () => {
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
      message.error('获取系统配置失败')
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
        message.success('系统配置已更新')
      } else {
        message.error(response.message || '更新失败')
      }
    } catch (error) {
      console.error('Failed to update system config:', error)
      message.error('更新系统配置失败')
    } finally {
      setSaving(false)
    }
  }

  return (
    <div className="system-config-container">
      <div style={{ marginBottom: 24, display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
        <div>
          <Title level={2} style={{ margin: 0, color: '#2f855a' }}>系统设置</Title>
          <Text type="secondary">管理系统的全局配置参数</Text>
        </div>
        <Button 
          type="primary" 
          icon={<SaveOutlined />} 
          onClick={() => form.submit()} 
          loading={saving}
          style={{ background: '#2f855a', borderColor: '#2f855a' }}
        >
          保存配置
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
              <Title level={4} style={{ margin: 0 }}>日志保留策略</Title>
            </div>
            <Divider style={{ margin: '12px 0 24px' }} />

            <Row gutter={24}>
              <Col span={12}>
                <Card title="访问日志 (Access Logs)" bordered={true} size="small">
                  <Form.Item
                    name="access_log_retention_days"
                    label="保留天数"
                    help="超过该天数的访问日志将被自动删除"
                  >
                    <InputNumber min={1} max={365} addonAfter="天" style={{ width: '100%' }} />
                  </Form.Item>

                  <Form.Item
                    name="access_log_max_size"
                    label="最大占用空间"
                    help="当日志总大小超过该值时，将自动删除最早的日志"
                  >
                    <InputNumber min={1} max={10240} addonAfter="MB" style={{ width: '100%' }} />
                  </Form.Item>
                </Card>
              </Col>
              
              <Col span={12}>
                <Card title="爬虫日志 (Crawler Logs)" bordered={true} size="small">
                  <Form.Item
                    name="crawler_log_retention_days"
                    label="保留天数"
                    help="超过该天数的爬虫日志将被自动删除"
                  >
                    <InputNumber min={1} max={365} addonAfter="天" style={{ width: '100%' }} />
                  </Form.Item>

                  <Form.Item
                    name="crawler_log_max_size"
                    label="最大占用空间"
                    help="当日志总大小超过该值时，将自动删除最早的日志"
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
