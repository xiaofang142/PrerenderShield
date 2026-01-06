import React, { useState, useEffect } from 'react'
import { Card, Form, InputNumber, Switch, Button, message, Spin, Typography, Divider } from 'antd'
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
        // Convert string values to appropriate types for the form
        const config = response.data
        form.setFieldsValue({
          max_users: parseInt(config.max_users || '1'),
          allow_registration: config.allow_registration === 'true',
          maintenance_mode: config.maintenance_mode === 'true',
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
      // Convert values back to strings for the API
      const config = {
        max_users: values.max_users.toString(),
        allow_registration: values.allow_registration.toString(),
        maintenance_mode: values.maintenance_mode.toString(),
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
              max_users: 1,
              allow_registration: false,
              maintenance_mode: false,
            }}
          >
            <div style={{ display: 'flex', alignItems: 'center', marginBottom: 16 }}>
              <SettingOutlined style={{ fontSize: 20, color: '#2f855a', marginRight: 8 }} />
              <Title level={4} style={{ margin: 0 }}>基础配置</Title>
            </div>
            <Divider style={{ margin: '12px 0 24px' }} />

            <Form.Item
              name="max_users"
              label="最大用户数"
              help="系统允许注册的最大管理员数量（当前架构建议保持为 1）"
            >
              <InputNumber min={1} max={100} style={{ width: 200 }} />
            </Form.Item>

            <Form.Item
              name="allow_registration"
              label="允许注册"
              valuePropName="checked"
              help="是否允许新用户注册（建议仅在初始化时开启）"
            >
              <Switch />
            </Form.Item>

            <Form.Item
              name="maintenance_mode"
              label="维护模式"
              valuePropName="checked"
              help="开启后，除管理员外的所有访问将被拦截并显示维护页面"
            >
              <Switch checkedChildren="开启" unCheckedChildren="关闭" />
            </Form.Item>
          </Form>
        </Card>
      </Spin>
    </div>
  )
}

export default SystemConfig
