import React, { useState, useEffect } from 'react'
import { Card, Table, Button, Modal, Form, Input, Switch, message, Select, Row, Col, Statistic, Upload, Typography, Space, Menu, Popconfirm } from 'antd'
import { 
  PlusOutlined, EditOutlined, DeleteOutlined, EyeOutlined, UploadOutlined, 
  UnorderedListOutlined, CloudUploadOutlined, FolderOpenOutlined, 
  FolderOutlined, FileOutlined, FolderOutlined as NewFolderOutlined, FileAddOutlined, UpOutlined, 
  DownloadOutlined, UnorderedListOutlined as MoveOutlined, UnorderedListOutlined as ExtractOutlined
} from '@ant-design/icons'
import { sitesApi, prerenderApi } from '../../services/api'
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
  const [certs, setCerts] = useState<string[]>([])
  const [sslLoading, setSslLoading] = useState(false)
  
  // 静态资源管理状态
  const [staticResModalVisible, setStaticResModalVisible] = useState(false)
  const [currentSite, setCurrentSite] = useState<any>(null)
  const [currentPath, setCurrentPath] = useState<string>('/')
  const [fileList, setFileList] = useState<any[]>([])
  const [selectedFile, setSelectedFile] = useState<any>(null)
  const [showNewFolderModal, setShowNewFolderModal] = useState(false)
  const [newFolderName, setNewFolderName] = useState<string>('')
  const [showNewFileModal, setShowNewFileModal] = useState(false)
  const [newFileName, setNewFileName] = useState<string>('')
  
  // 预渲染配置弹窗状态
  const [prerenderConfigModalVisible, setPrerenderConfigModalVisible] = useState(false)
  const [currentPrerenderSite, setCurrentPrerenderSite] = useState<any>(null)
  const [prerenderConfigForm] = Form.useForm()
  // 默认爬虫协议头列表
  const defaultCrawlerHeaders = [
    'Googlebot',
    'Bingbot',
    'Slurp',
    'DuckDuckBot',
    'Baiduspider',
    'Sogou spider',
    'YandexBot',
    'Exabot',
    'FacebookBot',
    'Twitterbot',
    'LinkedInBot',
    'WhatsAppBot',
    'TelegramBot',
    'DiscordBot',
    'PinterestBot',
    'InstagramBot',
    'Google-InspectionTool',
    'Google-Site-Verification',
    'AhrefsBot',
    'SEMrushBot',
    'Majestic',
    'Yahoo! Slurp'
  ]

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
      dataIndex: 'prerenderEnabled',
      key: 'prerenderEnabled',
      render: (enabled: boolean, record: any) => (
        <Switch checked={enabled} onChange={(checked) => handleSwitchChange(record, 'prerender', checked)} />
      ),
    },
    {
      title: '防火墙状态',
      dataIndex: 'firewallEnabled',
      key: 'firewallEnabled',
      render: (enabled: boolean, record: any) => (
        <Switch checked={enabled} onChange={(checked) => handleSwitchChange(record, 'firewall', checked)} />
      ),
    },
    {
      title: 'SSL状态',
      dataIndex: 'sslEnabled',
      key: 'sslEnabled',
      render: (enabled: boolean, record: any) => (
        <Switch checked={enabled} onChange={(checked) => handleSwitchChange(record, 'ssl', checked)} />
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
            icon={<FolderOpenOutlined />}
            onClick={() => handleStaticResources(record)}
            style={{ marginRight: 8 }}
          >
            静态资源
          </Button>
          <Button
            type="link"
            icon={<UnorderedListOutlined />}
            onClick={() => handlePrerenderConfig(record)}
            style={{ marginRight: 8 }}
          >
            预渲染配置
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
      console.log('Raw API response:', res);
      if (res.code === 200) {
        console.log('Sites data from API:', res.data);
        // 转换API响应中的大写键为小写键，以便Table组件正确显示数据
        const normalizedSites = res.data.map((site: any) => {
          // 直接从API响应中获取开关状态，确保数据一致性
          const firewallEnabled = site.Firewall && site.Firewall.Enabled === true;
          const prerenderEnabled = site.Prerender && site.Prerender.Enabled === true;
          const sslEnabled = site.SSL && site.SSL.Enabled === true;
          
          // 为了调试，打印每个站点的开关状态
          console.log(`站点: ${site.Name}`);
          console.log(`  API返回的防火墙状态: ${site.Firewall?.Enabled}`);
          console.log(`  转换后的防火墙状态: ${firewallEnabled}`);
          console.log(`  API返回的预渲染状态: ${site.Prerender?.Enabled}`);
          console.log(`  转换后的预渲染状态: ${prerenderEnabled}`);
          console.log(`  API返回的SSL状态: ${site.SSL?.Enabled}`);
          console.log(`  转换后的SSL状态: ${sslEnabled}`);
          
          const transformedSite = {
            name: site.Name || '',
            domain: site.Domain || '',
            port: site.Port || 80,
            proxy: {
              enabled: site.PROXY?.Enabled || false,
              targetURL: site.PROXY?.TargetURL || '',
              type: site.PROXY?.Type || 'direct'
            },
            firewallEnabled: firewallEnabled,
            prerenderEnabled: prerenderEnabled,
            sslEnabled: sslEnabled,
            firewall: {
              enabled: firewallEnabled,
              rulesPath: site.Firewall?.RulesPath || '/etc/prerender-shield/rules',
              action: {
                defaultAction: site.Firewall?.ActionConfig?.DefaultAction || 'block',
                blockMessage: site.Firewall?.ActionConfig?.BlockMessage || 'Request blocked by firewall'
              }
            },
            prerender: {
              enabled: prerenderEnabled,
              poolSize: site.Prerender?.PoolSize || 5,
              minPoolSize: site.Prerender?.MinPoolSize || 2,
              maxPoolSize: site.Prerender?.MaxPoolSize || 20,
              timeout: site.Prerender?.Timeout || 30,
              cacheTTL: site.Prerender?.CacheTTL || 3600,
              idleTimeout: site.Prerender?.IdleTimeout || 300,
              dynamicScaling: site.Prerender?.DynamicScaling || true,
              scalingFactor: site.Prerender?.ScalingFactor || 0.5,
              scalingInterval: site.Prerender?.ScalingInterval || 60,
              preheat: {
                enabled: site.Prerender?.Preheat?.Enabled || false,
                sitemapURL: site.Prerender?.Preheat?.SitemapURL || '',
                schedule: site.Prerender?.Preheat?.Schedule || '0 0 * * *',
                concurrency: site.Prerender?.Preheat?.Concurrency || 5,
                defaultPriority: site.Prerender?.Preheat?.DefaultPriority || 0
              }
            },
            routing: {
              rules: site.Routing?.Rules || []
            },
            ssl: {
              enabled: sslEnabled,
              letEncrypt: site.SSL?.LetEncrypt || false,
              domains: site.SSL?.Domains || [],
              acmeEmail: site.SSL?.ACMEEmail || '',
              acmeServer: site.SSL?.ACMEServer || 'https://acme-v02.api.letsencrypt.org/directory',
              acmeChallenge: site.SSL?.ACMEChallenge || 'http01',
              certPath: site.SSL?.CertPath || '/etc/prerender-shield/certs/cert.pem',
              keyPath: site.SSL?.KeyPath || '/etc/prerender-shield/certs/key.pem',
              sslCertificate: site.SSL?.SSLCertificate || ''
            }
          };
          
          return transformedSite;
        });
        
        // 调试：打印转换后的站点列表
        console.log('转换后的站点列表:', normalizedSites);
        setSites(normalizedSites)
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
  
  // 获取所有证书列表
  const fetchCerts = async () => {
    try {
      setSslLoading(true)
      // 这里应该获取所有可用证书列表
      // 暂时模拟数据
      const mockCerts = ['example.com', 'test.com', 'demo.com']
      setCerts(mockCerts)
    } catch (error) {
      console.error('Failed to fetch certs:', error)
      message.error('获取证书列表失败')
    } finally {
      setSslLoading(false)
    }
  }

  // 初始化数据
  useEffect(() => {
    fetchSites()
    fetchCerts()
  }, [])

  // 打开添加/编辑弹窗
  const showModal = (site: any = null) => {
    setEditingSite(site)
    if (site) {
      // 创建一个新对象，将端口转换为string类型，解决Input组件type="number"期望string类型的问题
      const siteWithStringPort = {
        ...site,
        port: String(site.port)
      };
      form.setFieldsValue(siteWithStringPort)
    } else {
      form.resetFields()
    }
    setVisible(true)
  }

  // 处理添加站点
  const handleAdd = () => {
    showModal()
  }

  // 处理开关变化
  const handleSwitchChange = async (record: any, type: 'prerender' | 'firewall' | 'ssl', enabled: boolean) => {
    try {
      // 确保record有name属性
      if (!record || !record.name) {
        throw new Error('站点名称不存在')
      }
      
      // 创建更新后的站点数据
      const updatedSite = {
        ...record,
        [type]: {
          ...record[type],
          enabled
        }
      }
      
      // 转换为后端API期望的格式（大写键）
      const apiSiteData = {
        Name: updatedSite.name,
        Domain: updatedSite.domain,
        Port: updatedSite.port || 80, // 保留端口信息，默认为80
        Proxy: {
          Enabled: true, // 默认启用代理
          TargetURL: '',
          Type: 'direct'
        },
        Firewall: {
          Enabled: updatedSite.firewall.enabled,
          RulesPath: updatedSite.firewall.rulesPath,
          ActionConfig: {
            DefaultAction: updatedSite.firewall.action.defaultAction,
            BlockMessage: updatedSite.firewall.action.blockMessage
          }
        },
        Prerender: {
          Enabled: updatedSite.prerender.enabled,
          PoolSize: updatedSite.prerender.poolSize,
          MinPoolSize: updatedSite.prerender.minPoolSize,
          MaxPoolSize: updatedSite.prerender.maxPoolSize,
          Timeout: updatedSite.prerender.timeout,
          CacheTTL: updatedSite.prerender.cacheTTL,
          IdleTimeout: updatedSite.prerender.idleTimeout,
          DynamicScaling: updatedSite.prerender.dynamicScaling,
          ScalingFactor: updatedSite.prerender.scalingFactor,
          ScalingInterval: updatedSite.prerender.scalingInterval,
          Preheat: {
            Enabled: updatedSite.prerender.preheat.enabled,
            SitemapURL: updatedSite.prerender.preheat.sitemapURL,
            Schedule: updatedSite.prerender.preheat.schedule,
            Concurrency: updatedSite.prerender.preheat.concurrency,
            DefaultPriority: updatedSite.prerender.preheat.defaultPriority
          }
        },
        Routing: {
          Rules: updatedSite.routing.rules
        },
        SSL: {
          Enabled: updatedSite.ssl.enabled,
          LetEncrypt: updatedSite.ssl.letEncrypt,
          Domains: updatedSite.ssl.domains,
          ACMEEmail: updatedSite.ssl.acmeEmail,
          ACMEServer: updatedSite.ssl.acmeServer,
          ACMEChallenge: updatedSite.ssl.acmeChallenge,
          CertPath: updatedSite.ssl.certPath,
          KeyPath: updatedSite.ssl.keyPath
        }
      }

      // 更新站点
      const res = await sitesApi.updateSite(record.name, apiSiteData)
      if (res.code === 200) {
        message.success('更新站点成功')
        fetchSites() // 刷新站点列表
      } else {
        message.error('更新站点失败')
      }
    } catch (error) {
      console.error('Switch change error:', error)
      message.error('更新失败')
    }
  }

  // 处理编辑站点
  const handleEdit = (site: any) => {
    showModal(site)
  }

  // 处理查看站点详情
  const handleView = (site: any) => {
    // 确保site、site.domain和site.port存在
    if (!site || typeof site.domain === 'undefined' || site.domain === '') {
      console.error('Invalid site domain, cannot open preview')
      message.error('站点域名无效，无法打开预览')
      return
    }
    
    // 调试：打印site对象，查看domain和port属性
    console.log('View site:', site);
    
    // 打开新窗口预览站点，80端口不拼接，其他端口需要拼接
    const port = site.port || 80;
    const domain = site.domain;
    const url = port === 80 
      ? `http://${domain}` 
      : `http://${domain}:${port}`;
    window.open(url, '_blank')
  }

  // 打开静态资源管理弹窗
  const handleStaticResources = (site: any) => {
    // 确保site和site.name存在
    if (!site || typeof site.name === 'undefined' || site.name === '') {
      console.error('Invalid site provided, cannot open static resources')
      message.error('站点信息无效，无法打开静态资源管理')
      return
    }
    
    setCurrentSite(site)
    setCurrentPath('/')
    setSelectedFile(null)
    setStaticResModalVisible(true)
    // 直接传递site.name给loadFileList，避免依赖currentSite的异步更新
    loadFileList('/', site.name)
  }

  // 加载当前路径下的文件列表
  const loadFileList = async (path: string, siteName?: string) => {
    // 优先使用传入的siteName，否则使用currentSite.name
    const finalSiteName = siteName || (currentSite && currentSite.name)
    
    // 确保站点名称存在
    if (typeof finalSiteName === 'undefined' || finalSiteName === '') {
      console.error('Site name is not available, cannot load file list')
      return
    }
    
    try {
      // 发送API请求获取文件列表
      const response = await sitesApi.getFileList(finalSiteName, path)
      if (response.code === 200) {
        setFileList(response.data)
      } else {
        message.error('获取文件列表失败')
      }
    } catch (error) {
      console.error('Failed to load file list:', error)
      // 如果目录不存在，不显示错误提示，而是显示空列表
      setFileList([])
    }
  }

  // 导航到上一级目录
  const navigateUp = () => {
    if (currentPath === '/') return
    const parentPath = currentPath.substring(0, currentPath.lastIndexOf('/')) || '/'
    setCurrentPath(parentPath)
    loadFileList(parentPath)
  }

  // 进入子目录
  const enterDirectory = (dir: any) => {
    const newPath = currentPath === '/' ? `/${dir.name}` : `${currentPath}/${dir.name}`
    setCurrentPath(newPath)
    loadFileList(newPath)
  }

  // 新建目录
  const handleNewFolder = () => {
    setShowNewFolderModal(true)
    setNewFolderName('')
  }

  // 确认新建目录
  const confirmNewFolder = () => {
    if (!newFolderName.trim()) {
      message.warning('请输入目录名称')
      return
    }
    
    // 模拟新建目录
    const newDir = {
      key: Date.now().toString(),
      name: newFolderName,
      type: 'dir',
      size: 0,
      path: `${currentPath === '/' ? '' : currentPath}/${newFolderName}`
    }
    
    setFileList(prev => [...prev, newDir])
    setShowNewFolderModal(false)
    message.success('目录创建成功')
  }

  // 新建文件
  const handleNewFile = () => {
    setShowNewFileModal(true)
    setNewFileName('')
  }

  // 确认新建文件
  const confirmNewFile = () => {
    if (!newFileName.trim()) {
      message.warning('请输入文件名称')
      return
    }
    
    // 模拟新建文件
    const newFile = {
      key: Date.now().toString(),
      name: newFileName,
      type: 'file',
      size: 0,
      path: `${currentPath === '/' ? '' : currentPath}/${newFileName}`
    }
    
    setFileList(prev => [...prev, newFile])
    setShowNewFileModal(false)
    message.success('文件创建成功')
  }

  // 处理文件上传
  const handleStaticResUpload = (file: any) => {
    // 模拟文件上传
    message.success(`${file.name} 上传成功`)
    const newFile = {
      key: Date.now().toString() + Math.random().toString(36).substr(2, 5),
      name: file.name,
      type: 'file',
      size: file.size,
      path: `${currentPath === '/' ? '' : currentPath}/${file.name}`
    }
    setFileList(prev => [...prev, newFile])
    return false // 阻止默认上传
  }

  // 下载文件
  const handleDownload = (file: any) => {
    message.info(`正在下载 ${file.name}`)
    // 创建临时下载链接
    const downloadLink = document.createElement('a');
    downloadLink.href = `/api/sites/${currentSite?.name}/static${file.path}`;
    downloadLink.download = file.name;
    downloadLink.target = '_blank';
    document.body.appendChild(downloadLink);
    downloadLink.click();
    document.body.removeChild(downloadLink);
  }

  // 解压文件
  const handleExtract = async (file: any) => {
    // 确保currentSite和currentSite.name存在
    if (!currentSite || typeof currentSite.name === 'undefined' || currentSite.name === '') {
      console.error('Current site is not set, cannot extract file')
      message.error('站点信息无效，无法解压文件')
      return
    }
    
    try {
      message.info(`正在解压 ${file.name}...`)
      
      // 发送解压请求到后端
      const response = await sitesApi.extractFile(currentSite.name, file.name, currentPath)
      
      if (response.code === 200) {
        message.success(`${file.name} 解压成功`)
        // 重新加载文件列表
        loadFileList(currentPath)
      } else {
        message.error(`${file.name} 解压失败: ${response.message}`)
      }
    } catch (error) {
      console.error('解压失败:', error)
      message.error(`${file.name} 解压失败`)
    }
  }

  // 移动文件
  const handleMove = (file: any) => {
    message.info(`正在移动 ${file.name}`)
    // 这里可以添加实际的移动逻辑
  }

  // 删除文件/目录
  const handleFileDelete = (file: any) => {
    message.success(`${file.name} 删除成功`)
    setFileList(prev => prev.filter(f => f.key !== file.key))
  }

  // 文件右键菜单
  const getFileMenu = (file: any) => {
    const menu = [
      {
        key: 'download',
        label: (
          <a onClick={() => handleDownload(file)}>
            <DownloadOutlined /> 下载
          </a>
        ),
      },
    ]

    if (file.type === 'file' && (file.name.endsWith('.zip') || file.name.endsWith('.rar'))) {
      menu.push({
        key: 'extract',
        label: (
          <a onClick={() => handleExtract(file)}>
            <ExtractOutlined /> 解压
          </a>
        ),
      })
    }

    menu.push(
      {
        key: 'move',
        label: (
          <a onClick={() => handleMove(file)}>
            <MoveOutlined /> 移动
          </a>
        ),
      },
      {
                key: 'delete',
                label: (
                  <Popconfirm
                    title="确定要删除吗？"
                    onConfirm={() => handleFileDelete(file)}
                    okText="确定"
                    cancelText="取消"
                  >
                    <a style={{ color: '#ff4d4f' }}>
                      <DeleteOutlined /> 删除
                    </a>
                  </Popconfirm>
                ),
              }
    )

    return (
      <Menu items={menu} />
    )
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
    
    // 调整rar/zip上传大小限制为不超过100m
    const isLt100M = file.size / 1024 / 1024 < 100
    if (!isLt100M) {
      message.error('文件大小不能超过100MB')
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
    
    // 确保currentSite和currentSite.name存在
    if (!currentSite || typeof currentSite.name === 'undefined' || currentSite.name === '') {
      console.error('Current site is not set, cannot upload file')
      message.error('站点信息无效，无法上传文件')
      onError(new Error('站点信息无效'))
      setUploading(false)
      return
    }
    
    setUploading(true)
    
    // 发送实际的API请求
    sitesApi.uploadFile(currentSite.name, file, currentPath, (progressEvent) => {
      if (progressEvent.total) {
        const percentCompleted = Math.round((progressEvent.loaded * 100) / progressEvent.total);
        onProgress({ percent: percentCompleted });
      }
    })
    .then((response) => {
      if (response.code === 200) {
        message.success(`${file.name} 上传成功`)
        // 重新加载文件列表
        loadFileList(currentPath)
        onSuccess({ status: 'ok', message: '上传成功' })
      } else {
        throw new Error(response.message || '上传失败')
      }
    })
    .catch((error) => {
      message.error(`${file.name} 上传失败: ${error.message}`)
      onError(error)
    })
    .finally(() => {
      setUploading(false)
    })
  }

  // 处理删除站点
  const handleDelete = async (site: any) => {
    try {
      // 确保site有name属性
      if (!site || !site.name) {
        throw new Error('站点名称不存在')
      }
      
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
  
  // 打开预渲染配置弹窗
  const handlePrerenderConfig = (site: any) => {
    setCurrentPrerenderSite(site)
    // 初始化表单值
    const initialValues = {
      enabled: site.prerender.enabled,
      poolSize: site.prerender.poolSize,
      minPoolSize: site.prerender.minPoolSize,
      maxPoolSize: site.prerender.maxPoolSize,
      timeout: site.prerender.timeout,
      cacheTTL: site.prerender.cacheTTL,
      idleTimeout: site.prerender.idleTimeout,
      dynamicScaling: site.prerender.dynamicScaling,
      scalingFactor: site.prerender.scalingFactor,
      scalingInterval: site.prerender.scalingInterval,
      useDefaultHeaders: site.prerender.useDefaultHeaders || false,
      crawlerHeaders: site.prerender.crawlerHeaders || [],
      preheat: {
        enabled: site.prerender.preheat.enabled,
        sitemapURL: site.prerender.preheat.sitemapURL,
        schedule: site.prerender.preheat.schedule,
        concurrency: site.prerender.preheat.concurrency,
        defaultPriority: site.prerender.preheat.defaultPriority
      }
    }
    prerenderConfigForm.setFieldsValue(initialValues)
    setPrerenderConfigModalVisible(true)
  }

  // 处理表单提交
  const handleSubmit = async () => {
    try {
      const values = await form.validateFields()
      
      // 转换表单数据格式，确保与后端API期望的结构匹配
      const siteData = {
        Name: values.name,
        Domain: values.domain,
        Port: parseInt(values.port, 10) || 80, // 转换为整数类型，默认为80
        // 添加Proxy字段，与后端SiteConfig结构体匹配
        Proxy: {
          Enabled: values.proxy?.enabled || false,
          TargetURL: values.proxy?.targetURL || "",
          Type: values.proxy?.type || "direct"
        },
        Firewall: {
          Enabled: values.firewall.enabled || false,
          RulesPath: values.firewall.rulesPath || '/etc/prerender-shield/rules',
          ActionConfig: {
            DefaultAction: values.firewall.action?.defaultAction || 'block',
            BlockMessage: values.firewall.action?.blockMessage || 'Request blocked by firewall'
          }
        },
        Prerender: {
          Enabled: values.prerender.enabled || false,
          PoolSize: values.prerender.poolSize || 5,
          MinPoolSize: values.prerender.minPoolSize || 2,
          MaxPoolSize: values.prerender.maxPoolSize || 20,
          Timeout: values.prerender.timeout || 30,
          CacheTTL: values.prerender.cacheTTL || 3600,
          IdleTimeout: values.prerender.idleTimeout || 300,
          DynamicScaling: values.prerender.dynamicScaling || true,
          ScalingFactor: values.prerender.scalingFactor || 0.5,
          ScalingInterval: values.prerender.scalingInterval || 60,
          UseDefaultHeaders: values.prerender.useDefaultHeaders || false,
          CrawlerHeaders: values.prerender.crawlerHeaders || [],
          Preheat: {
            Enabled: values.prerender.preheat?.enabled || false,
            SitemapURL: values.prerender.preheat?.sitemapURL || '',
            Schedule: values.prerender.preheat?.schedule || '0 0 * * *',
            Concurrency: values.prerender.preheat?.concurrency || 5,
            DefaultPriority: values.prerender.preheat?.defaultPriority || 0
          }
        },
        Routing: {
          Rules: values.routing?.rules || []
        },
        SSL: {
          Enabled: values.ssl?.enabled || false,
          LetEncrypt: values.ssl?.letEncrypt || false,
          Domains: values.ssl?.domains || [],
          ACMEEmail: values.ssl?.acmeEmail || '',
          ACMEServer: values.ssl?.acmeServer || 'https://acme-v02.api.letsencrypt.org/directory',
          ACMEChallenge: values.ssl?.acmeChallenge || 'http01',
          CertPath: values.ssl?.certPath || '/etc/prerender-shield/certs/cert.pem',
          KeyPath: values.ssl?.keyPath || '/etc/prerender-shield/certs/key.pem',
          SSLCertificate: values.ssl?.sslCertificate || ''
        }
      }
      
      let res
      
      // 显示加载状态
      Modal.confirm({
        title: '正在保存站点信息',
        content: '请稍候...',
        okButtonProps: { disabled: true },
        cancelButtonProps: { disabled: true },
        closable: false,
        keyboard: false,
        centered: true,
      });

      if (editingSite && editingSite.name) {
        // 更新站点
        res = await sitesApi.updateSite(editingSite.name, siteData)
      } else {
        // 添加站点
        res = await sitesApi.addSite(siteData)
      }

      // 关闭加载状态
      Modal.destroyAll();

      if (res.code === 200) {
        message.success(editingSite ? '更新站点成功' : '添加站点成功')
        setVisible(false)
        fetchSites()
      } else {
        message.error(editingSite ? '更新站点失败：' + res.message : '添加站点失败：' + res.message)
      }
    } catch (error: any) {
      // 关闭加载状态
      Modal.destroyAll();
      
      // 处理表单验证错误
      if (error.errorFields) {
        message.error('表单验证失败，请检查输入');
      } else {
        // 处理网络错误或其他错误
        message.error('表单提交失败：' + (error.message || '未知错误'));
      }
      console.error('Form submission error:', error)
    }
  }

  // 处理预渲染配置表单提交
  const handlePrerenderConfigSubmit = async () => {
    try {
      const values = await prerenderConfigForm.validateFields()
      
      // 转换表单数据格式，确保与后端API期望的结构匹配
      const prerenderConfigData = {
        Enabled: values.enabled || false,
        PoolSize: values.poolSize || 5,
        MinPoolSize: values.minPoolSize || 2,
        MaxPoolSize: values.maxPoolSize || 20,
        Timeout: values.timeout || 30,
        CacheTTL: values.cacheTTL || 3600,
        IdleTimeout: values.idleTimeout || 300,
        DynamicScaling: values.dynamicScaling || true,
        ScalingFactor: values.scalingFactor || 0.5,
        ScalingInterval: values.scalingInterval || 60,
        UseDefaultHeaders: values.useDefaultHeaders || false,
        CrawlerHeaders: values.crawlerHeaders || [],
        Preheat: {
          Enabled: values.preheat?.enabled || false,
          SitemapURL: values.preheat?.sitemapURL || '',
          Schedule: values.preheat?.schedule || '0 0 * * *',
          Concurrency: values.preheat?.concurrency || 5,
          DefaultPriority: values.preheat?.defaultPriority || 0
        }
      }
      
      // 显示加载状态
      Modal.confirm({
        title: '正在保存预渲染配置',
        content: '请稍候...',
        okButtonProps: { disabled: true },
        cancelButtonProps: { disabled: true },
        closable: false,
        keyboard: false,
        centered: true,
      });

      // 调用API更新预渲染配置
      const res = await prerenderApi.updateConfig(currentPrerenderSite.name, prerenderConfigData)

      // 关闭加载状态
      Modal.destroyAll();

      if (res.code === 200) {
        message.success('更新预渲染配置成功')
        setPrerenderConfigModalVisible(false)
        fetchSites() // 刷新站点列表
      } else {
        message.error('更新预渲染配置失败：' + res.message)
      }
    } catch (error: any) {
      // 关闭加载状态
      Modal.destroyAll();
      
      // 处理表单验证错误
      if (error.errorFields) {
        message.error('表单验证失败，请检查输入');
      } else {
        // 处理网络错误或其他错误
        message.error('表单提交失败：' + (error.message || '未知错误'));
      }
      console.error('Prerender config submission error:', error)
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
        width={800}
        okText="保存"
        cancelText="取消"
      >
        <Form
          form={form}
          layout="vertical"
          initialValues={{
            port: 8081, // 默认站点端口
            proxy: {
              enabled: false,
              type: 'direct',
              targetURL: ''
            },
            firewall: {
              enabled: true,
              action: {
                defaultAction: 'block'
              }
            },
            prerender: {
              enabled: true,
              poolSize: 5,
              minPoolSize: 2,
              maxPoolSize: 20,
              timeout: 30,
              cacheTTL: 3600,
              idleTimeout: 300,
              dynamicScaling: true,
              scalingFactor: 0.5,
              scalingInterval: 60,
              preheat: {
                enabled: false
              }
            },
            ssl: {
              enabled: true
            }
          }}
        >
          {/* 基本信息 */}
          <Card title="基本信息" size="small" style={{ marginBottom: 16 }}>
            <Row gutter={16}>
              <Col span={12}>
                <Form.Item
                  name="name"
                  label="站点名称"
                  rules={[
                    { required: true, message: '请输入站点名称' },
                    { min: 2, max: 50, message: '站点名称长度必须在2到50个字符之间' },
                    { pattern: /^[\w\s\u4e00-\u9fa5\p{L}]+$/, message: '站点名称只能包含字母、数字、下划线、空格、中文和其他语言字符' }
                  ]}
                >
                  <Input placeholder="请输入站点名称，例如：example" />
                </Form.Item>
              </Col>
              <Col span={12}>
                <Form.Item
                  name="domain"
                  label="域名"
                  rules={[
                    { required: true, message: '请输入域名' },
                    { pattern: /^(https?:\/\/)?(localhost|([\da-z.-]+)\.([a-z.]{2,6})([\/\w .-]*)*\/?$|^((25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.){3}(25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)$)/, message: '请输入有效的域名或IP地址' }
                  ]}
                >
                  <Input placeholder="请输入域名，例如：example.com 或 127.0.0.1" />
                </Form.Item>
              </Col>
            </Row>
            <Row gutter={16}>
              <Col span={12}>
                <Form.Item
                  name="port"
                  label="站点端口"
                  rules={[
                    { required: true, message: '请输入站点端口' },
                    { min: 1, max: 65535, message: '端口必须在1到65535之间' }
                  ]}
                >
                  <Input type="number" placeholder="请输入站点端口，例如：8081" min={1} max={65535} />
                </Form.Item>
              </Col>
            </Row>
          </Card>


          {/* 站点访问配置 */}
          <Card title="站点访问配置" size="small" style={{ marginBottom: 16 }}>
            <Form.Item name={['proxy', 'type']} label="访问模式">
              <Select>
                <Option value="direct">直接对外访问</Option>
                <Option value="upstream">作为反向代理上游</Option>
              </Select>
            </Form.Item>

            <Form.Item name={['proxy', 'enabled']} label="启用上游代理" valuePropName="checked">
              <Switch />
            </Form.Item>

            <Form.Item 
              dependencies={[['proxy', 'enabled']]} 
              noStyle
            >
              {({ getFieldValue }) => {
                const proxyEnabled = getFieldValue(['proxy', 'enabled']);
                if (!proxyEnabled) {
                  return null;
                }
                return (
                  <Form.Item
                    name={['proxy', 'targetURL']}
                    label="上游服务URL"
                    rules={[{ required: true, message: '请输入上游服务URL' }]}
                  >
                    <Input placeholder="请输入上游服务URL，例如：http://127.0.0.1:8080" />
                  </Form.Item>
                );
              }}
            </Form.Item>
          </Card>

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
            
            {/* 证书选择，仅当启用SSL时显示 */}
            <Form.Item 
              dependencies={[['ssl', 'enabled']]} 
              noStyle
            >
              {({ getFieldValue }) => {
                const sslEnabled = getFieldValue(['ssl', 'enabled']);
                if (!sslEnabled) {
                  return null;
                }
                return (
                  <Form.Item
                    name={['ssl', 'sslCertificate']}
                    label="选择证书"
                    rules={[{ required: true, message: '请选择证书' }]}
                  >
                    <Select loading={sslLoading} placeholder="请选择证书">
                      {certs.map(cert => (
                        <Option key={cert} value={cert}>
                          {cert}
                        </Option>
                      ))}
                    </Select>
                  </Form.Item>
                );
              }}
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
          showUploadList={false} // 隐藏文件上传列表
        >
          <div style={{
            border: '1px dashed #d9d9d9',
            borderRadius: '6px',
            padding: '50px 20px',
            textAlign: 'center',
            background: '#fafafa',
            cursor: 'pointer',
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
                支持 .zip, .rar, .html, .css, .js, .json, .txt 格式，单个文件不超过100MB
              </Typography.Text>
            </Space>
          </div>
        </Upload>
      </Modal>

      {/* 静态资源管理弹窗 */}
      <Modal
        title={`站点 "${currentSite?.name}" 静态资源管理`}
        open={staticResModalVisible}
        onCancel={() => setStaticResModalVisible(false)}
        width={1000}
        footer={null}
      >
        {/* 路径导航栏 */}
        <div style={{ marginBottom: 16, display: 'flex', alignItems: 'center', gap: 8 }}>
          <Button
            type="text"
            icon={<UpOutlined />}
            onClick={navigateUp}
            disabled={currentPath === '/'}>
            上一级
          </Button>
          <span style={{ fontWeight: 'bold' }}>当前路径：{currentPath}</span>
          <div style={{ flex: 1 }}></div>
          <Space>
            <Button
              type="primary"
              icon={<NewFolderOutlined />}
              onClick={handleNewFolder}
            >
              新建目录
            </Button>
            <Button
              type="primary"
              icon={<FileAddOutlined />}
              onClick={handleNewFile}
            >
              新建文件
            </Button>
            <Upload
              beforeUpload={handleStaticResUpload}
              showUploadList={false}
              multiple // 支持多文件上传
            >
              <Button icon={<UploadOutlined />}>
                上传文件
              </Button>
            </Upload>
          </Space>
        </div>

        {/* 文件列表 */}
        <div style={{ height: 500, overflow: 'auto', border: '1px solid #e8e8e8', borderRadius: 6 }}>
          <Table
            columns={[
              {
                title: '名称',
                dataIndex: 'name',
                key: 'name',
                render: (text: string, record: any) => (
                  <div style={{ display: 'flex', alignItems: 'center', cursor: 'pointer' }}>
                    {record.type === 'dir' ? (
                      <FolderOutlined style={{ color: '#faad14', marginRight: 8 }} onClick={() => enterDirectory(record)} />
                    ) : (
                      <FileOutlined style={{ color: '#1890ff', marginRight: 8 }} />
                    )}
                    <span onClick={() => record.type === 'dir' && enterDirectory(record)}>
                      {text}
                    </span>
                  </div>
                )
              },
              {
                title: '类型',
                dataIndex: 'type',
                key: 'type',
                render: (text: string) => (
                  <span>{text === 'dir' ? '目录' : '文件'}</span>
                )
              },
              {
                title: '大小',
                dataIndex: 'size',
                key: 'size',
                render: (size: number) => {
                  if (size === 0) return '0 B'
                  if (size < 1024) return `${size} B`
                  if (size < 1024 * 1024) return `${(size / 1024).toFixed(2)} KB`
                  return `${(size / (1024 * 1024)).toFixed(2)} MB`
                }
              },
              {
                title: '操作',
                key: 'action',
                render: (_, record: any) => (
                  <Space>
                    {record.type === 'file' && (
                      <Button
                        type="link"
                        size="small"
                        icon={<DownloadOutlined />}
                        onClick={() => handleDownload(record)}
                      >
                        下载
                      </Button>
                    )}
                    {(record.type === 'file' && (record.name.endsWith('.zip') || record.name.endsWith('.rar'))) && (
                      <Button
                        type="link"
                        size="small"
                        icon={<ExtractOutlined />}
                        onClick={() => handleExtract(record)}
                      >
                        解压
                      </Button>
                    )}
                    <Button
                      type="link"
                      size="small"
                      danger
                      icon={<DeleteOutlined />}
                      onClick={() => handleFileDelete(record)}
                    >
                      删除
                    </Button>
                  </Space>
                )
              }
            ]}
            dataSource={fileList}
            rowKey="key"
            pagination={false}
            onRow={(record) => ({
              onContextMenu: (e) => {
                e.preventDefault()
                setSelectedFile(record)
              }
            })}
          />
        </div>
      </Modal>

      {/* 新建目录弹窗 */}
      <Modal
        title="新建目录"
        open={showNewFolderModal}
        onOk={confirmNewFolder}
        onCancel={() => setShowNewFolderModal(false)}
        width={400}
      >
        <div style={{ marginBottom: 16 }}>
          <label style={{ display: 'block', marginBottom: 8 }}>目录名称：</label>
          <Input
            placeholder="请输入目录名称"
            value={newFolderName}
            onChange={(e) => setNewFolderName(e.target.value)}
          />
        </div>
        <p style={{ color: '#666', fontSize: 12 }}>
          当前路径：{currentPath}
        </p>
      </Modal>

      {/* 新建文件弹窗 */}
      <Modal
        title="新建文件"
        open={showNewFileModal}
        onOk={confirmNewFile}
        onCancel={() => setShowNewFileModal(false)}
        width={400}
      >
        <div style={{ marginBottom: 16 }}>
          <label style={{ display: 'block', marginBottom: 8 }}>文件名称：</label>
          <Input
            placeholder="请输入文件名称，例如：index.html"
            value={newFileName}
            onChange={(e) => setNewFileName(e.target.value)}
          />
        </div>
        <p style={{ color: '#666', fontSize: 12 }}>
          当前路径：{currentPath}
        </p>
      </Modal>

      {/* 预渲染配置弹窗 */}
      <Modal
        title={`站点 "${currentPrerenderSite?.name}" 预渲染配置`}
        open={prerenderConfigModalVisible}
        onOk={handlePrerenderConfigSubmit}
        onCancel={() => setPrerenderConfigModalVisible(false)}
        width={800}
        okText="保存"
        cancelText="取消"
      >
        <Form
          form={prerenderConfigForm}
          layout="vertical"
          initialValues={{
            enabled: true,
            poolSize: 5,
            minPoolSize: 2,
            maxPoolSize: 20,
            timeout: 30,
            cacheTTL: 3600,
            idleTimeout: 300,
            dynamicScaling: true,
            scalingFactor: 0.5,
            scalingInterval: 60,
            useDefaultHeaders: true,
            crawlerHeaders: [],
            preheat: {
              enabled: false
            }
          }}
        >
          {/* 基本配置 */}
          <Card title="基本配置" size="small" style={{ marginBottom: 16 }}>
            <Row gutter={16}>
              <Col span={12}>
                <Form.Item name="enabled" label="启用预渲染" valuePropName="checked">
                  <Switch />
                </Form.Item>
              </Col>
              <Col span={12}>
                <Form.Item name="poolSize" label="浏览器池大小">
                  <Input type="number" placeholder="请输入浏览器池大小" min={1} />
                </Form.Item>
              </Col>
            </Row>
            <Row gutter={16}>
              <Col span={12}>
                <Form.Item name="timeout" label="渲染超时(秒)">
                  <Input type="number" placeholder="请输入渲染超时时间" min={1} />
                </Form.Item>
              </Col>
              <Col span={12}>
                <Form.Item name="cacheTTL" label="缓存TTL(秒)">
                  <Input type="number" placeholder="请输入缓存TTL" min={0} />
                </Form.Item>
              </Col>
            </Row>
          </Card>

          {/* 浏览器池配置 */}
          <Card title="浏览器池配置" size="small" style={{ marginBottom: 16 }}>
            <Row gutter={16}>
              <Col span={12}>
                <Form.Item name="minPoolSize" label="最小浏览器数">
                  <Input type="number" placeholder="请输入最小浏览器数" min={1} />
                </Form.Item>
              </Col>
              <Col span={12}>
                <Form.Item name="maxPoolSize" label="最大浏览器数">
                  <Input type="number" placeholder="请输入最大浏览器数" min={1} />
                </Form.Item>
              </Col>
            </Row>
            <Row gutter={16}>
              <Col span={12}>
                <Form.Item name="idleTimeout" label="空闲超时(秒)">
                  <Input type="number" placeholder="请输入空闲超时时间" min={1} />
                </Form.Item>
              </Col>
              <Col span={12}>
                <Form.Item name="dynamicScaling" label="启用动态扩容" valuePropName="checked">
                  <Switch />
                </Form.Item>
              </Col>
            </Row>
            <Form.Item
              dependencies={['dynamicScaling']}
              noStyle
            >
              {({ getFieldValue }) => {
                const dynamicScalingEnabled = getFieldValue('dynamicScaling');
                if (!dynamicScalingEnabled) {
                  return null;
                }
                return (
                  <Row gutter={16}>
                    <Col span={12}>
                      <Form.Item name="scalingFactor" label="扩容因子">
                        <Input type="number" placeholder="请输入扩容因子，如0.5表示每次增加50%" min={0.1} max={2} step={0.1} />
                      </Form.Item>
                    </Col>
                    <Col span={12}>
                      <Form.Item name="scalingInterval" label="扩容检查间隔(秒)">
                        <Input type="number" placeholder="请输入扩容检查间隔" min={10} />
                      </Form.Item>
                    </Col>
                  </Row>
                );
              }}
            </Form.Item>
          </Card>

          {/* 爬虫协议头配置 */}
          <Card title="爬虫协议头配置" size="small" style={{ marginBottom: 16 }}>
            <Form.Item name="useDefaultHeaders" label="使用默认爬虫协议头" valuePropName="checked">
              <Switch />
            </Form.Item>
            <Form.Item label="默认爬虫协议头列表">
              <div style={{ maxHeight: 200, overflow: 'auto', border: '1px solid #e8e8e8', borderRadius: 4, padding: 12, backgroundColor: '#fafafa' }}>
                {defaultCrawlerHeaders.map((header, index) => (
                  <div key={index} style={{ marginBottom: 4, fontSize: 12, color: '#666' }}>
                    {header}
                  </div>
                ))}
              </div>
            </Form.Item>
            <Form.Item name="crawlerHeaders" label="自定义爬虫协议头">
              <Input.TextArea 
                placeholder="请输入自定义爬虫协议头，每行一个" 
                rows={6} 
                style={{ fontFamily: 'monospace' }}
              />
            </Form.Item>
            <Form.Item>
              <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                提示：如果同时启用了默认爬虫协议头和自定义爬虫协议头，系统会合并使用两者。
              </Typography.Text>
            </Form.Item>
          </Card>

          {/* 缓存预热配置 */}
          <Card title="缓存预热配置" size="small">
            <Row gutter={16}>
              <Col span={12}>
                <Form.Item name={['preheat', 'enabled']} label="启用缓存预热" valuePropName="checked">
                  <Switch />
                </Form.Item>
              </Col>
              <Col span={12}>
                <Form.Item 
                  dependencies={[['preheat', 'enabled']]} 
                  noStyle
                >
                  {({ getFieldValue }) => {
                    const preheatEnabled = getFieldValue(['preheat', 'enabled']);
                    if (!preheatEnabled) {
                      return null;
                    }
                    return (
                      <Form.Item name={['preheat', 'concurrency']} label="并发数">
                        <Input type="number" placeholder="请输入并发数" min={1} />
                      </Form.Item>
                    );
                  }}
                </Form.Item>
              </Col>
            </Row>
            <Form.Item 
              dependencies={[['preheat', 'enabled']]} 
              noStyle
            >
              {({ getFieldValue }) => {
                const preheatEnabled = getFieldValue(['preheat', 'enabled']);
                if (!preheatEnabled) {
                  return null;
                }
                return (
                  <Row gutter={16}>
                    <Col span={24}>
                      <Form.Item name={['preheat', 'sitemapURL']} label="Sitemap URL">
                        <Input placeholder="请输入Sitemap URL" />
                      </Form.Item>
                    </Col>
                    <Col span={24}>
                      <Form.Item name={['preheat', 'schedule']} label="预热计划">
                        <Input placeholder="请输入cron表达式，如0 0 * * *表示每天0点执行" />
                      </Form.Item>
                    </Col>
                  </Row>
                );
              }}
            </Form.Item>
          </Card>
        </Form>
      </Modal>
    </div>
  )
}

export default Sites
