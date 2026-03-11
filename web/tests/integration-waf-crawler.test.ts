/**
 * WAF 防护和爬虫识别集成测试
 * 测试场景：模拟真实攻击和爬虫行为
 */

import { test, expect } from '@playwright/test';

test.describe('WAF 防护集成测试', () => {
  test('SQL 注入攻击拦截测试', async ({ page, request }) => {
    // 1. 登录获取 token
    const loginResponse = await request.post('/api/v1/auth/login', {
      data: {
        username: 'admin',
        password: 'admin123'
      }
    });
    expect(loginResponse.ok()).toBeTruthy();
    const { token } = await loginResponse.json();

    // 2. 配置 SQL 注入防护规则
    const ruleResponse = await request.post('/api/v1/firewall/rules', {
      data: {
        type: 'sql_injection',
        action: 'block',
        description: 'E2E SQL 注入防护测试',
        enabled: true
      },
      headers: {
        'Authorization': `Bearer ${token}`
      }
    });
    expect(ruleResponse.ok()).toBeTruthy();

    // 3. 模拟 SQL 注入攻击
    const attackResponse = await request.get('/api/v1/sites', {
      headers: {
        'Authorization': `Bearer ${token}`,
        'User-Agent': 'Mozilla/5.0',
        'X-Forwarded-For': '192.168.1.100'
      },
      params: {
        search: "' OR '1'='1"
      }
    });

    // 应该被拦截
    expect([403, 400]).toContain(attackResponse.status());

    // 4. 验证攻击日志
    const logsResponse = await request.get('/api/v1/firewall/attacks', {
      headers: {
        'Authorization': `Bearer ${token}`
      },
      params: {
        attackType: 'sql_injection'
      }
    });
    expect(logsResponse.ok()).toBeTruthy();
    const logs = await logsResponse.json();
    expect(logs.data).toHaveLength({ min: 1 });
  });

  test('XSS 攻击拦截测试', async ({ page, request }) => {
    // 1. 登录
    const loginResponse = await request.post('/api/v1/auth/login', {
      data: {
        username: 'admin',
        password: 'admin123'
      }
    });
    const { token } = await loginResponse.json();

    // 2. 配置 XSS 防护规则
    await request.post('/api/v1/firewall/rules', {
      data: {
        type: 'xss',
        action: 'block',
        description: 'E2E XSS 防护测试',
        enabled: true
      },
      headers: {
        'Authorization': `Bearer ${token}`
      }
    });

    // 3. 模拟 XSS 攻击
    const attackResponse = await request.post('/api/v1/sites', {
      data: {
        name: '<script>alert("xss")</script>',
        domains: ['example.com']
      },
      headers: {
        'Authorization': `Bearer ${token}`,
        'Content-Type': 'application/json'
      }
    });

    // 应该被拦截
    expect([403, 400]).toContain(attackResponse.status());
  });

  test('速率限制测试', async ({ page, request }) => {
    // 1. 登录
    const loginResponse = await request.post('/api/v1/auth/login', {
      data: {
        username: 'admin',
        password: 'admin123'
      }
    });
    const { token } = await loginResponse.json();

    // 2. 配置速率限制
    await request.post('/api/v1/firewall/ratelimit', {
      data: {
        enabled: true,
        requestsPerMinute: 10,
        action: 'block'
      },
      headers: {
        'Authorization': `Bearer ${token}`
      }
    });

    // 3. 快速发送多个请求
    const requests = [];
    for (let i = 0; i < 15; i++) {
      requests.push(
        request.get('/api/v1/overview', {
          headers: {
            'Authorization': `Bearer ${token}`
          }
        })
      );
    }

    const responses = await Promise.all(requests);
    const statusCodes = responses.map(r => r.status());
    
    // 应该有请求被速率限制
    expect(statusCodes).toContain(429);
  });

  test('IP 黑白名单测试', async ({ page, request }) => {
    // 1. 登录
    const loginResponse = await request.post('/api/v1/auth/login', {
      data: {
        username: 'admin',
        password: 'admin123'
      }
    });
    const { token } = await loginResponse.json();

    // 2. 添加 IP 黑名单
    const blacklistResponse = await request.post('/api/v1/firewall/blacklist', {
      data: {
        ip: '192.168.1.100',
        reason: 'E2E 测试黑名单'
      },
      headers: {
        'Authorization': `Bearer ${token}`
      }
    });
    expect(blacklistResponse.ok()).toBeTruthy();

    // 3. 添加 IP 白名单
    const whitelistResponse = await request.post('/api/v1/firewall/whitelist', {
      data: {
        ip: '10.0.0.1',
        reason: 'E2E 测试白名单'
      },
      headers: {
        'Authorization': `Bearer ${token}`
      }
    });
    expect(whitelistResponse.ok()).toBeTruthy();

    // 4. 验证黑名单
    const blacklistCheck = await request.get('/api/v1/firewall/blacklist', {
      headers: {
        'Authorization': `Bearer ${token}`
      }
    });
    expect(blacklistCheck.ok()).toBeTruthy();
    const blacklist = await blacklistCheck.json();
    expect(blacklist.data).toContainEqual(
      expect.objectContaining({ ip: '192.168.1.100' })
    );

    // 5. 验证白名单
    const whitelistCheck = await request.get('/api/v1/firewall/whitelist', {
      headers: {
        'Authorization': `Bearer ${token}`
      }
    });
    expect(whitelistCheck.ok()).toBeTruthy();
    const whitelist = await whitelistCheck.json();
    expect(whitelist.data).toContainEqual(
      expect.objectContaining({ ip: '10.0.0.1' })
    );

    // 6. 删除黑名单
    const deleteResponse = await request.delete('/api/v1/firewall/blacklist/192.168.1.100', {
      headers: {
        'Authorization': `Bearer ${token}`
      }
    });
    expect(deleteResponse.ok()).toBeTruthy();
  });
});

