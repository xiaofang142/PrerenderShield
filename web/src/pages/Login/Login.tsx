import React, { useState } from 'react'
import { Card, Form, Input, Button, Typography, Spin, Modal, message } from 'antd'
import { LoginOutlined, LockOutlined } from '@ant-design/icons'
import { useNavigate } from 'react-router-dom'
import { useAuth } from '../../context/AuthContext'
import api from '../../services/api'

const { Title, Paragraph } = Typography

const Login: React.FC = () => {
  const [loading, setLoading] = useState(false)
  // Modal状态
  const [modalVisible, setModalVisible] = useState(false)
  const [modalTitle, setModalTitle] = useState('')
  const [modalContent, setModalContent] = useState('')
  const [modalType, setModalType] = useState<'success' | 'error' | 'info'>('info')
  const navigate = useNavigate()
  const { login: authLogin } = useAuth()

  // 显示提示Modal
  const showModal = (type: 'success' | 'error' | 'info', title: string, content: string) => {
    setModalType(type)
    setModalTitle(title)
    setModalContent(content)
    setModalVisible(true)
  }

  // 登录处理
  const handleLogin = async (values: { username: string; password: string }) => {
    setLoading(true)
    try {
      const response = await api.post('/auth/login', values)
      if (response.code === 200) {
        // 保存token并更新全局状态
        authLogin(response.data.token, response.data.username)
        // 显示成功提示
        showModal('success', '登录成功', '您已成功登录系统，即将跳转到首页')
        // 延迟跳转到首页
        setTimeout(() => {
          navigate('/')
        }, 1500)
      } else {
        // API返回错误，显示错误信息
        showModal('error', '登录失败', response.message || '登录失败')
      }
    } catch (error: any) {
      // 网络错误或其他错误
      console.error('登录错误:', error)
      if (error.response) {
        // 服务器返回了错误响应
        const errorMsg = error.response.data?.message || '用户名或密码错误'
        showModal('error', '登录失败', errorMsg)
      } else if (error.request) {
        // 请求已发出，但没有收到响应
        showModal('error', '网络错误', '网络错误，请检查网络连接')
      } else {
        // 请求配置错误
        showModal('error', '登录失败', '登录失败，请稍后重试')
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
      background: '#f0f2f5'
    }}>
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
              欢迎登录后台管理系统
            </Paragraph>
          </div>
        }
      >
        <Form
          name="login"
          initialValues={{ remember: true }}
          onFinish={handleLogin}
          onFinishFailed={(errorInfo) => {
            console.log('表单验证失败:', errorInfo);
            message.error('请输入用户名和密码');
          }}
        >
          <Form.Item
            name="username"
            rules={[
              { required: true, message: '请输入用户名!' },
              { min: 3, message: '用户名长度不能少于3个字符!' },
              { max: 20, message: '用户名长度不能超过20个字符!' }
            ]}
          >
            <Input 
              prefix={<LoginOutlined style={{ color: 'rgba(0,0,0,.25)' }} />} 
              placeholder="用户名"
              size="large"
            />
          </Form.Item>
          <Form.Item
            name="password"
            rules={[
              { required: true, message: '请输入密码!' },
              { min: 6, message: '密码长度不能少于6个字符!' },
              { max: 20, message: '密码长度不能超过20个字符!' }
            ]}
          >
            <Input
              prefix={<LockOutlined style={{ color: 'rgba(0,0,0,.25)' }} />}
              type="password"
              placeholder="密码"
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
              登录
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
            确定
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
