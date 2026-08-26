import React, { useState, useEffect } from 'react'
import { Card, Table, Button, Modal, Form, Input, Switch, Select, Row, Col, Statistic, Upload, Typography, Space, message, Divider, Checkbox, Empty } from 'antd'
import { 
  PlusOutlined, EditOutlined, DeleteOutlined, EyeOutlined, UploadOutlined, 
  UnorderedListOutlined, CloudUploadOutlined, FolderOpenOutlined, 
  FolderOutlined, FileOutlined, UpOutlined, 
  DownloadOutlined, UnorderedListOutlined as ExtractOutlined, ReloadOutlined,
  SecurityScanOutlined, SearchOutlined
} from '@ant-design/icons'
import { sitesApi } from '../../services/api'
import type { UploadProps } from 'antd'
import { COUNTRIES } from '../../constants/countries'
import { mapSiteResponse, buildEditFormValues } from './siteMapper'
import { useTranslation } from 'react-i18next'

const { Option } = Select

const Sites: React.FC = () => {
  const { t } = useTranslation()
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
      title: t('sites.columns.name'),
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
      title: t('sites.columns.domain'),
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
      title: t('sites.columns.port'),
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
      title: t('sites.columns.mode'),
      dataIndex: 'mode',
      key: 'mode',
      width: 120,
      render: (mode: string) => {
        const modeMap: { [key: string]: string } = {
          'proxy': t('sites.mode.proxy'),
          'static': t('sites.mode.static'),
          'redirect': t('sites.mode.redirect')
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
      title: t('sites.columns.prerenderStatus'),
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
      title: t('sites.columns.firewallStatus'),
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
      title: t('common.actions'),
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
            {t('common.view')}
          </Button>
          <Button
            type="link"
            icon={<EditOutlined />}
            onClick={() => handleEdit(record)}
            style={{ marginRight: 8, whiteSpace: 'nowrap' }}
          >
            {t('common.edit')}
          </Button>
          {record.mode === 'static' && (
            <>
              <Button
                type="link"
                icon={<FolderOpenOutlined />}
                onClick={() => handleStaticResources(record)}
                style={{ marginRight: 8, whiteSpace: 'nowrap' }}
              >
                {t('sites.staticManage')}
              </Button>
              <Button
                type="link"
                icon={<UnorderedListOutlined />}
                onClick={() => handlePrerenderConfig(record)}
                style={{ marginRight: 8, whiteSpace: 'nowrap' }}
              >
                {t('sites.prerenderConfig')}
              </Button>
              <Button
                type="link"
                icon={<SecurityScanOutlined />}
                onClick={() => handleWafConfig(record)}
                style={{ marginRight: 8, whiteSpace: 'nowrap' }}
              >
                {t('sites.wafConfig')}
              </Button>
              <Button
                type="link"
                icon={<CloudUploadOutlined />}
                onClick={() => handlePushConfig(record)}
                style={{ marginRight: 8, whiteSpace: 'nowrap' }}
              >
                {t('sites.pushConfig')}
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
            {t('common.delete')}
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

      // 使用配置好的sitesApi，自动携带Authorization头
      const response = await sitesApi.getSites();

      if (response && response.code === 200 && Array.isArray(response.data)) {
        // 直接使用原始数据，映射完整的渲染预热配置（纯函数已抽离至 siteMapper.ts）
        const mappedSites = response.data.map((site: any) => mapSiteResponse(site));

        setSites(mappedSites);
        messageApi.success(t('sites.messages.fetchSuccess'));
      } else {
        // 请求失败
        console.error('Failed to return valid sites data');
        messageApi.error(t('sites.messages.fetchFailed'));
      }
      
    } catch (error: any) {
      console.error('Unexpected error in fetchSites:', error);
      console.error('Error response:', error.response?.data);
      messageApi.error(t('sites.messages.fetchError', { error: error.message || t('sites.messages.unknownError') }));
    } finally {
      setLoading(false);
    }
  }
  


  // 初始化数据
  useEffect(() => {
    fetchSites()
  }, [])

  // 手动刷新站点列表
  const handleManualFetch = () => {
    fetchSites();
  }

  // 打开添加/编辑弹窗
  const showModal = (site: any = null) => {
    setEditingSite(site)
    if (site) {
      // 端口转字符串 + 下划线转驼峰（纯函数已抽离至 siteMapper.ts）
      const siteWithStringPort = buildEditFormValues(site);
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
          RulesPath: updatedSite.firewall.rulesPath || './rules',
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
        messageApi.success(t('sites.messages.updateSuccess'))
        fetchSites() // 刷新站点列表
      } else {
        messageApi.error(t('sites.messages.updateFailed'))
      }
    } catch (error) {
      console.error('Switch change error:', error)
      messageApi.error(t('sites.messages.updateFailed'))
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
      messageApi.error(t('sites.errors.invalidDomain'))
      return
    }
    
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
      messageApi.error(t('sites.errors.invalidSiteStatic'))
      return
    }
    
    // 确保站点ID存在且不为空
    const siteId = site.id || site.ID || '';
    if (!siteId.trim()) {
        console.error('Site ID is empty, cannot open static resources')
        messageApi.error(t('sites.errors.missingSiteId'))
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
    } else {
      // 如果finalSiteId看起来是站点名称而不是ID，尝试从sites数组中查找对应的ID
      // UUID格式的ID不需要查找
      const isUUID = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i.test(finalSiteId);
      if (!isUUID && (finalSiteId.includes(' ') || finalSiteId.length < 36)) {
        const site = sites.find(s => s.name === finalSiteId || s.Name === finalSiteId)
        if (site && site.id) {
          finalSiteId = site.id
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
        messageApi.error(t('sites.messages.loadFilesFailed'))
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

  // 下载文件
  const handleDownload = (file: any) => {
    message.info(t('sites.messages.downloading', { name: file.name }))
    // 创建临时下载链接
    const downloadLink = document.createElement('a');
    downloadLink.href = `/api/v1/sites/${currentSite?.id}/static${file.path}`;
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
      messageApi.error(t('sites.errors.invalidSiteExtract'))
      return
    }
    
    try {
      messageApi.info(t('sites.messages.extracting', { name: file.name }))
      
      // 发送解压请求到后端
      const response = await sitesApi.extractFile(currentSite.id, file.name, currentPath)
      
      if (response.code === 200) {
        messageApi.success(t('sites.messages.extractSuccess', { name: file.name }))
        // 重新加载文件列表
        loadFileList(currentPath)
      } else {
        messageApi.error(t('sites.messages.extractFailedReason', { name: file.name, reason: response.message || t('sites.messages.unknownError') }))
      }
    } catch (error) {
      console.error('解压失败:', error)
      messageApi.error(t('sites.messages.extractFailed', { name: file.name }))
    }
  }



  // 批量删除
  const handleBatchDelete = async () => {
    if (!currentSite || selectedRowKeys.length === 0) return;
    
    Modal.confirm({
      title: t('sites.confirm.batchDeleteTitle'),
      content: t('sites.confirm.batchDeleteContent', { count: selectedRowKeys.length }),
      okText: t('common.ok'),
      okType: 'danger',
      cancelText: t('common.cancel'),
      onOk: async () => {
        try {
          // 构建路径列表
          const pathsToDelete = selectedRowKeys.map(key => {
             const record = fileList.find(f => f.key === key);
             const fileName = record ? record.name : key.toString().replace(/\/$/, '');
             return (currentPath === '/' ? '' : currentPath) + '/' + fileName;
          });

          await sitesApi.batchDeleteStaticResources(currentSite.id, pathsToDelete);
          messageApi.success(t('sites.messages.batchDeleteSuccess'));
          setSelectedRowKeys([]); // 清空选择
          loadFileList(currentPath); // 刷新列表
        } catch (error: any) {
          console.error('Batch delete failed:', error);
          messageApi.error(t('sites.messages.batchDeleteFailed', { reason: error.message || t('sites.messages.unknownError') }));
        }
      },
    });
  };
  
  // 清空目录
  const handleDeleteAll = async () => {
    if (!currentSite) return;
    if (fileList.length === 0) {
        messageApi.info(t('sites.messages.dirEmpty'));
        return;
    }

    Modal.confirm({
      title: t('sites.confirm.deleteAllTitle'),
      content: t('sites.confirm.deleteAllContent'),
      okText: t('sites.confirm.deleteAllOk'),
      okType: 'danger',
      cancelText: t('common.cancel'),
      onOk: async () => {
         try {
             const pathsToDelete = fileList.map(file => {
                 return (currentPath === '/' ? '' : currentPath) + '/' + file.name;
             });
             
             await sitesApi.batchDeleteStaticResources(currentSite.id, pathsToDelete);
             messageApi.success(t('sites.messages.clearSuccess'));
             setSelectedRowKeys([]);
             loadFileList(currentPath);
         } catch (error: any) {
             console.error('Delete all failed:', error);
             messageApi.error(t('sites.messages.clearFailed', { reason: error.message || t('sites.messages.unknownError') }));
         }
      }
    });
  }

  // 删除文件/目录
  const handleFileDelete = (file: any) => {
    if (!currentSite) return;
    Modal.confirm({
      title: t('sites.confirm.fileDeleteTitle'),
      content: t('sites.confirm.fileDeleteContent', {
        type: t(file.type === 'dir' ? 'sites.fileType.directory' : 'sites.fileType.file'),
        name: file.name
      }),
      okText: t('common.ok'),
      okType: 'danger',
      cancelText: t('common.cancel'),
      onOk: async () => {
        try {
          const fullPath = (currentPath === '/' ? '' : currentPath) + '/' + file.name;
          await sitesApi.deleteStaticResources(currentSite.id, fullPath);
          messageApi.success(t('sites.messages.deleteFileSuccess', { name: file.name }));
          loadFileList(currentPath);
        } catch (error: any) {
          console.error('Delete file failed:', error);
          messageApi.error(t('sites.messages.deleteFileFailed', { reason: error.message || t('sites.messages.unknownError') }));
        }
      }
    });
  }



  // 文件上传前的处理
  const beforeUpload: UploadProps['beforeUpload'] = (file) => {
    // 调整rar/zip上传大小限制为不超过100m
    const isLt100M = file.size / 1024 / 1024 < 100
    if (!isLt100M) {
      messageApi.error(t('sites.errors.fileTooLarge'))
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
      messageApi.error(t('sites.errors.invalidSiteUpload'))
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
        messageApi.success(t('sites.messages.uploadSuccess', { name: typeof file === 'string' ? file : (file as any).name }))
        // 重新加载文件列表
        loadFileList(currentPath)
        if (onSuccess) onSuccess({ status: 'ok', message: '上传成功' })
      } else {
        throw new Error(response.message || '上传失败')
      }
    })
    .catch((error) => {
      messageApi.error(t('sites.messages.uploadFailed', { name: typeof file === 'string' ? file : (file as any).name, reason: error.message }))
      if (onError) onError(error)
    })
  }

  // 处理删除站点
  const handleDelete = (site: any) => {
    Modal.confirm({
      title: t('sites.confirm.deleteSiteTitle'),
      content: t('sites.confirm.deleteSiteContent', { name: site.name || site.Name || '' }),
      okText: t('sites.confirm.deleteSiteOk'),
      okType: 'danger',
      cancelText: t('common.cancel'),
      onOk: async () => {
        try {
          if (!site || typeof site !== 'object') {
            throw new Error('无效的站点对象')
          }
          
          const siteId = site.id || site.ID || '';
          if (!siteId.trim()) {
            throw new Error('站点ID不存在')
          }
          
          const res = await sitesApi.deleteSite(siteId)
          if (res.code === 200) {
            messageApi.success(t('sites.messages.deleteSiteSuccess'))
            fetchSites()
          } else {
            messageApi.error(t('sites.messages.deleteSiteFailed', { reason: res.message }))
          }
        } catch (error) {
          console.error('Failed to delete site:', error)
          messageApi.error(t('sites.messages.deleteSiteFailed', { reason: (error as any).message }))
        }
      },
    })
  }
  
  // 跳转到渲染预热配置页面
  const handlePrerenderConfig = async (site: any) => {
    // 打开预渲染配置模态框
    setEditingPrerenderSite(site)
    
    try {
      // 从Redis获取已保存的预渲染配置
      const configResponse = await sitesApi.getSiteConfig(site.id, 'prerender');
      
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
          rules_path: values.firewall?.rulesPath || './rules',
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
        title: t('sites.saving.site'),
        content: t('sites.pleaseWait'),
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
        res = await sitesApi.addSite(siteData)
      }

      // 关闭加载状态
      Modal.destroyAll();

      // 直接使用res，因为API响应拦截器已经返回了response.data
      if (res.code === 200) {
        messageApi.success(editingSite ? t('sites.messages.updateSuccess') : t('sites.messages.addSuccess'))
        setVisible(false)
        // 立即刷新站点列表
        fetchSites()
      } else {
        const reason = res.message || t('sites.messages.unknownError')
        messageApi.error(editingSite
          ? t('sites.messages.updateFailed') + ': ' + reason
          : t('sites.messages.addFailed') + ': ' + reason)
      }
    } catch (error: any) {
      // 关闭加载状态
      Modal.destroyAll();
      
      // 处理表单验证错误
      if (error.errorFields) {
        messageApi.error(t('sites.messages.formInvalid'));
      } else {
        // 处理网络错误或其他错误
        messageApi.error(t('sites.messages.submitFailed', { reason: error.message || t('sites.messages.unknownError') }));
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
        title: t('sites.saving.prerender'),
        content: t('sites.pleaseWait'),
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
        messageApi.success(t('sites.messages.prerenderSaved'));
        setPrerenderConfigModalVisible(false);
        fetchSites(); // 刷新站点列表
      } else {
        messageApi.error(res.message || t('sites.messages.prerenderSaveFailed'));
      }
    } catch (error: any) {
      // 关闭加载状态
      Modal.destroyAll();
      
      // 处理表单验证错误
      if (error.errorFields) {
        messageApi.error(t('sites.formValidationFailed'));
      } else {
        // 处理网络错误或其他错误
        messageApi.error(t('sites.messages.submitFailed', { reason: error.message || t('sites.messages.unknownError') }));
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
        title: t('sites.saving.push'),
        content: t('sites.pleaseWait'),
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
        messageApi.success(t('sites.messages.pushSaved'));
        setPushConfigModalVisible(false);
        fetchSites(); // 刷新站点列表
      } else {
        messageApi.error(res.message || t('sites.messages.pushSaveFailed'));
      }
    } catch (error: any) {
      // 关闭加载状态
      Modal.destroyAll();
      
      // 处理表单验证错误
      if (error.errorFields) {
        messageApi.error(t('sites.formValidationFailed'));
      } else {
        // 处理网络错误或其他错误
        messageApi.error(t('sites.messages.submitFailed', { reason: error.message || t('sites.messages.unknownError') }));
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
          rulesPath: site.firewall?.rulesPath || './rules',
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
      
      wafConfigForm.setFieldsValue(wafConfig);
      
    } catch (error) {
      console.error('Failed to load WAF config:', error);
      // 如果出错，使用默认配置
      const defaultConfig = {
        firewall: {
          enabled: false,
          rulesPath: './rules',
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
      
          // 构造WAF配置数据
          const configData = {
              enabled: values.firewall?.enabled || false,
              rules_path: values.firewall?.rulesPath || './rules',
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
        title: t('sites.saving.waf'),
        content: t('sites.pleaseWait'),
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
        messageApi.success(t('sites.messages.wafSaved'));
        setWafConfigModalVisible(false);
        fetchSites(); // 刷新站点列表
      } else {
        messageApi.error(res.message || t('sites.messages.wafSaveFailed'));
      }
    } catch (error: any) {
      // 关闭加载状态
      Modal.destroyAll();
      
      // 处理表单验证错误
      if (error.errorFields) {
        messageApi.error(t('sites.formValidationFailed'));
      } else {
        // 处理网络错误或其他错误
        messageApi.error(t('sites.messages.submitFailed', { reason: error.message || t('sites.messages.unknownError') }));
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
      <h1 className="page-title">{t('sites.title')}</h1>

      {/* 站点概览卡片 */}
      <Card className="card">
        <Row gutter={[16, 16]}>
          <Col span={8}>
            <Statistic
              title={t('sites.totalSites')}
              value={sites.length}
              valueStyle={{ color: '#1890ff' }}
            />
          </Col>
          <Col span={8}>
            <Statistic
              title={t('sites.prerenderEnabledSites')}
              value={sites.filter(site => site.prerender && site.prerender.enabled).length}
              valueStyle={{ color: '#52c41a' }}
            />
          </Col>
          <Col span={8}>
            <Statistic
              title={t('sites.firewallEnabledSites')}
              value={sites.filter(site => site.firewall && site.firewall.enabled).length}
              valueStyle={{ color: '#faad14' }}
            />
          </Col>
        </Row>
      </Card>

      {/* 站点列表 */}
      <Card className="card" title={t('sites.listTitle')} extra={
        <Space>
          <Button type="primary" icon={<PlusOutlined />} onClick={handleAdd}>
            {t('sites.addSite')}
          </Button>
          <Button icon={<ReloadOutlined />} onClick={handleManualFetch}>
            {t('sites.reload')}
          </Button>
        </Space>
      }>
        <Table
          columns={columns}
          dataSource={sites}
          rowKey="name"
          loading={loading}
          locale={{
            emptyText: (
              <Empty
                image={Empty.PRESENTED_IMAGE_SIMPLE}
                description={t('sites.empty.description')}
                style={{ padding: '24px 0' }}
              >
                <div style={{ maxWidth: 480, margin: '0 auto', textAlign: 'left', fontSize: 13, color: '#666' }}>
                  <p style={{ marginBottom: 8 }}><b>{t('sites.empty.stepsTitle')}</b></p>
                  <p>{t('sites.empty.step1')}</p>
                  <p>{t('sites.empty.step2')}</p>
                  <p>{t('sites.empty.step3')}</p>
                </div>
                <Button type="primary" icon={<PlusOutlined />} onClick={handleAdd} style={{ marginTop: 16 }}>
                  {t('sites.empty.addFirst')}
                </Button>
              </Empty>
            ),
          }}
          pagination={{ pageSize: 10 }}
          scroll={{ x: 1200 }}
          style={{ tableLayout: 'fixed' }}
        />
      </Card>

      {/* 添加/编辑站点弹窗 */}
      <Modal
        title={editingSite ? t('sites.modal.editSite') : t('sites.addSite')}
        open={visible}
        onOk={handleSubmit}
        onCancel={() => setVisible(false)}
        width={800}
        okText={t('common.save')}
        cancelText={t('common.cancel')}
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
          <Form.Item name="name" label={t('sites.columns.name')} rules={[{ required: true, message: t('sites.form.nameRequired') }]}>
            <Input placeholder={t('sites.form.nameRequired')} />
          </Form.Item>
          <Form.Item name="domain" label={t('sites.columns.domain')} rules={[{ required: true, message: t('sites.form.domainRequired') }]}>
            <Input placeholder={t('sites.form.domainPlaceholder')} />
          </Form.Item>
          <Form.Item name="port" label={t('sites.columns.port')} rules={[{ required: true, message: t('sites.form.portRequired') }]}>
            <Input type="number" placeholder={t('sites.form.portPlaceholder')} />
          </Form.Item>
          <Form.Item name="mode" label={t('sites.columns.mode')} rules={[{ required: true, message: t('sites.form.modeRequired') }]}>
            <Select>
              <Option value="proxy">{t('sites.mode.proxy')}</Option>
              <Option value="static">{t('sites.mode.static')}</Option>
              <Option value="redirect">{t('sites.mode.redirect')}</Option>
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
                  label={t('sites.form.targetURL')}
                  rules={[{ required: true, message: t('sites.form.targetURLRequired') }]}
                >
                  <Input placeholder="http://localhost:3000" />
                </Form.Item>
              ) : mode === 'redirect' ? (
                 <>
                  <Form.Item
                    name={['redirect', 'code']}
                    label={t('sites.form.statusCode')}
                    initialValue={302}
                  >
                    <Select>
                      <Option value={301}>{`301 (${t('sites.form.redirectPermanent')})`}</Option>
                      <Option value={302}>{`302 (${t('sites.form.redirectTemporary')})`}</Option>
                    </Select>
                  </Form.Item>
                  <Form.Item
                    name={['redirect', 'url']}
                    label={t('sites.form.targetURL')}
                    rules={[{ required: true, message: t('sites.form.targetURLRequired') }]}
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
        title={t('sites.modal.prerenderConfig')}
        open={prerenderConfigModalVisible}
        onOk={handlePrerenderConfigSubmit}
        onCancel={() => setPrerenderConfigModalVisible(false)}
        width={800}
      >
        <Form form={prerenderConfigForm} layout="vertical">
          <Form.Item name="enabled" label={t('sites.form.enablePrerender')} valuePropName="checked">
            <Switch />
          </Form.Item>
          
          <Row gutter={16}>
            <Col span={12}>
              <Form.Item name="poolSize" label={t('sites.form.initialPoolSize')}>
                <Input type="number" />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="maxPoolSize" label={t('sites.form.maxPoolSize')}>
                <Input type="number" />
              </Form.Item>
            </Col>
          </Row>
          
          <Divider orientation="left">{t('sites.sections.preheat')}</Divider>
          <Form.Item name={['preheat', 'enabled']} label={t('sites.form.enablePreheat')} valuePropName="checked">
            <Switch />
          </Form.Item>
          
          <Divider orientation="left">{t('sites.sections.crawler')}</Divider>
          <Form.Item name="crawlerHeaders" label={t('sites.form.crawlerUserAgents')} extra={t('sites.form.crawlerUserAgentsExtra')}>
             <Select 
               mode="tags" 
               style={{ width: '100%' }} 
               tokenSeparators={[',', '\n']}
               options={getDefaultCrawlerHeaders().map(ua => ({ label: ua, value: ua }))}
               placeholder={t('sites.form.crawlerUserAgentsPlaceholder')}
             />
          </Form.Item>
        </Form>
      </Modal>


      {/* 推送配置弹窗 */}
      <Modal
        title={t('sites.pushConfig')}
        open={pushConfigModalVisible}
        onOk={handlePushConfigSubmit}
        onCancel={() => setPushConfigModalVisible(false)}
        width={600}
      >
        <Form form={pushConfigForm} layout="vertical">
          <Form.Item name="enabled" label={t('sites.form.enablePush')} valuePropName="checked">
            <Switch />
          </Form.Item>
          <Divider orientation="left">{t('sites.sections.baiduPush')}</Divider>
          <Form.Item name="baiduAPI" label={t('sites.form.baiduAPI')}>
            <Input />
          </Form.Item>
          <Form.Item name="baiduToken" label={t('sites.form.baiduToken')}>
            <Input />
          </Form.Item>
          <Form.Item name="baiduDailyLimit" label={t('sites.form.baiduDailyLimit')} tooltip={t('sites.form.baiduQuota')}>
            <Input type="number" />
          </Form.Item>
          
          <Divider orientation="left">{t('sites.sections.bingPush')}</Divider>
          <Form.Item name="bingAPI" label={t('sites.form.bingAPI')}>
            <Input />
          </Form.Item>
          <Form.Item name="bingToken" label={t('sites.form.bingToken')}>
            <Input />
          </Form.Item>
          <Form.Item name="bingDailyLimit" label={t('sites.form.bingDailyLimit')} tooltip={t('sites.form.bingQuota')}>
            <Input type="number" />
          </Form.Item>
        </Form>
      </Modal>

      {/* WAF配置弹窗 */}
      <Modal
        title={t('sites.wafConfig')}
        open={wafConfigModalVisible}
        onOk={handleWafConfigSubmit}
        onCancel={() => setWafConfigModalVisible(false)}
        width={800}
      >
        <Form form={wafConfigForm} layout="vertical">
          <Divider orientation="left">{t('sites.sections.firewallBasics')}</Divider>
          <Form.Item name={['firewall', 'enabled']} label={t('sites.form.enableFirewall')} valuePropName="checked">
            <Switch />
          </Form.Item>
          <Form.Item name={['firewall', 'rulesPath']} label={t('sites.form.rulesPath')}>
            <Input placeholder="./rules" />
          </Form.Item>
          <Form.Item name={['firewall', 'action', 'defaultAction']} label={t('sites.form.defaultAction')}>
            <Select>
              <Option value="allow">{t('sites.form.allow')}</Option>
              <Option value="block">{t('sites.form.block')}</Option>
            </Select>
          </Form.Item>
          <Form.Item name={['firewall', 'action', 'blockMessage']} label={t('sites.form.blockMessage')}>
            <Input />
          </Form.Item>

          <Divider orientation="left">{t('sites.sections.geoip')}</Divider>
          <Form.Item name={['firewall', 'geoip', 'enabled']} label={t('sites.form.enableGeoIP')} valuePropName="checked">
            <Switch />
          </Form.Item>
          <Form.Item name={['firewall', 'geoip', 'allowList']} label={t('sites.form.allowCountries')}>
             <div onClick={() => handleOpenCountrySelector('allowList')}>
               <Select 
                 mode="tags" 
                 placeholder={t('sites.form.clickToSelect')} 
                 style={{ width: '100%', cursor: 'pointer' }}
                 open={false}
                 tokenSeparators={[',']} 
                 showSearch={false}
               />
             </div>
          </Form.Item>
          <Form.Item name={['firewall', 'geoip', 'blockList']} label={t('sites.form.blockCountries')}>
             <div onClick={() => handleOpenCountrySelector('blockList')}>
               <Select 
                 mode="tags" 
                 placeholder={t('sites.form.clickToSelect')} 
                 style={{ width: '100%', cursor: 'pointer' }}
                 open={false}
                 tokenSeparators={[',']} 
                 showSearch={false}
               />
             </div>
          </Form.Item>

          <Divider orientation="left">{t('sites.sections.rateLimit')}</Divider>
          <Form.Item name={['firewall', 'rateLimit', 'enabled']} label={t('sites.form.enableRateLimit')} valuePropName="checked">
            <Switch />
          </Form.Item>
          <Row gutter={16}>
            <Col span={8}>
              <Form.Item name={['firewall', 'rateLimit', 'requests']} label={t('sites.form.requestsLimit')}>
                <Input type="number" />
              </Form.Item>
            </Col>
            <Col span={8}>
              <Form.Item name={['firewall', 'rateLimit', 'window']} label={t('sites.form.timeWindowSec')}>
                <Input type="number" />
              </Form.Item>
            </Col>
            <Col span={8}>
              <Form.Item name={['firewall', 'rateLimit', 'banTime']} label={t('sites.form.banTimeSec')}>
                <Input type="number" />
              </Form.Item>
            </Col>
          </Row>

          <Divider orientation="left">{t('sites.sections.ipLists')}</Divider>
          <Form.Item name={['firewall', 'whitelist']} label={t('sites.form.whitelist')} extra={t('sites.form.ipFormatHintExample')}>
            <Select mode="tags" style={{ width: '100%' }} tokenSeparators={[',', '\n']} placeholder={t('sites.form.ipInputPlaceholder')} />
          </Form.Item>
          <Form.Item name={['firewall', 'blacklist']} label={t('sites.form.blacklist')} extra={t('sites.form.ipFormatHint')}>
            <Select mode="tags" style={{ width: '100%' }} tokenSeparators={[',', '\n']} placeholder={t('sites.form.ipInputPlaceholder')} />
          </Form.Item>

          <Divider orientation="left">{t('sites.sections.tamperProof')}</Divider>
          <Form.Item name={['fileIntegrity', 'enabled']} label={t('sites.form.enableTamperProof')} valuePropName="checked">
            <Switch />
          </Form.Item>
          <Row gutter={16}>
            <Col span={12}>
              <Form.Item name={['fileIntegrity', 'checkInterval']} label={t('sites.form.checkIntervalSec')}>
                <Input type="number" />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name={['fileIntegrity', 'hashAlgorithm']} label={t('sites.form.hashAlgorithm')}>
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
        title={t('sites.modal.staticResources', { name: currentSite?.name || '' })}
        open={staticResModalVisible}
        onCancel={() => setStaticResModalVisible(false)}
        width={900}
        footer={null}
      >
        <div style={{ marginBottom: 16, display: 'flex', justifyContent: 'space-between' }}>
          <Space>
            <Button icon={<UpOutlined />} onClick={navigateUp} disabled={currentPath === '/'}>{t('sites.static.back')}</Button>
            <Typography.Text strong>{t('sites.static.currentPath')} {currentPath}</Typography.Text>
          </Space>
          <Space>
            <Upload 
              customRequest={customRequest} 
              beforeUpload={beforeUpload} 
              showUploadList={false}
            >
              <Button icon={<UploadOutlined />}>{t('sites.static.uploadFile')}</Button>
            </Upload>
            {selectedRowKeys.length > 0 && (
                <Button danger icon={<DeleteOutlined />} onClick={handleBatchDelete}>{t('common.batchDelete')}</Button>
            )}
            <Button danger onClick={handleDeleteAll} disabled={!fileList || fileList.length === 0}>{t('common.deleteAll')}</Button>
          </Space>
        </div>
        
        <Table
          dataSource={fileList || []}
          rowKey="key"
          pagination={false}
          rowSelection={{
            selectedRowKeys,
            onChange: (newSelectedRowKeys) => setSelectedRowKeys(newSelectedRowKeys),
          }}
          columns={[
            {
              title: t('sites.static.fileName'),
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
              title: t('sites.static.size'),
              dataIndex: 'size',
              key: 'size',
              width: 100,
              render: (size) => size ? `${(size / 1024).toFixed(2)} KB` : '-'
            },
            {
              title: t('common.actions'),
              key: 'action',
              width: 250,
              render: (_, record) => (
                <Space>
                  {record.type === 'file' && (
                    <>
                      <Button type="link" size="small" icon={<DownloadOutlined />} onClick={() => handleDownload(record)}>{t('common.download')}</Button>
                      {record.name.endsWith('.zip') && (
                        <Button type="link" size="small" icon={<ExtractOutlined />} onClick={() => handleExtract(record)}>{t('common.extract')}</Button>
                      )}
                    </>
                  )}
                  <Button type="link" danger size="small" onClick={() => handleFileDelete(record)}>{t('common.delete')}</Button>
                </Space>
              )
            }
          ]}
        />
      </Modal>

      {/* 国家选择器弹窗 */}
      <Modal
        title={t('sites.modal.selectCountry', { target: t(countrySelectorTarget === 'allowList' ? 'sites.country.allowList' : 'sites.country.blockList') })}
        open={countrySelectorVisible}
        onOk={handleCountrySelectorOk}
        onCancel={() => setCountrySelectorVisible(false)}
        width={700}
        bodyStyle={{ maxHeight: '600px', overflowY: 'auto' }}
      >
        <div style={{ marginBottom: 16 }}>
          <Input 
            prefix={<SearchOutlined />} 
            placeholder={t('sites.country.searchPlaceholder')} 
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
              {t('sites.country.selectAllWithCount', { count: selectedCountries.length })}
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
          <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description={t('sites.country.noMatch')} />
        )}
      </Modal>

      </div>
    </>
  )
}

export default Sites