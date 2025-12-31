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
    console.log('Request URL:', config.url);
    console.log('Full Request Config:', config);
    // 可以在这里添加认证信息
    return config
  },
  (error) => {
    return Promise.reject(error)
  }
)

// 响应拦截器
api.interceptors.response.use(
  (response) => {
    console.log('Full Response:', response);
    console.log('Response Status:', response.status);
    console.log('Response Data:', response.data);
    return response.data
  },
  (error) => {
    console.error('API Error:', error);
    console.error('Error Config:', error.config);
    console.error('Error Response:', error.response);
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
  getSite: (name: string) => api.get(`/sites/${name}`),
  addSite: (site: any) => api.post('/sites', site),
  updateSite: (name: string, site: any) => api.put(`/sites/${name}`, site),
  deleteSite: (name: string) => api.delete(`/sites/${name}`),
  // 静态资源管理API
  getFileList: (siteName: string, path: string) => api.get(`/sites/${siteName}/static`, { params: { path } }),
  uploadFile: (siteName: string, file: any, path: string, onUploadProgress?: (progressEvent: any) => void) => {
    const formData = new FormData()
    formData.append('file', file)
    formData.append('path', path)
    return api.post(`/sites/${siteName}/static`, formData, { onUploadProgress })
  },
  extractFile: (siteName: string, filename: string, path: string) => {
    const formData = new FormData()
    formData.append('filename', filename)
    formData.append('path', path)
    return api.post(`/sites/${siteName}/static/extract`, formData)
  },
}

// 爬虫日志API
export const crawlerApi = {
  getLogs: (params: { site?: string; startTime: string; endTime: string; page: number; pageSize: number }) => api.get('/crawler/logs', { params }),
  getStats: (params: { site?: string; startTime: string; endTime: string; granularity: string }) => api.get('/crawler/stats', { params }),
}

// 系统API
export const systemApi = {
  health: () => api.get('/health'),
  version: () => api.get('/version'),
}

export default api
