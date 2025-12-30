import React, { useState, useEffect } from 'react'
import { Card, Row, Col, Statistic, Button, Table, Modal, Input, message, Popconfirm, Switch, Select } from 'antd'
import { LockOutlined, PlusOutlined, DeleteOutlined, ReloadOutlined } from '@ant-design/icons'
import { sslApi, sitesApi } from '../../services/api'

const { Search } = Input
const { Option } = Select

const SSL: React.FC = () => {
  const [sites, setSites] = useState<any[]>([])
  const [selectedSite, setSelectedSite] = useState<string>('')
  const [status, setStatus] = useState({
    enabled: false,
  })
  const [certs, setCerts] = useState<string[]>([])
  const [loading, setLoading] = useState(true)
  const [addModalVisible, setAddModalVisible] = useState(false)
  const [domain, setDomain] = useState('')
  const [addLoading, setAddLoading] = useState(false)

  // 表格列配置
  const columns = [
    {
      title: '域名',
      dataIndex: 'domain',
      key: 'domain',
      ellipsis: true,
    },
    {
      title: '证书状态',
      dataIndex: 'status',
      key: 'status',
      render: (text: string) => {
        const color = text === 'valid' ? '#52c41a' : '#f5222d'
        return <span style={{ color }}>{text}</span>
      },
    },
    {
      title: '有效期',
      dataIndex: 'validity',
      key: 'validity',
    },
    {
      title: '操作',
      key: 'action',
      render: (_: any, record: any) => (
        <Popconfirm
          title="确定要删除该域名的证书吗？"
          onConfirm={() => handleDeleteCert(record.domain)}
          okText="确定"
          cancelText="取消"
        >
          <Button
            type="text"
            danger
            icon={<DeleteOutlined />}
          >
            删除
          </Button>
        </Popconfirm>
      ),
    },
  ]

  // 格式化证书数据
  const formatCertsData = (domains: string[]) => {
    return domains.map(domain => ({
      domain,
      status: 'valid',
      validity: '30天',
      key: domain,
    }))
  }

  // 获取站点列表
  const fetchSites = async () => {
    try {
      const res = await sitesApi.getSites()
      if (res.code === 200) {
        setSites(res.data)
        if (res.data.length > 0) {
          setSelectedSite(res.data[0].name)
        }
      }
    } catch (error) {
      console.error('Failed to fetch sites:', error)
      message.error('获取站点列表失败')
    }
  }

  // 获取SSL状态和证书列表
  const fetchData = async () => {
    if (!selectedSite) return
    
    try {
      setLoading(true)
      const [statusRes, certsRes] = await Promise.all([
        sslApi.getStatus(selectedSite),
        sslApi.getCerts(selectedSite),
      ])
      
      if (statusRes.code === 200) {
        // 处理单站点数据结构
        const statusData = typeof statusRes.data === 'object' && statusRes.data.enabled !== undefined ? statusRes.data : statusRes.data[selectedSite]
        setStatus(statusData)
      }
      
      if (certsRes.code === 200) {
        setCerts(certsRes.data)
      }
    } catch (error) {
      console.error('Failed to fetch SSL data:', error)
      message.error('获取SSL数据失败')
    } finally {
      setLoading(false)
    }
  }

  // 初始化数据
  useEffect(() => {
    fetchSites()
  }, [])

  // 当选择的站点变化时，重新获取状态
  useEffect(() => {
    if (selectedSite) {
      fetchData()
    }
  }, [selectedSite])

  // 添加域名证书
  const handleAddCert = async () => {
    if (!selectedSite) {
      message.warning('请选择站点')
      return
    }
    
    if (!domain) {
      message.warning('请输入域名')
      return
    }

    try {
      setAddLoading(true)
      const res = await sslApi.addCert({ site: selectedSite, domain })
      if (res.code === 200) {
        message.success('证书添加成功')
        setCerts(prev => [...prev, domain])
        setAddModalVisible(false)
        setDomain('')
      } else {
        message.error('证书添加失败')
      }
    } catch (error) {
      console.error('Failed to add cert:', error)
      message.error('证书添加失败')
    } finally {
      setAddLoading(false)
    }
  }

  // 删除域名证书
  const handleDeleteCert = async (domain: string) => {
    try {
      const res = await sslApi.deleteCert(selectedSite, domain)
      if (res.code === 200) {
        message.success('证书删除成功')
        setCerts(prev => prev.filter(d => d !== domain))
      } else {
        message.error('证书删除失败')
      }
    } catch (error) {
      console.error('Failed to delete cert:', error)
      message.error('证书删除失败')
    }
  }

  return (
    <div>
      <h1 className="page-title">SSL管理</h1>
      
      {/* 站点选择器 */}
      <Card className="card" style={{ marginBottom: 16 }}>
        <Row align="middle">
          <Col span={8}>
            <label style={{ marginRight: 8 }}>选择站点：</label>
            <Select
              value={selectedSite}
              onChange={setSelectedSite}
              style={{ width: 200 }}
              loading={sites.length === 0}
            >
              {sites.map((site) => (
                <Option key={site.name} value={site.name}>
                  {site.name} ({site.domain})
                </Option>
              ))}
            </Select>
          </Col>
          <Col span={8}>
            <Button type="primary" icon={<ReloadOutlined />} onClick={fetchData} loading={loading}>
              刷新状态
            </Button>
          </Col>
        </Row>
      </Card>
      
      {/* SSL状态卡片 */}
      <Card className="card">
        <Row gutter={[16, 16]}>
          <Col span={6}>
            <Statistic
              title="SSL状态"
              value={status.enabled ? '已启用' : '已禁用'}
              prefix={<LockOutlined />}
              valueStyle={{ color: status.enabled ? '#52c41a' : '#faad14' }}
            />
          </Col>
        </Row>
      </Card>

      {/* 证书列表 */}
      <Card 
        className="card" 
        title="证书列表"
        extra={
          <Button
            type="primary"
            icon={<PlusOutlined />}
            onClick={() => setAddModalVisible(true)}
          >
            添加域名
          </Button>
        }
      >
        <Table
          columns={columns}
          dataSource={formatCertsData(certs)}
          rowKey="domain"
          loading={loading}
          pagination={{ pageSize: 10 }}
          emptyText="暂无证书"
        />
      </Card>

      {/* 添加域名模态框 */}
      <Modal
        title="添加域名"
        open={addModalVisible}
        onOk={handleAddCert}
        onCancel={() => setAddModalVisible(false)}
        confirmLoading={addLoading}
        okText="添加"
        cancelText="取消"
      >
        <div style={{ marginBottom: 16 }}>
          <label style={{ display: 'block', marginBottom: 8 }}>域名：</label>
          <Search
            placeholder="请输入域名（例如：example.com）"
            allowClear
            value={domain}
            onChange={(e) => setDomain(e.target.value)}
            onPressEnter={handleAddCert}
            style={{ width: '100%' }}
          />
        </div>
        <p style={{ color: '#666', fontSize: 12 }}>
          注意：添加域名后，系统将自动申请和管理SSL证书
        </p>
      </Modal>
    </div>
  )
}

export default SSL