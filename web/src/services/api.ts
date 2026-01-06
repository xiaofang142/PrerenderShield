import axios, { AxiosInstance } from 'axios'

// 定义API响应类型
export interface ApiResponse<T = any> {
  code: number
  message: string
  data: T
}

// 创建axios实例
const api: AxiosInstance = axios.create({
  baseURL: '/api/v1',
  timeout: 10000,
})

// 请求拦截器
api.interceptors.request.use(
  (config) => {
    // 获取当前请求URL
    const url = config.url || ''
    
    // 从localStorage获取token
    const token = localStorage.getItem('token')
    
    // 检查是否是登录相关API（不需要携带token）
    // 登录相关API包括：/auth/开头的所有API，/auth/first-run, /auth/login
    // 注意：url可能是相对路径，比如"/sites"而不是"/api/v1/sites"
    const isAuthApi = 
      url.startsWith('/auth/') || 
      url === '/auth/first-run' || 
      url === '/auth/login' ||
      // 处理相对路径情况
      url === '/first-run' ||
      url === '/login'
    
    // 非登录API需要携带token
    if (token) {
      // 为所有非登录API添加Authorization头
      if (!isAuthApi) {
        // 使用axios的headers.set方法确保Authorization头被正确设置
        if (config.headers.set) {
          config.headers.set('Authorization', `Bearer ${token}`)
        } else {
          // 兼容不同的headers对象类型
          config.headers.Authorization = `Bearer ${token}`
        }
      }
    }
    
    console.log('=== API Request Debug ===');
    console.log('Request URL:', url);
    console.log('Full URL with baseURL:', config.baseURL + url);
    console.log('Token found in localStorage:', !!token);
    console.log('Token value:', token ? '***' + token.slice(-8) : 'null'); // 只显示token的最后8位
    console.log('Is Auth API:', isAuthApi);
    console.log('Authorization Header:', config.headers.Authorization || config.headers.get?.('Authorization'));
    
    return config
  },
  (error) => {
    console.error('=== API Request Error ===');
    console.error('Error:', error);
    return Promise.reject(error)
  }
)

// 响应拦截器
api.interceptors.response.use(
  (response) => {
    console.log('=== API Response Debug ===');
    console.log('Response URL:', response.config.url);
    console.log('Response Status:', response.status);
    console.log('Response Data:', response.data);
    return response.data
  },
  (error) => {
    console.error('=== API Response Error Debug ===');
    console.error('Error URL:', error.config?.url);
    console.error('Error Status:', error.response?.status);
    console.error('Error Headers:', error.response?.headers);
    console.error('Error Data:', error.response?.data);
    console.error('Error Message:', error.message);
    console.error('Request Config:', error.config);
    
    // 处理401未授权错误
    if (error.response && error.response.status === 401) {
      console.error('=== 401 Unauthorized Error ===');
      console.error('Token in localStorage:', localStorage.getItem('token') ? '***' + localStorage.getItem('token')?.slice(-8) : 'null');
      console.error('Request had Authorization header:', error.config?.headers?.Authorization ? 'Yes' : 'No');
      
      // 清除本地存储的token
      localStorage.removeItem('token')
      localStorage.removeItem('username')
      
      // 检查当前是否已经在登录页面，如果不在则跳转到登录页面
      // 避免登录失败时页面刷新
      if (!window.location.pathname.includes('/login')) {
        console.log('Redirecting to login page...');
        window.location.href = '/login'
      }
    }
    
    return Promise.reject(error)
  }
)

// 重新定义axios方法的类型
declare module 'axios' {
  interface AxiosInstance {
    get<T = any>(url: string, config?: any): Promise<ApiResponse<T>>
    post<T = any>(url: string, data?: any, config?: any): Promise<ApiResponse<T>>
    put<T = any>(url: string, data?: any, config?: any): Promise<ApiResponse<T>>
    delete<T = any>(url: string, config?: any): Promise<ApiResponse<T>>
  }
}

// 概览API
export const overviewApi = {
  getStats: () => api.get('/overview'),
}

// 防火墙API
export const firewallApi = {
  getStatus: (site?: string) => api.get('/firewall/status', { params: site ? { site } : {} }),
  getRules: (site?: string) => api.get('/firewall/rules', { params: site ? { site } : {} }),
  scan: (data: { site?: string; url: string }) => api.post('/firewall/scan', data),
}

// 渲染预热API
export const prerenderApi = {
  getStatus: (site?: string) => api.get('/prerender/status', { params: site ? { site } : {} }),
  render: (data: { site: string; url: string }) => api.post('/prerender/render', data),
  preheat: (data: { site: string }) => api.post('/prerender/preheat', data),
  updateConfig: (site: string, config: any) => api.put('/prerender/config', { site, config }),
  // 渲染预热扩展API
  getPreheatStats: (siteId?: string) => api.get('/preheat/stats', { params: siteId ? { siteId } : {} }),
  triggerPreheat: (siteId: string) => api.post('/preheat/trigger', { siteId }),
  getUrls: (siteId?: string, page: number = 1, pageSize: number = 20) => api.get('/preheat/urls', { params: { siteId, page, pageSize } }),
  getCrawlerHeaders: () => api.get('/preheat/crawler-headers'),
  clearCache: (siteId: string) => api.post('/preheat/clear-cache', { siteId }),
}

// 路由API
export const routingApi = {
  getRules: () => api.get('/routing/rules'),
  addRule: (rule: any) => api.post('/routing/rules', rule),
  deleteRule: (id: string) => api.delete(`/routing/rules/${id}`),
}

// 监控API
export const monitoringApi = {
  getStats: () => api.get('/monitoring/stats'),
  getLogs: () => api.get('/monitoring/logs'),
}

// 站点管理API
export const sitesApi = {
  getSites: () => api.get('/sites'),
  getSite: (id: string) => api.get(`/sites/${id}`),
  getSiteConfig: (id: string, type: 'prerender' | 'push' | 'waf') => api.get(`/sites/${id}/config`, { params: { type } }),
  addSite: (site: any) => api.post('/sites', site),
  updateSite: (id: string, site: any) => api.put(`/sites/${id}`, site),
  deleteSite: (id: string) => api.delete(`/sites/${id}`),
  // 静态资源管理API
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
}

// 爬虫日志API
export const crawlerApi = {
  getLogs: (params: { site?: string; startTime: string; endTime: string; page: number; pageSize: number }) => api.get('/crawler/logs', { params }),
  getStats: (params: { site?: string; startTime: string; endTime: string; granularity: string }) => api.get('/crawler/stats', { params }),
}

// 推送API
export const pushApi = {
  getStats: (siteId?: string) => api.get('/push/stats', { params: siteId ? { siteId } : {} }),
  getLogs: (siteId?: string, page: number = 1, pageSize: number = 20) => api.get('/push/logs', { params: { siteId, page, pageSize } }),
  triggerPush: (siteId: string) => api.post('/push/trigger', { siteId }),
  getConfig: (siteId: string) => api.get('/push/config', { params: { siteId } }),
  updateConfig: (siteId: string, config: any) => api.post('/push/config', { siteId, config }),
  getSites: () => api.get('/push/sites'),
}

// 系统API
export const systemApi = {
  health: () => api.get('/health'),
  version: () => api.get('/version'),
  getConfig: () => api.get('/system/config'),
  updateConfig: (config: any) => api.post('/system/config', config),
}

export default api