import axios, { AxiosInstance, AxiosError, InternalAxiosRequestConfig } from 'axios'

// 定义 API 响应类型
export interface ApiResponse<T = any> {
  code: number
  message: string
  data: T
}

// 创建 axios 实例
// 在 Playwright 测试环境中，使用 /api 前缀以便 Vite 代理转发
const api: AxiosInstance = axios.create({
  baseURL: '/api/v1',
  timeout: 10000,
})

// ─── 请求去重：相同 method+url+params 的进行中请求共享同一 Promise ───
const pendingRequests = new Map<string, Promise<ApiResponse>>()

function buildRequestKey(config: InternalAxiosRequestConfig): string {
  return `${config.method?.toUpperCase()}:${config.url}:${JSON.stringify(config.params ?? {})}:${JSON.stringify(config.data ?? {})}`
}

// ─── 自动重试：仅对幂等 GET 且网络/5xx 错误重试 ───
const RETRY_COUNT = 2
const RETRY_DELAY = 800

function shouldRetry(error: AxiosError): boolean {
  const config = error.config as (InternalAxiosRequestConfig & { _retryCount?: number }) | undefined
  if (!config || config.method?.toLowerCase() !== 'get') return false
  if ((config._retryCount ?? 0) >= RETRY_COUNT) return false
  // 网络错误（无响应）或 5xx 服务端错误才重试
  return !error.response || error.response.status >= 500
}

// ─── 错误消息提取：统一从各层错误结构中取出可读文案 ───
export function extractErrorMessage(error: unknown): string {
  const axiosError = error as AxiosError<{ message?: string; error?: string }>
  if (axiosError?.response) {
    const data = axiosError.response.data
    return data?.message || data?.error || `请求失败 (${axiosError.response.status})`
  }
  if (axiosError?.code === 'ECONNABORTED') return '请求超时，请稍后重试'
  if (axiosError?.request) return '网络连接失败，请检查网络'
  return error instanceof Error ? error.message : '未知错误'
}

// 请求拦截器
api.interceptors.request.use(
  (config) => {
    // 获取当前请求 URL
    const url = config.url || ''

    // 从 localStorage 获取 token
    const token = localStorage.getItem('token')

    // 检查是否是登录相关 API（不需要携带 token）
    // 登录相关 API 包括：/auth/开头的所有 API，/auth/first-run, /auth/login
    // 注意：url 可能是相对路径，比如"/sites"而不是"/api/v1/sites"
    const isAuthApi =
      url.startsWith('/auth/') ||
      url === '/auth/first-run' ||
      url === '/auth/login' ||
      // 处理相对路径情况
      url === '/first-run' ||
      url === '/login'

    // 非登录 API 需要携带 token
    if (token) {
      if (!isAuthApi) {
        if (config.headers.set) {
          config.headers.set('Authorization', `Bearer ${token}`)
        } else {
          config.headers.Authorization = `Bearer ${token}`
        }
      }
    }

    return config
  },
  (error) => {
    return Promise.reject(error)
  }
)

// 响应拦截器
api.interceptors.response.use(
  (response) => {
    // 请求完成，从去重表中移除
    if (response.config) {
      pendingRequests.delete(buildRequestKey(response.config))
    }
    return response.data
  },
  async (error: AxiosError) => {
    const config = error.config as (InternalAxiosRequestConfig & { _retryCount?: number }) | undefined

    // 从去重表中移除失败的请求
    if (config) {
      pendingRequests.delete(buildRequestKey(config))
    }

    // 自动重试：幂等 GET + 网络/5xx 错误
    if (config && shouldRetry(error)) {
      config._retryCount = (config._retryCount ?? 0) + 1
      await new Promise((resolve) => setTimeout(resolve, RETRY_DELAY * config._retryCount!))
      return api.request(config)
    }

    // 处理 401 未授权错误
    if (error.response && error.response.status === 401) {
      // 清除本地存储的 token
      localStorage.removeItem('token')
      localStorage.removeItem('username')

      // 检查当前是否已经在登录页面，如果不在则跳转到登录页面
      if (!window.location.pathname.includes('/login')) {
        window.location.href = '/login'
      }
    }

    // 处理 402 站点授权超限：后端已返回友好文案与价格信息，
    // 此处统一弹出升级引导（避免各调用点只显示原始 status 文案）
    if (error.response && error.response.status === 402) {
      const data = error.response.data as { message?: string; data?: { message?: string } } | undefined
      const upgradeMsg =
        data?.data?.message ||
        data?.message ||
        '免费版支持 1 个站点，添加更多站点需要购买站点授权（$99/站点/年）'

      // 动态导入 antd，避免 api 模块与 UI 库的循环依赖
      import('antd').then(({ Modal }) => {
        Modal.confirm({
          title: '站点授权升级',
          content: `${upgradeMsg}。如需购买，请访问定价页或通过"联系"渠道开通。`,
          okText: '查看定价',
          cancelText: '稍后再说',
          onOk: () => window.open('https://prerender.websitetool.cn/pricing', '_blank'),
        })
      }).catch(() => {
        console.warn('站点授权超限: ' + upgradeMsg)
      })
    }

    return Promise.reject(error)
  }
)

