import React, { useState, useEffect } from 'react'
import { Card, Table, Button, Modal, Form, Input, Switch, Select, Row, Col, Statistic, Upload, Typography, Space, message } from 'antd'
import { 
  PlusOutlined, EditOutlined, DeleteOutlined, EyeOutlined, UploadOutlined, 
  UnorderedListOutlined, CloudUploadOutlined, FolderOpenOutlined, 
  FolderOutlined, FileOutlined, FolderOutlined as NewFolderOutlined, FileAddOutlined, UpOutlined, 
  DownloadOutlined, UnorderedListOutlined as ExtractOutlined, ReloadOutlined
} from '@ant-design/icons'
import { sitesApi } from '../../services/api'
import type { UploadProps } from 'antd'

const { Option } = Select

const Sites: React.FC = () => {
  // 使用useMessage hook来获取message实例，支持主题配置
  const [messageApi, contextHolder] = message.useMessage();
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
  
  // 预渲染配置模态框状态
  const [prerenderConfigModalVisible, setPrerenderConfigModalVisible] = useState(false)
  const [editingPrerenderSite, setEditingPrerenderSite] = useState<any>(null)
  const [prerenderConfigForm] = Form.useForm()


  // 表格列配置
  const columns = [
    {
      title: '站点名称',
      dataIndex: 'name',
      key: 'name',
      width: 150,
      ellipsis: true,
      onCell: () => ({
        style: {
          whiteSpace: 'nowrap',
          overflow: 'hidden',
          textOverflow: 'ellipsis',
        }
      }),
    },
    {
      title: '域名',
      dataIndex: 'domain',
      key: 'domain',
      width: 150,
      ellipsis: true,
      onCell: () => ({
        style: {
          whiteSpace: 'nowrap',
          overflow: 'hidden',
          textOverflow: 'ellipsis',
        }
      }),
    },
    {
      title: '端口',
      dataIndex: 'port',
      key: 'port',
      width: 80,
      align: 'center',
      onCell: () => ({
        style: {
          whiteSpace: 'nowrap',
        }
      }),
    },
    {
      title: '站点模式',
      dataIndex: 'mode',
      key: 'mode',
      width: 120,
      render: (mode: string) => {
        const modeMap: { [key: string]: string } = {
          'proxy': '代理已有应用',
          'static': '静态资源站',
          'redirect': '重定向'
        };
        return modeMap[mode] || mode;
      },
      onCell: () => ({
        style: {
          whiteSpace: 'nowrap',
        }
      }),
    },
    {
      title: '渲染预热状态',
      dataIndex: 'prerenderEnabled',
      key: 'prerenderEnabled',
      width: 120,
      align: 'center',
      render: (enabled: boolean, record: any) => (
        record.mode === 'static' ? (
          <Switch checked={enabled} onChange={(checked) => handleSwitchChange(record, 'prerender', checked)} />
        ) : null
      ),
      onCell: () => ({
        style: {
          whiteSpace: 'nowrap',
        }
      }),
    },
    {
      title: '防火墙状态',
      dataIndex: 'firewallEnabled',
      key: 'firewallEnabled',
      width: 120,
      align: 'center',
      render: (enabled: boolean, record: any) => (
        <Switch checked={enabled} onChange={(checked) => handleSwitchChange(record, 'firewall', checked)} />
      ),
      onCell: () => ({
        style: {
          whiteSpace: 'nowrap',
        }
      }),
    },
    {
      title: '操作',
      key: 'action',
      width: 400,
      fixed: 'right',
      render: (_: any, record: any) => (
        <div style={{ display: 'flex', flexWrap: 'nowrap' }}>
          <Button
            type="link"
            icon={<EyeOutlined />}
            onClick={() => handleView(record)}
            style={{ marginRight: 8, whiteSpace: 'nowrap' }}
          >
            查看
          </Button>
          <Button
            type="link"
            icon={<EditOutlined />}
            onClick={() => handleEdit(record)}
            style={{ marginRight: 8, whiteSpace: 'nowrap' }}
          >
            编辑
          </Button>
          {record.mode === 'static' && (
            <>
              <Button
                type="link"
                icon={<FolderOpenOutlined />}
                onClick={() => handleStaticResources(record)}
                style={{ marginRight: 8, whiteSpace: 'nowrap' }}
              >
                静态资源
              </Button>
              <Button
                type="link"
                icon={<UnorderedListOutlined />}
                onClick={() => handlePrerenderConfig(record)}
                style={{ marginRight: 8, whiteSpace: 'nowrap' }}
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
            style={{ whiteSpace: 'nowrap' }}
          >
            删除
          </Button>
        </div>
      ),
      onCell: () => ({
        style: {
          whiteSpace: 'nowrap',
        }
      }),
    },
  ]

  // 获取站点列表
  const fetchSites = async () => {
    try {
      setLoading(true)
      console.log('=== Starting to fetch sites ===');
      
      // 使用配置好的sitesApi，自动携带Authorization头
      const response = await sitesApi.getSites();
      
      console.log('sitesApi.getSites() response:', response);
      
      if (response && response.code === 200 && Array.isArray(response.data)) {
        console.log('Found valid sites data!');
        console.log('Sites count:', response.data.length);
        
        // 直接使用原始数据，映射完整的渲染预热配置
        const mappedSites = response.data.map((site: any) => ({
          id: site.id || site.ID,
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
        messageApi.success('获取站点列表成功');
      } else {
        // 请求失败
        console.error('Failed to return valid sites data');
        messageApi.error('获取站点列表失败');
      }
      
    } catch (error: any) {
      console.error('Unexpected error in fetchSites:', error);
      console.error('Error response:', error.response?.data);
      messageApi.error('获取站点列表失败: ' + (error.message || '未知错误'));
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
      const res = await sitesApi.updateSite(record.id, apiSiteData)
      if (res.code === 200) {
        messageApi.success('更新站点成功')
        fetchSites() // 刷新站点列表
      } else {
        messageApi.error('更新站点失败')
      }
    } catch (error) {
      console.error('Switch change error:', error)
      messageApi.error('更新失败')
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
      messageApi.error('站点域名无效，无法打开预览')
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
    // 确保site和site.id存在
    if (!site || typeof site !== 'object') {
      console.error('Invalid site provided, cannot open static resources')
      messageApi.error('站点信息无效，无法打开静态资源管理')
      return
    }
    
    // 确保站点ID存在且不为空
    const siteId = site.id || site.ID || '';
    if (!siteId.trim()) {
      console.error('Site ID is empty, cannot open static resources')
      messageApi.error('站点ID不存在，无法打开静态资源管理')
      return
    }
    
    setCurrentSite(site)
    setCurrentPath('/')
    setStaticResModalVisible(true)
    // 直接传递site.id给loadFileList，避免依赖currentSite的异步更新
    loadFileList('/', site.id)
  }

  // 加载当前路径下的文件列表
  const loadFileList = async (path: string, siteId?: string) => {
    // 优先使用传入的siteId，否则使用currentSite.id
    let finalSiteId = siteId || (currentSite && currentSite.id)
    
    // 确保站点ID存在
    if (typeof finalSiteId === 'undefined' || finalSiteId === '') {
      console.error('Site ID is invalid, cannot load file list')
      return
    }
    
    // 特殊处理默认站点，其ID为"default"
    if (finalSiteId === 'default') {
      // 直接使用默认站点ID，无需查找
      console.log('Using default site ID:', finalSiteId);
    } else {
      // 如果finalSiteId看起来是站点名称而不是ID，尝试从sites数组中查找对应的ID
      // UUID格式的ID不需要查找
      const isUUID = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i.test(finalSiteId);
      if (!isUUID && (finalSiteId.includes(' ') || finalSiteId.length < 36)) {
        const site = sites.find(s => s.name === finalSiteId || s.Name === finalSiteId)
        if (site && site.id) {
          finalSiteId = site.id
          console.log('Corrected site ID from name to ID:', finalSiteId)
        } else {
          console.error('Failed to find site ID for name:', finalSiteId)
          return
        }
      }
    }
    
    try {
      // 发送API请求获取文件列表
      const response = await sitesApi.getFileList(finalSiteId, path)
      if (response.code === 200) {
        setFileList(response.data)
      } else {
        messageApi.error('获取文件列表失败')
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
      messageApi.warning('请输入目录名称')
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
    messageApi.success('目录创建成功')
  }

  // 新建文件
  const handleNewFile = () => {
    setShowNewFileModal(true)
    setNewFileName('')
  }

  // 确认新建文件
  const confirmNewFile = () => {
    if (!newFileName.trim()) {
      messageApi.warning('请输入文件名称')
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
    messageApi.success('文件创建成功')
  }



  // 下载文件
  const handleDownload = (file: any) => {
    message.info(`正在下载 ${file.name}`)
    // 创建临时下载链接
    const downloadLink = document.createElement('a');
    downloadLink.href = `/api/sites/${currentSite?.id}/static${file.path}`;
    downloadLink.download = file.name;
    downloadLink.target = '_blank';
    document.body.appendChild(downloadLink);
    downloadLink.click();
    document.body.removeChild(downloadLink);
  }

  // 解压文件
  const handleExtract = async (file: any) => {
    // 确保currentSite和currentSite.id存在
    if (!currentSite || typeof currentSite.id === 'undefined' || currentSite.id === '') {
      console.error('Current site is not set, cannot extract file')
      messageApi.error('站点信息无效，无法解压文件')
      return
    }
    
    try {
      messageApi.info(`正在解压 ${file.name}...`)
      
      // 发送解压请求到后端
      const response = await sitesApi.extractFile(currentSite.id, file.name, currentPath)
      
      if (response.code === 200) {
        messageApi.success(`${file.name} 解压成功`)
        // 重新加载文件列表
        loadFileList(currentPath)
      } else {
        messageApi.error(`${file.name} 解压失败: ${response.message || '未知错误'}`)
      }
    } catch (error) {
      console.error('解压失败:', error)
      messageApi.error(`${file.name} 解压失败`)
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
      messageApi.error('文件大小不能超过100MB')
      return Upload.LIST_IGNORE
    }
    
    return true
  }

  // 自定义上传逻辑
  const customRequest: UploadProps['customRequest'] = (options) => {
    const { onSuccess, onError, file, onProgress } = options
    
    // 确保站点和站点ID存在
    if (!currentSite || typeof currentSite.id === 'undefined' || currentSite.id === '') {
      console.error('Site is not set, cannot upload file')
      messageApi.error('站点信息无效，无法上传文件')
      if (onError) onError(new Error('站点信息无效'))
      return
    }
    
    // 发送实际的API请求，使用当前路径
    sitesApi.uploadFile(currentSite.id, file, currentPath, (progressEvent) => {
      if (progressEvent.total && onProgress) {
        const percentCompleted = Math.round((progressEvent.loaded * 100) / progressEvent.total);
        onProgress({ percent: percentCompleted });
      }
    })
    .then((response) => {
      if (response.code === 200) {
        messageApi.success(`${typeof file === 'string' ? file : (file as any).name} 上传成功`)
        // 重新加载文件列表
        loadFileList(currentPath)
        if (onSuccess) onSuccess({ status: 'ok', message: '上传成功' })
      } else {
        throw new Error(response.message || '上传失败')
      }
    })
    .catch((error) => {
      messageApi.error(`${typeof file === 'string' ? file : (file as any).name} 上传失败: ${error.message}`)
      if (onError) onError(error)
    })
  }

  // 处理删除站点
  const handleDelete = async (site: any) => {
    try {
      // 确保site对象有效且有id属性
      if (!site || typeof site !== 'object') {
        throw new Error('无效的站点对象')
      }
      
      // 确保站点ID存在且不为空
      const siteId = site.id || site.ID || '';
      if (!siteId.trim()) {
        throw new Error('站点ID不存在')
      }
      
      console.log('Deleting site with id:', siteId);
      const res = await sitesApi.deleteSite(siteId)
      // 直接使用res.code，因为API响应拦截器已经返回了response.data
      if (res.code === 200) {
        messageApi.success('删除站点成功')
        fetchSites()
      } else {
        messageApi.error('删除站点失败：' + res.message)
      }
    } catch (error) {
      console.error('Failed to delete site:', error)
      messageApi.error('删除站点失败：' + (error as any).message)
    }
  }
  
  // 跳转到渲染预热配置页面
  const handlePrerenderConfig = (site: any) => {
    // 打开预渲染配置模态框
    setEditingPrerenderSite(site)
    // 设置表单初始值
    prerenderConfigForm.setFieldsValue({
      ...site.prerender,
      preheat: site.prerender.preheat || {
        enabled: false,
        sitemapURL: '',
        schedule: '0 0 * * *',
        concurrency: 5,
        defaultPriority: 0
      }
    })
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

      if (editingSite && editingSite.id) {
        // 更新站点
        res = await sitesApi.updateSite(editingSite.id, siteData)
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
        messageApi.success(editingSite ? '更新站点成功' : '添加站点成功')
        setVisible(false)
        // 立即刷新站点列表
        console.log('Refreshing sites list...');
        fetchSites()
      } else {
        messageApi.error(editingSite ? '更新站点失败：' + (res.message || '未知错误') : '添加站点失败：' + (res.message || '未知错误'))
      }
    } catch (error: any) {
      // 关闭加载状态
      Modal.destroyAll();
      
      // 处理表单验证错误
      if (error.errorFields) {
        messageApi.error('表单验证失败，请检查输入');
      } else {
        // 处理网络错误或其他错误
        messageApi.error('表单提交失败：' + (error.message || '未知错误'));
      }
      console.error('Form submission error:', error)
    }
  }

  // 处理预渲染配置表单提交
  const handlePrerenderConfigSubmit = async () => {
    try {
      const values = await prerenderConfigForm.validateFields()
      
      // 转换表单数据格式，确保与后端API期望的结构匹配
      const siteData = {
        Name: editingPrerenderSite.name,
        Domain: editingPrerenderSite.domain,
        Domains: editingPrerenderSite.domains || [editingPrerenderSite.domain],
        Port: editingPrerenderSite.port || 80,
        Mode: editingPrerenderSite.mode || 'proxy',
        // 保留原有的其他配置
        Proxy: {
          Enabled: editingPrerenderSite.proxy?.enabled || false,
          TargetURL: editingPrerenderSite.proxy?.targetURL || "",
          Type: "direct"
        },
        Redirect: {
          Enabled: editingPrerenderSite.redirect?.enabled || false,
          Code: editingPrerenderSite.redirect?.code || 302,
          URL: editingPrerenderSite.redirect?.url || ""
        },
        Firewall: {
          Enabled: editingPrerenderSite.firewall?.enabled || false,
          RulesPath: editingPrerenderSite.firewall?.rulesPath || '/etc/prerender-shield/rules',
          ActionConfig: {
            DefaultAction: editingPrerenderSite.firewall?.action?.defaultAction || 'block',
            BlockMessage: editingPrerenderSite.firewall?.action?.blockMessage || 'Request blocked by firewall'
          },
          GeoIPConfig: {
            Enabled: editingPrerenderSite.firewall?.geoip?.enabled || false,
            AllowList: editingPrerenderSite.firewall?.geoip?.allowList || [],
            BlockList: editingPrerenderSite.firewall?.geoip?.blockList || []
          },
          RateLimitConfig: {
            Enabled: editingPrerenderSite.firewall?.rateLimit?.enabled || false,
            Requests: editingPrerenderSite.firewall?.rateLimit?.requests || 100,
            Window: editingPrerenderSite.firewall?.rateLimit?.window || 60,
            BanTime: editingPrerenderSite.firewall?.rateLimit?.banTime || 3600
          }
        },
        // 网页防篡改配置
        FileIntegrityConfig: {
          Enabled: editingPrerenderSite.fileIntegrity?.enabled || false,
          CheckInterval: editingPrerenderSite.fileIntegrity?.checkInterval || 300,
          HashAlgorithm: editingPrerenderSite.fileIntegrity?.hashAlgorithm || 'sha256'
        },
        Prerender: {
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
        },
        Routing: {
          Rules: editingPrerenderSite.routing?.rules || []
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
      
      // 更新站点配置
      const res = await sitesApi.updateSite(editingPrerenderSite.id, siteData)
      
      // 关闭加载状态
      Modal.destroyAll();
      
      if (res.code === 200) {
        messageApi.success('更新渲染预热配置成功')
        setPrerenderConfigModalVisible(false)
        fetchSites() // 刷新站点列表
      } else {
        messageApi.error(res.message || '更新渲染预热配置失败')
      }
    } catch (error: any) {
      // 关闭加载状态
      Modal.destroyAll();
      
      // 处理表单验证错误
      if (error.errorFields) {
        messageApi.error('表单验证失败，请检查输入');
      } else {
        // 处理网络错误或其他错误
        messageApi.error('表单提交失败：' + (error.message || '未知错误'));
      }
      console.error('Prerender config submission error:', error)
    }
  }



  return (
    <>
      {contextHolder}
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
          scroll={{ x: 1200 }}
          style={{ tableLayout: 'fixed' }}
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

                  {/* 依赖于启用渲染预热的配置 */}
                  <Form.Item dependencies={[['prerender', 'enabled']]} noStyle>
                    {({ getFieldValue }) => {
                      const prerenderEnabled = getFieldValue(['prerender', 'enabled']);
                      if (!prerenderEnabled) {
                        return null;
                      }
                      return (
                        <div style={{ marginBottom: 16, padding: 10, backgroundColor: '#f0f9ff', borderRadius: 4, border: '1px solid #91d5ff' }}>
                          <p style={{ margin: 0, color: '#1890ff', fontWeight: 'bold' }}>提示：</p>
                          <p style={{ margin: '8px 0 0 0', color: '#40a9ff' }}>浏览器池大小和缓存TTL（秒）等高级配置，请在站点列表页面点击「渲染预热配置」按钮进行设置。</p>
                        </div>
                      );
                    }}
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
              danger
              icon={<DeleteOutlined />}
              onClick={() => {
                Modal.confirm({
                  title: '确认删除',
                  content: `确定要删除当前路径下的所有文件吗？此操作不可恢复。`,
                  okText: '删除',
                  okType: 'danger',
                  cancelText: '取消',
                  onOk: async () => {
                    try {
                      // 确保站点ID存在
                      const siteId = currentSite && currentSite.id;
                      if (!siteId) {
                        throw new Error('站点ID不存在');
                      }
                        
                      // 调用API删除当前路径下的所有静态资源
                      const response = await sitesApi.deleteStaticResources(siteId, currentPath);
                      if (response.code === 200) {
                        message.success('删除成功');
                        // 重新加载文件列表
                        loadFileList(currentPath, siteId);
                      } else {
                        message.error(`删除失败: ${response.message}`);
                      }
                    } catch (error) {
                      console.error('删除静态资源失败:', error);
                      message.error('删除静态资源失败: ' + (error as any).message);
                    }
                  },
                });
              }}
            >
              一键删除全部
            </Button>
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
            <Button
              icon={<ReloadOutlined />}
              onClick={() => {
                // 确保站点ID存在
                const siteId = currentSite && currentSite.id;
                if (siteId) {
                  loadFileList(currentPath, siteId);
                  messageApi.info('正在刷新文件列表...');
                }
              }}
            >
              刷新
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

      {/* 预渲染配置弹窗 */}
      <Modal
        title="渲染预热配置"
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
        >
          {/* 基础配置 */}
          <Card title="基础配置" size="small" style={{ marginBottom: 16 }}>
            <Form.Item
              name="enabled"
              label="启用渲染预热"
              valuePropName="checked"
            >
              <Switch />
            </Form.Item>

            {/* 依赖于启用渲染预热的配置 */}
            <Form.Item dependencies={['enabled']} noStyle>
              {({ getFieldValue }) => {
                const enabled = getFieldValue('enabled');
                if (!enabled) {
                  return null;
                }
                return (
                  <Row gutter={16}>
                    <Col span={8}>
                      <Form.Item
                        name="poolSize"
                        label="初始浏览器池大小"
                        rules={[{ required: true, message: '请输入初始浏览器池大小' }]}
                      >
                        <Input type="number" min={1} max={100} placeholder="请输入初始浏览器池大小" />
                      </Form.Item>
                    </Col>
                    <Col span={8}>
                      <Form.Item
                        name="minPoolSize"
                        label="最小浏览器池大小"
                        rules={[{ required: true, message: '请输入最小浏览器池大小' }]}
                      >
                        <Input type="number" min={1} max={100} placeholder="请输入最小浏览器池大小" />
                      </Form.Item>
                    </Col>
                    <Col span={8}>
                      <Form.Item
                        name="maxPoolSize"
                        label="最大浏览器池大小"
                        rules={[{ required: true, message: '请输入最大浏览器池大小' }]}
                      >
                        <Input type="number" min={1} max={100} placeholder="请输入最大浏览器池大小" />
                      </Form.Item>
                    </Col>
                    <Col span={8}>
                      <Form.Item
                        name="timeout"
                        label="渲染超时时间（秒）"
                        rules={[{ required: true, message: '请输入渲染超时时间' }]}
                      >
                        <Input type="number" min={5} max={300} placeholder="请输入渲染超时时间" />
                      </Form.Item>
                    </Col>
                    <Col span={8}>
                      <Form.Item
                        name="cacheTTL"
                        label="缓存过期时间（秒）"
                        rules={[{ required: true, message: '请输入缓存过期时间' }]}
                      >
                        <Input type="number" min={60} max={86400} placeholder="请输入缓存过期时间" />
                      </Form.Item>
                    </Col>
                  </Row>
                );
              }}
            </Form.Item>
          </Card>

          {/* 高级配置 */}
          <Form.Item dependencies={['enabled']} noStyle>
            {({ getFieldValue }) => {
              const enabled = getFieldValue('enabled');
              if (!enabled) {
                return null;
              }
              return (
                <Card title="高级配置" size="small" style={{ marginBottom: 16 }}>
                  <Row gutter={16}>
                    <Col span={8}>
                      <Form.Item
                        name="idleTimeout"
                        label="浏览器空闲超时（秒）"
                      >
                        <Input type="number" min={60} max={3600} placeholder="请输入浏览器空闲超时时间" />
                      </Form.Item>
                    </Col>
                    <Col span={8}>
                      <Form.Item
                        name="dynamicScaling"
                        label="启用动态扩容"
                        valuePropName="checked"
                      >
                        <Switch />
                      </Form.Item>
                    </Col>
                    <Col span={8}>
                      <Form.Item
                        name="scalingFactor"
                        label="扩容因子"
                      >
                        <Input type="number" min={0.1} max={2} step={0.1} placeholder="请输入扩容因子" />
                      </Form.Item>
                    </Col>
                    <Col span={8}>
                      <Form.Item
                        name="scalingInterval"
                        label="扩容检查间隔（秒）"
                      >
                        <Input type="number" min={10} max={300} placeholder="请输入扩容检查间隔" />
                      </Form.Item>
                    </Col>
                    <Col span={8}>
                      <Form.Item
                        name="useDefaultHeaders"
                        label="使用默认爬虫头"
                        valuePropName="checked"
                      >
                        <Switch />
                      </Form.Item>
                    </Col>
                  </Row>
                  <Form.Item
                    name="crawlerHeaders"
                    label="自定义爬虫协议头"
                    description="每行一个，支持多个"
                  >
                    <Input.TextArea rows={4} placeholder="请输入自定义爬虫协议头，每行一个" />
                  </Form.Item>
                </Card>
              );
            }}
          </Form.Item>

          {/* 预热配置 */}
          <Form.Item dependencies={['enabled']} noStyle>
            {({ getFieldValue }) => {
              const enabled = getFieldValue('enabled');
              if (!enabled) {
                return null;
              }
              return (
                <Card title="预热配置" size="small">
                  <Row gutter={16}>
                    <Col span={8}>
                      <Form.Item
                        name={["preheat", "enabled"]}
                        label="启用自动预热"
                        valuePropName="checked"
                      >
                        <Switch />
                      </Form.Item>
                    </Col>
                    <Col span={8}>
                      <Form.Item
                        name={["preheat", "sitemapURL"]}
                        label="Sitemap URL"
                      >
                        <Input placeholder="请输入Sitemap URL" />
                      </Form.Item>
                    </Col>
                    <Col span={8}>
                      <Form.Item
                        name={["preheat", "schedule"]}
                        label="预热调度规则"
                      >
                        <Input placeholder="Cron表达式，如：0 0 * * *" />
                      </Form.Item>
                    </Col>
                    <Col span={8}>
                      <Form.Item
                        name={["preheat", "concurrency"]}
                        label="预热并发数"
                      >
                        <Input type="number" min={1} max={50} placeholder="请输入预热并发数" />
                      </Form.Item>
                    </Col>
                    <Col span={8}>
                      <Form.Item
                        name={["preheat", "defaultPriority"]}
                        label="默认优先级"
                      >
                        <Input type="number" min={0} max={100} placeholder="请输入默认优先级" />
                      </Form.Item>
                    </Col>
                  </Row>
                </Card>
              );
            }}
          </Form.Item>
        </Form>
      </Modal>

    </div>
    </>
  )
}

export default Sites
