import axios, { AxiosInstance } from 'axios'

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
      // 为所有非登录 API 添加 Authorization 头
      if (!isAuthApi) {
        // 使用 axios 的 headers.set 方法确保 Authorization 头被正确设置
        if (config.headers.set) {
          config.headers.set('Authorization', `Bearer ${token}`)
        } else {
          // 兼容不同的 headers 对象类型
          config.headers.Authorization = `Bearer ${token}`
        }
      }
    }

    // 开发环境调试日志
    if (import.meta.env.DEV) {
      console.log('=== API Request Debug ===')
      console.log('Request URL:', url)
      console.log('Token found:', !!token)
      console.log('Is Auth API:', isAuthApi)
    }

    return config
  },
  (error) => {
    if (import.meta.env.DEV) {
      console.error('=== API Request Error ===')
      console.error('Error:', error)
    }
    return Promise.reject(error)
  }
)

// 响应拦截器
api.interceptors.response.use(
  (response) => {
    // 开发环境调试日志
    if (import.meta.env.DEV) {
      console.log('=== API Response Debug ===')
      console.log('Response URL:', response.config.url)
      console.log('Response Status:', response.status)
    }
    return response.data
  },
  (error) => {
    // 开发环境调试日志
    if (import.meta.env.DEV) {
      console.error('=== API Response Error Debug ===')
      console.error('Error URL:', error.config?.url)
      console.error('Error Status:', error.response?.status)
      console.error('Error Message:', error.message)
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

    return Promise.reject(error)
  }
)

// 重新定义 axios 方法的类型
declare module 'axios' {
  interface AxiosInstance {
    get<T = any>(url: string, config?: any): Promise<ApiResponse<T>>
    post<T = any>(url: string, data?: any, config?: any): Promise<ApiResponse<T>>
    put<T = any>(url: string, data?: any, config?: any): Promise<ApiResponse<T>>
    delete<T = any>(url: string, config?: any): Promise<ApiResponse<T>>
  }
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
  // 为了兼容 Firewall.tsx 增加的方法
  getAttackLogs: (params: { site_id: string; page: number; limit: number }) => api.get('/firewall/attacks', { params }),
  addToWhitelist: (siteId: string, ip: string) => api.post(`/firewall/whitelist`, { site_id: siteId, ip }),
  addToBlacklist: (siteId: string, ip: string) => api.post(`/firewall/blacklist`, { site_id: siteId, ip }),
  getStatus: (siteId: string) => api.get(`/sites/${siteId}/waf`),
  getRules: (siteId: string) => api.get(`/sites/${siteId}/waf`), // 规则包含在配置中
  scan: (data: { site: string; url: string }) => api.post(`/sites/${data.site}/scan`, data),
}

// 渲染预热 API
export const prerenderApi = {
  getStatus: (siteId?: string) => api.get('/preheat/stats', { params: siteId ? { siteId } : {} }),
  triggerPreheat: (siteId: string) => api.post('/preheat/trigger', { siteId }),
  getUrls: (siteId?: string, page: number = 1, pageSize: number = 20) => api.get('/preheat/urls', { params: { siteId, page, pageSize } }),
  getCrawlerHeaders: () => api.get('/preheat/crawler-headers'),
  clearCache: (siteId: string) => api.post('/preheat/clear-cache', { siteId }),
  getPreheatStats: (siteId?: string) => api.get('/preheat/stats', { params: siteId ? { siteId } : {} }),
}

// 路由 API
export const routingApi = {
  getRules: () => api.get('/system/config'),
}

// 监控 API
export const monitoringApi = {
  getStats: () => api.get('/monitoring/stats'),
  getLogs: (params?: { site_id?: string; page?: number; limit?: number }) => api.get('/logs', { params }),
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
}

// 推送 API
export const pushApi = {
  getStats: (siteId?: string) => api.get('/push/stats', { params: siteId ? { siteId } : {} }),
  getLogs: (siteId?: string, page: number = 1, pageSize: number = 20) => api.get('/push/logs', { params: { siteId, page, pageSize } }),
  getConfig: (siteId: string) => api.get('/push/config', { params: { siteId } }),
  updateConfig: (siteId: string, config: any) => api.post('/push/config', { siteId, config }),
  getSites: () => api.get('/push/sites'),
}

// 系统 API
export const systemApi = {
  health: () => api.get('/health'),
  version: () => api.get('/version'),
  getConfig: () => api.get('/system/config'),
  updateConfig: (config: any) => api.post('/system/config', config),
}

export default api
