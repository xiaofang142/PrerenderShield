import { test, expect } from '@playwright/test';

test.describe('Dashboard Page', () => {
  test.beforeEach(async ({ page }) => {
    // 登录
    await page.goto('/login');
    // 等待页面完全加载
    await page.waitForLoadState('domcontentloaded');
    await page.waitForTimeout(2000);
    // 等待登录表单出现
    await page.waitForSelector('input[placeholder="Username"]');
    // 填写登录表单
    await page.fill('input[placeholder="Username"]', 'admin');
    await page.fill('input[placeholder="Password"]', '123456');
    // 点击登录按钮
    await page.click('button[type="submit"]');
    // 等待导航到首页
    await page.waitForURL('/');
    // 直接导航到仪表板
    await page.goto('/dashboard');
    await page.waitForURL('/dashboard');
  });

  test('Dashboard page loads successfully', async ({ page }) => {
    await expect(page.locator('h1.page-title')).toContainText('控制台首页');
  });

  test('Dashboard shows system status', async ({ page }) => {
    await expect(page.locator('.ant-card:has-text("系统状态")')).toBeVisible();
  });

  test('Dashboard shows site statistics', async ({ page }) => {
    await expect(page.locator('.ant-card:has-text("访问统计")')).toBeVisible();
  });

  test('Dashboard shows traffic trends', async ({ page }) => {
    await expect(page.locator('.ant-card:has-text("流量趋势")')).toBeVisible();
  });

  test('Dashboard shows key metrics', async ({ page }) => {
    await expect(page.locator('.ant-statistic-title')).toBeVisible();
  });

  test('Navigation to other pages from dashboard', async ({ page }) => {
    // 测试导航到站点管理
    await page.click('text="站点管理"');
    await page.waitForURL('/sites');
    await expect(page).toHaveURL('/sites');

    // 测试导航到防火墙
    await page.click('text="防火墙"');
    await page.waitForURL('/firewall');
    await expect(page).toHaveURL('/firewall');

    // 测试导航到预渲染
    await page.click('text="预渲染配置"');
    await page.waitForURL('/prerender/preheat');
    await expect(page).toHaveURL('/prerender/preheat');
  });
});
