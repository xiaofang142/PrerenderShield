import React, { useState, useEffect } from 'react'
import { Card, Table, Select, message, Spin } from 'antd'
import { firewallApi } from '../../services/api'
import dayjs from 'dayjs'

const { Option } = Select

const Logs: React.FC = () => {
  const [logs, setLogs] = useState<any[]>([])
  const [total, setTotal] = useState(0)
  const [loading, setLoading] = useState(false)
  const [currentPage, setCurrentPage] = useState(1)
  const [pageSize, setPageSize] = useState(20)

  const columns = [
    { title: '时间', dataIndex: 'time', key: 'time', render: (t: string) => dayjs(t).format('YYYY-MM-DD HH:mm:ss') },
    { title: 'IP', dataIndex: 'ip', key: 'ip' },
    { title: '方法', dataIndex: 'method', key: 'method', width: 80 },
    { title: '路径', dataIndex: 'path', key: 'path', ellipsis: true },
    { title: '状态码', dataIndex: 'status', key: 'status', width: 80 },
    { title: '耗时(ms)', dataIndex: 'duration', key: 'duration', width: 100 },
  ]

  const fetchLogs = async (page = 1) => {
    try {
      setLoading(true)
      const res = await firewallApi.getAccessLogs({ page, limit: pageSize })
      if (res.code === 200) {
        setLogs(res.data.logs || [])
        setTotal(res.data.total || 0)
        setCurrentPage(page)
      }
    } catch (error) {
      console.error('Failed to fetch access logs:', error)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => { fetchLogs() }, [pageSize])

  return (
    <Spin spinning={loading}>
      <div>
        <h1 className="page-title">日志管理</h1>
        <Card className="card">
          <Table
            columns={columns}
            dataSource={logs}
            rowKey="id"
            pagination={{
              current: currentPage,
              pageSize,
              total,
              onChange: (p, ps) => { setCurrentPage(p); setPageSize(ps); fetchLogs(p) },
              showSizeChanger: true,
              showTotal: (t) => `共 ${t} 条记录`,
            }}
            size="middle"
          />
        </Card>
      </div>
    </Spin>
  )
}

export default Logs