test.describe('爬虫识别集成测试', () => {
  test('搜索引擎爬虫识别测试', async ({ page, request }) => {
    // 1. 登录
    const loginResponse = await request.post('/api/v1/auth/login', {
      data: {
        username: 'admin',
        password: 'admin123'
      }
    });
    const { token } = await loginResponse.json();

    // 2. 模拟 Google 爬虫
    const googleBotResponse = await request.get('/api/v1/sites', {
      headers: {
        'Authorization': `Bearer ${token}`,
        'User-Agent': 'Googlebot/2.1 (+http://www.google.com/bot.html)'
      }
    });
    expect(googleBotResponse.ok()).toBeTruthy();

    // 3. 模拟 Bing 爬虫
    const bingBotResponse = await request.get('/api/v1/sites', {
      headers: {
        'Authorization': `Bearer ${token}`,
        'User-Agent': 'Mozilla/5.0 (compatible; bingbot/2.0; +http://www.bing.com/bingbot.htm)'
      }
    });
    expect(bingBotResponse.ok()).toBeTruthy();

    // 4. 验证爬虫日志
    const crawlerLogsResponse = await request.get('/api/v1/crawler/logs', {
      headers: {
        'Authorization': `Bearer ${token}`
      },
      params: {
        crawlerType: 'search_engine'
      }
    });
    expect(crawlerLogsResponse.ok()).toBeTruthy();
  });

  test('恶意爬虫识别测试', async ({ page, request }) => {
    // 1. 登录
    const loginResponse = await request.post('/api/v1/auth/login', {
      data: {
        username: 'admin',
        password: 'admin123'
      }
    });
    const { token } = await loginResponse.json();

    // 2. 模拟恶意爬虫 (高频请求)
    const requests = [];
    for (let i = 0; i < 20; i++) {
      requests.push(
        request.get('/api/v1/sites', {
          headers: {
            'Authorization': `Bearer ${token}`,
            'User-Agent': 'python-requests/2.28.0'
          }
        })
      );
    }

    const responses = await Promise.all(requests);
    const statusCodes = responses.map(r => r.status());
    
    // 应该有请求被限制
    expect(statusCodes).toContain(429);

    // 3. 验证爬虫日志
    const crawlerLogsResponse = await request.get('/api/v1/crawler/logs', {
      headers: {
        'Authorization': `Bearer ${token}`
      },
      params: {
        crawlerType: 'suspicious'
      }
    });
    expect(crawlerLogsResponse.ok()).toBeTruthy();
  });

  test('爬虫请求头配置测试', async ({ page, request }) => {
    // 1. 登录
    const loginResponse = await request.post('/api/v1/auth/login', {
      data: {
        username: 'admin',
        password: 'admin123'
      }
    });
    const { token } = await loginResponse.json();

    // 2. 获取爬虫头配置
    const headersResponse = await request.get('/api/v1/preheat/crawler-headers', {
      headers: {
        'Authorization': `Bearer ${token}`
      }
    });
    expect(headersResponse.ok()).toBeTruthy();
    const headers = await headersResponse.json();
    expect(headers.data).toHaveProperty('userAgents');
    expect(headers.data).toHaveProperty('languages');

    // 3. 更新爬虫头配置
    const updateResponse = await request.post('/api/v1/preheat/crawler-headers', {
      data: {
        userAgent: 'CustomBot/1.0',
        language: 'zh-CN'
      },
      headers: {
        'Authorization': `Bearer ${token}`
      }
    });
    expect(updateResponse.ok()).toBeTruthy();

    // 4. 验证配置更新
    const checkResponse = await request.get('/api/v1/preheat/crawler-headers', {
      headers: {
        'Authorization': `Bearer ${token}`
      }
    });
    expect(checkResponse.ok()).toBeTruthy();
  });

  test('预渲染缓存测试', async ({ page, request }) => {
    // 1. 登录
    const loginResponse = await request.post('/api/v1/auth/login', {
      data: {
        username: 'admin',
        password: 'admin123'
      }
    });
    const { token } = await loginResponse.json();

    // 2. 创建测试站点
    const siteResponse = await request.post('/api/v1/sites', {
      data: {
        name: 'E2E 缓存测试站点',
        domains: ['cache-test.example.com'],
        mode: 'prerender',
        prerender: {
          enabled: true,
          poolSize: 3,
          timeout: 30,
          cacheTTL: 3600
        }
      },
      headers: {
        'Authorization': `Bearer ${token}`
      }
    });
    expect(siteResponse.ok()).toBeTruthy();
    const site = await siteResponse.json();

    // 3. 触发预渲染
    const preheatResponse = await request.post('/api/v1/preheat/trigger', {
      data: {
        siteId: site.data.id,
        urls: ['https://cache-test.example.com/']
      },
      headers: {
        'Authorization': `Bearer ${token}`
      }
    });
    expect(preheatResponse.ok()).toBeTruthy();

    // 4. 等待预渲染完成
    await page.waitForTimeout(3000);

    // 5. 检查预渲染状态
    const statusResponse = await request.get('/api/v1/preheat/task/status', {
      headers: {
        'Authorization': `Bearer ${token}`
      },
      params: {
        siteId: site.data.id
      }
    });
    expect(statusResponse.ok()).toBeTruthy();
    const status = await statusResponse.json();
    expect(status.data).toHaveProperty('status');

    // 6. 检查缓存状态
    const cacheResponse = await request.get('/api/v1/preheat/stats', {
      headers: {
        'Authorization': `Bearer ${token}`
      },
      params: {
        siteId: site.data.id
      }
    });
    expect(cacheResponse.ok()).toBeTruthy();
    const cacheStats = await cacheResponse.json();
    expect(cacheStats.data).toHaveProperty('cacheSize');

    // 7. 清除缓存
    const clearResponse = await request.post('/api/v1/preheat/clear-cache', {
      data: {
        siteId: site.data.id
      },
      headers: {
        'Authorization': `Bearer ${token}`
      }
    });
    expect(clearResponse.ok()).toBeTruthy();

    // 8. 删除测试站点
    await request.delete(`/api/v1/sites/${site.data.id}`, {
      headers: {
        'Authorization': `Bearer ${token}`
      }
    });
  });
});

