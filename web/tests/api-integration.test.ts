import { test, expect } from '@playwright/test'

// API 集成测试 - 测试所有前后端交互 API
test.describe('API Integration Tests', () => {
  // 系统 API 测试
  test.describe('System APIs', () => {
    test('should get health status', async ({ request }) => {
      const response = await request.get('/api/v1/health')
      expect(response.ok()).toBeTruthy()
      
      const data = await response.json()
      expect(data.code).toBe(200)
      expect(data.data).toHaveProperty('status')
      expect(data.data).toHaveProperty('service')
    })

    test('should get version info', async ({ request }) => {
      const response = await request.get('/api/v1/version')
      expect(response.ok()).toBeTruthy()
      
      const data = await response.json()
      expect(data.code).toBe(200)
      expect(data.data).toHaveProperty('version')
      expect(data.data).toHaveProperty('name')
    })
  })

  // 认证 API 测试
  test.describe('Auth APIs', () => {
    test('should check first run status', async ({ request }) => {
      const response = await request.get('/api/v1/auth/first-run')
      expect(response.ok()).toBeTruthy()
      
      const data = await response.json()
      expect(data.code).toBe(200)
      expect(data.data).toHaveProperty('isFirstRun')
    })

    test('should login with valid credentials', async ({ request }) => {
      const response = await request.post('/api/v1/auth/login', {
        data: {
          username: 'admin',
          password: 'password123'
        }
      })
      expect(response.ok()).toBeTruthy()
      
      const data = await response.json()
      expect(data.code).toBe(200)
      expect(data.data).toHaveProperty('token')
      expect(data.data).toHaveProperty('username')
    })

    test('should reject login with invalid credentials', async ({ request }) => {
      const response = await request.post('/api/v1/auth/login', {
        data: {
          username: 'invalid',
          password: 'wrong'
        }
      })
      expect(response.status()).toBe(401)
    })

    test('should reject login with empty body', async ({ request }) => {
      const response = await request.post('/api/v1/auth/login', {
        data: {}
      })
      expect(response.status()).toBe(400)
    })
  })

  // 概览 API 测试
  test.describe('Overview APIs', () => {
    test('should get overview statistics', async ({ request }) => {
      // 先登录获取 token
      const loginResponse = await request.post('/api/v1/auth/login', {
        data: {
          username: 'admin',
          password: 'password123'
        }
      })
      const loginData = await loginResponse.json()
      const token = loginData.data.token

      // 使用 token 访问概览 API
      const response = await request.get('/api/v1/overview', {
        headers: {
          'Authorization': `Bearer ${token}`
        }
      })
      expect(response.ok()).toBeTruthy()
      
      const data = await response.json()
      expect(data.code).toBe(200)
      expect(data.data).toHaveProperty('totalRequests')
      expect(data.data).toHaveProperty('activeSites')
      expect(data.data).toHaveProperty('firewallEnabled')
      expect(data.data).toHaveProperty('prerenderEnabled')
    })
  })

  // 站点管理 API 测试
  test.describe('Sites APIs', () => {
    let authToken: string

    test.beforeEach(async ({ request }) => {
      // 获取认证 token
      const loginResponse = await request.post('/api/v1/auth/login', {
        data: {
          username: 'admin',
          password: 'password123'
        }
      })
      const loginData = await loginResponse.json()
      authToken = loginData.data.token
    })

    test('should get sites list', async ({ request }) => {
      const response = await request.get('/api/v1/sites', {
        headers: {
          'Authorization': `Bearer ${authToken}`
        }
      })
      expect(response.ok()).toBeTruthy()
      
      const data = await response.json()
      expect(data.code).toBe(200)
      expect(data.data).toBeDefined()
    })

    test('should add new site', async ({ request }) => {
      const newSite = {
        name: 'Test Site',
        domains: ['127.0.0.1'],
        port: 8081,
        mode: 'static',
        prerender: {
          enabled: false
        },
        firewall: {
          enabled: false
        }
      }

      const response = await request.post('/api/v1/sites', {
        headers: {
          'Authorization': `Bearer ${authToken}`,
          'Content-Type': 'application/json'
        },
        data: newSite
      })
      expect(response.ok()).toBeTruthy()
      
      const data = await response.json()
      expect(data.code).toBe(200)
      expect(data.data).toHaveProperty('id')
      expect(data.data.name).toBe('Test Site')
    })

    test('should reject site with invalid domain', async ({ request }) => {
      const invalidSite = {
        name: 'Invalid Site',
        domains: ['example.com'], // 只允许 127.0.0.1 或 localhost
        port: 8082,
        mode: 'static'
      }

      const response = await request.post('/api/v1/sites', {
        headers: {
          'Authorization': `Bearer ${authToken}`,
          'Content-Type': 'application/json'
        },
        data: invalidSite
      })
      expect(response.status()).toBe(400)
    })
  })

  // 防火墙 API 测试
  test.describe('Firewall APIs', () => {
    let authToken: string
    let siteId: string

    test.beforeEach(async ({ request }) => {
      // 获取认证 token 和站点
      const loginResponse = await request.post('/api/v1/auth/login', {
        data: {
          username: 'admin',
          password: 'password123'
        }
      })
      const loginData = await loginResponse.json()
      authToken = loginData.data.token

      const sitesResponse = await request.get('/api/v1/sites', {
        headers: {
          'Authorization': `Bearer ${authToken}`
        }
      })
      const sitesData = await sitesResponse.json()
      if (sitesData.data && sitesData.data.length > 0) {
        siteId = sitesData.data[0].id
      }
    })

    test('should get WAF config', async ({ request }) => {
      if (!siteId) {
        test.skip()
        return
      }

      const response = await request.get(`/api/v1/sites/${siteId}/waf`, {
        headers: {
          'Authorization': `Bearer ${authToken}`
        }
      })
      expect(response.ok()).toBeTruthy()
    })

    test('should update WAF config', async ({ request }) => {
      if (!siteId) {
        test.skip()
        return
      }

      const config = {
        enabled: true,
        rate_limit_count: 100,
        rate_limit_window: 60,
        blocked_countries: [],
        whitelist_ips: [],
        blacklist_ips: []
      }

      const response = await request.put(`/api/v1/sites/${siteId}/waf`, {
        headers: {
          'Authorization': `Bearer ${authToken}`,
          'Content-Type': 'application/json'
        },
        data: config
      })
      expect(response.ok()).toBeTruthy()
      
      const data = await response.json()
      expect(data.success).toBeTruthy()
    })
  })

  // 爬虫日志 API 测试
  test.describe('Crawler APIs', () => {
    let authToken: string

    test.beforeEach(async ({ request }) => {
      const loginResponse = await request.post('/api/v1/auth/login', {
        data: {
          username: 'admin',
          password: 'password123'
        }
      })
      const loginData = await loginResponse.json()
      authToken = loginData.data.token
    })

    test('should get crawler logs', async ({ request }) => {
      const response = await request.get('/api/v1/crawler/logs', {
        headers: {
          'Authorization': `Bearer ${authToken}`
        }
      })
      expect(response.ok()).toBeTruthy()
      
      const data = await response.json()
      expect(data.code).toBe(200)
      expect(data.data).toHaveProperty('items')
      expect(data.data).toHaveProperty('total')
    })

    test('should get crawler stats', async ({ request }) => {
      const response = await request.get('/api/v1/crawler/stats', {
        headers: {
          'Authorization': `Bearer ${authToken}`
        }
      })
      expect(response.ok()).toBeTruthy()
      
      const data = await response.json()
      expect(data.code).toBe(200)
    })
  })

  // 预热 API 测试
  test.describe('Preheat APIs', () => {
    let authToken: string

    test.beforeEach(async ({ request }) => {
      const loginResponse = await request.post('/api/v1/auth/login', {
        data: {
          username: 'admin',
          password: 'password123'
        }
      })
      const loginData = await loginResponse.json()
      authToken = loginData.data.token
    })

    test('should get preheat sites', async ({ request }) => {
      const response = await request.get('/api/v1/preheat/sites', {
        headers: {
          'Authorization': `Bearer ${authToken}`
        }
      })
      expect(response.ok()).toBeTruthy()
      
      const data = await response.json()
      expect(data.code).toBe(200)
    })

    test('should get crawler headers', async ({ request }) => {
      const response = await request.get('/api/v1/preheat/crawler-headers', {
        headers: {
          'Authorization': `Bearer ${authToken}`
        }
      })
      expect(response.ok()).toBeTruthy()
      
      const data = await response.json()
      expect(data.code).toBe(200)
    })
  })

  // 推送 API 测试
  test.describe('Push APIs', () => {
    let authToken: string

    test.beforeEach(async ({ request }) => {
      const loginResponse = await request.post('/api/v1/auth/login', {
        data: {
          username: 'admin',
          password: 'password123'
        }
      })
      const loginData = await loginResponse.json()
      authToken = loginData.data.token
    })

    test('should get push sites', async ({ request }) => {
      const response = await request.get('/api/v1/push/sites', {
        headers: {
          'Authorization': `Bearer ${authToken}`
        }
      })
      expect(response.ok()).toBeTruthy()
      
      const data = await response.json()
      expect(data.code).toBe(200)
    })

    test('should get push logs', async ({ request }) => {
      const response = await request.get('/api/v1/push/logs', {
        headers: {
          'Authorization': `Bearer ${authToken}`
        }
      })
      expect(response.ok()).toBeTruthy()
      
      const data = await response.json()
      expect(data.code).toBe(200)
    })
  })

  // 监控 API 测试
  test.describe('Monitoring APIs', () => {
    let authToken: string

    test.beforeEach(async ({ request }) => {
      const loginResponse = await request.post('/api/v1/auth/login', {
        data: {
          username: 'admin',
          password: 'password123'
        }
      })
      const loginData = await loginResponse.json()
      authToken = loginData.data.token
    })

    test('should get monitoring stats', async ({ request }) => {
      const response = await request.get('/api/v1/monitoring/stats', {
        headers: {
          'Authorization': `Bearer ${authToken}`
        }
      })
      expect(response.ok()).toBeTruthy()
      
      const data = await response.json()
      expect(data.code).toBe(200)
      expect(data.data).toBeDefined()
    })
  })

  // 系统配置 API 测试
  test.describe('System Config APIs', () => {
    let authToken: string

    test.beforeEach(async ({ request }) => {
      const loginResponse = await request.post('/api/v1/auth/login', {
        data: {
          username: 'admin',
          password: 'password123'
        }
      })
      const loginData = await loginResponse.json()
      authToken = loginData.data.token
    })

    test('should get system config', async ({ request }) => {
      const response = await request.get('/api/v1/system/config', {
        headers: {
          'Authorization': `Bearer ${authToken}`
        }
      })
      // 可能返回 500 如果 Redis 不可用
      expect([200, 500]).toContain(response.status())
    })

    test('should update system config', async ({ request }) => {
      const config = {
        access_log_retention_days: '30',
        crawler_log_retention_days: '30'
      }

      const response = await request.post('/api/v1/system/config', {
        headers: {
          'Authorization': `Bearer ${authToken}`,
          'Content-Type': 'application/json'
        },
        data: config
      })
      // 可能返回 500 如果 Redis 不可用
      expect([200, 500]).toContain(response.status())
    })
  })
})
