import { test, expect } from '@playwright/test';

test.describe('Prerender Page', () => {
  test.beforeEach(async ({ page }) => {
    // 登录
    await page.goto('/login');
    // 等待页面完全加载
    await page.waitForLoadState('networkidle');
    
    // 检查是否显示系统初始化向导
    if (await page.locator('h1:has-text("系统初始化向导")').count() > 0) {
      // 处理系统初始化向导
      // 点击同意使用声明复选框
      await page.click('input[type="checkbox"]');
      
      // 点击下一步按钮
      await page.click('button:has-text("下一步")');
      
      // 等待设置管理员页面
      await page.waitForSelector('input[name="username"]');
      
      // 填写管理员信息
      await page.fill('input[name="username"]', 'admin');
      await page.fill('input[name="password"]', '123456');
      await page.fill('input[name="confirmPassword"]', '123456');
      await page.fill('input[name="email"]', 'admin@example.com');
      await page.fill('input[name="company"]', 'Test Company');
      
      // 点击下一步按钮
      await page.click('button:has-text("下一步")');
      
      // 等待完成页面
      await page.waitForSelector('button:has-text("完成")');
      
      // 点击完成按钮
      await page.click('button:has-text("完成")');
      
      // 等待页面跳转
      await page.waitForURL('/');
    } else {
      // 等待登录表单出现
      await page.waitForSelector('form[name="login"]');
      // 填写登录表单
      await page.fill('form[name="login"] input[name="username"]', 'admin');
      await page.fill('form[name="login"] input[name="password"]', '123456');
      // 点击登录按钮
      await page.click('form[name="login"] button[type="submit"]');
      // 等待导航到首页
      await page.waitForURL('/');
    }
    
    // 直接导航到预渲染配置页面
    await page.goto('/prerender/preheat');
    await page.waitForURL('/prerender/preheat');
  });

  test('Prerender page loads successfully', async ({ page }) => {
    await expect(page.locator('h1.page-title')).toContainText('渲染预热');
  });

  test('Prerender shows configuration options', async ({ page }) => {
    await expect(page.locator('.card')).toBeVisible();
  });

  test('Prerender shows cache status', async ({ page }) => {
    await expect(page.locator('.ant-statistic-title:has-text("渲染预热状态")')).toBeVisible();
  });

  test('Prerender allows manual rendering', async ({ page }) => {
    // 点击手动渲染按钮
    await page.click('button:has-text("手动渲染")');
    await expect(page.locator('.ant-modal-title:has-text("手动渲染")')).toBeVisible();
    
    // 填写渲染URL
    await page.fill('input[placeholder="请输入要渲染的URL"]', 'https://example.com');
    
    // 点击开始渲染按钮
    await page.click('button:has-text("开始渲染")');
    
    // 等待成功提示
    await page.waitForSelector('.ant-message-success');
    await expect(page.locator('.ant-message-success')).toBeVisible();
  });

  test('Prerender allows starting preheat', async ({ page }) => {
    // 点击缓存预热按钮
    await page.click('button:has-text("缓存预热")');
    await expect(page.locator('.ant-modal-title:has-text("缓存预热")')).toBeVisible();
    
    // 点击开始预热按钮
    await page.click('button:has-text("开始预热")');
    
    // 等待成功提示
    await page.waitForSelector('.ant-message-success');
    await expect(page.locator('.ant-message-success')).toBeVisible();
  });

  test('Prerender shows site selector', async ({ page }) => {
    await expect(page.locator('select')).toBeVisible();
  });

  test('Prerender allows refreshing status', async ({ page }) => {
    // 点击刷新状态按钮
    await page.click('button:has-text("刷新状态")');
    
    // 等待刷新完成
    await page.waitForLoadState('networkidle');
  });

  test('Prerender shows rendering history', async ({ page }) => {
    await expect(page.locator('table')).toBeVisible();
  });

  test('Prerender navigation to push page', async ({ page }) => {
    // 导航到推送配置页面
    await page.goto('/prerender/push');
    await page.waitForURL('/prerender/push');
    
    // 验证页面加载成功
    await expect(page.locator('h1.page-title')).toContainText('推送配置');
  });

  test('Prerender push configuration test', async ({ page }) => {
    // 导航到推送配置页面
    await page.goto('/prerender/push');
    await page.waitForURL('/prerender/push');
    
    // 验证推送配置页面元素
    await expect(page.locator('.card')).toBeVisible();
    await expect(page.locator('select')).toBeVisible();
  });

  test('Prerender cache management test', async ({ page }) => {
    // 导航到缓存管理页面
    await page.goto('/prerender/cache');
    await page.waitForURL('/prerender/cache');
    
    // 验证页面加载成功
    await expect(page.locator('h1.page-title')).toContainText('缓存管理');
    
    // 测试缓存清理
    await page.click('button:has-text("清理缓存")');
    await page.waitForSelector('.ant-modal-confirm');
    await page.click('button[type="primary"]:has-text("确定")');
    await page.waitForSelector('.ant-message-success');
    await expect(page.locator('.ant-message-success')).toBeVisible();
  });

  test('Prerender status monitoring test', async ({ page }) => {
    // 导航到状态监控页面
    await page.goto('/prerender/status');
    await page.waitForURL('/prerender/status');
    
    // 验证页面加载成功
    await expect(page.locator('h1.page-title')).toContainText('预渲染状态');
    
    // 验证状态指标显示
    await expect(page.locator('.ant-statistic')).toBeVisible();
  });

  test('Prerender crawler header configuration test', async ({ page }) => {
    // 导航到爬虫头配置页面
    await page.goto('/prerender/crawler');
    await page.waitForURL('/prerender/crawler');
    
    // 验证页面加载成功
    await expect(page.locator('h1.page-title')).toContainText('爬虫头配置');
    
    // 测试添加爬虫头
    await page.click('button:has-text("添加爬虫头")');
    await page.waitForSelector('.ant-modal');
    await page.fill('input[name="name"]', 'Test-Header');
    await page.fill('input[name="value"]', 'test-value');
    await page.click('button[type="primary"]:has-text("保存")');
    await page.waitForSelector('.ant-message-success');
    await expect(page.locator('.ant-message-success')).toBeVisible();
  });

  test('Prerender batch rendering test', async ({ page }) => {
    // 导航到批量渲染页面
    await page.goto('/prerender/batch');
    await page.waitForURL('/prerender/batch');
    
    // 验证页面加载成功
    await expect(page.locator('h1.page-title')).toContainText('批量渲染');
    
    // 测试批量渲染
    await page.fill('textarea[placeholder="请输入多个URL，每行一个"]', 'https://example.com\nhttps://example.org');
    await page.click('button:has-text("开始批量渲染")');
    await page.waitForSelector('.ant-message-success');
    await expect(page.locator('.ant-message-success')).toBeVisible();
  });
});