// ─── 幂等 GET 去重：相同 url+params 的进行中请求共享同一 Promise ───
// 直接重写实例的 get 方法，使所有调用点（api.get / 各 Api 对象）自动获得去重能力
const originalGet = api.get.bind(api)
api.get = function <T = any>(url: string, config?: any): Promise<ApiResponse<T>> {
  const key = `GET:${url}:${JSON.stringify(config?.params ?? {})}:${JSON.stringify({})}`
  const pending = pendingRequests.get(key)
  if (pending) {
    return pending as Promise<ApiResponse<T>>
  }
  const promise = originalGet<T>(url, config)
  pendingRequests.set(key, promise as Promise<ApiResponse>)
  // 完成后移除；catch 防止链上 rejection 触发 unhandledrejection
  promise.finally(() => { pendingRequests.delete(key) }).catch(() => {})
  return promise
} as typeof api.get

// 重新定义 axios 方法的类型
declare module 'axios' {
  interface AxiosInstance {
    get<T = any>(url: string, config?: any): Promise<ApiResponse<T>>
    post<T = any>(url: string, data?: any, config?: any): Promise<ApiResponse<T>>
    put<T = any>(url: string, data?: any, config?: any): Promise<ApiResponse<T>>
    delete<T = any>(url: string, config?: any): Promise<ApiResponse<T>>
  }
}

// 登录 API
export const authApi = {
  firstRun: () => api.get('/auth/first-run'),
  login: (username: string, password: string) => api.post('/auth/login', { username, password }),
  logout: () => api.post('/auth/logout'),
  changePassword: (oldPassword: string, newPassword: string) => api.post('/auth/change-password', { old_password: oldPassword, new_password: newPassword }),
}

// 概览 API
export const overviewApi = {
  getStats: () => api.get('/overview'),
}

// 防火墙 API
export const firewallApi = {
  getWafConfig: (siteId: string) => api.get(`/sites/${siteId}/waf`),
  updateWafConfig: (siteId: string, config: any) => api.put(`/sites/${siteId}/waf`, config),
  getAccessLogs: (params: { site_id?: string; page?: number; limit?: number }) => api.get('/logs', { params }),
  exportLogs: (siteId?: string) => `/api/v1/logs/export${siteId ? `?site_id=${siteId}` : ''}`,
  getAttackLogs: (params: { site_id: string; page: number; limit: number }) => api.get('/firewall/attacks', { params }),
  addToWhitelist: (siteId: string, ip: string) => api.post(`/firewall/whitelist`, { site_id: siteId, ip }),
  addToBlacklist: (siteId: string, ip: string) => api.post(`/firewall/blacklist`, { site_id: siteId, ip }),
  getStatus: (siteId: string) => api.get(`/sites/${siteId}/waf`),
  getRules: (siteId: string) => api.get(`/sites/${siteId}/waf`), // 规则包含在配置中
  getFirewallRules: (siteId: string) => api.get(`/firewall/rules`, { params: { site_id: siteId } }),
  saveFirewallRules: (siteId: string, rules: any[]) => api.post(`/firewall/rules`, { site_id: siteId, rules }),
  deleteFirewallRule: (siteId: string, ruleId: string) => api.delete(`/firewall/rules/${ruleId}`, { params: { site_id: siteId } }),
}

// 渲染预热 API
export const prerenderApi = {
  getStatus: (siteId?: string) => api.get('/preheat/stats', { params: siteId ? { siteId } : {} }),
  triggerPreheat: (siteId: string) => api.post('/preheat/trigger', { siteId }),
  getUrls: (siteId?: string, page: number = 1, pageSize: number = 20) => api.get('/preheat/urls', { params: { siteId, page, pageSize } }),
  getCrawlerHeaders: () => api.get('/preheat/crawler-headers'),
  clearCache: (siteId: string) => api.post('/preheat/clear-cache', { siteId }),
  getPreheatStats: (siteId?: string) => api.get('/preheat/stats', { params: siteId ? { siteId } : {} }),
  // 缓存条目管理：单 URL 失效 / 强制重渲 / 列表 / 单条删除
  invalidateCache: (siteId: string, url: string) => api.post('/preheat/invalidate', { siteId, url }),
  recacheUrl: (siteId: string, url: string) => api.post('/preheat/recache', { siteId, url }),
  listCacheEntries: (siteId: string, limit: number = 200) => api.get('/preheat/entries', { params: { siteId, limit } }),
  deleteCacheEntry: (siteId: string, url: string) => api.delete('/preheat/entries', { params: { siteId, url } }),
}

// 路由 API
export const routingApi = {
  getRules: () => api.get('/system/config'),
}

