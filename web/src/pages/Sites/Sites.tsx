import React, { useState, useEffect } from 'react'
import { Card, Table, Button, Modal, Form, Input, Switch, message, Select, Row, Col, Statistic, Upload, Typography, Space } from 'antd'
import { 
  PlusOutlined, EditOutlined, DeleteOutlined, EyeOutlined, UploadOutlined, 
  UnorderedListOutlined, CloudUploadOutlined, FolderOpenOutlined, 
  FolderOutlined, FileOutlined, FolderOutlined as NewFolderOutlined, FileAddOutlined, UpOutlined, 
  DownloadOutlined, UnorderedListOutlined as ExtractOutlined, ReloadOutlined
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
  const [form] = Form.useForm()
  
  // 静态资源管理状态
  const [staticResModalVisible, setStaticResModalVisible] = useState(false)
  const [currentSite, setCurrentSite] = useState<any>(null)
  const [currentPath, setCurrentPath] = useState<string>('/')
  const [fileList, setFileList] = useState<any[]>([])
  const [showNewFolderModal, setShowNewFolderModal] = useState(false)
  const [newFolderName, setNewFolderName] = useState<string>('')
  const [showNewFileModal, setShowNewFileModal] = useState(false)
  const [newFileName, setNewFileName] = useState<string>('')
  
  // 渲染预热配置弹窗状态
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
      title: '端口',
      dataIndex: 'port',
      key: 'port',
    },
    {
      title: '站点模式',
      dataIndex: 'mode',
      key: 'mode',
      render: (mode: string) => {
        const modeMap: { [key: string]: string } = {
          'proxy': '代理已有应用',
          'static': '静态资源站',
          'redirect': '重定向'
        };
        return modeMap[mode] || mode;
      }
    },
    {
      title: '渲染预热状态',
      dataIndex: 'prerenderEnabled',
      key: 'prerenderEnabled',
      render: (enabled: boolean, record: any) => (
        record.mode === 'static' ? (
          <Switch checked={enabled} onChange={(checked) => handleSwitchChange(record, 'prerender', checked)} />
        ) : null
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
      title: '操作',
      key: 'action',
      render: (_: any, record: any) => (
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
          {record.mode === 'static' && (
            <>
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
                渲染预热配置
              </Button>
            </>
          )}
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
      console.log('=== Starting to fetch sites ===');
      
      // 尝试不同的URL格式
      const urls = ['/api/v1/sites', 'http://localhost:5173/api/v1/sites', 'http://localhost:9598/api/v1/sites'];
      
      for (const url of urls) {
        try {
          console.log(`Trying URL: ${url}`);
          
          const response = await fetch(url, {
            method: 'GET',
            headers: {
              'Content-Type': 'application/json',
            },
            // 允许跨域请求
            credentials: 'same-origin',
          });
          
          console.log(`Response status for ${url}:`, response.status);
          
          const text = await response.text();
          console.log(`Raw response text for ${url}:`, text);
          
          if (response.ok) {
            const res = JSON.parse(text);
            console.log(`Parsed response for ${url}:`, res);
            
            if (res && res.code === 200 && Array.isArray(res.data)) {
              console.log('Found valid sites data!');
              console.log('Sites count:', res.data.length);
              
              // 直接使用原始数据，映射完整的渲染预热配置
              const mappedSites = res.data.map((site: any) => ({
                name: site.name || site.Name || '未知站点',
                domain: site.domains?.[0] || site.domain || '127.0.0.1',
                domains: site.domains || [],
                port: site.port || 80,
                mode: site.mode || 'proxy',
                firewallEnabled: Boolean(site.firewall?.Enabled),
                prerenderEnabled: Boolean(site.prerender?.Enabled),
                // 映射完整的渲染预热配置对象
                prerender: {
                  enabled: site.prerender?.Enabled || false,
                  poolSize: site.prerender?.PoolSize || 5,
                  minPoolSize: site.prerender?.MinPoolSize || 2,
                  maxPoolSize: site.prerender?.MaxPoolSize || 20,
                  timeout: site.prerender?.Timeout || 30,
                  cacheTTL: site.prerender?.CacheTTL || 3600,
                  idleTimeout: site.prerender?.IdleTimeout || 300,
                  dynamicScaling: site.prerender?.DynamicScaling !== false,
                  scalingFactor: site.prerender?.ScalingFactor || 0.5,
                  scalingInterval: site.prerender?.ScalingInterval || 60,
                  useDefaultHeaders: site.prerender?.UseDefaultHeaders || false,
                  crawlerHeaders: site.prerender?.CrawlerHeaders || [],
                  preheat: {
                    enabled: site.prerender?.Preheat?.Enabled || false,
                    sitemapURL: site.prerender?.Preheat?.SitemapURL || '',
                    schedule: site.prerender?.Preheat?.Schedule || '0 0 * * *',
                    concurrency: site.prerender?.Preheat?.Concurrency || 5,
                    defaultPriority: site.prerender?.Preheat?.DefaultPriority || 0
                  }
                }
              }));

              
              console.log('Mapped sites:', mappedSites);
              setSites(mappedSites);
              message.success('获取站点列表成功');
              return; // 成功后退出循环
            }
          }
        } catch (error) {
          console.error(`Error with ${url}:`, error);
        }
      }
      
      // 如果所有URL都失败
      console.error('All URLs failed to return valid sites data');
      message.error('获取站点列表失败');
      
    } catch (error) {
      console.error('Unexpected error in fetchSites:', error);
      message.error('获取站点列表失败');
    } finally {
      setLoading(false);
    }
  }
  


  // 初始化数据
  useEffect(() => {
    console.log('useEffect triggered, calling fetchSites...');
    fetchSites()
  }, [])

  // 手动触发获取站点列表（用于调试）
  const handleManualFetch = () => {
    console.log('Manual fetch button clicked');
    fetchSites();
  }

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
  const handleSwitchChange = async (record: any, type: 'prerender' | 'firewall', enabled: boolean) => {
    try {
      // 确保record对象有效
      if (!record || typeof record !== 'object') {
        throw new Error('无效的站点对象')
      }
      
      // 确保站点名称存在且不为空
      const siteName = record.name || record.Name || '';
      if (!siteName.trim()) {
        throw new Error('站点名称不存在')
      }
      
      // 创建更新后的站点数据，添加空对象默认值，增强代码健壮性
      const updatedSite = {
        ...record,
        [type]: {
          ...(record[type] || {}),
          enabled
        }
      }
      
      // 转换为后端API期望的格式（大写键）
      const apiSiteData = {
        Name: siteName,
        Domain: updatedSite.domain,
        Domains: updatedSite.domains || [updatedSite.domain], // 支持多个域名
        Port: updatedSite.port || 80, // 保留端口信息，默认为80
        Mode: updatedSite.mode || 'proxy', // 添加站点模式
        Proxy: {
          Enabled: updatedSite.proxy?.enabled || false,
          TargetURL: updatedSite.proxy?.targetURL || '',
          Type: updatedSite.proxy?.type || 'direct'
        },
        // 重定向配置
        Redirect: {
          Enabled: updatedSite.mode === 'redirect',
          Code: updatedSite.redirect?.code || 302,
          URL: updatedSite.redirect?.url || ''
        },
        Firewall: {
          Enabled: updatedSite.firewall.enabled,
          RulesPath: updatedSite.firewall.rulesPath || '/etc/prerender-shield/rules',
          ActionConfig: {
            DefaultAction: updatedSite.firewall.action?.defaultAction || 'block',
            BlockMessage: updatedSite.firewall.action?.blockMessage || 'Request blocked by firewall'
          },
          // 地理位置访问控制配置
          GeoIPConfig: {
            Enabled: updatedSite.firewall.geoip?.enabled || false,
            AllowList: updatedSite.firewall.geoip?.allowList || [],
            BlockList: updatedSite.firewall.geoip?.blockList || []
          },
          // 频率限制配置
          RateLimitConfig: {
            Enabled: updatedSite.firewall.rate_limit?.enabled || false,
            Requests: updatedSite.firewall.rate_limit?.requests || 100,
            Window: updatedSite.firewall.rate_limit?.window || 60,
            BanTime: updatedSite.firewall.rate_limit?.ban_time || 3600
          }
        },
        // 网页防篡改配置
        FileIntegrityConfig: {
          Enabled: updatedSite.file_integrity?.enabled || false,
          CheckInterval: updatedSite.file_integrity?.check_interval || 300,
          HashAlgorithm: updatedSite.file_integrity?.hash_algorithm || 'sha256'
        },
        Prerender: {
          Enabled: updatedSite.prerender.enabled,
          PoolSize: updatedSite.prerender.poolSize || 5,
          MinPoolSize: updatedSite.prerender.minPoolSize || 2,
          MaxPoolSize: updatedSite.prerender.maxPoolSize || 20,
          Timeout: updatedSite.prerender.timeout || 30,
          CacheTTL: updatedSite.prerender.cacheTTL || 3600,
          IdleTimeout: updatedSite.prerender.idleTimeout || 300,
          DynamicScaling: updatedSite.prerender.dynamicScaling || true,
          ScalingFactor: updatedSite.prerender.scalingFactor || 0.5,
          ScalingInterval: updatedSite.prerender.scalingInterval || 60,
          UseDefaultHeaders: updatedSite.prerender.useDefaultHeaders || false,
          CrawlerHeaders: updatedSite.prerender.crawlerHeaders || [],
          Preheat: {
            Enabled: updatedSite.prerender.preheat?.enabled || false,
            SitemapURL: updatedSite.prerender.preheat?.sitemapURL || '',
            Schedule: updatedSite.prerender.preheat?.schedule || '0 0 * * *',
            Concurrency: updatedSite.prerender.preheat?.concurrency || 5,
            DefaultPriority: updatedSite.prerender.preheat?.defaultPriority || 0
          }
        },
        Routing: {
          Rules: updatedSite.routing?.rules || []
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
    if (!site || typeof site !== 'object') {
      console.error('Invalid site provided, cannot open static resources')
      message.error('站点信息无效，无法打开静态资源管理')
      return
    }
    
    // 确保站点名称存在且不为空
    const siteName = site.name || site.Name || '';
    if (!siteName.trim()) {
      console.error('Site name is empty, cannot open static resources')
      message.error('站点名称不存在，无法打开静态资源管理')
      return
    }
    
    setCurrentSite(site)
    setCurrentPath('/')
    setStaticResModalVisible(true)
    // 直接传递site.name给loadFileList，避免依赖currentSite的异步更新
    loadFileList('/', siteName)
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
        message.error(`${file.name} 解压失败: ${response.message || '未知错误'}`)
      }
    } catch (error) {
      console.error('解压失败:', error)
      message.error(`${file.name} 解压失败`)
    }
  }



  // 删除文件/目录
  const handleFileDelete = (file: any) => {
    message.success(`${file.name} 删除成功`)
    setFileList(prev => prev.filter(f => f.key !== file.key))
  }



  // 文件上传前的处理
  const beforeUpload: UploadProps['beforeUpload'] = (file) => {
    // 调整rar/zip上传大小限制为不超过100m
    const isLt100M = file.size / 1024 / 1024 < 100
    if (!isLt100M) {
      message.error('文件大小不能超过100MB')
      return Upload.LIST_IGNORE
    }
    
    return true
  }

  // 自定义上传逻辑
  const customRequest: UploadProps['customRequest'] = (options) => {
    const { onSuccess, onError, file, onProgress } = options
    
    // 确保站点和站点名称存在
    if (!currentSite || typeof currentSite.name === 'undefined' || currentSite.name === '') {
      console.error('Site is not set, cannot upload file')
      message.error('站点信息无效，无法上传文件')
      if (onError) onError(new Error('站点信息无效'))
      return
    }
    
    // 发送实际的API请求，使用当前路径
    sitesApi.uploadFile(currentSite.name, file, currentPath, (progressEvent) => {
      if (progressEvent.total && onProgress) {
        const percentCompleted = Math.round((progressEvent.loaded * 100) / progressEvent.total);
        onProgress({ percent: percentCompleted });
      }
    })
    .then((response) => {
      if (response.code === 200) {
        message.success(`${typeof file === 'string' ? file : (file as any).name} 上传成功`)
        // 重新加载文件列表
        loadFileList(currentPath)
        if (onSuccess) onSuccess({ status: 'ok', message: '上传成功' })
      } else {
        throw new Error(response.message || '上传失败')
      }
    })
    .catch((error) => {
      message.error(`${typeof file === 'string' ? file : (file as any).name} 上传失败: ${error.message}`)
      if (onError) onError(error)
    })
  }

  // 处理删除站点
  const handleDelete = async (site: any) => {
    try {
      // 确保site对象有效且有name属性
      if (!site || typeof site !== 'object') {
        throw new Error('无效的站点对象')
      }
      
      // 确保站点名称存在且不为空
      const siteName = site.name || site.Name || '';
      if (!siteName.trim()) {
        throw new Error('站点名称不存在')
      }
      
      console.log('Deleting site with name:', siteName);
      const res = await sitesApi.deleteSite(siteName)
      // 直接使用res.code，因为API响应拦截器已经返回了response.data
      if (res.code === 200) {
        message.success('删除站点成功')
        fetchSites()
      } else {
        message.error('删除站点失败：' + res.message)
      }
    } catch (error) {
      console.error('Failed to delete site:', error)
      message.error('删除站点失败：' + (error as any).message)
    }
  }
  
  // 打开渲染预热配置弹窗
  const handlePrerenderConfig = (site: any) => {
    setCurrentPrerenderSite(site)
    // 初始化表单值，添加安全检查，处理site.prerender为undefined的情况
    const initialValues = {
      enabled: site.prerender?.enabled || site.prerenderEnabled || false,
      poolSize: site.prerender?.poolSize || 5,
      minPoolSize: site.prerender?.minPoolSize || 2,
      maxPoolSize: site.prerender?.maxPoolSize || 20,
      timeout: site.prerender?.timeout || 30,
      cacheTTL: site.prerender?.cacheTTL || 3600,
      idleTimeout: site.prerender?.idleTimeout || 300,
      dynamicScaling: site.prerender?.dynamicScaling !== false,
      scalingFactor: site.prerender?.scalingFactor || 0.5,
      scalingInterval: site.prerender?.scalingInterval || 60,
      useDefaultHeaders: site.prerender?.useDefaultHeaders || false,
      crawlerHeaders: site.prerender?.crawlerHeaders || [],
      preheat: {
        enabled: site.prerender?.preheat?.enabled || false,
        sitemapURL: site.prerender?.preheat?.sitemapURL || '',
        schedule: site.prerender?.preheat?.schedule || '0 0 * * *',
        concurrency: site.prerender?.preheat?.concurrency || 5,
        defaultPriority: site.prerender?.preheat?.defaultPriority || 0
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
        Domains: [values.domain], // 支持多个域名，先添加主域名
        Port: parseInt(values.port, 10) || 80, // 转换为整数类型，默认为80
        Mode: values.mode, // 添加站点模式
        // 代理配置 - 根据模式决定是否启用
        Proxy: {
          Enabled: values.mode === 'proxy',
          TargetURL: values.mode === 'proxy' ? (values.proxy?.targetURL || "") : "",
          Type: "direct" // 简化为固定值
        },
        // 重定向配置 - 根据模式决定是否启用
        Redirect: {
          Enabled: values.mode === 'redirect',
          Code: values.mode === 'redirect' ? (values.redirect?.code || 302) : 302,
          URL: values.mode === 'redirect' ? (values.redirect?.url || "") : ""
        },
        Firewall: {
          Enabled: values.firewall.enabled || false,
          RulesPath: values.firewall.rulesPath || '/etc/prerender-shield/rules',
          ActionConfig: {
            DefaultAction: values.firewall.action?.defaultAction || 'block',
            BlockMessage: values.firewall.action?.blockMessage || 'Request blocked by firewall'
          },
          // 地理位置访问控制配置
          GeoIPConfig: {
            Enabled: values.firewall.geoip?.enabled || false,
            AllowList: values.firewall.geoip?.allowList ? values.firewall.geoip.allowList.split(',').map((s: string) => s.trim()) : [],
            BlockList: values.firewall.geoip?.blockList ? values.firewall.geoip.blockList.split(',').map((s: string) => s.trim()) : []
          },
          // 频率限制配置
          RateLimitConfig: {
            Enabled: values.firewall.rate_limit?.enabled || false,
            Requests: values.firewall.rate_limit?.requests || 100,
            Window: values.firewall.rate_limit?.window || 60,
            BanTime: values.firewall.rate_limit?.ban_time || 3600
          }
        },
        // 网页防篡改配置
        FileIntegrityConfig: {
          Enabled: values.file_integrity?.enabled || false,
          CheckInterval: values.file_integrity?.check_interval || 300,
          HashAlgorithm: values.file_integrity?.hash_algorithm || 'sha256'
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
        console.log('Adding site with data:', siteData);
        res = await sitesApi.addSite(siteData)
        console.log('Add site response:', res);
      }

      // 关闭加载状态
      Modal.destroyAll();

      // 直接使用res，因为API响应拦截器已经返回了response.data
      if (res.code === 200) {
        message.success(editingSite ? '更新站点成功' : '添加站点成功')
        setVisible(false)
        // 立即刷新站点列表
        console.log('Refreshing sites list...');
        fetchSites()
      } else {
        message.error(editingSite ? '更新站点失败：' + (res.message || '未知错误') : '添加站点失败：' + (res.message || '未知错误'))
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

  // 处理渲染预热配置表单提交
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
        title: '正在保存渲染预热配置',
        content: '请稍候...',
        okButtonProps: { disabled: true },
        cancelButtonProps: { disabled: true },
        closable: false,
        keyboard: false,
        centered: true,
      });

      // 调用API更新渲染预热配置
      const res = await prerenderApi.updateConfig(currentPrerenderSite.name, prerenderConfigData)

      // 关闭加载状态
      Modal.destroyAll();

      if (res.code === 200) {
        message.success('更新渲染预热配置成功')
        setPrerenderConfigModalVisible(false)
        fetchSites() // 刷新站点列表
      } else {
        message.error('更新渲染预热配置失败：' + (res.message || '未知错误'))
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
              title="启用渲染预热的站点"
              value={sites.filter(site => site.prerender && site.prerender.enabled).length}
              valueStyle={{ color: '#52c41a' }}
            />
          </Col>
          <Col span={8}>
            <Statistic
              title="启用防火墙的站点"
              value={sites.filter(site => site.firewall && site.firewall.enabled).length}
              valueStyle={{ color: '#faad14' }}
            />
          </Col>
        </Row>
      </Card>

      {/* 站点列表 */}
      <Card className="card" title="站点列表" extra={
        <Space>
          <Button type="primary" icon={<PlusOutlined />} onClick={handleAdd}>
            添加站点
          </Button>
          <Button icon={<ReloadOutlined />} onClick={handleManualFetch}>
            重新加载
          </Button>
        </Space>
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
                    { pattern: /^[\w\s\u4e00-\u9fa5a-zA-Z]+$/, message: '站点名称只能包含字母、数字、下划线、空格、中文和其他语言字符' }
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
                    { pattern: /^(localhost|127\.0\.0\.1)$/, message: '只允许输入 localhost 或 127.0.0.1' }
                  ]}
                >
                  <Input placeholder="请输入域名，仅允许 localhost 或 127.0.0.1" />
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
              <Col span={12}>
                <Form.Item
                  name="mode"
                  label="站点模式"
                  rules={[{ required: true, message: '请选择站点模式' }]}
                >
                  <Select placeholder="请选择站点模式">
                    <Option value="proxy">代理已有应用</Option>
                    <Option value="static">静态资源站</Option>
                    <Option value="redirect">重定向</Option>
                  </Select>
                </Form.Item>
              </Col>
            </Row>
          </Card>

          {/* 站点模式配置 */}
          <Card title="站点模式配置" size="small" style={{ marginBottom: 16 }}>
            <Form.Item 
              dependencies={['mode']} 
              noStyle
            >
              {({ getFieldValue }) => {
                const mode = getFieldValue('mode');
                
                // 代理模式配置
                if (mode === 'proxy') {
                  return (
                    <Form.Item
                      name="proxy.targetURL"
                      label="上游服务器地址"
                      rules={[{ required: true, message: '请输入上游服务器地址' }, { type: 'url', message: '请输入完整域名，例如：http://example.com' }]}
                      extra="提示：输入完整域名，不支持路径"
                    >
                      <Input placeholder="请输入上游服务器地址，例如：http://127.0.0.1:8080" />
                    </Form.Item>
                  );
                }
                
                // 静态资源站配置
                if (mode === 'static') {
                  return (
                    <div>
                      <p style={{ color: '#8c8c8c', marginBottom: 16 }}>提示：在静态资源管理中上传资源</p>
                      <p style={{ color: '#8c8c8c' }}>说明：站点列表中仅静态资源站允许上传资源</p>
                    </div>
                  );
                }
                
                // 重定向配置
                if (mode === 'redirect') {
                  return (
                    <>
                      <Row gutter={16}>
                        <Col span={12}>
                          <Form.Item
                            name="redirect.code"
                            label="重定向类型"
                            rules={[{ required: true, message: '请选择重定向类型' }]}
                          >
                            <Select>
                              <Option value={301}>301 Moved Permanently</Option>
                              <Option value={302}>302 Found</Option>
                              <Option value={307}>307 Temporary Redirect</Option>
                              <Option value={308}>308 Permanent Redirect</Option>
                            </Select>
                          </Form.Item>
                        </Col>
                        <Col span={12}>
                          <Form.Item
                            name="redirect.url"
                            label="重定向地址"
                            rules={[{ required: true, message: '请输入重定向地址' }, { type: 'url', message: '请输入完整域名，例如：http://example.com' }]}
                            extra="提示：输入完整域名，不支持路径"
                          >
                            <Input placeholder="请输入重定向地址，例如：http://example.com" />
                          </Form.Item>
                        </Col>
                      </Row>
                    </>
                  );
                }
                
                return null;
              }}
            </Form.Item>
          </Card>

          {/* 渲染预热配置 */}
          <Form.Item dependencies={['mode']} noStyle>
            {({ getFieldValue }) => {
              const mode = getFieldValue('mode');
              if (mode !== 'static') {
                return null;
              }
              return (
                <Card title="渲染预热配置" size="small" style={{ marginBottom: 16 }}>
                  <Form.Item name={['prerender', 'enabled']} label="启用渲染预热" valuePropName="checked">
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
              );
            }}
          </Form.Item>

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

            {/* 地理位置访问控制 */}
            <Form.Item label="地理位置访问控制" extra="配置允许或阻止的国家/地区列表">
              <Form.Item name={['firewall', 'geoip', 'enabled']} label="启用地理位置访问控制" valuePropName="checked" noStyle>
                <Switch />
              </Form.Item>
              
              <Form.Item 
                dependencies={[['firewall', 'geoip', 'enabled']]} 
                noStyle
              >
                {({ getFieldValue }) => {
                  const geoipEnabled = getFieldValue(['firewall', 'geoip', 'enabled']);
                  if (!geoipEnabled) {
                    return null;
                  }
                  return (
                    <>
                      <Form.Item
                        name={['firewall', 'geoip', 'allowList']}
                        label="允许的国家/地区代码"
                        extra="例如：CN,US,JP，多个代码用逗号分隔"
                      >
                        <Input placeholder="请输入允许的国家/地区代码，用逗号分隔" />
                      </Form.Item>
                      
                      <Form.Item
                        name={['firewall', 'geoip', 'blockList']}
                        label="阻止的国家/地区代码"
                        extra="例如：CN,US,JP，多个代码用逗号分隔"
                      >
                        <Input placeholder="请输入阻止的国家/地区代码，用逗号分隔" />
                      </Form.Item>
                    </>
                  );
                }}
              </Form.Item>
            </Form.Item>

            {/* 频率限制 / CC 攻击防护 */}
            <Form.Item label="频率限制 / CC 攻击防护" extra="配置请求频率限制，防止CC攻击">
              <Form.Item name={['firewall', 'rate_limit', 'enabled']} label="启用频率限制" valuePropName="checked" noStyle>
                <Switch />
              </Form.Item>
              
              <Form.Item 
                dependencies={[['firewall', 'rate_limit', 'enabled']]} 
                noStyle
              >
                {({ getFieldValue }) => {
                  const rateLimitEnabled = getFieldValue(['firewall', 'rate_limit', 'enabled']);
                  if (!rateLimitEnabled) {
                    return null;
                  }
                  return (
                    <>
                      <Row gutter={16}>
                        <Col span={8}>
                          <Form.Item
                            name={['firewall', 'rate_limit', 'requests']}
                            label="时间窗口内允许的请求数"
                          >
                            <Input type="number" placeholder="例如：100" min={1} />
                          </Form.Item>
                        </Col>
                        <Col span={8}>
                          <Form.Item
                            name={['firewall', 'rate_limit', 'window']}
                            label="时间窗口（秒）"
                          >
                            <Input type="number" placeholder="例如：60" min={1} />
                          </Form.Item>
                        </Col>
                        <Col span={8}>
                          <Form.Item
                            name={['firewall', 'rate_limit', 'ban_time']}
                            label="封禁时间（秒）"
                          >
                            <Input type="number" placeholder="例如：3600" min={1} />
                          </Form.Item>
                        </Col>
                      </Row>
                    </>
                  );
                }}
              </Form.Item>
            </Form.Item>

            {/* 网页防篡改配置 */}
            <Form.Item label="网页防篡改" extra="定期检查文件完整性，防止文件被篡改">
              <Form.Item name={['file_integrity', 'enabled']} label="启用网页防篡改" valuePropName="checked" noStyle>
                <Switch />
              </Form.Item>
              
              <Form.Item 
                dependencies={[['file_integrity', 'enabled']]} 
                noStyle
              >
                {({ getFieldValue }) => {
                  const fileIntegrityEnabled = getFieldValue(['file_integrity', 'enabled']);
                  if (!fileIntegrityEnabled) {
                    return null;
                  }
                  return (
                    <>
                      <Row gutter={16}>
                        <Col span={12}>
                          <Form.Item
                            name={['file_integrity', 'check_interval']}
                            label="检查间隔（秒）"
                          >
                            <Input type="number" placeholder="例如：300" min={10} />
                          </Form.Item>
                        </Col>
                        <Col span={12}>
                          <Form.Item
                            name={['file_integrity', 'hash_algorithm']}
                            label="哈希算法"
                          >
                            <Select>
                              <Option value="md5">MD5</Option>
                              <Option value="sha1">SHA-1</Option>
                              <Option value="sha256">SHA-256</Option>
                              <Option value="sha512">SHA-512</Option>
                            </Select>
                          </Form.Item>
                        </Col>
                      </Row>
                    </>
                  );
                }}
              </Form.Item>
            </Form.Item>
          </Card>


        </Form>
      </Modal>

      {/* 文件上传弹窗 */}
      <Modal
        title={`站点 "${currentSite?.name}" 文件管理`}
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
              beforeUpload={beforeUpload}
              customRequest={customRequest}
              showUploadList={false}
              multiple // 支持多文件上传
              accept=".zip,.rar,.html,.css,.js,.json,.txt"
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

      {/* 渲染预热配置弹窗 */}
      <Modal
        title={`站点 "${currentPrerenderSite?.name}" 渲染预热配置`}
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
                <Form.Item name="enabled" label="启用渲染预热" valuePropName="checked">
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
