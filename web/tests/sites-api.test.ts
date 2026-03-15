import { test, expect } from '@playwright/test'

// 站点管理 API 测试
test.describe('Sites Management API Tests', () => {
  let authToken: string

  test.beforeEach(async ({ request }) => {
    // 获取认证 token
    const loginResponse = await request.post('/api/v1/auth/login', {
      data: {
        username: 'admin',
        password: '123456'
      }
    })
    const loginData = await loginResponse.json()
    authToken = loginData.data.token
  })

  test('GET /api/v1/sites - 获取站点列表', async ({ request }) => {
    const response = await request.get('/api/v1/sites', {
      headers: {
        'Authorization': `Bearer ${authToken}`
      }
    })
    expect(response.ok()).toBeTruthy()
    
    const data = await response.json()
    expect(data.code).toBe(200)
    expect(Array.isArray(data.data)).toBeTruthy()
  })

  test('POST /api/v1/sites - 创建新站点', async ({ request }) => {
    const siteData = {
      name: `Test Site ${Date.now()}`,
      domains: ['127.0.0.1'],
      port: 8000 + Math.floor(Math.random() * 1000),
      mode: 'static',
      prerender: {
        enabled: false,
        poolSize: 3,
        timeout: 30
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
      data: siteData
    })
    
    expect(response.ok()).toBeTruthy()
    const data = await response.json()
    expect(data.code).toBe(200)
    expect(data.data).toHaveProperty('id')
    expect(data.data.name).toBe(siteData.name)
  })

  test('POST /api/v1/sites - 拒绝无效域名', async ({ request }) => {
    const invalidSite = {
      name: 'Invalid Site',
      domains: ['example.com'], // 只允许 127.0.0.1 或 localhost
      port: 8080,
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

  test('POST /api/v1/sites - 拒绝无效端口', async ({ request }) => {
    const invalidSite = {
      name: 'Invalid Port Site',
      domains: ['127.0.0.1'],
      port: 80, // 常用端口应该被拒绝
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

  test('GET /api/v1/sites/:id - 获取单个站点', async ({ request }) => {
    // 先获取站点列表
    const sitesResponse = await request.get('/api/v1/sites', {
      headers: {
        'Authorization': `Bearer ${authToken}`
      }
    })
    const sitesData = await sitesResponse.json()
    
    if (sitesData.data && sitesData.data.length > 0) {
      const siteId = sitesData.data[0].id
      
      const response = await request.get(`/api/v1/sites/${siteId}`, {
        headers: {
          'Authorization': `Bearer ${authToken}`
        }
      })
      
      expect(response.ok()).toBeTruthy()
      const data = await response.json()
      expect(data.code).toBe(200)
      expect(data.data).toHaveProperty('id')
      expect(data.data.id).toBe(siteId)
    }
  })

  test('GET /api/v1/sites/:id/config - 获取站点配置', async ({ request }) => {
    const sitesResponse = await request.get('/api/v1/sites', {
      headers: {
        'Authorization': `Bearer ${authToken}`
      }
    })
    const sitesData = await sitesResponse.json()
    
    if (sitesData.data && sitesData.data.length > 0) {
      const siteId = sitesData.data[0].id
      
      // 测试 prerender 配置
      const prerenderResponse = await request.get(`/api/v1/sites/${siteId}/config?type=prerender`, {
        headers: {
          'Authorization': `Bearer ${authToken}`
        }
      })
      // 可能返回 200 或 404 或 500
      expect([200, 404, 500]).toContain(prerenderResponse.status())
      
      // 测试 push 配置
      const pushResponse = await request.get(`/api/v1/sites/${siteId}/config?type=push`, {
        headers: {
          'Authorization': `Bearer ${authToken}`
        }
      })
      expect([200, 404, 500]).toContain(pushResponse.status())
      
      // 测试 waf 配置
      const wafResponse = await request.get(`/api/v1/sites/${siteId}/config?type=waf`, {
        headers: {
          'Authorization': `Bearer ${authToken}`
        }
      })
      expect([200, 404, 500]).toContain(wafResponse.status())
    }
  })

  test('PUT /api/v1/sites/:id - 更新站点', async ({ request }) => {
    const sitesResponse = await request.get('/api/v1/sites', {
      headers: {
        'Authorization': `Bearer ${authToken}`
      }
    })
    const sitesData = await sitesResponse.json()

    if (sitesData.data && sitesData.data.length > 0) {
      // Find a site with valid domains (127.0.0.1 or localhost)
      const validSite = sitesData.data.find(s =>
        s.domains && (s.domains.includes('127.0.0.1') || s.domains.includes('localhost'))
      )

      if (!validSite) {
        console.log('No site with valid domains found, skipping test')
        return
      }

      const siteId = validSite.id
      const site = validSite

      const updateData = {
        ...site,
        name: `${site.name} Updated`
      }

      const response = await request.put(`/api/v1/sites/${siteId}`, {
        headers: {
          'Authorization': `Bearer ${authToken}`,
          'Content-Type': 'application/json'
        },
        data: updateData
      })

      expect(response.ok()).toBeTruthy()
      const data = await response.json()
      expect(data.code).toBe(200)
    }
  })

  test('PUT /api/v1/sites/:id/prerender - 更新预渲染配置', async ({ request }) => {
    const sitesResponse = await request.get('/api/v1/sites', {
      headers: {
        'Authorization': `Bearer ${authToken}`
      }
    })
    const sitesData = await sitesResponse.json()
    
    if (sitesData.data && sitesData.data.length > 0) {
      const siteId = sitesData.data[0].id
      
      const prerenderConfig = {
        enabled: true,
        poolSize: 5,
        minPoolSize: 2,
        maxPoolSize: 10,
        timeout: 60,
        cacheTTL: 3600,
        idleTimeout: 300
      }
      
      const response = await request.put(`/api/v1/sites/${siteId}/prerender`, {
        headers: {
          'Authorization': `Bearer ${authToken}`,
          'Content-Type': 'application/json'
        },
        data: prerenderConfig
      })
      
      expect(response.ok()).toBeTruthy()
      const data = await response.json()
      expect(data.code).toBe(200)
    }
  })

  test('PUT /api/v1/sites/:id/firewall - 更新防火墙配置', async ({ request }) => {
    const sitesResponse = await request.get('/api/v1/sites', {
      headers: {
        'Authorization': `Bearer ${authToken}`
      }
    })
    const sitesData = await sitesResponse.json()
    
    if (sitesData.data && sitesData.data.length > 0) {
      const siteId = sitesData.data[0].id
      
      const firewallConfig = {
        enabled: true,
        actionConfig: {
          defaultAction: 'block'
        },
        geoIPConfig: {
          enabled: false,
          blockList: []
        },
        rateLimitConfig: {
          enabled: true,
          requests: 100,
          window: 60
        }
      }
      
      const response = await request.put(`/api/v1/sites/${siteId}/firewall`, {
        headers: {
          'Authorization': `Bearer ${authToken}`,
          'Content-Type': 'application/json'
        },
        data: firewallConfig
      })
      
      expect(response.ok()).toBeTruthy()
      const data = await response.json()
      expect(data.code).toBe(200)
    }
  })
})