// 监控 API
export const monitoringApi = {
  getStats: () => api.get('/monitoring/stats'),
  getLogs: (params?: { site_id?: string; page?: number; limit?: number }) => api.get('/logs', { params }),
  getAlertHistory: (limit?: number) => api.get('/monitoring/alerts/history', { params: { limit: limit || 50 } }),
  getAlertRules: () => api.get('/monitoring/alert-rules'),
  saveAlertRule: (rule: any) => api.post('/monitoring/alert-rules', rule),
  saveAlertRules: (rules: any[]) => api.post('/monitoring/alert-rules', { rules }),
  deleteAlertRule: (ruleId: string) => api.delete(`/monitoring/alert-rules/${ruleId}`),
  getNotificationChannels: () => api.get('/monitoring/alerts/channels'),
  saveNotificationChannels: (channels: any[]) => api.post('/monitoring/alerts/channels', { channels }),
}

// 站点管理 API
export const sitesApi = {
  getSites: () => api.get('/sites'),
  getSite: (id: string) => api.get(`/sites/${id}`),
  getSiteConfig: (id: string, type: 'prerender' | 'push' | 'waf') => api.get(`/sites/${id}/config`, { params: { type } }),
  addSite: (site: any) => api.post('/sites', site),
  updateSite: (id: string, site: any) => api.put(`/sites/${id}`, site),
  updatePrerenderConfig: (id: string, config: any) => api.put(`/sites/${id}/prerender`, config),
  updatePushConfig: (id: string, config: any) => api.put(`/sites/${id}/push`, config),
  updateFirewallConfig: (id: string, config: any) => api.put(`/sites/${id}/firewall`, config),
  deleteSite: (id: string) => api.delete(`/sites/${id}`),
  // 静态资源管理 API
  getFileList: (siteId: string, path: string) => api.get(`/sites/${siteId}/static`, { params: { path } }),
  uploadFile: (siteId: string, file: any, path: string, onUploadProgress?: (progressEvent: any) => void) => {
    const formData = new FormData()
    formData.append('file', file)
    formData.append('path', path)
    return api.post(`/sites/${siteId}/static`, formData, { onUploadProgress })
  },
  extractFile: (siteId: string, filename: string, path: string) => {
    const formData = new FormData()
    formData.append('filename', filename)
    formData.append('path', path)
    return api.post(`/sites/${siteId}/static/extract`, formData)
  },
  deleteStaticResources: (siteId: string, path: string) => api.delete(`/sites/${siteId}/static`, { params: { path } }),
  batchDeleteStaticResources: (siteId: string, paths: string[]) => api.post(`/sites/${siteId}/static/batch-delete`, { paths }),
}

// 爬虫日志 API
export const crawlerApi = {
  getLogs: (params: { site?: string; startTime: string; endTime: string; page: number; pageSize: number }) => api.get('/crawler/logs', { params }),
  getStats: (params: { site?: string; startTime: string; endTime: string; granularity: string }) => api.get('/crawler/stats', { params }),
  getUrlStats: (params: { site?: string; startTime: string; endTime: string; limit?: number }) => api.get('/crawler/url-stats', { params }),
}

// 推送 API
export const pushApi = {
  getStats: (siteId?: string) => api.get('/push/stats', { params: siteId ? { siteId } : {} }),
  getLogs: (siteId?: string, page: number = 1, pageSize: number = 20) => api.get('/push/logs', { params: { siteId, page, pageSize } }),
  getConfig: (siteId: string) => api.get('/push/config', { params: { siteId } }),
  updateConfig: (siteId: string, config: any) => api.post('/push/config', { siteId, config }),
  getSites: () => api.get('/push/sites'),
  getTrend: (siteId?: string) => api.get('/push/trend', { params: siteId ? { siteId } : {} }),
}

// 系统 API
export const systemApi = {
  health: () => api.get('/health'),
  version: () => api.get('/version'),
  getConfig: () => api.get('/system/config'),
  updateConfig: (config: any) => api.post('/system/config', config),
  backup: () => api.post('/system/backup'),
  restore: (backupKey: string) => api.post('/system/restore', { backup_key: backupKey }),
  listBackups: () => api.get('/system/backups'),
}

// SSL 证书 API
export const sslApi = {
  listCertificates: () => api.get('/ssl/certificates'),
  getCertificate: (domain: string) => api.get(`/ssl/certificates/${domain}`),
  requestCertificate: (domains: string[]) => api.post('/ssl/certificates', { domains }),
  renewCertificate: (domain: string) => api.post(`/ssl/certificates/${domain}/renew`),
  deleteCertificate: (domain: string) => api.delete(`/ssl/certificates/${domain}`),
  getExpiringCertificates: (days: number = 30) => api.get('/ssl/certificates/expiring', { params: { days } }),
  requestWildcardCertificate: (baseDomain: string, subdomains: string[]) => api.post('/ssl/certificates/wildcard', { base_domain: baseDomain, subdomains }),
  getRenewalHistory: (domain: string) => api.get(`/ssl/certificates/${domain}/renewal-history`),
}

export default api
