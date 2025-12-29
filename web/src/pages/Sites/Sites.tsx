import React, { useState, useEffect } from 'react'
import { Card, Table, Button, Modal, Form, Input, Switch, message, Select, Row, Col, Statistic, Upload, Typography, Space } from 'antd'
import { PlusOutlined, EditOutlined, DeleteOutlined, EyeOutlined, UploadOutlined, UnorderedListOutlined, FileTextOutlined, CloudUploadOutlined } from '@ant-design/icons'
import { sitesApi } from '../../services/api'
import type { UploadProps } from 'antd'

const { Option } = Select

const Sites: React.FC = () => {
  const [sites, setSites] = useState<any[]>([])
  const [loading, setLoading] = useState(false)
  const [visible, setVisible] = useState(false)
  const [uploadModalVisible, setUploadModalVisible] = useState(false)
  const [editingSite, setEditingSite] = useState<any>(null)
  const [selectedSite, setSelectedSite] = useState<any>(null)
  const [form] = Form.useForm()
  const [uploading, setUploading] = useState(false)

  // 表格列配置
  const columns = [
    {
      title: '站点名称',
      dataIndex: 'name',
      key: 'name',
    },
    {
      title: '域名',
      dataIndex: 'domain',
      key: 'domain',
    },
    {
      title: '预渲染状态',
      dataIndex: 'prerender.enabled',
      key: 'prerenderEnabled',
      render: (enabled: boolean) => (
        <Switch checked={enabled} disabled={true} />
      ),
    },
    {
      title: '防火墙状态',
      dataIndex: 'firewall.enabled',
      key: 'firewallEnabled',
      render: (enabled: boolean) => (
        <Switch checked={enabled} disabled={true} />
      ),
    },
    {
      title: 'SSL状态',
      dataIndex: 'ssl.enabled',
      key: 'sslEnabled',
      render: (enabled: boolean) => (
        <Switch checked={enabled} disabled={true} />
      ),
    },
    {
      title: '操作',
      key: 'action',
      render: (_, record: any) => (
        <div>
          <Button
            type="link"
            icon={<EyeOutlined />}
            onClick={() => handleView(record)}
            style={{ marginRight: 8 }}
          >
            查看
          </Button>
          <Button
            type="link"
            icon={<EditOutlined />}
            onClick={() => handleEdit(record)}
            style={{ marginRight: 8 }}
          >
            编辑
          </Button>
          <Button
            type="link"
            icon={<UploadOutlined />}
            onClick={() => handleFileUpload(record)}
            style={{ marginRight: 8 }}
          >
            文件管理
          </Button>
          <Button
            type="link"
            icon={<DeleteOutlined />}
            danger
            onClick={() => handleDelete(record)}
          >
            删除
          </Button>
        </div>
      ),
    },
  ]

  // 获取站点列表
  const fetchSites = async () => {
    try {
      setLoading(true)
      const res = await sitesApi.getSites()
      if (res.code === 200) {
        setSites(res.data)
      } else {
        message.error('获取站点列表失败')
      }
    } catch (error) {
      console.error('Failed to fetch sites:', error)
      message.error('获取站点列表失败')
    } finally {
      setLoading(false)
    }
  }

  // 初始化数据
  useEffect(() => {
    fetchSites()
  }, [])

  // 打开添加/编辑弹窗
  const showModal = (site: any = null) => {
    setEditingSite(site)
    if (site) {
      form.setFieldsValue(site)
    } else {
      form.resetFields()
    }
    setVisible(true)
  }

  // 处理添加站点
  const handleAdd = () => {
    showModal()
  }

  // 处理编辑站点
  const handleEdit = (site: any) => {
    showModal(site)
  }

  // 处理查看站点详情
  const handleView = (site: any) => {
    message.info('查看站点详情功能开发中')
  }

  // 处理文件上传
  const handleFileUpload = (site: any) => {
    setSelectedSite(site)
    setUploadModalVisible(true)
  }

  // 文件上传前的处理
  const beforeUpload: UploadProps['beforeUpload'] = (file) => {
    // 检查文件类型
    const isCompressed = file.type === 'application/zip' || file.name.endsWith('.rar') || file.name.endsWith('.zip')
    const isSingleFile = !isCompressed
    
    // 这里可以添加文件大小限制
    const isLt20M = file.size / 1024 / 1024 < 20
    if (!isLt20M) {
      message.error('文件大小不能超过20MB')
      return Upload.LIST_IGNORE
    }
    
    return true
  }

  // 文件上传进度处理
  const handleUploadProgress = (percentage: number) => {
    console.log('Upload progress:', percentage)
  }

  // 文件上传成功处理
  const handleUploadSuccess = (response: any, file: any) => {
    message.success(`${file.name} 上传成功`)
    // 这里可以添加文件上传后的处理逻辑，例如刷新文件列表
  }

  // 文件上传失败处理
  const handleUploadError = (error: any, file: any) => {
    message.error(`${file.name} 上传失败: ${error.message}`)
  }

  // 自定义上传逻辑
  const customRequest: UploadProps['customRequest'] = (options) => {
    const { onSuccess, onError, file, onProgress } = options
    
    setUploading(true)
    
    // 模拟文件上传
    setTimeout(() => {
      // 检查是否是压缩文件
      const isCompressed = file.type === 'application/zip' || file.name.endsWith('.rar') || file.name.endsWith('.zip')
      
      if (isCompressed) {
        // 模拟解压处理
        message.info(`正在解压 ${file.name}...`)
        setTimeout(() => {
          message.success(`${file.name} 解压成功`)
          onSuccess({ status: 'ok', message: '解压成功' })
          setUploading(false)
        }, 1000)
      } else {
        // 模拟普通文件上传
        onSuccess({ status: 'ok', message: '上传成功' })
        setUploading(false)
      }
    }, 1500)
  }

  // 处理删除站点
  const handleDelete = async (site: any) => {
    try {
      const res = await sitesApi.deleteSite(site.name)
      if (res.code === 200) {
        message.success('删除站点成功')
        fetchSites()
      } else {
        message.error('删除站点失败')
      }
    } catch (error) {
      console.error('Failed to delete site:', error)
      message.error('删除站点失败')
    }
  }

  // 处理表单提交
  const handleSubmit = async () => {
    try {
      const values = await form.validateFields()
      let res

      if (editingSite) {
        // 更新站点
        res = await sitesApi.updateSite(editingSite.name, values)
      } else {
        // 添加站点
        res = await sitesApi.addSite(values)
      }

      if (res.code === 200) {
        message.success(editingSite ? '更新站点成功' : '添加站点成功')
        setVisible(false)
        fetchSites()
      } else {
        message.error(editingSite ? '更新站点失败' : '添加站点失败')
      }
    } catch (error) {
      console.error('Form submission error:', error)
      message.error('表单提交失败')
    }
  }

  return (
    <div>
      <h1 className="page-title">站点管理</h1>

      {/* 站点概览卡片 */}
      <Card className="card">
        <Row gutter={[16, 16]}>
          <Col span={8}>
            <Statistic
              title="总站点数"
              value={sites.length}
              valueStyle={{ color: '#1890ff' }}
            />
          </Col>
          <Col span={8}>
            <Statistic
              title="启用预渲染的站点"
              value={sites.filter(site => site.prerender.enabled).length}
              valueStyle={{ color: '#52c41a' }}
            />
          </Col>
          <Col span={8}>
            <Statistic
              title="启用防火墙的站点"
              value={sites.filter(site => site.firewall.enabled).length}
              valueStyle={{ color: '#faad14' }}
            />
          </Col>
        </Row>
      </Card>

      {/* 站点列表 */}
      <Card className="card" title="站点列表" extra={
        <Button type="primary" icon={<PlusOutlined />} onClick={handleAdd}>
          添加站点
        </Button>
      }>
        <Table
          columns={columns}
          dataSource={sites}
          rowKey="name"
          loading={loading}
          pagination={{ pageSize: 10 }}
        />
      </Card>

      {/* 添加/编辑站点弹窗 */}
      <Modal
        title={editingSite ? '编辑站点' : '添加站点'}
        open={visible}
        onOk={handleSubmit}
        onCancel={() => setVisible(false)}
        width={600}
      >
        <Form
          form={form}
          layout="vertical"
          initialValues={{}}
        >
          <Form.Item
            name="name"
            label="站点名称"
            rules={[{ required: true, message: '请输入站点名称' }]}
          >
            <Input placeholder="请输入站点名称" />
          </Form.Item>

          <Form.Item
            name="domain"
            label="域名"
            rules={[{ required: true, message: '请输入域名' }]}
          >
            <Input placeholder="请输入域名，例如：example.com" />
          </Form.Item>

          {/* 预渲染配置 */}
          <Card title="预渲染配置" size="small" style={{ marginBottom: 16 }}>
            <Form.Item name={['prerender', 'enabled']} label="启用预渲染" valuePropName="checked">
              <Switch />
            </Form.Item>

            <Form.Item name={['prerender', 'poolSize']} label="浏览器池大小">
              <Input type="number" placeholder="请输入浏览器池大小" />
            </Form.Item>

            <Form.Item name={['prerender', 'timeout']} label="渲染超时(秒)">
              <Input type="number" placeholder="请输入渲染超时时间" />
            </Form.Item>

            <Form.Item name={['prerender', 'cacheTTL']} label="缓存TTL(秒)">
              <Input type="number" placeholder="请输入缓存TTL" />
            </Form.Item>
          </Card>

          {/* 防火墙配置 */}
          <Card title="防火墙配置" size="small" style={{ marginBottom: 16 }}>
            <Form.Item name={['firewall', 'enabled']} label="启用防火墙" valuePropName="checked">
              <Switch />
            </Form.Item>

            <Form.Item name={['firewall', 'action', 'defaultAction']} label="默认动作">
              <Select>
                <Option value="allow">允许</Option>
                <Option value="block">阻止</Option>
              </Select>
            </Form.Item>
          </Card>

          {/* SSL配置 */}
          <Card title="SSL配置" size="small">
            <Form.Item name={['ssl', 'enabled']} label="启用SSL" valuePropName="checked">
              <Switch />
            </Form.Item>
          </Card>
        </Form>
      </Modal>

      {/* 文件上传弹窗 */}
      <Modal
        title={`站点 "${selectedSite?.name}" 文件管理`}
        open={uploadModalVisible}
        onCancel={() => setUploadModalVisible(false)}
        width={800}
        footer={null}
      >
        <div style={{ marginBottom: 20 }}>
          <Typography.Title level={5} style={{ marginBottom: 10 }}>
            文件上传
          </Typography.Title>
          <Typography.Text type="secondary">
            支持拖拽上传，RAR/ZIP文件将自动解压，单个文件直接上传
          </Typography.Text>
        </div>
        
        {/* 拖拽上传区域 */}
        <Upload
          name="file"
          beforeUpload={beforeUpload}
          customRequest={customRequest}
          onSuccess={handleUploadSuccess}
          onError={handleUploadError}
          accept=".zip,.rar,.html,.css,.js,.json,.txt"
          multiple
          listType="text"
          showUploadList={{ showRemoveIcon: true, showPreviewIcon: true }}
        >
          <div style={{
            border: '1px dashed #d9d9d9',
            borderRadius: '6px',
            padding: '50px 20px',
            textAlign: 'center',
            background: '#fafafa',
            cursor: 'pointer',
            marginBottom: '20px',
          }}>
            <Space direction="vertical" align="center">
              <CloudUploadOutlined style={{ fontSize: '32px', color: '#1890ff' }} />
              <Typography.Text>
                拖拽文件到此处或
                <Button type="link" size="small">
                  点击上传
                </Button>
              </Typography.Text>
              <Typography.Text type="secondary" style={{ fontSize: '12px' }}>
                支持 .zip, .rar, .html, .css, .js, .json, .txt 格式，单个文件不超过20MB
              </Typography.Text>
            </Space>
          </div>
        </Upload>
        
        {/* 文件列表 */}
        <div style={{ marginBottom: 20 }}>
          <Typography.Title level={5} style={{ marginBottom: 10 }}>
            <UnorderedListOutlined /> 文件列表
          </Typography.Title>
          <Card bordered={false}>
            <Table
              columns={[
                {
                  title: '文件名',
                  dataIndex: 'name',
                  key: 'name',
                  render: (text: string) => <FileTextOutlined /> {text},
                },
                {
                  title: '大小',
                  dataIndex: 'size',
                  key: 'size',
                  render: (size: number) => `${(size / 1024).toFixed(2)} KB`,
                },
                {
                  title: '类型',
                  dataIndex: 'type',
                  key: 'type',
                },
                {
                  title: '上传时间',
                  dataIndex: 'uploadTime',
                  key: 'uploadTime',
                },
                {
                  title: '操作',
                  key: 'action',
                  render: () => (
                    <Space>
                      <Button type="link" size="small">查看</Button>
                      <Button type="link" danger size="small">删除</Button>
                    </Space>
                  ),
                },
              ]}
              dataSource={[
                {
                  key: '1',
                  name: 'index.html',
                  size: 10240,
                  type: 'HTML',
                  uploadTime: '2025-12-29 19:00:00',
                },
                {
                  key: '2',
                  name: 'style.css',
                  size: 5120,
                  type: 'CSS',
                  uploadTime: '2025-12-29 19:00:00',
                },
                {
                  key: '3',
                  name: 'script.js',
                  size: 8192,
                  type: 'JavaScript',
                  uploadTime: '2025-12-29 19:00:00',
                },
              ]}
              pagination={false}
              size="small"
            />
          </Card>
        </div>
      </Modal>
    </div>
  )
}

export default Sites
