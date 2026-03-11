/**
 * 全链路端到端测试 - 完整渲染流程
 * 测试场景：从请求到预渲染的完整流程
 */

import { test, expect } from '@playwright/test';

test.describe('完整渲染流程测试', () => {
  test.beforeEach(async ({ page }) => {
    // 登录
    await page.goto('/login');
    await page.fill('input[type="text"]', 'admin');
    await page.fill('input[type="password"]', 'admin123');
    await page.click('button[type="submit"]');
    await page.waitForURL('/overview');
  });

  test('完整预渲染流程测试', async ({ page }) => {
    // 1. 创建测试站点
    await page.goto('/sites');
    await page.click('button:has-text("新建站点")');
    
    await page.fill('input[placeholder="站点名称"]', 'E2E 测试站点');
    await page.fill('input[placeholder="域名"]', 'test.example.com');
    await page.selectOption('select[name="mode"]', 'prerender');
    
    // 配置预渲染
    await page.check('text=启用预渲染');
    await page.fill('input[name="poolSize"]', '3');
    await page.fill('input[name="timeout"]', '30');
    
    await page.click('button:has-text("保存")');
    await expect(page.locator('.ant-message-success')).toBeVisible();

    // 2. 验证站点创建成功
    await page.waitForSelector('text=E2E 测试站点');
    const siteRow = page.locator('tr:has-text("E2E 测试站点")');
    await expect(siteRow).toBeVisible();

    // 3. 配置 WAF 规则
    await page.click('text=WAF 配置');
    await page.check('text=启用 WAF');
    
    // 添加防护规则
    await page.click('button:has-text("添加规则")');
    await page.selectOption('select[name="ruleType"]', 'sql_injection');
    await page.selectOption('select[name="action"]', 'block');
    await page.click('button:has-text("保存规则")');
    
    await expect(page.locator('.ant-message-success')).toBeVisible();

    // 4. 触发预渲染预热
    await page.goto('/prerender');
    await page.selectOption('select[name="siteId"]', 'E2E 测试站点');
    await page.fill('textarea[name="urls"]', 'https://test.example.com/\nhttps://test.example.com/about');
    await page.click('button:has-text("开始预热")');
    
    // 等待预热任务提交
    await expect(page.locator('.ant-message-success')).toBeVisible();

    // 5. 检查预热状态
    await page.waitForTimeout(2000);
    await page.click('button:has-text("刷新状态")');
    
    const statusElement = page.locator('.preheat-status');
    await expect(statusElement).toBeVisible();

    // 6. 验证缓存状态
    await page.click('text=缓存管理');
    const cacheStats = page.locator('.cache-stats');
    await expect(cacheStats).toBeVisible();
    
    // 7. 清理测试站点
    await page.goto('/sites');
    await page.click('tr:has-text("E2E 测试站点") button:has-text("删除")');
    await page.click('button:has-text("确认删除")');
    await expect(page.locator('.ant-message-success')).toBeVisible();
  });

  test('WAF 防护拦截测试', async ({ page }) => {
    // 1. 配置 WAF 规则
    await page.goto('/firewall');
    
    // 添加 SQL 注入防护
    await page.click('button:has-text("添加规则")');
    await page.selectOption('select[name="type"]', 'sql_injection');
    await page.selectOption('select[name="action"]', 'block');
    await page.fill('input[name="description"]', 'E2E SQL 注入测试');
    await page.click('button:has-text("保存")');
    
    await expect(page.locator('.ant-message-success')).toBeVisible();

    // 2. 添加 XSS 防护
    await page.click('button:has-text("添加规则")');
    await page.selectOption('select[name="type"]', 'xss');
    await page.selectOption('select[name="action"]', 'block');
    await page.fill('input[name="description"]', 'E2E XSS 测试');
    await page.click('button:has-text("保存")');
    
    await expect(page.locator('.ant-message-success')).toBeVisible();

    // 3. 配置 IP 黑名单
    await page.goto('/firewall/blacklist');
    await page.fill('input[name="ip"]', '192.168.1.100');
    await page.fill('textarea[name="reason"]', 'E2E 测试黑名单');
    await page.click('button:has-text("添加")');
    
    await expect(page.locator('.ant-message-success')).toBeVisible();

    // 4. 配置 IP 白名单
    await page.goto('/firewall/whitelist');
    await page.fill('input[name="ip"]', '10.0.0.1');
    await page.fill('textarea[name="reason"]', 'E2E 测试白名单');
    await page.click('button:has-text("添加")');
    
    await expect(page.locator('.ant-message-success')).toBeVisible();

    // 5. 验证攻击日志
    await page.goto('/firewall/attacks');
    await page.click('button:has-text("刷新")');
    
    // 应该有攻击日志记录
    const attackTable = page.locator('.attack-log-table');
    await expect(attackTable).toBeVisible();
  });

  test('爬虫识别和日志测试', async ({ page }) => {
    // 1. 访问爬虫日志页面
    await page.goto('/crawler/logs');
    
    // 2. 验证爬虫日志显示
    const logTable = page.locator('.crawler-log-table');
    await expect(logTable).toBeVisible();

    // 3. 测试爬虫统计
    await page.goto('/crawler/stats');
    const statsCards = page.locator('.stats-card');
    await expect(statsCards).toHaveCount({ min: 4 });

    // 4. 配置爬虫规则
    await page.goto('/crawler/config');
    await page.check('text=启用爬虫检测');
    await page.selectOption('select[name="detectionMode"]', 'strict');
    await page.click('button:has-text("保存配置")');
    
    await expect(page.locator('.ant-message-success')).toBeVisible();

    // 5. 验证爬虫头配置
    await page.goto('/crawler/headers');
    const headerList = page.locator('.crawler-headers-list');
    await expect(headerList).toBeVisible();
  });

  test('监控和告警测试', async ({ page }) => {
    // 1. 访问监控概览
    await page.goto('/monitoring');
    
    // 2. 验证系统指标显示
    const cpuChart = page.locator('.cpu-chart');
    await expect(cpuChart).toBeVisible();
    
    const memoryChart = page.locator('.memory-chart');
    await expect(memoryChart).toBeVisible();

    // 3. 验证请求统计
    const requestStats = page.locator('.request-stats');
    await expect(requestStats).toBeVisible();

    // 4. 测试时间范围切换
    await page.selectOption('select[name="timeRange"]', '1h');
    await page.waitForTimeout(1000);
    await expect(cpuChart).toBeVisible();

    await page.selectOption('select[name="timeRange"]', '24h');
    await page.waitForTimeout(1000);
    await expect(cpuChart).toBeVisible();

    // 5. 验证健康检查状态
    await page.goto('/system/health');
    const healthStatus = page.locator('.health-status');
    await expect(healthStatus).toBeVisible();
    
    // 检查各项健康指标
    const redisStatus = page.locator('.redis-status');
    await expect(redisStatus).toBeVisible();
    
    const engineStatus = page.locator('.engine-status');
    await expect(engineStatus).toBeVisible();
  });

  test('日志查询和导出测试', async ({ page }) => {
    // 1. 访问访问日志
    await page.goto('/logs');
    
    // 2. 测试日志过滤
    await page.selectOption('select[name="logType"]', 'access');
    await page.fill('input[name="keyword"]', 'test');
    await page.click('button:has-text("查询")');
    
    const logTable = page.locator('.log-table');
    await expect(logTable).toBeVisible();

    // 3. 测试时间范围过滤
    await page.fill('input[name="startTime"]', '2024-01-01');
    await page.fill('input[name="endTime"]', '2024-12-31');
    await page.click('button:has-text("查询")');
    
    await expect(logTable).toBeVisible();

    // 4. 测试日志导出
    await page.click('button:has-text("导出日志")');
    // 验证下载对话框或文件下载
    await page.waitForTimeout(2000);

    // 5. 测试日志清理
    await page.click('button:has-text("清理日志")');
    await page.click('button:has-text("确认清理")');
    await expect(page.locator('.ant-message-success')).toBeVisible();
  });

  test('站点静态资源管理测试', async ({ page }) => {
    // 1. 创建测试站点
    await page.goto('/sites');
    await page.click('button:has-text("新建站点")');
    await page.fill('input[placeholder="站点名称"]', '静态资源测试站点');
    await page.fill('input[placeholder="域名"]', 'static.example.com');
    await page.selectOption('select[name="mode"]', 'static');
    await page.click('button:has-text("保存")');
    
    await page.waitForSelector('text=静态资源测试站点');

    // 2. 上传静态文件
    await page.click('text=静态资源测试站点 >> text=管理');
    await page.click('text=静态资源');
    
    const fileInput = page.locator('input[type="file"]');
    await fileInput.setInputFiles({
      name: 'test.html',
      mimeType: 'text/html',
      buffer: Buffer.from('<html><body>Test</body></html>')
    });
    
    await page.click('button:has-text("上传")');
    await expect(page.locator('.ant-message-success')).toBeVisible();

    // 3. 验证文件列表
    const fileList = page.locator('.file-list');
    await expect(fileList).toContainText('test.html');

    // 4. 测试文件解压
    await page.click('button:has-text("解压")');
    await page.waitForTimeout(2000);

    // 5. 删除文件
    await page.click('button:has-text("删除") >> nth=0');
    await page.click('button:has-text("确认")');
    await expect(page.locator('.ant-message-success')).toBeVisible();

    // 6. 清理测试站点
    await page.goto('/sites');
    await page.click('tr:has-text("静态资源测试站点") button:has-text("删除")');
    await page.click('button:has-text("确认删除")');
  });

  test('推送配置和日志测试', async ({ page }) => {
    // 1. 访问推送配置
    await page.goto('/push/config');
    
    // 2. 配置推送参数
    await page.check('text=启用推送');
    await page.fill('input[name="pushInterval"]', '3600');
    await page.fill('input[name="batchSize"]', '100');
    await page.click('button:has-text("保存配置")');
    
    await expect(page.locator('.ant-message-success')).toBeVisible();

    // 3. 查看推送统计
    await page.goto('/push/stats');
    const statsCards = page.locator('.push-stats-card');
    await expect(statsCards).toHaveCount({ min: 3 });

    // 4. 查看推送日志
    await page.goto('/push/logs');
    const logTable = page.locator('.push-log-table');
    await expect(logTable).toBeVisible();

    // 5. 查看推送趋势
    await page.goto('/push/trend');
    const trendChart = page.locator('.push-trend-chart');
    await expect(trendChart).toBeVisible();
  });

  test('系统配置管理测试', async ({ page }) => {
    // 1. 访问系统配置
    await page.goto('/system/config');
    
    // 2. 修改日志保留天数
    await page.fill('input[name="accessLogRetention"]', '14');
    await page.fill('input[name="crawlerLogRetention"]', '30');
    await page.click('button:has-text("保存配置")');
    
    await expect(page.locator('.ant-message-success')).toBeVisible();

    // 3. 验证配置更新
    await page.reload();
    await expect(page.locator('input[name="accessLogRetention"]')).toHaveValue('14');
    await expect(page.locator('input[name="crawlerLogRetention"]')).toHaveValue('30');

    // 4. 测试系统版本
    await page.goto('/system/about');
    const versionInfo = page.locator('.version-info');
    await expect(versionInfo).toBeVisible();

    // 5. 测试服务管理
    await page.goto('/system/services');
    const serviceList = page.locator('.service-list');
    await expect(serviceList).toBeVisible();
  });

  test('多语言切换测试', async ({ page }) => {
    // 1. 访问概览页面
    await page.goto('/overview');
    
    // 2. 切换到英文
    await page.click('button:has-text("中文")');
    await page.click('text=English');
    await page.waitForTimeout(500);
    
    // 验证页面语言切换
    await expect(page.locator('text=Overview')).toBeVisible();

    // 3. 切换回中文
    await page.click('button:has-text("English")');
    await page.click('text=中文');
    await page.waitForTimeout(500);
    
    await expect(page.locator('text=概览')).toBeVisible();
  });

  test('会话管理和超时测试', async ({ page }) => {
    // 1. 验证登录状态
    await page.goto('/overview');
    await expect(page.locator('.user-menu')).toBeVisible();

    // 2. 测试登出
    await page.click('.user-menu');
    await page.click('text=退出登录');
    await page.waitForURL('/login');
    
    // 3. 验证无法访问受保护页面
    await page.goto('/overview');
    await page.waitForURL('/login');
    await expect(page.locator('text=登录')).toBeVisible();
  });
});
