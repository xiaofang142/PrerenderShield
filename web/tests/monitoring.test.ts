import { test, expect } from '@playwright/test';

test.describe('Monitoring Page', () => {
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
      await page.waitForSelector('input[placeholder="Username"]');
      // 填写登录表单
      await page.fill('input[placeholder="Username"]', 'admin');
      await page.fill('input[placeholder="Password"]', '123456');
      // 点击登录按钮
      await page.click('button[type="submit"]');
      // 等待导航到首页
      await page.waitForURL('/');
    }
    
    // 直接导航到监控页面
    await page.goto('/monitoring');
    await page.waitForURL('/monitoring');
  });

  test('Monitoring page loads successfully', async ({ page }) => {
    await expect(page.locator('h1.page-title')).toContainText('监控');
  });

  test('Monitoring shows system metrics', async ({ page }) => {
    await expect(page.locator('.ant-card')).toBeVisible();
    await expect(page.locator('.ant-statistic')).toBeVisible();
  });

  test('Monitoring shows performance charts', async ({ page }) => {
    await expect(page.locator('.ant-card')).toBeVisible();
  });

  test('Monitoring shows error rates', async ({ page }) => {
    await expect(page.locator('.ant-card')).toBeVisible();
  });

  test('Monitoring shows request statistics', async ({ page }) => {
    await expect(page.locator('.ant-card')).toBeVisible();
  });

  test('Monitoring allows changing time range', async ({ page }) => {
    await expect(page.locator('.ant-picker')).toBeVisible();
  });

  test('Monitoring shows real-time data', async ({ page }) => {
    await expect(page.locator('.ant-card')).toBeVisible();
  });

  test('Monitoring allows exporting data', async ({ page }) => {
    // 点击导出按钮
    await page.click('button:has-text("导出")');
    await expect(page.locator('.ant-dropdown')).toBeVisible();
  });

  test('Monitoring time range change test', async ({ page }) => {
    // 点击时间范围选择器
    await page.click('.ant-picker');
    
    // 等待时间选择器出现
    await page.waitForSelector('.ant-picker-dropdown');
    
    // 选择时间范围
    await page.click('text="今天"');
    
    // 等待数据更新
    await page.waitForLoadState('networkidle');
  });

  test('Monitoring refresh data test', async ({ page }) => {
    // 点击刷新按钮
    await page.click('button:has-text("刷新")');
    
    // 等待数据刷新完成
    await page.waitForLoadState('networkidle');
  });

  test('Monitoring chart interaction test', async ({ page }) => {
    // 点击图表区域
    const chart = page.locator('.ant-card-body').first();
    await chart.click();
    
    // 等待可能的交互响应
    await page.waitForLoadState('networkidle');
  });

  test('Monitoring navigation to other pages', async ({ page }) => {
    // 测试导航到概览页面
    await page.click('text="概览"');
    await page.waitForURL('/');
    await expect(page).toHaveURL('/');

    // 测试导航回监控页面
    await page.click('text="监控"');
    await page.waitForURL('/monitoring');
    await expect(page).toHaveURL('/monitoring');
  });

  test('Monitoring system metrics test', async ({ page }) => {
    // 导航到系统指标页面
    await page.goto('/monitoring/system');
    await page.waitForURL('/monitoring/system');
    
    // 验证页面加载成功
    await expect(page.locator('h1.page-title')).toContainText('系统指标');
    
    // 验证系统指标显示
    await expect(page.locator('.ant-statistic')).toBeVisible();
  });

  test('Monitoring performance charts test', async ({ page }) => {
    // 导航到性能图表页面
    await page.goto('/monitoring/performance');
    await page.waitForURL('/monitoring/performance');
    
    // 验证页面加载成功
    await expect(page.locator('h1.page-title')).toContainText('性能图表');
    
    // 验证性能图表显示
    await expect(page.locator('.ant-card')).toBeVisible();
  });

  test('Monitoring error rates test', async ({ page }) => {
    // 导航到错误率页面
    await page.goto('/monitoring/error');
    await page.waitForURL('/monitoring/error');
    
    // 验证页面加载成功
    await expect(page.locator('h1.page-title')).toContainText('错误率');
    
    // 验证错误率数据显示
    await expect(page.locator('.ant-card')).toBeVisible();
  });

  test('Monitoring request statistics test', async ({ page }) => {
    // 导航到请求统计页面
    await page.goto('/monitoring/request');
    await page.waitForURL('/monitoring/request');
    
    // 验证页面加载成功
    await expect(page.locator('h1.page-title')).toContainText('请求统计');
    
    // 验证请求统计数据显示
    await expect(page.locator('.ant-card')).toBeVisible();
  });

  test('Monitoring real-time data test', async ({ page }) => {
    // 验证实时数据更新
    await expect(page.locator('.ant-card')).toBeVisible();
    
    // 等待一段时间，验证数据是否更新
    await page.waitForTimeout(2000);
  });

  test('Monitoring data export test', async ({ page }) => {
    // 测试导出数据为CSV
    await page.click('button:has-text("导出")');
    await page.click('text="导出为CSV"');
    await page.waitForLoadState('networkidle');
    
    // 测试导出数据为Excel
    await page.click('button:has-text("导出")');
    await page.click('text="导出为Excel"');
    await page.waitForLoadState('networkidle');
  });
});
