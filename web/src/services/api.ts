import axios from 'axios'

// 创建axios实例
const api = axios.create({
  baseURL: '/api/v1',
  timeout: 10000,
})

// 请求拦截器
api.interceptors.request.use(
  (config) => {
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
    return response.data
  },
  (error) => {
    console.error('API Error:', error)
    return Promise.reject(error)
  }
)

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

// 预渲染API
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

// SSL API
export const sslApi = {
  getStatus: (site?: string) => api.get('/ssl/status', { params: site ? { site } : {} }),
  getCerts: (site?: string) => api.get('/ssl/certs', { params: site ? { site } : {} }),
  addCert: (data: { site?: string; domain: string }) => api.post('/ssl/certs', data),
  deleteCert: (domain: string, site?: string) => {
    const params = site ? { site } : {};
    return api.delete(`/ssl/certs/${domain}`, { params });
  },
}

// 监控API
export const monitoringApi = {
  getStats: () => api.get('/monitoring/stats'),
  getLogs: () => api.get('/monitoring/logs'),
}

// 站点管理API
export const sitesApi = {
  getSites: () => api.get('/sites/'),
  getSite: (name: string) => api.get(`/sites/${name}/`),
  addSite: (site: any) => api.post('/sites/', site),
  updateSite: (name: string, site: any) => api.put(`/sites/${name}/`, site),
  deleteSite: (name: string) => api.delete(`/sites/${name}/`),
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

// 系统API
export const systemApi = {
  health: () => api.get('/health'),
  version: () => api.get('/version'),
}

export default api
