import React, { useState, useEffect } from 'react'
import { Card, Table, Button, Modal, Form, Input, Switch, Select, Row, Col, Statistic, Upload, Typography, Space, message, Divider, Checkbox, Empty } from 'antd'
import { 
  PlusOutlined, EditOutlined, DeleteOutlined, EyeOutlined, UploadOutlined, 
  UnorderedListOutlined, CloudUploadOutlined, FolderOpenOutlined, 
  FolderOutlined, FileOutlined, FolderOutlined as NewFolderOutlined, FileAddOutlined, UpOutlined, 
  DownloadOutlined, UnorderedListOutlined as ExtractOutlined, ReloadOutlined,
  SecurityScanOutlined, SearchOutlined
} from '@ant-design/icons'
import { sitesApi } from '../../services/api'
import type { UploadProps } from 'antd'
import { COUNTRIES } from '../../constants/countries'

const { Option } = Select

const Sites: React.FC = () => {
  // 使用useMessage hook来获取message实例，支持主题配置
  const [messageApi, contextHolder] = message.useMessage();
  const [sites, setSites] = useState<any[]>([])
  const [loading, setLoading] = useState(false)
  const [visible, setVisible] = useState(false)
  const [editingSite, setEditingSite] = useState<any>(null)
  const [form] = Form.useForm()
  
  // 国家选择器状态
  const [countrySelectorVisible, setCountrySelectorVisible] = useState(false)
  const [countrySelectorTarget, setCountrySelectorTarget] = useState<'allowList' | 'blockList'>('allowList')
  const [selectedCountries, setSelectedCountries] = useState<string[]>([])
  const [countrySearchKeyword, setCountrySearchKeyword] = useState('')
  
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
  
  // 推送配置模态框状态
  const [pushConfigModalVisible, setPushConfigModalVisible] = useState(false)
  const [editingPushSite, setEditingPushSite] = useState<any>(null)
  const [pushConfigForm] = Form.useForm()
  
  // WAF配置模态框状态
  const [wafConfigModalVisible, setWafConfigModalVisible] = useState(false)
  const [editingWafSite, setEditingWafSite] = useState<any>(null)
  const [wafConfigForm] = Form.useForm()
  
  // 静态资源管理选中的行
  const [selectedRowKeys, setSelectedRowKeys] = useState<React.Key[]>([])

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
      align: 'center' as const,
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
      align: 'center' as const,
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
      align: 'center' as const,
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
      fixed: 'right' as const,
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
                渲染预热
              </Button>
              <Button
                type="link"
                icon={<SecurityScanOutlined />}
                onClick={() => handleWafConfig(record)}
                style={{ marginRight: 8, whiteSpace: 'nowrap' }}
              >
                WAF配置
              </Button>
              <Button
                type="link"
                icon={<CloudUploadOutlined />}
                onClick={() => handlePushConfig(record)}
                style={{ marginRight: 8, whiteSpace: 'nowrap' }}
              >
                推送配置
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
          firewallEnabled: Boolean(site.firewall?.enabled),
          prerenderEnabled: Boolean(site.prerender?.enabled),
          
          // 映射完整的配置对象，确保编辑时表单能回填数据
          proxy: site.proxy || {},
          redirect: site.redirect || {},
          firewall: site.firewall || {},
          file_integrity: site.file_integrity || {},
          routing: site.routing || {},
          
          // 映射完整的渲染预热配置对象
          prerender: {
            enabled: site.prerender?.enabled || false,
            poolSize: site.prerender?.pool_size || site.prerender?.poolSize || 5,
            minPoolSize: site.prerender?.min_pool_size || site.prerender?.minPoolSize || 2,
            maxPoolSize: site.prerender?.max_pool_size || site.prerender?.maxPoolSize || 20,
            timeout: site.prerender?.timeout || 30,
            cacheTTL: site.prerender?.cache_ttl || site.prerender?.cacheTTL || 3600,
            idleTimeout: site.prerender?.idle_timeout || site.prerender?.idleTimeout || 300,
            dynamicScaling: site.prerender?.dynamic_scaling !== false && site.prerender?.dynamicScaling !== false,
            scalingFactor: site.prerender?.scaling_factor || site.prerender?.scalingFactor || 0.5,
            scalingInterval: site.prerender?.scaling_interval || site.prerender?.scalingInterval || 60,
            useDefaultHeaders: site.prerender?.use_default_headers || site.prerender?.useDefaultHeaders || false,
            crawlerHeaders: site.prerender?.crawler_headers || site.prerender?.crawlerHeaders || [],
            preheat: {
              enabled: site.prerender?.preheat?.enabled || false,
              sitemapURL: site.prerender?.preheat?.sitemap_url || site.prerender?.preheat?.sitemapURL || '',
              schedule: site.prerender?.preheat?.schedule || '0 0 * * *',
              concurrency: site.prerender?.preheat?.concurrency || 5,
              defaultPriority: site.prerender?.preheat?.default_priority || site.prerender?.preheat?.defaultPriority || 0,
              maxDepth: site.prerender?.preheat?.max_depth || site.prerender?.preheat?.maxDepth || 3
            },
            push: {
              enabled: site.prerender?.push?.enabled || false,
              baiduAPI: site.prerender?.push?.baidu_api || site.prerender?.push?.baiduAPI || 'http://data.zz.baidu.com/urls',
              baiduToken: site.prerender?.push?.baidu_token || site.prerender?.push?.baiduToken || '',
              bingAPI: site.prerender?.push?.bing_api || site.prerender?.push?.bingAPI || 'https://ssl.bing.com/webmaster/api.svc/json/SubmitUrl',
              bingToken: site.prerender?.push?.bing_token || site.prerender?.push?.bingToken || '',
              baiduDailyLimit: site.prerender?.push?.baidu_daily_limit || site.prerender?.push?.baiduDailyLimit || 1000,
              bingDailyLimit: site.prerender?.push?.bing_daily_limit || site.prerender?.push?.bingDailyLimit || 1000,
              pushDomain: site.prerender?.push?.push_domain || site.prerender?.push?.pushDomain || '',
              schedule: site.prerender?.push?.schedule || '0 1 * * *'
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
      // 同时将后端返回的下划线命名转换为前端表单期望的驼峰命名
      const siteWithStringPort = {
        ...site,
        port: String(site.port),
        // 转换firewall配置
        firewall: {
          ...site.firewall,
          action: {
            ...site.firewall?.action,
            defaultAction: site.firewall?.action?.default_action || 'block',
            blockMessage: site.firewall?.action?.block_message || 'Request blocked by firewall'
          },
          geoip: {
            ...site.firewall?.geoip,
            allowList: site.firewall?.geoip?.allow_list || [],
            blockList: site.firewall?.geoip?.block_list || []
          },
          rate_limit: site.firewall?.rate_limit ? {
            ...site.firewall.rate_limit,
            requests: site.firewall.rate_limit.requests || 100,
            window: site.firewall.rate_limit.window || 60,
            ban_time: site.firewall.rate_limit.ban_time || 3600
          } : {
            enabled: false,
            requests: 100,
            window: 60,
            ban_time: 3600
          }
        },
        // 转换file_integrity配置
        file_integrity: site.file_integrity ? {
          ...site.file_integrity,
          check_interval: site.file_integrity.check_interval || 300,
          hash_algorithm: site.file_integrity.hash_algorithm || 'sha256'
        } : {
          enabled: false,
          check_interval: 300,
          hash_algorithm: 'sha256'
        },
        // 转换prerender配置
        prerender: {
          ...site.prerender,
          poolSize: site.prerender?.pool_size || 5,
          minPoolSize: site.prerender?.min_pool_size || 2,
          maxPoolSize: site.prerender?.max_pool_size || 20,
          cacheTTL: site.prerender?.cache_ttl || 3600,
          idleTimeout: site.prerender?.idle_timeout || 300,
          dynamicScaling: site.prerender?.dynamic_scaling !== false,
          scalingFactor: site.prerender?.scaling_factor || 0.5,
          scalingInterval: site.prerender?.scaling_interval || 60,
          useDefaultHeaders: site.prerender?.use_default_headers || false,
          crawlerHeaders: site.prerender?.crawler_headers || [],
          preheat: {
            ...site.prerender?.preheat,
            sitemapURL: site.prerender?.preheat?.sitemap_url || '',
            defaultPriority: site.prerender?.preheat?.default_priority || 0,
            maxDepth: site.prerender?.preheat?.max_depth || 3
          },
          push: {
            ...site.prerender?.push,
            baiduAPI: site.prerender?.push?.baidu_api || 'http://data.zz.baidu.com/urls',
            baiduToken: site.prerender?.push?.baidu_token || '',
            baiduDailyLimit: site.prerender?.push?.baidu_daily_limit || 1000,
            bingAPI: site.prerender?.push?.bing_api || 'https://ssl.bing.com/webmaster/api.svc/json/SubmitUrl',
            bingToken: site.prerender?.push?.bing_token || '',
            bingDailyLimit: site.prerender?.push?.bing_daily_limit || 1000,
            pushDomain: site.prerender?.push?.push_domain || ''
          }
        }
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
        setSelectedRowKeys([])
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



  // 批量删除
  const handleBatchDelete = async () => {
    if (!currentSite || selectedRowKeys.length === 0) return;
    
    Modal.confirm({
      title: '批量删除确认',
      content: `确定要删除选中的 ${selectedRowKeys.length} 个文件/目录吗？`,
      okText: '确定',
      okType: 'danger',
      cancelText: '取消',
      onOk: async () => {
        try {
          // 构建路径列表
          const pathsToDelete = selectedRowKeys.map(key => {
             const record = fileList.find(f => f.key === key);
             const fileName = record ? record.name : key.toString().replace(/\/$/, '');
             return (currentPath === '/' ? '' : currentPath) + '/' + fileName;
          });

          await sitesApi.batchDeleteStaticResources(currentSite.id, pathsToDelete);
          messageApi.success('批量删除成功');
          setSelectedRowKeys([]); // 清空选择
          loadFileList(currentPath); // 刷新列表
        } catch (error: any) {
          console.error('Batch delete failed:', error);
          messageApi.error('批量删除失败: ' + (error.message || '未知错误'));
        }
      },
    });
  };
  
  // 清空目录
  const handleDeleteAll = async () => {
    if (!currentSite) return;
    if (fileList.length === 0) {
        messageApi.info('当前目录为空');
        return;
    }

    Modal.confirm({
      title: '清空目录确认',
      content: '确定要删除当前目录下的所有文件和子目录吗？此操作不可恢复！',
      okText: '确定删除全部',
      okType: 'danger',
      cancelText: '取消',
      onOk: async () => {
         try {
             const pathsToDelete = fileList.map(file => {
                 return (currentPath === '/' ? '' : currentPath) + '/' + file.name;
             });
             
             await sitesApi.batchDeleteStaticResources(currentSite.id, pathsToDelete);
             messageApi.success('清空目录成功');
             setSelectedRowKeys([]);
             loadFileList(currentPath);
         } catch (error: any) {
             console.error('Delete all failed:', error);
             messageApi.error('清空目录失败: ' + (error.message || '未知错误'));
         }
      }
    });
  }

  // 删除文件/目录
  const handleFileDelete = (file: any) => {
    if (!currentSite) return;
    Modal.confirm({
      title: '删除确认',
      content: `确定要删除 ${file.type === 'dir' ? '目录' : '文件'} "${file.name}" 吗？`,
      okText: '确定',
      okType: 'danger',
      cancelText: '取消',
      onOk: async () => {
        try {
          const fullPath = (currentPath === '/' ? '' : currentPath) + '/' + file.name;
          await sitesApi.deleteStaticResources(currentSite.id, fullPath);
          messageApi.success(`${file.name} 删除成功`);
          loadFileList(currentPath);
        } catch (error: any) {
          console.error('Delete file failed:', error);
          messageApi.error('删除失败: ' + (error.message || '未知错误'));
        }
      }
    });
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
  const handlePrerenderConfig = async (site: any) => {
    // 打开预渲染配置模态框
    setEditingPrerenderSite(site)
    
    try {
      // 从Redis获取已保存的预渲染配置
      const configResponse = await sitesApi.getSiteConfig(site.id, 'prerender');
      console.log('Redis prerender config:', configResponse);
      
      let redisConfig: any = {};
      if (configResponse.code === 200 && configResponse.data) {
        redisConfig = configResponse.data;
      }
      
      // 合并配置：优先使用Redis中的配置，如果没有则使用站点默认配置
      const mergedConfig = {
        // 基础配置
        enabled: redisConfig.enabled !== undefined ? (redisConfig.enabled === '1' || redisConfig.enabled === true || redisConfig.enabled === 'true') : (site.prerender?.enabled || false),
        poolSize: parseInt(redisConfig.pool_size) || site.prerender?.poolSize || 5,
        minPoolSize: parseInt(redisConfig.min_pool_size) || site.prerender?.minPoolSize || 2,
        maxPoolSize: parseInt(redisConfig.max_pool_size) || site.prerender?.maxPoolSize || 20,
        timeout: parseInt(redisConfig.timeout) || site.prerender?.timeout || 30,
        cacheTTL: parseInt(redisConfig.cache_ttl) || site.prerender?.cacheTTL || 3600,
        idleTimeout: parseInt(redisConfig.idle_timeout) || site.prerender?.idleTimeout || 300,
        dynamicScaling: redisConfig.dynamic_scaling !== undefined ? (redisConfig.dynamic_scaling === '1' || redisConfig.dynamic_scaling === true || redisConfig.dynamic_scaling === 'true') : (site.prerender?.dynamicScaling !== false),
        scalingFactor: parseFloat(redisConfig.scaling_factor) || site.prerender?.scalingFactor || 0.5,
        scalingInterval: parseInt(redisConfig.scaling_interval) || site.prerender?.scalingInterval || 60,
        useDefaultHeaders: redisConfig.use_default_headers !== undefined ? (redisConfig.use_default_headers === '1' || redisConfig.use_default_headers === true || redisConfig.use_default_headers === 'true') : (site.prerender?.useDefaultHeaders || false),
        
        // 预热配置
        preheat: {
          enabled: redisConfig.preheat_enabled !== undefined ? (redisConfig.preheat_enabled === '1' || redisConfig.preheat_enabled === true || redisConfig.preheat_enabled === 'true') : (site.prerender?.preheat?.enabled || false),
          sitemapURL: redisConfig.preheat_sitemap_url || site.prerender?.preheat?.sitemapURL || '',
          schedule: redisConfig.preheat_schedule || site.prerender?.preheat?.schedule || '0 0 * * *',
          concurrency: parseInt(redisConfig.preheat_concurrency) || site.prerender?.preheat?.concurrency || 5,
          defaultPriority: parseInt(redisConfig.preheat_default_priority) || site.prerender?.preheat?.defaultPriority || 0,
          maxDepth: parseInt(redisConfig.preheat_max_depth) || site.prerender?.preheat?.maxDepth || 3
        },
        
        // 爬虫头配置
        crawlerHeaders: site.prerender?.crawlerHeaders || getDefaultCrawlerHeaders()
      };
      
      console.log('Merged prerender config:', mergedConfig);
      prerenderConfigForm.setFieldsValue(mergedConfig);
      
    } catch (error) {
      console.error('Failed to load prerender config from Redis:', error);
      // 如果获取Redis配置失败，使用站点默认配置
      const defaultConfig = {
        enabled: site.prerender?.enabled || false,
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
        crawlerHeaders: site.prerender?.crawlerHeaders || getDefaultCrawlerHeaders(),
        preheat: {
          enabled: site.prerender?.preheat?.enabled || false,
          sitemapURL: site.prerender?.preheat?.sitemapURL || '',
          schedule: site.prerender?.preheat?.schedule || '0 0 * * *',
          concurrency: site.prerender?.preheat?.concurrency || 5,
          defaultPriority: site.prerender?.preheat?.defaultPriority || 0,
          maxDepth: site.prerender?.preheat?.maxDepth || 3
        }
      };
      prerenderConfigForm.setFieldsValue(defaultConfig);
    }
    
    setPrerenderConfigModalVisible(true);
  }
  
  // 获取默认爬虫头列表
  const getDefaultCrawlerHeaders = () => {
    return [
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
      'Yahoo! Slurp',
      'Applebot',
      'Mediapartners-Google',
      'AdsBot-Google',
      'Feedfetcher-Google',
      'Googlebot-Image',
      'Googlebot-News',
      'Googlebot-Video',
      'Googlebot-Extended',
      'bingbot/2.0',
      'msnbot',
      'MSNbot-Media',
      'bingbot/1.0',
      'msnbot-media/1.1',
      'adidxbot',
      'BingPreview',
      'BingSiteAuth',
      'BingLocalSearchBot',
      'Baiduspider-image',
      'Baiduspider-video',
      'Baiduspider-mobile',
      'Baiduspider-news',
      'Baiduspider-favo',
      'Baiduspider-cpro',
      'Baiduspider-ads',
      'Sogou web spider',
      'Sogou inst spider',
      'Sogou spider2',
      'Sogou blog',
      'Sogou News Spider',
      'Sogou Orion spider',
      'Sogou video spider',
      'Sogou image spider',
      'YandexBot/3.0',
      'YandexMobileBot',
      'YandexImages',
      'YandexVideo',
      'YandexMedia',
      'YandexBlogs',
      'YandexNews',
      'YandexCatalog'
    ];
  }
  
  // 处理推送配置
  const handlePushConfig = async (site: any) => {
    // 打开推送配置模态框
    setEditingPushSite(site)
    
    try {
      // 从Redis获取已保存的推送配置
      const configResponse = await sitesApi.getSiteConfig(site.id, 'push');
      console.log('Redis push config:', configResponse);
      
      let redisConfig: any = {};
      if (configResponse.code === 200 && configResponse.data) {
        redisConfig = configResponse.data;
      }
      
      // 合并配置：优先使用Redis中的配置，如果没有则使用站点默认配置
      const mergedConfig = {
        enabled: redisConfig.enabled !== undefined ? (redisConfig.enabled === '1' || redisConfig.enabled === true || redisConfig.enabled === 'true') : (site.prerender?.push?.enabled || false),
        baiduAPI: redisConfig.baidu_api || site.prerender?.push?.baiduAPI || 'http://data.zz.baidu.com/urls',
        baiduToken: redisConfig.baidu_token || site.prerender?.push?.baiduToken || '',
        baiduDailyLimit: parseInt(redisConfig.baidu_daily_limit) || site.prerender?.push?.baiduDailyLimit || 1000,
        bingAPI: redisConfig.bing_api || site.prerender?.push?.bingAPI || 'https://ssl.bing.com/webmaster/api.svc/json/SubmitUrl',
        bingToken: redisConfig.bing_token || site.prerender?.push?.bingToken || '',
        bingDailyLimit: parseInt(redisConfig.bing_daily_limit) || site.prerender?.push?.bingDailyLimit || 1000,
        pushDomain: redisConfig.push_domain || site.prerender?.push?.pushDomain || '',
        schedule: redisConfig.schedule || site.prerender?.push?.schedule || '0 1 * * *'
      };
      
      console.log('Merged push config:', mergedConfig);
      pushConfigForm.setFieldsValue(mergedConfig);
      
    } catch (error) {
      console.error('Failed to load push config from Redis:', error);
      // 如果获取Redis配置失败，使用站点默认配置
      const defaultConfig = {
        enabled: site.prerender?.push?.enabled || false,
        baiduAPI: site.prerender?.push?.baiduAPI || 'http://data.zz.baidu.com/urls',
        baiduToken: site.prerender?.push?.baiduToken || '',
        baiduDailyLimit: site.prerender?.push?.baiduDailyLimit || 1000,
        bingAPI: site.prerender?.push?.bingAPI || 'https://ssl.bing.com/webmaster/api.svc/json/SubmitUrl',
        bingToken: site.prerender?.push?.bingToken || '',
        bingDailyLimit: site.prerender?.push?.bingDailyLimit || 1000,
        pushDomain: site.prerender?.push?.pushDomain || '',
        schedule: site.prerender?.push?.schedule || '0 1 * * *'
      };
      pushConfigForm.setFieldsValue(defaultConfig);
    }
    
    setPushConfigModalVisible(true);
  }


  // 处理表单提交
  const handleSubmit = async () => {
    try {
      const values = await form.validateFields()
      
      // 转换表单数据格式，确保与后端API期望的结构匹配
      const siteData = {
        name: values.name,
        domain: values.domain,
        domains: [values.domain], // 支持多个域名，先添加主域名
        port: parseInt(values.port, 10) || 80, // 转换为整数类型，默认为80
        mode: values.mode, // 添加站点模式
        // 代理配置 - 根据模式决定是否启用
        proxy: {
          enabled: values.mode === 'proxy',
          target_url: values.mode === 'proxy' ? (values.proxy?.targetURL || "") : "",
          type: "direct" // 简化为固定值
        },
        // 重定向配置 - 根据模式决定是否启用
        redirect: {
          enabled: values.mode === 'redirect',
          status_code: values.mode === 'redirect' ? (values.redirect?.code || 302) : 302,
          target_url: values.mode === 'redirect' ? (values.redirect?.url || "") : ""
        },
        firewall: {
          enabled: values.firewall?.enabled || false,
          rules_path: values.firewall?.rulesPath || '/etc/prerender-shield/rules',
          action: {
            default_action: values.firewall?.action?.defaultAction || 'block',
            block_message: values.firewall?.action?.blockMessage || 'Request blocked by firewall'
          },
          // 地理位置访问控制配置
          geoip: {
            enabled: values.firewall?.geoip?.enabled || false,
            allow_list: values.firewall?.geoip?.allowList ? values.firewall.geoip.allowList.split(',').map((s: string) => s.trim()) : [],
            block_list: values.firewall?.geoip?.blockList ? values.firewall.geoip.blockList.split(',').map((s: string) => s.trim()) : []
          },
          // 频率限制配置
          rate_limit: {
            enabled: values.firewall?.rate_limit?.enabled || false,
            requests: values.firewall?.rate_limit?.requests || 100,
            window: values.firewall?.rate_limit?.window || 60,
            ban_time: values.firewall?.rate_limit?.ban_time || 3600
          }
        },
        // 网页防篡改配置
        file_integrity: {
          enabled: values.file_integrity?.enabled || false,
          check_interval: values.file_integrity?.check_interval || 300,
          hash_algorithm: values.file_integrity?.hash_algorithm || 'sha256'
        },
        prerender: {
          enabled: values.prerender?.enabled || false,
          pool_size: values.prerender?.poolSize || 5,
          min_pool_size: values.prerender?.minPoolSize || 2,
          max_pool_size: values.prerender?.maxPoolSize || 20,
          timeout: values.prerender?.timeout || 30,
          cache_ttl: values.prerender?.cacheTTL || 3600,
          idle_timeout: values.prerender?.idleTimeout || 300,
          dynamic_scaling: values.prerender?.dynamicScaling || true,
          scaling_factor: values.prerender?.scalingFactor || 0.5,
          scaling_interval: values.prerender?.scalingInterval || 60,
          use_default_headers: values.prerender?.useDefaultHeaders || false,
          crawler_headers: values.prerender?.crawlerHeaders || [],
          preheat: {
            enabled: values.prerender?.preheat?.enabled || false,
            sitemap_url: values.prerender?.preheat?.sitemapURL || '',
            schedule: values.prerender?.preheat?.schedule || '0 0 * * *',
            concurrency: values.prerender?.preheat?.concurrency || 5,
            default_priority: values.prerender?.preheat?.defaultPriority || 0
          }
        },
        routing: {
          rules: values.routing?.rules || []
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
      const values = await prerenderConfigForm.validateFields();
      
      // 转换爬虫协议头为数组格式
      const crawlerHeadersArray = typeof values.crawlerHeaders === 'string' 
        ? values.crawlerHeaders.split('\n').filter((header: string) => header.trim() !== '')
        : (values.crawlerHeaders || []);
      
      // 构造预渲染配置数据
      const configData = {
          enabled: values.enabled || false,
          pool_size: parseInt(values.poolSize) || 5,
          min_pool_size: parseInt(values.minPoolSize) || 2,
          max_pool_size: parseInt(values.maxPoolSize) || 20,
          timeout: parseInt(values.timeout) || 30,
          cache_ttl: parseInt(values.cacheTTL) || 3600,
          idle_timeout: parseInt(values.idleTimeout) || 300,
          dynamic_scaling: values.dynamicScaling !== false,
          scaling_factor: parseFloat(values.scalingFactor) || 0.5,
          scaling_interval: parseInt(values.scalingInterval) || 60,
          use_default_headers: values.useDefaultHeaders || false,
          crawler_headers: crawlerHeadersArray,
          preheat: {
            enabled: values.preheat?.enabled || false,
            sitemap_url: values.preheat?.sitemapURL || '',
            schedule: values.preheat?.schedule || '0 0 * * *',
            concurrency: parseInt(values.preheat?.concurrency) || 5,
            default_priority: parseInt(values.preheat?.defaultPriority) || 0,
            max_depth: parseInt(values.preheat?.maxDepth) || 1
          }
      };
      
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
      const res = await sitesApi.updatePrerenderConfig(editingPrerenderSite.id, configData);
      
      // 关闭加载状态
      Modal.destroyAll();
      
      if (res.code === 200) {
        messageApi.success('更新预渲染配置成功');
        setPrerenderConfigModalVisible(false);
        fetchSites(); // 刷新站点列表
      } else {
        messageApi.error(res.message || '更新预渲染配置失败');
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
      console.error('Prerender config submission error:', error);
    }
  }
  
  // 处理推送配置表单提交
  const handlePushConfigSubmit = async () => {
    try {
      const values = await pushConfigForm.validateFields();
      
      // 构造推送配置数据
      const configData = {
          enabled: values.enabled || false,
          baidu_api: values.baiduAPI || 'http://data.zz.baidu.com/urls',
          baidu_token: values.baiduToken || '',
          baidu_daily_limit: parseInt(values.baiduDailyLimit) || 1000,
          bing_api: values.bingAPI || 'https://ssl.bing.com/webmaster/api.svc/json/SubmitUrl',
          bing_token: values.bingToken || '',
          bing_daily_limit: parseInt(values.bingDailyLimit) || 1000,
          push_domain: values.pushDomain || '',
          schedule: values.schedule || '0 1 * * *'
      };
      
      // 显示加载状态
      Modal.confirm({
        title: '正在保存推送配置',
        content: '请稍候...',
        okButtonProps: { disabled: true },
        cancelButtonProps: { disabled: true },
        closable: false,
        keyboard: false,
        centered: true,
      });
      
      // 更新站点配置
      const res = await sitesApi.updatePushConfig(editingPushSite.id, configData);
      
      // 关闭加载状态
      Modal.destroyAll();
      
      if (res.code === 200) {
        messageApi.success('更新推送配置成功');
        setPushConfigModalVisible(false);
        fetchSites(); // 刷新站点列表
      } else {
        messageApi.error(res.message || '更新推送配置失败');
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
      console.error('Push config submission error:', error);
    }
  }

  // 处理WAF配置
  const handleWafConfig = async (site: any) => {
    // 打开WAF配置模态框
    setEditingWafSite(site)
    
    try {
      // 从Redis获取已保存的防火墙配置
      let redisConfig: any = {};
      try {
        const res = await sitesApi.getSiteConfig(site.id, 'waf');
        if (res.code === 200 && res.data) {
          redisConfig = res.data;
        }
      } catch (err) {
        console.warn('Failed to fetch WAF config from Redis, falling back to site config', err);
      }
      
      // 准备表单初始值
      const wafConfig = {
        // 防火墙基础配置
        firewall: {
          enabled: redisConfig.firewall_enabled !== undefined ? (redisConfig.firewall_enabled === true || redisConfig.firewall_enabled === 'true') : (site.firewall?.enabled || false),
          rulesPath: site.firewall?.rulesPath || '/etc/prerender-shield/rules',
          action: {
            defaultAction: redisConfig.default_action || site.firewall?.action?.defaultAction || 'block',
            blockMessage: redisConfig.block_message || site.firewall?.action?.blockMessage || 'Request blocked by firewall'
          },
          // 地理位置访问控制配置
          geoip: {
            enabled: redisConfig.geoip_enabled !== undefined ? (redisConfig.geoip_enabled === true || redisConfig.geoip_enabled === 'true') : (site.firewall?.geoip?.enabled || false),
            allowList: site.firewall?.geoip?.allowList || [],
            blockList: redisConfig.geoip_block_list ? (typeof redisConfig.geoip_block_list === 'string' ? redisConfig.geoip_block_list.split(',').filter(Boolean) : redisConfig.geoip_block_list) : (site.firewall?.geoip?.blockList || [])
          },
          // 频率限制配置
          rateLimit: {
            enabled: redisConfig.ratelimit_enabled !== undefined ? (redisConfig.ratelimit_enabled === true || redisConfig.ratelimit_enabled === 'true') : (site.firewall?.rate_limit?.enabled || false),
            requests: parseInt(redisConfig.ratelimit_requests) || site.firewall?.rate_limit?.requests || 100,
            window: parseInt(redisConfig.ratelimit_window) || site.firewall?.rate_limit?.window || 60,
            banTime: parseInt(redisConfig.ratelimit_ban_time) || site.firewall?.rate_limit?.ban_time || 3600
          },
          // IP黑白名单
          whitelist: redisConfig.whitelist ? (typeof redisConfig.whitelist === 'string' ? redisConfig.whitelist.split(',').filter(Boolean) : redisConfig.whitelist) : (site.firewall?.whitelist || []),
          blacklist: redisConfig.blacklist ? (typeof redisConfig.blacklist === 'string' ? redisConfig.blacklist.split(',').filter(Boolean) : redisConfig.blacklist) : (site.firewall?.blacklist || [])
        },
        // 网页防篡改配置
        fileIntegrity: {
          enabled: site.file_integrity?.enabled || false,
          checkInterval: site.file_integrity?.check_interval || 300,
          hashAlgorithm: site.file_integrity?.hash_algorithm || 'sha256'
        }
      };
      
      console.log('WAF config initial values:', wafConfig);
      wafConfigForm.setFieldsValue(wafConfig);
      
    } catch (error) {
      console.error('Failed to load WAF config:', error);
      // 如果出错，使用默认配置
      const defaultConfig = {
        firewall: {
          enabled: false,
          rulesPath: '/etc/prerender-shield/rules',
          action: {
            defaultAction: 'block',
            blockMessage: 'Request blocked by firewall'
          },
          geoip: {
            enabled: false,
            allowList: [],
            blockList: []
          },
          rateLimit: {
            enabled: false,
            requests: 100,
            window: 60,
            banTime: 3600
          }
        },
        fileIntegrity: {
          enabled: false,
          checkInterval: 300,
          hashAlgorithm: 'sha256'
        }
      };
      wafConfigForm.setFieldsValue(defaultConfig);
    }
    
    setWafConfigModalVisible(true);
  }

  // 处理WAF配置表单提交
  const handleWafConfigSubmit = async () => {
    try {
      const values = await wafConfigForm.validateFields();
      console.log('WAF form values:', values);
      
      // 构造WAF配置数据
      const configData = {
          enabled: values.firewall?.enabled || false,
          rules_path: values.firewall?.rulesPath || '/etc/prerender-shield/rules',
          action: {
            default_action: values.firewall?.action?.defaultAction || 'block',
            block_message: values.firewall?.action?.blockMessage || 'Request blocked by firewall'
          },
          geoip: {
            enabled: values.firewall?.geoip?.enabled || false,
            allow_list: values.firewall?.geoip?.allowList || [],
            block_list: values.firewall?.geoip?.blockList || []
          },
          rate_limit: {
            enabled: values.firewall?.rateLimit?.enabled || false,
            requests: parseInt(values.firewall?.rateLimit?.requests) || 100,
            window: parseInt(values.firewall?.rateLimit?.window) || 60,
            ban_time: parseInt(values.firewall?.rateLimit?.banTime) || 3600
          },
          blacklist: values.firewall?.blacklist || [],
          whitelist: values.firewall?.whitelist || []
      };
      
      // 显示加载状态
      Modal.confirm({
        title: '正在保存WAF配置',
        content: '请稍候...',
        okButtonProps: { disabled: true },
        cancelButtonProps: { disabled: true },
        closable: false,
        keyboard: false,
        centered: true,
      });
      
      // 更新站点配置
      const res = await sitesApi.updateFirewallConfig(editingWafSite.id, configData);
      
      // 关闭加载状态
      Modal.destroyAll();
      
      if (res.code === 200) {
        messageApi.success('更新WAF配置成功');
        setWafConfigModalVisible(false);
        fetchSites(); // 刷新站点列表
      } else {
        messageApi.error(res.message || '更新WAF配置失败');
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
      console.error('WAF config submission error:', error);
    }
  }

  // 打开国家选择器
  const handleOpenCountrySelector = (target: 'allowList' | 'blockList') => {
    setCountrySelectorTarget(target)
    setCountrySearchKeyword('')
    
    // 从表单获取当前选中的国家
    const formValues = wafConfigForm.getFieldsValue()
    const currentList = formValues.firewall?.geoip?.[target] || []
    
    // 确保是数组
    const currentArray = Array.isArray(currentList) ? currentList : []
    setSelectedCountries(currentArray)
    
    setCountrySelectorVisible(true)
  }

  // 确认国家选择
  const handleCountrySelectorOk = () => {
    // 更新表单字段
    const fieldPath = ['firewall', 'geoip', countrySelectorTarget]
    wafConfigForm.setFieldValue(fieldPath, selectedCountries)
    
    setCountrySelectorVisible(false)
  }

  // 全选/取消全选国家
  const handleToggleSelectAllCountries = (e: any) => {
    if (e.target.checked) {
      // 全选当前过滤后的国家
      const filteredCodes = filteredCountries.map(c => c.code)
      // 合并已选和新选，去重
      const newSelected = Array.from(new Set([...selectedCountries, ...filteredCodes]))
      setSelectedCountries(newSelected)
    } else {
      // 取消全选当前过滤后的国家
      const filteredCodes = new Set(filteredCountries.map(c => c.code))
      const newSelected = selectedCountries.filter(code => !filteredCodes.has(code))
      setSelectedCountries(newSelected)
    }
  }

  // 过滤国家列表
  const filteredCountries = COUNTRIES.filter(country => 
    country.name.toLowerCase().includes(countrySearchKeyword.toLowerCase()) || 
    country.cnName.includes(countrySearchKeyword) ||
    country.code.toLowerCase().includes(countrySearchKeyword.toLowerCase())
  )

  // 检查当前过滤列表是否已全选
  const isAllFilteredSelected = filteredCountries.length > 0 && 
    filteredCountries.every(c => selectedCountries.includes(c.code))
  
  // 检查当前过滤列表是否部分选中
  const isFilteredIndeterminate = filteredCountries.some(c => selectedCountries.includes(c.code)) && !isAllFilteredSelected




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
              crawlerHeaders: getDefaultCrawlerHeaders()
            }
          }}
        >
          <Form.Item name="name" label="站点名称" rules={[{ required: true, message: '请输入站点名称' }]}>
            <Input placeholder="请输入站点名称" />
          </Form.Item>
          <Form.Item name="domain" label="域名" rules={[{ required: true, message: '请输入域名' }]}>
            <Input placeholder="请输入域名，例如: example.com" />
          </Form.Item>
          <Form.Item name="port" label="端口" rules={[{ required: true, message: '请输入端口' }]}>
            <Input type="number" placeholder="请输入端口，例如: 80" />
          </Form.Item>
          <Form.Item name="mode" label="站点模式" rules={[{ required: true, message: '请选择站点模式' }]}>
            <Select>
              <Option value="proxy">代理已有应用</Option>
              <Option value="static">静态资源站</Option>
              <Option value="redirect">重定向</Option>
            </Select>
          </Form.Item>

          <Form.Item
            noStyle
            shouldUpdate={(prevValues, currentValues) => prevValues.mode !== currentValues.mode}
          >
            {({ getFieldValue }) => {
              const mode = getFieldValue('mode');
              return mode === 'proxy' ? (
                <Form.Item
                  name={['proxy', 'targetURL']}
                  label="目标URL"
                  rules={[{ required: true, message: '请输入目标URL' }]}
                >
                  <Input placeholder="http://localhost:3000" />
                </Form.Item>
              ) : mode === 'redirect' ? (
                 <>
                  <Form.Item
                    name={['redirect', 'code']}
                    label="状态码"
                    initialValue={302}
                  >
                    <Select>
                      <Option value={301}>301 (永久重定向)</Option>
                      <Option value={302}>302 (临时重定向)</Option>
                    </Select>
                  </Form.Item>
                  <Form.Item
                    name={['redirect', 'url']}
                    label="目标URL"
                    rules={[{ required: true, message: '请输入目标URL' }]}
                  >
                    <Input placeholder="https://example.com" />
                  </Form.Item>
                </>
              ) : null;
            }}
          </Form.Item>

          {/* Firewall and Prerender configurations removed from here as they have dedicated configuration buttons */}
          
        </Form>
      </Modal>

      {/* 渲染预热配置弹窗 */}
      <Modal
        title="渲染预热配置"
        open={prerenderConfigModalVisible}
        onOk={handlePrerenderConfigSubmit}
        onCancel={() => setPrerenderConfigModalVisible(false)}
        width={800}
      >
        <Form form={prerenderConfigForm} layout="vertical">
          <Form.Item name="enabled" label="启用预渲染" valuePropName="checked">
            <Switch />
          </Form.Item>
          
          <Row gutter={16}>
            <Col span={12}>
              <Form.Item name="poolSize" label="初始池大小">
                <Input type="number" />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="maxPoolSize" label="最大池大小">
                <Input type="number" />
              </Form.Item>
            </Col>
          </Row>
          
          <Divider orientation="left">预热设置</Divider>
          <Form.Item name={['preheat', 'enabled']} label="启用预热" valuePropName="checked">
            <Switch />
          </Form.Item>
          
          <Divider orientation="left">爬虫设置</Divider>
          <Form.Item name="crawlerHeaders" label="爬虫User-Agent列表" extra="支持输入自定义User-Agent，回车确认">
             <Select 
               mode="tags" 
               style={{ width: '100%' }} 
               tokenSeparators={[',', '\n']}
               options={getDefaultCrawlerHeaders().map(ua => ({ label: ua, value: ua }))}
               placeholder="请选择或输入常见的爬虫协议头"
             />
          </Form.Item>
        </Form>
      </Modal>


      {/* 推送配置弹窗 */}
      <Modal
        title="推送配置"
        open={pushConfigModalVisible}
        onOk={handlePushConfigSubmit}
        onCancel={() => setPushConfigModalVisible(false)}
        width={600}
      >
        <Form form={pushConfigForm} layout="vertical">
          <Form.Item name="enabled" label="启用推送" valuePropName="checked">
            <Switch />
          </Form.Item>
          <Divider orientation="left">百度推送</Divider>
          <Form.Item name="baiduAPI" label="百度推送API">
            <Input />
          </Form.Item>
          <Form.Item name="baiduToken" label="百度推送Token">
            <Input />
          </Form.Item>
          <Form.Item name="baiduDailyLimit" label="百度每日限制提交数量" tooltip="百度站长平台每日配额">
            <Input type="number" />
          </Form.Item>
          
          <Divider orientation="left">必应推送</Divider>
          <Form.Item name="bingAPI" label="Bing推送API">
            <Input />
          </Form.Item>
          <Form.Item name="bingToken" label="Bing推送Token">
            <Input />
          </Form.Item>
          <Form.Item name="bingDailyLimit" label="必应每日限制提交数量" tooltip="必应站长平台每日配额">
            <Input type="number" />
          </Form.Item>
        </Form>
      </Modal>

      {/* WAF配置弹窗 */}
      <Modal
        title="WAF配置"
        open={wafConfigModalVisible}
        onOk={handleWafConfigSubmit}
        onCancel={() => setWafConfigModalVisible(false)}
        width={800}
      >
        <Form form={wafConfigForm} layout="vertical">
          <Divider orientation="left">防火墙基础设置</Divider>
          <Form.Item name={['firewall', 'enabled']} label="启用防火墙" valuePropName="checked">
            <Switch />
          </Form.Item>
          <Form.Item name={['firewall', 'rulesPath']} label="规则路径">
            <Input placeholder="/etc/prerender-shield/rules" />
          </Form.Item>
          <Form.Item name={['firewall', 'action', 'defaultAction']} label="默认动作">
            <Select>
              <Option value="allow">允许</Option>
              <Option value="block">拦截</Option>
            </Select>
          </Form.Item>
          <Form.Item name={['firewall', 'action', 'blockMessage']} label="拦截消息">
            <Input />
          </Form.Item>

          <Divider orientation="left">地理位置访问控制</Divider>
          <Form.Item name={['firewall', 'geoip', 'enabled']} label="启用GeoIP" valuePropName="checked">
            <Switch />
          </Form.Item>
          <Form.Item name={['firewall', 'geoip', 'allowList']} label="允许国家列表">
             <div onClick={() => handleOpenCountrySelector('allowList')}>
               <Select 
                 mode="tags" 
                 placeholder="点击选择国家" 
                 style={{ width: '100%', cursor: 'pointer' }}
                 open={false}
                 tokenSeparators={[',']} 
                 showSearch={false}
               />
             </div>
          </Form.Item>
          <Form.Item name={['firewall', 'geoip', 'blockList']} label="禁止国家列表">
             <div onClick={() => handleOpenCountrySelector('blockList')}>
               <Select 
                 mode="tags" 
                 placeholder="点击选择国家" 
                 style={{ width: '100%', cursor: 'pointer' }}
                 open={false}
                 tokenSeparators={[',']} 
                 showSearch={false}
               />
             </div>
          </Form.Item>

          <Divider orientation="left">频率限制</Divider>
          <Form.Item name={['firewall', 'rateLimit', 'enabled']} label="启用频率限制" valuePropName="checked">
            <Switch />
          </Form.Item>
          <Row gutter={16}>
            <Col span={8}>
              <Form.Item name={['firewall', 'rateLimit', 'requests']} label="请求数限制">
                <Input type="number" />
              </Form.Item>
            </Col>
            <Col span={8}>
              <Form.Item name={['firewall', 'rateLimit', 'window']} label="时间窗口(秒)">
                <Input type="number" />
              </Form.Item>
            </Col>
            <Col span={8}>
              <Form.Item name={['firewall', 'rateLimit', 'banTime']} label="封禁时间(秒)">
                <Input type="number" />
              </Form.Item>
            </Col>
          </Row>

          <Divider orientation="left">IP黑白名单</Divider>
          <Form.Item name={['firewall', 'whitelist']} label="白名单IP列表" extra="一行一个IP，支持CIDR格式 (例如: 192.168.1.0/24)">
            <Select mode="tags" style={{ width: '100%' }} tokenSeparators={[',', '\n']} placeholder="请输入IP并回车" />
          </Form.Item>
          <Form.Item name={['firewall', 'blacklist']} label="黑名单IP列表" extra="一行一个IP，支持CIDR格式">
            <Select mode="tags" style={{ width: '100%' }} tokenSeparators={[',', '\n']} placeholder="请输入IP并回车" />
          </Form.Item>

          <Divider orientation="left">网页防篡改</Divider>
          <Form.Item name={['fileIntegrity', 'enabled']} label="启用防篡改" valuePropName="checked">
            <Switch />
          </Form.Item>
          <Row gutter={16}>
            <Col span={12}>
              <Form.Item name={['fileIntegrity', 'checkInterval']} label="检查间隔(秒)">
                <Input type="number" />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name={['fileIntegrity', 'hashAlgorithm']} label="哈希算法">
                <Select>
                  <Option value="md5">MD5</Option>
                  <Option value="sha1">SHA1</Option>
                  <Option value="sha256">SHA256</Option>
                </Select>
              </Form.Item>
            </Col>
          </Row>
        </Form>
      </Modal>

      {/* 静态资源管理弹窗 */}
      <Modal
        title={`静态资源管理 - ${currentSite?.name || ''}`}
        open={staticResModalVisible}
        onCancel={() => setStaticResModalVisible(false)}
        width={900}
        footer={null}
      >
        <div style={{ marginBottom: 16, display: 'flex', justifyContent: 'space-between' }}>
          <Space>
            <Button icon={<UpOutlined />} onClick={navigateUp} disabled={currentPath === '/'}>返回上级</Button>
            <Typography.Text strong>当前路径: {currentPath}</Typography.Text>
          </Space>
          <Space>
            <Button icon={<NewFolderOutlined />} onClick={handleNewFolder}>新建目录</Button>
            <Button icon={<FileAddOutlined />} onClick={handleNewFile}>新建文件</Button>
            <Upload 
              customRequest={customRequest} 
              beforeUpload={beforeUpload} 
              showUploadList={false}
            >
              <Button icon={<UploadOutlined />}>上传文件</Button>
            </Upload>
            {selectedRowKeys.length > 0 && (
                <Button danger icon={<DeleteOutlined />} onClick={handleBatchDelete}>批量删除</Button>
            )}
            <Button danger onClick={handleDeleteAll} disabled={fileList.length === 0}>一键删除全部</Button>
          </Space>
        </div>
        
        <Table
          dataSource={fileList}
          rowKey="key"
          pagination={false}
          rowSelection={{
            selectedRowKeys,
            onChange: (newSelectedRowKeys) => setSelectedRowKeys(newSelectedRowKeys),
          }}
          columns={[
            {
              title: '名称',
              dataIndex: 'name',
              key: 'name',
              render: (text, record) => (
                <Space>
                  {record.type === 'dir' ? <FolderOutlined style={{ color: '#1890ff' }} /> : <FileOutlined />}
                  {record.type === 'dir' ? (
                    <a onClick={() => enterDirectory(record)}>{text}</a>
                  ) : (
                    <span>{text}</span>
                  )}
                </Space>
              )
            },
            {
              title: '大小',
              dataIndex: 'size',
              key: 'size',
              width: 100,
              render: (size) => size ? `${(size / 1024).toFixed(2)} KB` : '-'
            },
            {
              title: '操作',
              key: 'action',
              width: 250,
              render: (_, record) => (
                <Space>
                  {record.type === 'file' && (
                    <>
                      <Button type="link" size="small" icon={<DownloadOutlined />} onClick={() => handleDownload(record)}>下载</Button>
                      {record.name.endsWith('.zip') && (
                        <Button type="link" size="small" icon={<ExtractOutlined />} onClick={() => handleExtract(record)}>解压</Button>
                      )}
                    </>
                  )}
                  <Button type="link" danger size="small" onClick={() => handleFileDelete(record)}>删除</Button>
                </Space>
              )
            }
          ]}
        />
      </Modal>

      {/* 新建目录弹窗 */}
      <Modal
        title="新建目录"
        open={showNewFolderModal}
        onOk={confirmNewFolder}
        onCancel={() => setShowNewFolderModal(false)}
      >
        <Input 
          placeholder="请输入目录名称" 
          value={newFolderName} 
          onChange={e => setNewFolderName(e.target.value)} 
        />
      </Modal>

      {/* 新建文件弹窗 */}
      <Modal
        title="新建文件"
        open={showNewFileModal}
        onOk={confirmNewFile}
        onCancel={() => setShowNewFileModal(false)}
      >
        <Input 
          placeholder="请输入文件名称" 
          value={newFileName} 
          onChange={e => setNewFileName(e.target.value)} 
        />
      </Modal>

      {/* 国家选择器弹窗 */}
      <Modal
        title={`选择国家 - ${countrySelectorTarget === 'allowList' ? '允许列表' : '禁止列表'}`}
        open={countrySelectorVisible}
        onOk={handleCountrySelectorOk}
        onCancel={() => setCountrySelectorVisible(false)}
        width={700}
        bodyStyle={{ maxHeight: '600px', overflowY: 'auto' }}
      >
        <div style={{ marginBottom: 16 }}>
          <Input 
            prefix={<SearchOutlined />} 
            placeholder="搜索国家 (代码、英文名或中文名)" 
            value={countrySearchKeyword}
            onChange={e => setCountrySearchKeyword(e.target.value)}
            allowClear
            style={{ marginBottom: 12 }}
          />
          <div style={{ padding: '8px 0', borderBottom: '1px solid #f0f0f0' }}>
            <Checkbox 
              checked={isAllFilteredSelected} 
              indeterminate={isFilteredIndeterminate}
              onChange={handleToggleSelectAllCountries}
              disabled={filteredCountries.length === 0}
            >
              全选当前列表 ({selectedCountries.length} 已选)
            </Checkbox>
          </div>
        </div>
        
        {filteredCountries.length > 0 ? (
          <Checkbox.Group 
            style={{ width: '100%' }} 
            value={selectedCountries} 
            onChange={(list) => setSelectedCountries(list as string[])}
          >
            <Row gutter={[8, 8]}>
              {filteredCountries.map(country => (
                <Col span={6} key={country.code}>
                  <Checkbox value={country.code} style={{ width: '100%', overflow: 'hidden', whiteSpace: 'nowrap', textOverflow: 'ellipsis' }} title={`${country.cnName} (${country.code})`}>
                    {country.cnName} <Typography.Text type="secondary">({country.code})</Typography.Text>
                  </Checkbox>
                </Col>
              ))}
            </Row>
          </Checkbox.Group>
        ) : (
          <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="未找到匹配的国家" />
        )}
      </Modal>

      </div>
    </>
  )
}

export default Sites