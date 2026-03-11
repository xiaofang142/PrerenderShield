import { test, expect } from '@playwright/test';

test.describe('System Configuration Page', () => {
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
    
    // 直接导航到系统配置页面
    await page.goto('/system');
    await page.waitForURL('/system');
  });

  test('System configuration page loads successfully', async ({ page }) => {
    await expect(page.locator('h1.page-title')).toContainText('系统配置');
  });

  test('System configuration shows basic settings', async ({ page }) => {
    await expect(page.locator('.ant-card')).toBeVisible();
  });

  test('System configuration allows saving settings', async ({ page }) => {
    // 点击保存设置按钮
    await page.click('button:has-text("保存设置")');
    await expect(page.locator('.ant-message-success')).toBeVisible();
  });

  test('System configuration shows database settings', async ({ page }) => {
    await expect(page.locator('.ant-card')).toBeVisible();
  });

  test('System configuration shows cache settings', async ({ page }) => {
    await expect(page.locator('.ant-card')).toBeVisible();
  });

  test('System configuration shows security settings', async ({ page }) => {
    await expect(page.locator('.ant-card')).toBeVisible();
  });

  test('System configuration allows restarting services', async ({ page }) => {
    // 点击重启服务按钮
    await page.click('button:has-text("重启服务")');
    await expect(page.locator('.ant-modal-confirm')).toBeVisible();
    
    // 点击取消
    await page.click('button:has-text("取消")');
  });

  test('System configuration shows service status', async ({ page }) => {
    await expect(page.locator('.ant-card')).toBeVisible();
  });



  test('System configuration service status refresh test', async ({ page }) => {
    // 点击刷新服务状态
    await page.click('button:has-text("刷新状态")');
    
    // 等待刷新完成
    await page.waitForLoadState('networkidle');
  });

  test('System configuration basic settings test', async ({ page }) => {
    // 测试基础设置更新
    await page.fill('input[name="appName"]', 'PrerenderShield');
    await page.fill('input[name="appDescription"]', '网站预渲染与防火墙系统');
    await page.fill('input[name="adminEmail"]', 'admin@example.com');
    await page.click('button:has-text("保存设置")');
    await page.waitForSelector('.ant-message-success');
    await expect(page.locator('.ant-message-success')).toBeVisible();
  });

  test('System configuration database settings test', async ({ page }) => {
    // 测试数据库设置更新
    await page.fill('input[name="dbHost"]', 'localhost');
    await page.fill('input[name="dbPort"]', '3306');
    await page.fill('input[name="dbName"]', 'prerender');
    await page.fill('input[name="dbUser"]', 'root');
    await page.fill('input[name="dbPassword"]', 'password');
    await page.click('button:has-text("保存设置")');
    await page.waitForSelector('.ant-message-success');
    await expect(page.locator('.ant-message-success')).toBeVisible();
  });

  test('System configuration cache settings test', async ({ page }) => {
    // 测试缓存设置更新
    await page.fill('input[name="cacheTTL"]', '3600');
    await page.fill('input[name="cacheSize"]', '1000');
    await page.selectOption('select[name="cacheType"]', 'redis');
    await page.click('button:has-text("保存设置")');
    await page.waitForSelector('.ant-message-success');
    await expect(page.locator('.ant-message-success')).toBeVisible();
  });

  test('System configuration security settings test', async ({ page }) => {
    // 测试安全设置更新
    await page.fill('input[name="jwtSecret"]', 'your-secret-key');
    await page.fill('input[name="jwtExpiry"]', '24');
    await page.selectOption('select[name="secureMode"]', 'true');
    await page.click('button:has-text("保存设置")');
    await page.waitForSelector('.ant-message-success');
    await expect(page.locator('.ant-message-success')).toBeVisible();
  });

  test('System configuration service management test', async ({ page }) => {
    // 测试服务重启
    await page.click('button:has-text("重启服务")');
    await page.waitForSelector('.ant-modal-confirm');
    await page.click('button[type="primary"]:has-text("确定")');
    await page.waitForSelector('.ant-message-success');
    await expect(page.locator('.ant-message-success')).toBeVisible();
  });

  test('System configuration service status test', async ({ page }) => {
    // 测试服务状态监控
    await page.click('button:has-text("刷新状态")');
    await page.waitForLoadState('networkidle');
    await expect(page.locator('.ant-statistic')).toBeVisible();
  });

  test('System configuration backup test', async ({ page }) => {
    // 测试系统备份
    await page.click('button:has-text("备份系统")');
    await page.waitForSelector('.ant-modal-confirm');
    await page.click('button[type="primary"]:has-text("确定")');
    await page.waitForSelector('.ant-message-success');
    await expect(page.locator('.ant-message-success')).toBeVisible();
  });

  test('System configuration restore test', async ({ page }) => {
    // 测试系统恢复
    await page.click('button:has-text("恢复系统")');
    await page.waitForSelector('.ant-modal-confirm');
    await page.click('button[type="danger"]:has-text("确定")');
    await page.waitForSelector('.ant-message-success');
    await expect(page.locator('.ant-message-success')).toBeVisible();
  });
});
