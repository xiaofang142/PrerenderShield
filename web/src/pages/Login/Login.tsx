import React, { useState, useEffect, useRef } from 'react'
import { Card, Form, Input, Button, Typography, Modal, message, Alert, Dropdown } from 'antd'
import { LoginOutlined, LockOutlined, InfoCircleOutlined, GlobalOutlined } from '@ant-design/icons'
import { useNavigate } from 'react-router-dom'
import { useAuth } from '../../context/AuthContext'
import { authApi } from '../../services/api'
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
  // 强制修改密码状态
  const [forceChangeVisible, setForceChangeVisible] = useState(false)
  const [forceChangeLoading, setForceChangeLoading] = useState(false)
  const [pendingToken, setPendingToken] = useState('')
  const [pendingUsername, setPendingUsername] = useState('')
  // R16-BUG-1 配套：2FA 登录第二因子状态
  const [twoFAVisible, setTwoFAVisible] = useState(false)
  const [twoFALoading, setTwoFALoading] = useState(false)
  const [twoFATmpToken, setTwoFATmpToken] = useState('')
  const [twoFACode, setTwoFACode] = useState('')
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

  // 导航定时器登记：组件卸载时统一清理，避免卸载后触发 navigate
  const navTimersRef = useRef<ReturnType<typeof setTimeout>[]>([])
  useEffect(() => () => {
    navTimersRef.current.forEach(clearTimeout)
  }, [])

  const scheduleNavigateHome = () => {
    navTimersRef.current.push(setTimeout(() => { navigate('/') }, 1500))
  }

  // 检查是否是首次运行
  useEffect(() => {
    const checkFirstRun = async () => {
      try {
        const response = await authApi.firstRun()
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

  // 2FA 第二因子验证提交（R16-BUG-1 配套）
  const handleTwoFAVerify = async (code: string) => {
    if (!code || code.trim().length < 6) {
      message.error(t('login.twoFA.codeRequired'))
      return
    }
    setTwoFALoading(true)
    try {
      const res = await authApi.verify2FA(twoFATmpToken, code.trim())
      if (res.code === 200 && res.data?.token) {
        setTwoFAVisible(false)
        setTwoFACode('')
        authLogin(res.data.token, res.data.username || 'admin')
        showModal('success', t('login.successTitle'), t('login.successContent'))
        scheduleNavigateHome()
      } else {
        message.error(res.message || t('login.twoFA.invalid'))
      }
    } catch (error: any) {
      const errorMsg = error.response?.data?.message || t('login.twoFA.invalid')
      message.error(errorMsg)
    } finally {
      setTwoFALoading(false)
    }
  }

  // 登录处理
  const handleLogin = async (values: { username: string; password: string }) => {
    setLoading(true)
    try {
      const response = await authApi.login(values.username, values.password)
      if (response.code === 200) {
        // R16-BUG-1 配套：服务端要求第二因子验证
        if (response.data.require_2fa) {
          setTwoFATmpToken(response.data.tmp_token)
          setTwoFAVisible(true)
          return
        }
        if (response.data.force_change_password) {
          setPendingToken(response.data.token)
          setPendingUsername(response.data.username)
          authLogin(response.data.token, response.data.username)
          setForceChangeVisible(true)
        } else {
          authLogin(response.data.token, response.data.username)
          showModal('success', t('login.successTitle'), t('login.successContent'))
          scheduleNavigateHome()
        }
      } else {
        showModal('error', t('login.failedTitle'), response.message || t('login.failedDefault'))
      }
    } catch (error: any) {
      console.error('Login error:', error)
      if (error.response) {
        const errorMsg = error.response.data?.message || t('login.failedDefault')
        showModal('error', t('login.failedTitle'), errorMsg)
      } else if (error.request) {
        showModal('error', t('login.failedNetwork'), t('login.failedNetwork'))
      } else {
        showModal('error', t('login.failedTitle'), t('login.failedRetry'))
      }
    } finally {
      setLoading(false)
    }
  }

  // 强制修改密码
  const handleForceChangePassword = async (values: { old_password: string; new_password: string; confirm_password: string }) => {
    if (values.new_password !== values.confirm_password) {
      message.error(t('login.passwordMismatch') || '两次输入的密码不一致')
      return
    }
    setForceChangeLoading(true)
    try {
      const res = await authApi.changePassword(values.old_password, values.new_password)
      if (res.code === 200) {
        message.success(t('login.passwordChanged') || '密码修改成功')
        setForceChangeVisible(false)
        authLogin(pendingToken, pendingUsername)
        showModal('success', t('login.successTitle'), t('login.successContent'))
        scheduleNavigateHome()
      } else {
        message.error(res.message || t('login.passwordChangeFailed') || '密码修改失败')
      }
    } catch (error: any) {
      const errorMsg = error.response?.data?.message || t('login.passwordChangeFailed') || '密码修改失败'
      message.error(errorMsg)
    } finally {
      setForceChangeLoading(false)
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
          onFinishFailed={() => {
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

      {/* R16-BUG-1 配套：2FA 第二因子验证弹窗 */}
      <Modal
        title={t('login.twoFA.title')}
        open={twoFAVisible}
        closable={false}
        maskClosable={false}
        keyboard={false}
        footer={null}
        width={380}
      >
        <Alert
          message={t('login.twoFA.desc')}
          type="info"
          showIcon
          style={{ marginBottom: 16 }}
        />
        <Input
          prefix={<LockOutlined style={{ color: 'rgba(0,0,0,.25)' }} />}
          placeholder={t('login.twoFA.placeholder')}
          size="large"
          maxLength={6}
          style={{ marginBottom: 16, letterSpacing: 8, textAlign: 'center' }}
          value={twoFACode}
          onChange={(e) => setTwoFACode(e.target.value)}
          onPressEnter={() => handleTwoFAVerify(twoFACode)}
          disabled={twoFALoading}
        />
        <Button
          type="primary"
          block
          size="large"
          loading={twoFALoading}
          onClick={() => handleTwoFAVerify(twoFACode)}
          style={{ background: '#2f855a', borderColor: '#2f855a' }}
        >
          {t('login.twoFA.verify')}
        </Button>
      </Modal>

      {/* 强制修改密码弹窗 */}
      <Modal
        title={t('login.forceChangeTitle') || '安全提示：请修改默认密码'}
        open={forceChangeVisible}
        closable={false}
        maskClosable={false}
        keyboard={false}
        footer={null}
        width={420}
      >
        <Alert
          message={t('login.forceChangeDesc') || '检测到您仍在使用默认密码，为了系统安全，请立即修改密码。'}
          type="warning"
          showIcon
          style={{ marginBottom: 16 }}
        />
        <Form layout="vertical" onFinish={handleForceChangePassword}>
          <Form.Item
            name="old_password"
            label={t('login.currentPassword') || '当前密码'}
            rules={[{ required: true, message: t('login.inputPassword') || '请输入当前密码' }]}
          >
            <Input.Password prefix={<LockOutlined />} placeholder={t('login.currentPassword') || '当前密码'} size="large" />
          </Form.Item>
          <Form.Item
            name="new_password"
            label={t('login.newPassword') || '新密码'}
            rules={[
              { required: true, message: t('login.inputPassword') || '请输入新密码' },
              { min: 6, message: t('login.passwordMin') || '密码至少6个字符' },
            ]}
          >
            <Input.Password prefix={<LockOutlined />} placeholder={t('login.newPassword') || '新密码'} size="large" />
          </Form.Item>
          <Form.Item
            name="confirm_password"
            label={t('login.confirmPassword') || '确认新密码'}
            dependencies={['new_password']}
            rules={[
              { required: true, message: t('login.inputPassword') || '请确认新密码' },
              ({ getFieldValue }) => ({
                validator(_, value) {
                  if (!value || getFieldValue('new_password') === value) {
                    return Promise.resolve()
                  }
                  return Promise.reject(new Error(t('login.passwordMismatch') || '两次输入的密码不一致'))
                },
              }),
            ]}
          >
            <Input.Password prefix={<LockOutlined />} placeholder={t('login.confirmPassword') || '确认新密码'} size="large" />
          </Form.Item>
          <Form.Item>
            <Button type="primary" htmlType="submit" loading={forceChangeLoading} style={{ width: '100%', background: '#2f855a', borderColor: '#2f855a' }} size="large">
              {t('login.confirmChangeBtn') || '确认修改'}
            </Button>
          </Form.Item>
        </Form>
      </Modal>
    </div>
  )
}

export default Login
