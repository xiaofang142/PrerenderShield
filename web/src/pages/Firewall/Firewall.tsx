import React, { useState, useEffect } from 'react'
import { Card, Row, Col, Button, Table, message, Select, Tag, Space, Tooltip } from 'antd'
import { ReloadOutlined, StopOutlined, CheckCircleOutlined, GlobalOutlined, ClockCircleOutlined } from '@ant-design/icons'
import { firewallApi, sitesApi } from '../../services/api'
import dayjs from 'dayjs'

const { Option } = Select

const Firewall: React.FC = () => {
  const [sites, setSites] = useState<any[]>([])
  const [selectedSite, setSelectedSite] = useState<string>('')
  const [loading, setLoading] = useState(false)
  
  // Attack Logs State
  const [logs, setLogs] = useState<any[]>([])
  const [total, setTotal] = useState(0)
  const [currentPage, setCurrentPage] = useState(1)
  const [pageSize, setPageSize] = useState(10)

  // Fetch Sites
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

  // Fetch Attack Logs
  const fetchLogs = async (page = 1) => {
    if (!selectedSite) return
    
    try {
      setLoading(true)
      const res = await firewallApi.getAttackLogs({
        site_id: selectedSite,
        page: page,
        limit: pageSize
      })
      
      if (res.code === 200) {
        setLogs(res.data.logs || [])
        setTotal(res.data.total || 0)
        setCurrentPage(page)
      }
    } catch (error) {
      console.error('Failed to fetch attack logs:', error)
      message.error('获取攻击记录失败')
    } finally {
      setLoading(false)
    }
  }

  // Handle Add to Whitelist
  const handleAddToWhitelist = async (ip: string) => {
    try {
      const res = await firewallApi.addToWhitelist(selectedSite, ip)
      if (res.code === 200) {
        message.success(`已将 IP ${ip} 加入白名单`)
      } else {
        message.error(res.message || '操作失败')
      }
    } catch (error) {
      message.error('操作失败')
    }
  }

  // Handle Add to Blacklist
  const handleAddToBlacklist = async (ip: string) => {
    try {
      const res = await firewallApi.addToBlacklist(selectedSite, ip)
      if (res.code === 200) {
        message.success(`已将 IP ${ip} 加入黑名单`)
      } else {
        message.error(res.message || '操作失败')
      }
    } catch (error) {
      message.error('操作失败')
    }
  }

  // Initialize
  useEffect(() => {
    fetchSites()
  }, [])

  // On Site Change
  useEffect(() => {
    if (selectedSite) {
      fetchLogs(1)
    }
  }, [selectedSite])

  // Table Columns
  const columns = [
    {
      title: 'IP地址',
      dataIndex: 'ip_address',
      key: 'ip_address',
      render: (text: string) => <Tag color="blue">{text}</Tag>
    },
    {
      title: '地理位置',
      key: 'location',
      render: (_: any, record: any) => (
        <Space>
          <GlobalOutlined />
          <span>{record.country || '未知'} {record.city}</span>
        </Space>
      )
    },
    {
      title: '攻击时间',
      dataIndex: 'created_at',
      key: 'created_at',
      render: (text: string) => (
        <Space>
          <ClockCircleOutlined />
          <span>{dayjs(text).format('YYYY-MM-DD HH:mm:ss')}</span>
        </Space>
      )
    },
    {
      title: '拦截原因',
      key: 'reason',
      render: (_: any, record: any) => (
        <span>{record.reason || record.rule_id || 'Unknown'}</span>
      )
    },
    {
      title: '操作',
      key: 'action',
      render: (_: any, record: any) => (
        <Space>
          <Tooltip title="加入白名单">
            <Button 
              type="link" 
              size="small" 
              icon={<CheckCircleOutlined />} 
              onClick={() => handleAddToWhitelist(record.ip_address)}
              style={{ color: '#52c41a' }}
            >
              白名单
            </Button>
          </Tooltip>
          <Tooltip title="加入黑名单">
            <Button 
              type="link" 
              size="small" 
              icon={<StopOutlined />} 
              danger
              onClick={() => handleAddToBlacklist(record.ip_address)}
            >
              黑名单
            </Button>
          </Tooltip>
        </Space>
      )
    }
  ]

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 24 }}>
        <div>
          <h1 className="page-title" style={{ margin: 0 }}>防火墙拦截记录</h1>
          <div style={{ color: '#666', marginTop: 8 }}>
            查看被防火墙拦截的恶意请求记录
          </div>
        </div>
      </div>
      
      {/* Site Selector */}
      <Card className="card" style={{ marginBottom: 16 }}>
        <Row align="middle">
          <Col span={12}>
            <Space>
              <label>选择站点：</label>
              <Select
                value={selectedSite}
                onChange={setSelectedSite}
                style={{ width: 250 }}
                loading={sites.length === 0}
                placeholder="请选择站点"
              >
                {sites.map((site) => (
                  <Option key={site.name} value={site.name}>
                    {site.name} ({site.domain})
                  </Option>
                ))}
              </Select>
            </Space>
          </Col>
          <Col span={12} style={{ textAlign: 'right' }}>
            <Button type="primary" icon={<ReloadOutlined />} onClick={() => fetchLogs(currentPage)} loading={loading}>
              刷新列表
            </Button>
          </Col>
        </Row>
      </Card>
      
      {/* Attack Log List */}
      <Card className="card" title="攻击记录列表">
        <Table
          columns={columns}
          dataSource={logs}
          rowKey={(record) => record.id || Math.random().toString()}
          loading={loading}
          pagination={{
            current: currentPage,
            pageSize: pageSize,
            total: total,
            showSizeChanger: true,
            showQuickJumper: true,
            showTotal: (total) => `共 ${total} 条记录`,
            onChange: (page, size) => {
              setPageSize(size)
              fetchLogs(page)
            }
          }}
        />
      </Card>
    </div>
  )
}

export default Firewall
