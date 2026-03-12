import { test, expect } from '@playwright/test'

// 防火墙 API 测试
test.describe('Firewall API Tests', () => {
  let authToken: string
  let siteId: string

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

    // 获取站点列表
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

  test('GET /api/v1/sites/:id/waf - 获取 WAF 配置', async ({ request }) => {
    test.skip(!siteId, 'No site available')
    
    const response = await request.get(`/api/v1/sites/${siteId}/waf`, {
      headers: {
        'Authorization': `Bearer ${authToken}`
      }
    })
    
    expect(response.ok()).toBeTruthy()
    const data = await response.json()
    expect(data.success).toBeTruthy()
  })

  test('PUT /api/v1/sites/:id/waf - 更新 WAF 配置', async ({ request }) => {
    test.skip(!siteId, 'No site available')
    
    const wafConfig = {
      enabled: true,
      rate_limit_count: 100,
      rate_limit_window: 60,
      blocked_countries: ['CN', 'RU'],
      whitelist_ips: ['127.0.0.1'],
      blacklist_ips: ['192.168.1.100'],
      custom_block_page: '<h1>Access Denied</h1>'
    }

    const response = await request.put(`/api/v1/sites/${siteId}/waf`, {
      headers: {
        'Authorization': `Bearer ${authToken}`,
        'Content-Type': 'application/json'
      },
      data: wafConfig
    })
    
    expect(response.ok()).toBeTruthy()
    const data = await response.json()
    expect(data.success).toBeTruthy()
  })

  test('PUT /api/v1/sites/:id/waf - 缺少站点 ID', async ({ request }) => {
    const response = await request.put('/api/v1/sites//waf', {
      headers: {
        'Authorization': `Bearer ${authToken}`,
        'Content-Type': 'application/json'
      },
      data: {}
    })
    
    expect(response.status()).toBeGreaterThanOrEqual(400)
  })

  test('GET /api/v1/logs - 获取访问日志', async ({ request }) => {
    const response = await request.get('/api/v1/logs', {
      headers: {
        'Authorization': `Bearer ${authToken}`
      },
      params: {
        page: '1',
        limit: '20'
      }
    })
    
    expect(response.ok()).toBeTruthy()
    const data = await response.json()
    expect(data.success).toBeTruthy()
  })

  test('GET /api/v1/firewall/attacks - 获取攻击日志', async ({ request }) => {
    const response = await request.get('/api/v1/firewall/attacks', {
      headers: {
        'Authorization': `Bearer ${authToken}`
      },
      params: {
        site_id: siteId || 'test',
        page: '1',
        limit: '20'
      }
    })
    
    expect(response.ok()).toBeTruthy()
    const data = await response.json()
    expect(data.success).toBeTruthy()
  })

  test('POST /api/v1/firewall/whitelist - 添加 IP 到白名单', async ({ request }) => {
    const response = await request.post('/api/v1/firewall/whitelist', {
      headers: {
        'Authorization': `Bearer ${authToken}`,
        'Content-Type': 'application/json'
      },
      data: {
        site_id: siteId || 'test-site',
        ip: '192.168.1.1'
      }
    })
    
    expect(response.ok()).toBeTruthy()
    const data = await response.json()
    expect(data.success).toBeTruthy()
  })

  test('POST /api/v1/firewall/blacklist - 添加 IP 到黑名单', async ({ request }) => {
    const response = await request.post('/api/v1/firewall/blacklist', {
      headers: {
        'Authorization': `Bearer ${authToken}`,
        'Content-Type': 'application/json'
      },
      data: {
        site_id: siteId || 'test-site',
        ip: '192.168.1.100'
      }
    })
    
    expect(response.ok()).toBeTruthy()
    const data = await response.json()
    expect(data.success).toBeTruthy()
  })

  test('POST /api/v1/firewall/whitelist - 缺少必填字段', async ({ request }) => {
    const response = await request.post('/api/v1/firewall/whitelist', {
      headers: {
        'Authorization': `Bearer ${authToken}`,
        'Content-Type': 'application/json'
      },
      data: {}
    })
    
    expect(response.status()).toBe(400)
  })

  test('POST /api/v1/firewall/blacklist - 缺少必填字段', async ({ request }) => {
    const response = await request.post('/api/v1/firewall/blacklist', {
      headers: {
        'Authorization': `Bearer ${authToken}`,
        'Content-Type': 'application/json'
      },
      data: {}
    })
    
    expect(response.status()).toBe(400)
  })
})
