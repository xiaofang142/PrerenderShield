import React, { useState, useEffect } from 'react'
import { Card, Form, Input, Button, Typography, Modal, message, Alert, Dropdown } from 'antd'
import { LoginOutlined, LockOutlined, InfoCircleOutlined, GlobalOutlined } from '@ant-design/icons'
import { useNavigate } from 'react-router-dom'
import { useAuth } from '../../context/AuthContext'
import api from '../../services/api'
import { useTranslation } from 'react-i18next'

const { Title, Paragraph } = Typography

const Login: React.FC = () => {
  const { t, i18n } = useTranslation()
  const [loading, setLoading] = useState(false)
  // Modal状态
  const [modalVisible, setModalVisible] = useState(false)
  const [modalTitle, setModalTitle] = useState('')
  const [modalContent, setModalContent] = useState('')
  const [modalType, setModalType] = useState<'success' | 'error' | 'info'>('info')
  // 首次运行状态
  const [isFirstRun, setIsFirstRun] = useState(false)
  const [checkingFirstRun, setCheckingFirstRun] = useState(true)
  const navigate = useNavigate()
  const { login: authLogin } = useAuth()

  // 语言切换菜单项
  const langItems = [
    { key: 'zh', label: '简体中文' },
    { key: 'en', label: 'English' },
    { key: 'ar', label: 'العربية' },
    { key: 'fr', label: 'Français' },
    { key: 'ru', label: 'Русский' },
    { key: 'es', label: 'Español' },
  ]

  const handleLangChange = (key: string) => {
    i18n.changeLanguage(key)
    message.success(t('common.success'))
  }

  // 显示提示Modal
  const showModal = (type: 'success' | 'error' | 'info', title: string, content: string) => {
    setModalType(type)
    setModalTitle(title)
    setModalContent(content)
    setModalVisible(true)
  }

  // 检查是否是首次运行
  useEffect(() => {
    const checkFirstRun = async () => {
      try {
        const response = await api.get('/auth/first-run')
        if (response.code === 200) {
          setIsFirstRun(response.data.isFirstRun)
        }
      } catch (error) {
        console.error('Check first run status failed:', error)
      } finally {
        setCheckingFirstRun(false)
      }
    }

    checkFirstRun()
  }, [])

  // 登录处理
  const handleLogin = async (values: { username: string; password: string }) => {
    setLoading(true)
    try {
      const response = await api.post('/auth/login', values)
      if (response.code === 200) {
        // 保存token并更新全局状态
        authLogin(response.data.token, response.data.username)
        // 显示成功提示
        showModal('success', t('login.successTitle'), t('login.successContent'))
        // 延迟跳转到首页
        setTimeout(() => {
          navigate('/')
        }, 1500)
      } else {
        // API返回错误，显示错误信息
        showModal('error', t('login.failedTitle'), response.message || t('login.failedDefault'))
      }
    } catch (error: any) {
      // 网络错误或其他错误
      console.error('Login error:', error)
      if (error.response) {
        // 服务器返回了错误响应
        const errorMsg = error.response.data?.message || t('login.failedDefault')
        showModal('error', t('login.failedTitle'), errorMsg)
      } else if (error.request) {
        // 请求已发出，但没有收到响应
        showModal('error', t('login.failedNetwork'), t('login.failedNetwork'))
      } else {
        // 请求配置错误
        showModal('error', t('login.failedTitle'), t('login.failedRetry'))
      }
    } finally {
      setLoading(false)
    }
  }

  return (
    <div style={{
      display: 'flex',
      justifyContent: 'center',
      alignItems: 'center',
      minHeight: '100vh',
      background: '#f0f2f5',
      position: 'relative'
    }}>
      {/* 语言切换按钮 */}
      <div style={{ position: 'absolute', top: 20, right: 20 }}>
        <Dropdown 
          menu={{ 
            items: langItems, 
            onClick: ({ key }) => handleLangChange(key) 
          }} 
          placement="bottomRight"
        >
          <Button icon={<GlobalOutlined />}>
            {langItems.find(i => i.key === (i18n.language.split('-')[0]))?.label || 'Language'}
          </Button>
        </Dropdown>
      </div>

      <Card 
        style={{
          width: 400,
          borderRadius: 8,
          boxShadow: '0 4px 12px rgba(0, 0, 0, 0.15)'
        }}
        title={
          <div style={{ textAlign: 'center' }}>
            <Title level={3} style={{ margin: 0, color: '#2f855a' }}>PrerenderShield</Title>
            <Paragraph style={{ margin: '8px 0 0 0', color: '#666' }}>
              {t('login.welcome')}
            </Paragraph>
          </div>
        }
      >
        {!checkingFirstRun && isFirstRun && (
          <Alert
            message={
              <div style={{ display: 'flex', alignItems: 'center' }}>
                <InfoCircleOutlined style={{ marginRight: 8, color: '#faad14' }} />
                <span>{t('login.firstRun.title')}</span>
              </div>
            }
            description={
              <div>
                <p>{t('login.firstRun.desc1')}</p>
                <p style={{ color: '#ff4d4f', fontWeight: 'bold' }}>{t('login.firstRun.desc2')}</p>
              </div>
            }
            type="warning"
            showIcon
            style={{ marginBottom: 16 }}
          />
        )}
        <Form
          name="login"
          initialValues={{ remember: true }}
          onFinish={handleLogin}
          onFinishFailed={(errorInfo) => {
            console.log('Form validation failed:', errorInfo);
            message.error(t('login.inputUsername'));
          }}
        >
          <Form.Item
            name="username"
            rules={[
              { required: true, message: t('login.inputUsername') },
              { min: 3, message: t('login.usernameMin') },
              { max: 20, message: t('login.usernameMax') }
            ]}
          >
            <Input 
              prefix={<LoginOutlined style={{ color: 'rgba(0,0,0,.25)' }} />} 
              placeholder={t('login.username')}
              size="large"
            />
          </Form.Item>
          <Form.Item
            name="password"
            rules={[
              { required: true, message: t('login.inputPassword') },
              { min: 6, message: t('login.passwordMin') },
              { max: 20, message: t('login.passwordMax') }
            ]}
          >
            <Input
              prefix={<LockOutlined style={{ color: 'rgba(0,0,0,.25)' }} />}
              type="password"
              placeholder={t('login.password')}
              size="large"
            />
          </Form.Item>
          <Form.Item>
            <Button 
              type="primary" 
              htmlType="submit" 
              style={{ width: '100%', background: '#2f855a', borderColor: '#2f855a' }}
              size="large"
              loading={loading}
            >
              {isFirstRun ? t('login.setupBtn') : t('login.loginBtn')}
            </Button>
          </Form.Item>
        </Form>
      </Card>
      
      {/* 提示Modal */}
      <Modal
        title={modalTitle}
        open={modalVisible}
        onCancel={() => setModalVisible(false)}
        footer={[
          <Button key="ok" type="primary" onClick={() => setModalVisible(false)}>
            {t('common.ok')}
          </Button>
        ]}
        className={`modal-${modalType}`}
      >
        <div style={{ color: modalType === 'error' ? '#ff4d4f' : modalType === 'success' ? '#52c41a' : '#1890ff' }}>
          {modalContent}
        </div>
      </Modal>
    </div>
  )
}

export default Login