test.describe('SSL 证书管理测试', () => {
  test('SSL 证书状态检查', async ({ page, request }) => {
    // 1. 登录
    const loginResponse = await request.post('/api/v1/auth/login', {
      data: {
        username: 'admin',
        password: 'admin123'
      }
    });
    const { token } = await loginResponse.json();

    // 2. 获取 SSL 证书状态
    const sslResponse = await request.get('/api/v1/ssl/status', {
      headers: {
        'Authorization': `Bearer ${token}`
      }
    });
    expect(sslResponse.ok()).toBeTruthy();
    const sslStatus = await sslResponse.json();
    expect(sslStatus.data).toHaveProperty('certificates');
  });
});

test.describe('监控指标测试', () => {
  test('Prometheus 指标暴露', async ({ request }) => {
    // 直接访问 Prometheus 端点
    const metricsResponse = await request.get('http://localhost:9090/metrics');
    expect(metricsResponse.ok()).toBeTruthy();
    const metrics = await metricsResponse.text();
    expect(metrics).toContain('prerender_');
  });

  test('健康检查 API', async ({ request }) => {
    const healthResponse = await request.get('/api/v1/health');
    expect(healthResponse.ok()).toBeTruthy();
    const health = await healthResponse.json();
    expect(health.data).toHaveProperty('status');
    expect(health.data).toHaveProperty('checks');
  });
});
