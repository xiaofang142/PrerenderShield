import React, { useState, useEffect } from 'react'
import { Card, Table, Button, Modal, Form, Input, Space, Tag, message, Row, Col, Statistic, Tooltip, Popconfirm, Empty } from 'antd'
import { 
  PlusOutlined, 
  ReloadOutlined, 
  DeleteOutlined, 
  SyncOutlined, 
  ExclamationCircleOutlined,
  SafetyCertificateOutlined,
  ClockCircleOutlined,
  CheckCircleOutlined,
  WarningOutlined
} from '@ant-design/icons'
import { sslApi } from '../../services/api'
import dayjs from 'dayjs'

interface Certificate {
  domain: string
  expires: string
  issuer: string
  status: string
  serialNumber: string
  fingerprint: string
  san: string[]
}

const SSLPage: React.FC = () => {
  const [certificates, setCertificates] = useState<Certificate[]>([])
  const [loading, setLoading] = useState(false)
  const [requestModalVisible, setRequestModalVisible] = useState(false)
  const [wildcardModalVisible, setWildcardModalVisible] = useState(false)
  const [detailModalVisible, setDetailModalVisible] = useState(false)
  const [selectedCert, setSelectedCert] = useState<Certificate | null>(null)
  const [expiringCertsList, setExpiringCertsList] = useState<Certificate[]>([])
  
  const [requestForm] = Form.useForm()
  const [wildcardForm] = Form.useForm()
  
  const [requestLoading, setRequestLoading] = useState(false)
  const [wildcardLoading, setWildcardLoading] = useState(false)

  // 获取证书列表
  const fetchCertificates = async () => {
    try {
      setLoading(true)
      const res = await sslApi.listCertificates()
      if (res.code === 200) {
        setCertificates(res.data.certificates || [])
      }
    } catch (error) {
      console.error('Failed to fetch certificates:', error)
      message.error('获取证书列表失败')
    } finally {
      setLoading(false)
    }
  }

  // 获取即将过期的证书
  const fetchExpiringCertificates = async () => {
    try {
      const res = await sslApi.getExpiringCertificates(30)
      if (res.code === 200) {
        setExpiringCertsList(res.data.certificates || [])
      }
    } catch (error) {
      console.error('Failed to fetch expiring certificates:', error)
    }
  }

  // 初始化
  useEffect(() => {
    fetchCertificates()
    fetchExpiringCertificates()
  }, [])

  // 申请证书
  const handleRequestCert = async (values: any) => {
    try {
      setRequestLoading(true)
      const domains = values.domains.split('\n').map((d: string) => d.trim()).filter((d: string) => d)
      
      if (domains.length === 0) {
        message.warning('请输入至少一个域名')
        return
      }

      const res = await sslApi.requestCertificate(domains)
      if (res.code === 200) {
        message.success('证书申请成功')
        setRequestModalVisible(false)
        requestForm.resetFields()
        fetchCertificates()
      } else {
        message.error(res.message || '证书申请失败')
      }
    } catch (error) {
      console.error('Failed to request certificate:', error)
      message.error('证书申请失败')
    } finally {
      setRequestLoading(false)
    }
  }

  // 申请通配符证书
  const handleRequestWildcard = async (values: any) => {
    try {
      setWildcardLoading(true)
      const subdomains = values.subdomains ? values.subdomains.split('\n').map((s: string) => s.trim()).filter((s: string) => s) : []
      
      const res = await sslApi.requestWildcardCertificate(values.baseDomain, subdomains)
      if (res.code === 200) {
        message.success('通配符证书申请成功')
        setWildcardModalVisible(false)
        wildcardForm.resetFields()
        fetchCertificates()
      } else {
        message.error(res.message || '通配符证书申请失败')
      }
    } catch (error) {
      console.error('Failed to request wildcard certificate:', error)
      message.error('通配符证书申请失败')
    } finally {
      setWildcardLoading(false)
    }
  }

  // 续签证书
  const handleRenewCert = async (domain: string) => {
    try {
      const res = await sslApi.renewCertificate(domain)
      if (res.code === 200) {
        message.success('证书续签成功')
        fetchCertificates()
      } else {
        message.error(res.message || '证书续签失败')
      }
    } catch (error) {
      console.error('Failed to renew certificate:', error)
      message.error('证书续签失败')
    }
  }

  // 删除证书
  const handleDeleteCert = async (domain: string) => {
    try {
      const res = await sslApi.deleteCertificate(domain)
      if (res.code === 200) {
        message.success('证书删除成功')
        fetchCertificates()
      } else {
        message.error(res.message || '证书删除失败')
      }
    } catch (error) {
      console.error('Failed to delete certificate:', error)
      message.error('证书删除失败')
    }
  }

  // 查看证书详情
  const handleViewDetail = async (domain: string) => {
    try {
      const res = await sslApi.getCertificate(domain)
      if (res.code === 200) {
        setSelectedCert(res.data)
        setDetailModalVisible(true)
      }
    } catch (error) {
      console.error('Failed to get certificate detail:', error)
      message.error('获取证书详情失败')
    }
  }

  // 检查证书状态
  const getCertStatus = (expires: string) => {
    if (!expires || expires === 'unknown') return { color: 'default', text: '未知' }
    
    const expiryDate = dayjs(expires)
    const now = dayjs()
    const daysLeft = expiryDate.diff(now, 'day')
    
    if (daysLeft < 0) return { color: 'error', text: '已过期' }
    if (daysLeft < 7) return { color: 'error', text: `${daysLeft}天后过期` }
    if (daysLeft < 30) return { color: 'warning', text: `${daysLeft}天后过期` }
    return { color: 'success', text: '有效' }
  }

  // 表格列配置
  const columns = [
    {
      title: '域名',
      dataIndex: 'domain',
      key: 'domain',
      render: (text: string) => (
        <Space>
          <SafetyCertificateOutlined style={{ color: '#52c41a' }} />
          <span style={{ fontWeight: 500 }}>{text}</span>
        </Space>
      ),
    },
    {
      title: '颁发者',
      dataIndex: 'issuer',
      key: 'issuer',
      render: (text: string) => text || 'Let\'s Encrypt',
    },
    {
      title: '过期时间',
      dataIndex: 'expires',
      key: 'expires',
      render: (text: string) => {
        if (!text || text === 'unknown') return <Tag>未知</Tag>
        const status = getCertStatus(text)
        return (
          <Tooltip title={dayjs(text).format('YYYY-MM-DD HH:mm:ss')}>
            <Tag color={status.color}>
              <ClockCircleOutlined /> {dayjs(text).format('YYYY-MM-DD')}
            </Tag>
          </Tooltip>
        )
      },
    },
    {
      title: '状态',
      dataIndex: 'expires',
      key: 'status',
      render: (text: string) => {
        const status = getCertStatus(text)
        return <Tag color={status.color}>{status.text}</Tag>
      },
    },
    {
      title: '操作',
      key: 'action',
      render: (_: any, record: Certificate) => (
        <Space size="small">
          <Tooltip title="查看详情">
            <Button 
              type="link" 
              size="small" 
              onClick={() => handleViewDetail(record.domain)}
            >
              详情
            </Button>
          </Tooltip>
          <Tooltip title="续签证书">
            <Button 
              type="link" 
              size="small" 
              icon={<SyncOutlined />} 
              onClick={() => handleRenewCert(record.domain)}
            >
              续签
            </Button>
          </Tooltip>
          <Popconfirm
            title="确定要删除这个证书吗？"
            onConfirm={() => handleDeleteCert(record.domain)}
            okText="确定"
            cancelText="取消"
          >
            <Tooltip title="删除证书">
              <Button 
                type="link" 
                size="small" 
                danger 
                icon={<DeleteOutlined />}
              >
                删除
              </Button>
            </Tooltip>
          </Popconfirm>
        </Space>
      ),
    },
  ]

  // 计算统计数据
  const totalCerts = certificates.length
  const validCerts = certificates.filter(c => {
    const status = getCertStatus(c.expires)
    return status.color === 'success'
  }).length
  const expiringCount = certificates.filter(c => {
    const status = getCertStatus(c.expires)
    return status.color === 'warning'
  }).length
  const expiredCerts = certificates.filter(c => {
    const status = getCertStatus(c.expires)
    return status.color === 'error'
  }).length

  return (
    <div>
      <h1 className="page-title">SSL 证书管理</h1>
      
      {/* 统计卡片 */}
      <Row gutter={[16, 16]} style={{ marginBottom: 24 }}>
        <Col span={6}>
          <Card className="card">
            <Statistic
              title="证书总数"
              value={totalCerts}
              prefix={<SafetyCertificateOutlined style={{ color: '#1890ff' }} />}
              valueStyle={{ color: '#1890ff' }}
            />
          </Card>
        </Col>
        <Col span={6}>
          <Card className="card">
            <Statistic
              title="有效证书"
              value={validCerts}
              prefix={<CheckCircleOutlined style={{ color: '#52c41a' }} />}
              valueStyle={{ color: '#52c41a' }}
            />
          </Card>
        </Col>
        <Col span={6}>
          <Card className="card">
            <Statistic
              title="即将过期"
              value={expiringCount}
              prefix={<WarningOutlined style={{ color: '#faad14' }} />}
              valueStyle={{ color: '#faad14' }}
            />
          </Card>
        </Col>
        <Col span={6}>
          <Card className="card">
            <Statistic
              title="已过期"
              value={expiredCerts}
              prefix={<ExclamationCircleOutlined style={{ color: '#ff4d4f' }} />}
              valueStyle={{ color: '#ff4d4f' }}
            />
          </Card>
        </Col>
      </Row>

      {/* 操作栏 */}
      <Card className="card" style={{ marginBottom: 16 }}>
        <Row justify="space-between" align="middle">
          <Col>
            <Space>
              <Button 
                type="primary" 
                icon={<PlusOutlined />} 
                onClick={() => setRequestModalVisible(true)}
              >
                申请证书
              </Button>
              <Button 
                icon={<PlusOutlined />} 
                onClick={() => setWildcardModalVisible(true)}
              >
                申请通配符证书
              </Button>
            </Space>
          </Col>
          <Col>
            <Button 
              icon={<ReloadOutlined />} 
              onClick={() => {
                fetchCertificates()
                fetchExpiringCertificates()
              }}
              loading={loading}
            >
              刷新
            </Button>
          </Col>
        </Row>
      </Card>

      {/* 即将过期提醒 */}
      {expiringCertsList.length > 0 && (
        <Card 
          className="card" 
          style={{ marginBottom: 16, borderColor: '#faad14' }}
          title={
            <Space>
              <WarningOutlined style={{ color: '#faad14' }} />
              <span>即将过期的证书</span>
            </Space>
          }
        >
          <Table
            columns={[
              { title: '域名', dataIndex: 'domain', key: 'domain' },
              { 
                title: '过期时间', 
                dataIndex: 'expires', 
                key: 'expires',
                render: (text: string) => (
                  <Tag color="warning">
                    <ClockCircleOutlined /> {dayjs(text).format('YYYY-MM-DD')}
                  </Tag>
                )
              },
              {
                title: '操作',
                key: 'action',
                render: (_: any, record: Certificate) => (
                  <Button 
                    type="primary" 
                    size="small" 
                    icon={<SyncOutlined />}
                    onClick={() => handleRenewCert(record.domain)}
                  >
                    立即续签
                  </Button>
                )
              }
            ]}
            dataSource={expiringCertsList}
            rowKey="domain"
            pagination={false}
            size="small"
          />
        </Card>
      )}

      {/* 证书列表 */}
      <Card className="card" title="证书列表">
        <Table
          columns={columns}
          dataSource={certificates}
          rowKey="domain"
          loading={loading}
          pagination={{
            showSizeChanger: true,
            showQuickJumper: true,
            showTotal: (total) => `共 ${total} 个证书`,
          }}
          locale={{
            emptyText: <Empty description="暂无证书" />
          }}
        />
      </Card>

      {/* 申请证书弹窗 */}
      <Modal
        title="申请 SSL 证书"
        open={requestModalVisible}
        onCancel={() => {
          setRequestModalVisible(false)
          requestForm.resetFields()
        }}
        footer={null}
        width={600}
      >
        <Form
          form={requestForm}
          layout="vertical"
          onFinish={handleRequestCert}
        >
          <Form.Item
            name="domains"
            label="域名列表"
            rules={[{ required: true, message: '请输入域名' }]}
            help="每行一个域名，例如：example.com"
          >
            <Input.TextArea 
              rows={6} 
              placeholder={`example.com\nwww.example.com\napi.example.com`}
            />
          </Form.Item>
          <Form.Item>
            <Space>
              <Button type="primary" htmlType="submit" loading={requestLoading}>
                申请证书
              </Button>
              <Button onClick={() => {
                setRequestModalVisible(false)
                requestForm.resetFields()
              }}>
                取消
              </Button>
            </Space>
          </Form.Item>
        </Form>
      </Modal>

      {/* 申请通配符证书弹窗 */}
      <Modal
        title="申请通配符 SSL 证书"
        open={wildcardModalVisible}
        onCancel={() => {
          setWildcardModalVisible(false)
          wildcardForm.resetFields()
        }}
        footer={null}
        width={600}
      >
        <Form
          form={wildcardForm}
          layout="vertical"
          onFinish={handleRequestWildcard}
        >
          <Form.Item
            name="baseDomain"
            label="基础域名"
            rules={[{ required: true, message: '请输入基础域名' }]}
            help="例如：example.com"
          >
            <Input placeholder="example.com" />
          </Form.Item>
          <Form.Item
            name="subdomains"
            label="子域名列表"
            help="每行一个子域名，留空则只申请通配符证书"
          >
            <Input.TextArea 
              rows={4} 
              placeholder={`www\napi\nadmin`}
            />
          </Form.Item>
          <Form.Item>
            <Space>
              <Button type="primary" htmlType="submit" loading={wildcardLoading}>
                申请通配符证书
              </Button>
              <Button onClick={() => {
                setWildcardModalVisible(false)
                wildcardForm.resetFields()
              }}>
                取消
              </Button>
            </Space>
          </Form.Item>
        </Form>
      </Modal>

      {/* 证书详情弹窗 */}
      <Modal
        title="证书详情"
        open={detailModalVisible}
        onCancel={() => {
          setDetailModalVisible(false)
          setSelectedCert(null)
        }}
        footer={null}
        width={600}
      >
        {selectedCert && (
          <div>
            <Row gutter={[16, 16]}>
              <Col span={12}>
                <Card size="small" title="基本信息">
                  <p><strong>域名：</strong>{selectedCert.domain}</p>
                  <p><strong>颁发者：</strong>{selectedCert.issuer || 'Let\'s Encrypt'}</p>
                  <p><strong>序列号：</strong>{selectedCert.serialNumber || 'N/A'}</p>
                </Card>
              </Col>
              <Col span={12}>
                <Card size="small" title="有效期">
                  <p><strong>过期时间：</strong></p>
                  {selectedCert.expires && selectedCert.expires !== 'unknown' ? (
                    <Tag color={getCertStatus(selectedCert.expires).color}>
                      {dayjs(selectedCert.expires).format('YYYY-MM-DD HH:mm:ss')}
                    </Tag>
                  ) : (
                    <Tag>未知</Tag>
                  )}
                  <p style={{ marginTop: 8 }}><strong>状态：</strong></p>
                  <Tag color={getCertStatus(selectedCert.expires).color}>
                    {getCertStatus(selectedCert.expires).text}
                  </Tag>
                </Card>
              </Col>
            </Row>
            {selectedCert.san && selectedCert.san.length > 0 && (
              <Card size="small" title="SAN 域名" style={{ marginTop: 16 }}>
                <Space wrap>
                  {selectedCert.san.map((domain, index) => (
                    <Tag key={index}>{domain}</Tag>
                  ))}
                </Space>
              </Card>
            )}
            {selectedCert.fingerprint && (
              <Card size="small" title="指纹" style={{ marginTop: 16 }}>
                <p style={{ wordBreak: 'break-all', fontSize: 12 }}>
                  {selectedCert.fingerprint}
                </p>
              </Card>
            )}
          </div>
        )}
      </Modal>
    </div>
  )
}

export default SSLPage
