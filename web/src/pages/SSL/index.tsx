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
import { useTranslation } from 'react-i18next'
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

interface SSLCertificateFromAPI {
  domain: string
  subject: string
  issuer: string
  not_before: string
  not_after: string
  dns_names: string[]
  expires_in: number
  expired: boolean
}

const transformCertificate = (apiCert: SSLCertificateFromAPI): Certificate => ({
  domain: apiCert.domain,
  expires: apiCert.not_after,
  issuer: apiCert.issuer,
  status: apiCert.expired ? 'expired' : 'valid',
  serialNumber: '',
  fingerprint: '',
  san: apiCert.dns_names || [],
})

const SSLPage: React.FC = () => {
  const { t } = useTranslation()
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
        const apiCerts = (res.data.certificates || []) as SSLCertificateFromAPI[]
        setCertificates(apiCerts.map(transformCertificate))
      }
    } catch (error) {
      console.error('Failed to fetch certificates:', error)
      message.error(t('ssl.messages.fetchListFailed'))
    } finally {
      setLoading(false)
    }
  }

  // 获取即将过期的证书
  const fetchExpiringCertificates = async () => {
    try {
      const res = await sslApi.getExpiringCertificates(30)
      if (res.code === 200) {
        const apiCerts = (res.data.certificates || []) as SSLCertificateFromAPI[]
        setExpiringCertsList(apiCerts.map(transformCertificate))
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
        message.warning(t('ssl.messages.needDomain'))
        return
      }

      const res = await sslApi.requestCertificate(domains)
      if (res.code === 200) {
        message.success(t('ssl.messages.requestSuccess'))
        setRequestModalVisible(false)
        requestForm.resetFields()
        fetchCertificates()
      } else {
        message.error(res.message || t('ssl.messages.requestFailed'))
      }
    } catch (error) {
      console.error('Failed to request certificate:', error)
      message.error(t('ssl.messages.requestFailed'))
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
        message.success(t('ssl.messages.wildcardSuccess'))
        setWildcardModalVisible(false)
        wildcardForm.resetFields()
        fetchCertificates()
      } else {
        message.error(res.message || t('ssl.messages.wildcardFailed'))
      }
    } catch (error) {
      console.error('Failed to request wildcard certificate:', error)
      message.error(t('ssl.messages.wildcardFailed'))
    } finally {
      setWildcardLoading(false)
    }
  }

  // 续签证书
  const handleRenewCert = async (domain: string) => {
    try {
      const res = await sslApi.renewCertificate(domain)
      if (res.code === 200) {
        message.success(t('ssl.messages.renewSuccess'))
        fetchCertificates()
      } else {
        message.error(res.message || t('ssl.messages.renewFailed'))
      }
    } catch (error) {
      console.error('Failed to renew certificate:', error)
      message.error(t('ssl.messages.renewFailed'))
    }
  }

  // 删除证书
  const handleDeleteCert = async (domain: string) => {
    try {
      const res = await sslApi.deleteCertificate(domain)
      if (res.code === 200) {
        message.success(t('ssl.messages.deleteSuccess'))
        fetchCertificates()
      } else {
        message.error(res.message || t('ssl.messages.deleteFailed'))
      }
    } catch (error) {
      console.error('Failed to delete certificate:', error)
      message.error(t('ssl.messages.deleteFailed'))
    }
  }

  // 查看证书详情
  const handleViewDetail = async (domain: string) => {
    try {
      const res = await sslApi.getCertificate(domain)
      if (res.code === 200) {
        const apiCert = res.data as SSLCertificateFromAPI
        setSelectedCert(transformCertificate(apiCert))
        setDetailModalVisible(true)
      }
    } catch (error) {
      console.error('Failed to get certificate detail:', error)
      message.error(t('ssl.messages.detailFailed'))
    }
  }

  // 检查证书状态
  const getCertStatus = (expires: string) => {
    if (!expires || expires === 'unknown') return { color: 'default', text: t('ssl.status.unknown') }
    
    const expiryDate = dayjs(expires)
    const now = dayjs()
    const daysLeft = expiryDate.diff(now, 'day')
    
    if (daysLeft < 0) return { color: 'error', text: t('ssl.status.expired') }
    if (daysLeft < 7) return { color: 'error', text: t('ssl.status.expiresInDays', { days: daysLeft }) }
    if (daysLeft < 30) return { color: 'warning', text: t('ssl.status.expiresInDays', { days: daysLeft }) }
    return { color: 'success', text: t('ssl.status.valid') }
  }

  // 表格列配置
  const columns = [
    {
      title: t('ssl.columns.domain'),
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
      title: t('ssl.columns.issuer'),
      dataIndex: 'issuer',
      key: 'issuer',
      render: (text: string) => text || 'Let\'s Encrypt',
    },
    {
      title: t('ssl.columns.expires'),
      dataIndex: 'expires',
      key: 'expires',
      render: (text: string) => {
        if (!text || text === 'unknown') return <Tag>{t('ssl.status.unknown')}</Tag>
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
      title: t('ssl.columns.status'),
      dataIndex: 'expires',
      key: 'status',
      render: (text: string) => {
        const status = getCertStatus(text)
        return <Tag color={status.color}>{status.text}</Tag>
      },
    },
    {
      title: t('common.actions'),
      key: 'action',
      render: (_: any, record: Certificate) => (
        <Space size="small">
          <Tooltip title={t('ssl.tooltips.viewDetail')}>
            <Button 
              type="link" 
              size="small" 
              onClick={() => handleViewDetail(record.domain)}
            >
              {t('ssl.buttons.detail')}
            </Button>
          </Tooltip>
          <Tooltip title={t('ssl.tooltips.renew')}>
            <Button 
              type="link" 
              size="small" 
              icon={<SyncOutlined />} 
              onClick={() => handleRenewCert(record.domain)}
            >
              {t('ssl.buttons.renew')}
            </Button>
          </Tooltip>
          <Popconfirm
            title={t('ssl.deleteConfirm.title')}
            onConfirm={() => handleDeleteCert(record.domain)}
            okText={t('common.ok')}
            cancelText={t('common.cancel')}
          >
            <Tooltip title={t('ssl.tooltips.delete')}>
              <Button 
                type="link" 
                size="small" 
                danger 
                icon={<DeleteOutlined />}
              >
                {t('common.delete')}
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
      <h1 className="page-title">{t('ssl.title')}</h1>
      
      {/* 统计卡片 */}
      <Row gutter={[16, 16]} style={{ marginBottom: 24 }}>
        <Col span={6}>
          <Card className="card">
            <Statistic
              title={t('ssl.stats.total')}
              value={totalCerts}
              prefix={<SafetyCertificateOutlined style={{ color: '#1890ff' }} />}
              valueStyle={{ color: '#1890ff' }}
            />
          </Card>
        </Col>
        <Col span={6}>
          <Card className="card">
            <Statistic
              title={t('ssl.stats.valid')}
              value={validCerts}
              prefix={<CheckCircleOutlined style={{ color: '#52c41a' }} />}
              valueStyle={{ color: '#52c41a' }}
            />
          </Card>
        </Col>
        <Col span={6}>
          <Card className="card">
            <Statistic
              title={t('ssl.stats.expiringSoon')}
              value={expiringCount}
              prefix={<WarningOutlined style={{ color: '#faad14' }} />}
              valueStyle={{ color: '#faad14' }}
            />
          </Card>
        </Col>
        <Col span={6}>
          <Card className="card">
            <Statistic
              title={t('ssl.stats.expired')}
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
                {t('ssl.actions.requestCert')}
              </Button>
              <Button 
                icon={<PlusOutlined />} 
                onClick={() => setWildcardModalVisible(true)}
              >
                {t('ssl.actions.requestWildcard')}
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
              {t('ssl.actions.refresh')}
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
              <span>{t('ssl.expiring.title')}</span>
            </Space>
          }
        >
          <Table
            columns={[
              { title: t('ssl.columns.domain'), dataIndex: 'domain', key: 'domain' },
              { 
                title: t('ssl.columns.expires'), 
                dataIndex: 'expires', 
                key: 'expires',
                render: (text: string) => (
                  <Tag color="warning">
                    <ClockCircleOutlined /> {dayjs(text).format('YYYY-MM-DD')}
                  </Tag>
                )
              },
              {
                title: t('common.actions'),
                key: 'action',
                render: (_: any, record: Certificate) => (
                  <Button 
                    type="primary" 
                    size="small" 
                    icon={<SyncOutlined />}
                    onClick={() => handleRenewCert(record.domain)}
                  >
                    {t('ssl.expiring.renewNow')}
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
      <Card className="card" title={t('ssl.list.title')}>
        <Table
          columns={columns}
          dataSource={certificates}
          rowKey="domain"
          loading={loading}
          pagination={{
            showSizeChanger: true,
            showQuickJumper: true,
            showTotal: (total) => t('ssl.table.totalText', { total }),
          }}
          locale={{
            emptyText: <Empty description={t('ssl.table.empty')} />
          }}
        />
      </Card>

      {/* 申请证书弹窗 */}
      <Modal
        title={t('ssl.modal.requestTitle')}
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
            label={t('ssl.form.domains')}
            rules={[{ required: true, message: t('ssl.form.domainsRequired') }]}
            help={t('ssl.form.domainsHelp')}
          >
            <Input.TextArea 
              rows={6} 
              placeholder={`example.com\nwww.example.com\napi.example.com`}
            />
          </Form.Item>
          <Form.Item>
            <Space>
              <Button type="primary" htmlType="submit" loading={requestLoading}>
                {t('ssl.actions.requestCert')}
              </Button>
              <Button onClick={() => {
                setRequestModalVisible(false)
                requestForm.resetFields()
              }}>
                {t('common.cancel')}
              </Button>
            </Space>
          </Form.Item>
        </Form>
      </Modal>

      {/* 申请通配符证书弹窗 */}
      <Modal
        title={t('ssl.modal.wildcardTitle')}
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
            label={t('ssl.form.baseDomain')}
            rules={[{ required: true, message: t('ssl.form.baseDomainRequired') }]}
            help={t('ssl.form.baseDomainHelp')}
          >
            <Input placeholder="example.com" />
          </Form.Item>
          <Form.Item
            name="subdomains"
            label={t('ssl.form.subdomains')}
            help={t('ssl.form.subdomainsHelp')}
          >
            <Input.TextArea 
              rows={4} 
              placeholder={`www\napi\nadmin`}
            />
          </Form.Item>
          <Form.Item>
            <Space>
              <Button type="primary" htmlType="submit" loading={wildcardLoading}>
                {t('ssl.actions.requestWildcard')}
              </Button>
              <Button onClick={() => {
                setWildcardModalVisible(false)
                wildcardForm.resetFields()
              }}>
                {t('common.cancel')}
              </Button>
            </Space>
          </Form.Item>
        </Form>
      </Modal>

      {/* 证书详情弹窗 */}
      <Modal
        title={t('ssl.modal.detailTitle')}
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
                <Card size="small" title={t('ssl.detail.basicInfo')}>
                  <p><strong>{t('ssl.detail.domainLabel')}</strong>{selectedCert.domain}</p>
                  <p><strong>{t('ssl.detail.issuerLabel')}</strong>{selectedCert.issuer || 'Let\'s Encrypt'}</p>
                  <p><strong>{t('ssl.detail.serialNumber')}</strong>{selectedCert.serialNumber || 'N/A'}</p>
                </Card>
              </Col>
              <Col span={12}>
                <Card size="small" title={t('ssl.detail.validity')}>
                  <p><strong>{t('ssl.detail.expiresLabel')}</strong></p>
                  {selectedCert.expires && selectedCert.expires !== 'unknown' ? (
                    <Tag color={getCertStatus(selectedCert.expires).color}>
                      {dayjs(selectedCert.expires).format('YYYY-MM-DD HH:mm:ss')}
                    </Tag>
                  ) : (
                    <Tag>{t('ssl.status.unknown')}</Tag>
                  )}
                  <p style={{ marginTop: 8 }}><strong>{t('ssl.detail.statusLabel')}</strong></p>
                  <Tag color={getCertStatus(selectedCert.expires).color}>
                    {getCertStatus(selectedCert.expires).text}
                  </Tag>
                </Card>
              </Col>
            </Row>
            {selectedCert.san && selectedCert.san.length > 0 && (
              <Card size="small" title={t('ssl.detail.sanDomains')} style={{ marginTop: 16 }}>
                <Space wrap>
                  {selectedCert.san.map((domain, index) => (
                    <Tag key={index}>{domain}</Tag>
                  ))}
                </Space>
              </Card>
            )}
            {selectedCert.fingerprint && (
              <Card size="small" title={t('ssl.detail.fingerprint')} style={{ marginTop: 16 }}>
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
