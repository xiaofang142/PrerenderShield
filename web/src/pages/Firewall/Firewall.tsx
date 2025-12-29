import React, { useState, useEffect } from 'react'
import { Card, Row, Col, Statistic, Button, Table, Modal, Input, message, Select } from 'antd'
import { SecurityScanOutlined, ReloadOutlined, PlayCircleOutlined } from '@ant-design/icons'
import { firewallApi, sitesApi } from '../../services/api'

const { Search } = Input
const { Option } = Select

const Firewall: React.FC = () => {
  const [sites, setSites] = useState<any[]>([])
  const [selectedSite, setSelectedSite] = useState<string>('')
  const [status, setStatus] = useState({
    enabled: false,
    defaultAction: 'block',
    blockMessage: 'Request blocked by firewall',
  })
  const [rules, setRules] = useState([])
  const [loading, setLoading] = useState(true)
  const [scanModalVisible, setScanModalVisible] = useState(false)
  const [scanUrl, setScanUrl] = useState('')
  const [scanLoading, setScanLoading] = useState(false)

  // 表格列配置
  const columns = [
    {
      title: '规则ID',
      dataIndex: 'id',
      key: 'id',
    },
    {
      title: '名称',
      dataIndex: 'name',
      key: 'name',
    },
    {
      title: '分类',
      dataIndex: 'category',
      key: 'category',
    },
    {
      title: '模式',
      dataIndex: 'pattern',
      key: 'pattern',
    },
    {
      title: '严重程度',
      dataIndex: 'severity',
      key: 'severity',
      render: (text: string) => {
        const color = text === 'high' ? '#f5222d' : text === 'medium' ? '#faad14' : '#52c41a'
        return <span style={{ color }}>{text}</span>
      },
    },
  ]

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

  // 获取防火墙状态
  const fetchStatus = async () => {
    if (!selectedSite) return
    
    try {
      setLoading(true)
      const [statusRes, rulesRes] = await Promise.all([
        firewallApi.getStatus(selectedSite),
        firewallApi.getRules(selectedSite),
      ])
      
      if (statusRes.code === 200) {
        // 处理单站点数据结构
        const statusData = typeof statusRes.data === 'object' && statusRes.data.enabled !== undefined ? statusRes.data : statusRes.data[selectedSite]
        setStatus(statusData)
      }
      
      if (rulesRes.code === 200) {
        setRules(rulesRes.data)
      }
    } catch (error) {
      console.error('Failed to fetch firewall data:', error)
      message.error('获取防火墙数据失败')
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
      fetchStatus()
    }
  }, [selectedSite])

  // 触发扫描
  const handleScan = async () => {
    if (!selectedSite) {
      message.warning('请选择站点')
      return
    }
    
    if (!scanUrl) {
      message.warning('请输入要扫描的URL')
      return
    }

    try {
      setScanLoading(true)
      const res = await firewallApi.scan({ site: selectedSite, url: scanUrl })
      if (res.code === 200) {
        message.success('扫描已触发')
        setScanModalVisible(false)
        setScanUrl('')
      } else {
        message.error('扫描触发失败')
      }
    } catch (error) {
      console.error('Failed to trigger scan:', error)
      message.error('扫描触发失败')
    } finally {
      setScanLoading(false)
    }
  }

  return (
    <div>
      <h1 className="page-title">防火墙</h1>
      
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
            <Button type="primary" icon={<ReloadOutlined />} onClick={fetchStatus} loading={loading}>
              刷新状态
            </Button>
          </Col>
        </Row>
      </Card>
      
      {/* 防火墙状态卡片 */}
      <Card className="card">
        <Row gutter={[16, 16]}>
          <Col span={6}>
            <Statistic
              title="防火墙状态"
              value={status.enabled ? '已启用' : '已禁用'}
              prefix={<SecurityScanOutlined />}
              valueStyle={{ color: status.enabled ? '#52c41a' : '#faad14' }}
            />
          </Col>
          <Col span={12}>
            <Statistic
              title="默认动作"
              value={status.defaultAction}
              valueStyle={{ color: status.defaultAction === 'block' ? '#f5222d' : '#52c41a' }}
            />
          </Col>
          <Col span={6}>
            <Button
              type="primary"
              icon={<PlayCircleOutlined />}
              onClick={() => setScanModalVisible(true)}
            >
              手动扫描
            </Button>
          </Col>
        </Row>
      </Card>

      {/* 规则列表 */}
      <Card className="card" title="防火墙规则">
        <Table
          columns={columns}
          dataSource={rules}
          rowKey="id"
          loading={loading}
          pagination={{ pageSize: 10 }}
        />
      </Card>

      {/* 扫描模态框 */}
      <Modal
        title="手动扫描"
        open={scanModalVisible}
        onOk={handleScan}
        onCancel={() => setScanModalVisible(false)}
        confirmLoading={scanLoading}
        okText="开始扫描"
        cancelText="取消"
      >
        <div style={{ marginBottom: 16 }}>
          <label style={{ display: 'block', marginBottom: 8 }}>URL地址：</label>
          <Search
            placeholder="请输入要扫描的URL"
            allowClear
            value={scanUrl}
            onChange={(e) => setScanUrl(e.target.value)}
            onPressEnter={handleScan}
            style={{ width: '100%' }}
          />
        </div>
        <p style={{ color: '#666', fontSize: 12 }}>
          注意：扫描可能需要一段时间，请耐心等待
        </p>
      </Modal>
    </div>
  )
}

export default Firewall