import React, { useState, useEffect, useRef } from 'react'
import { Card, Row, Col, Button, Table, message, Select, Tag, Space, Tooltip } from 'antd'
import { ReloadOutlined, StopOutlined, CheckCircleOutlined, GlobalOutlined, ClockCircleOutlined } from '@ant-design/icons'
import { firewallApi, extractErrorMessage } from '../../services/api'
import { useSites } from '../../hooks/useSites'
import { useTranslation } from 'react-i18next'
import dayjs from 'dayjs'

const { Option } = Select

const Firewall: React.FC = () => {
  const { t } = useTranslation()
  const { sites, selectedSiteId: selectedSite, setSelectedSiteId: setSelectedSite } = useSites({
    autoSelectFirst: true,
    onFetchError: (msg) => message.error(msg),
  })
  const [loading, setLoading] = useState(false)
  
  // Attack Logs State
  const [logs, setLogs] = useState<any[]>([])
  const [total, setTotal] = useState(0)
  const [currentPage, setCurrentPage] = useState(1)
  const [pageSize, setPageSize] = useState(10)
  // 竞态防护：站点快速切换时，旧请求的响应不再写入 state
  const requestVersionRef = useRef(0)

  // Fetch Attack Logs
  const fetchLogs = async (page = 1) => {
    if (!selectedSite) {
      message.warning(t('firewallPage.selectSiteFirst'))
      return
    }

    const version = ++requestVersionRef.current
    try {
      setLoading(true)
      const res = await firewallApi.getAttackLogs({
        site_id: selectedSite,
        page: page,
        limit: pageSize
      })

      if (version !== requestVersionRef.current) return
      if (res.code === 200) {
        setLogs(res.data.logs || [])
        setTotal(res.data.total || 0)
        setCurrentPage(page)
      }
    } catch (error) {
      console.error('Failed to fetch attack logs:', error)
      message.error(t('firewallPage.fetchLogsFailed'))
    } finally {
      setLoading(false)
    }
  }

  // 白/黑名单操作统一处理（原两份重复函数收敛）
  const handleIpAction = async (ip: string, action: 'whitelist' | 'blacklist') => {
    try {
      const apiCall = action === 'whitelist' ? firewallApi.addToWhitelist : firewallApi.addToBlacklist
      const res = await apiCall(selectedSite, ip)
      if (res.code === 200) {
        message.success(
          action === 'whitelist'
            ? t('firewallPage.whitelistAdded', { ip })
            : t('firewallPage.blacklistAdded', { ip })
        )
      } else {
        message.error(res.message || t('firewallPage.operationFailed'))
      }
    } catch (error) {
      message.error(extractErrorMessage(error))
    }
  }

  // Initialize（站点列表由 useSites 自动加载）

  // On Site Change
  useEffect(() => {
    if (selectedSite) {
      fetchLogs(1)
    }
  }, [selectedSite])

  // Table Columns
  const columns = [
    {
      title: t('firewallPage.columns.ipAddress'),
      dataIndex: 'ip_address',
      key: 'ip_address',
      render: (text: string) => <Tag color="blue">{text}</Tag>
    },
    {
      title: t('firewallPage.columns.location'),
      key: 'location',
      render: (_: any, record: any) => (
        <Space>
          <GlobalOutlined />
          <span>{record.country || t('firewallPage.unknown')} {record.city}</span>
        </Space>
      )
    },
    {
      title: t('firewallPage.columns.attackTime'),
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
      title: t('firewallPage.columns.blockReason'),
      key: 'reason',
      render: (_: any, record: any) => (
        <span>{record.reason || record.rule_id || 'Unknown'}</span>
      )
    },
    {
      title: t('firewallPage.columns.actions'),
      key: 'action',
      render: (_: any, record: any) => (
        <Space>
          <Tooltip title={t('firewallPage.addToWhitelist')}>
            <Button
              type="link"
              size="small"
              icon={<CheckCircleOutlined />}
              onClick={() => handleIpAction(record.ip_address, 'whitelist')}
              style={{ color: '#52c41a' }}
            >
              {t('firewallPage.whitelist')}
            </Button>
          </Tooltip>
          <Tooltip title={t('firewallPage.addToBlacklist')}>
            <Button
              type="link"
              size="small"
              icon={<StopOutlined />}
              danger
              onClick={() => handleIpAction(record.ip_address, 'blacklist')}
            >
              {t('firewallPage.blacklist')}
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
          <h1 className="page-title" style={{ margin: 0 }}>{t('firewallPage.title')}</h1>
          <div style={{ color: '#666', marginTop: 8 }}>
            {t('firewallPage.subtitle')}
          </div>
        </div>
      </div>
      
      {/* Site Selector */}
      <Card className="card" style={{ marginBottom: 16 }}>
        <Row align="middle">
          <Col span={12}>
            <Space>
              <label>{t('firewallPage.selectSite')}</label>
              <Select
                value={selectedSite}
                onChange={setSelectedSite}
                style={{ width: 250 }}
                loading={sites.length === 0}
                placeholder={t('firewallPage.sitePlaceholder')}
              >
                {sites.map((site) => (
                  <Option key={site.id} value={site.id}>
                    {site.name} ({site.domains?.[0] || site.id})
                  </Option>
                ))}
              </Select>
            </Space>
          </Col>
          <Col span={12} style={{ textAlign: 'right' }}>
            <Button type="primary" icon={<ReloadOutlined />} onClick={() => fetchLogs(currentPage)} loading={loading}>
              {t('firewallPage.refreshList')}
            </Button>
          </Col>
        </Row>
      </Card>
      
      {/* Attack Log List */}
      <Card className="card" title={t('firewallPage.logListTitle')}>
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
            showTotal: (total) => t('firewallPage.totalRecords', { total }),
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
