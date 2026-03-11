import { test, expect } from '@playwright/test';

test.describe('Overview Page', () => {
  test.beforeEach(async ({ page }) => {
    // 登录
    await page.goto('/login');
    // 等待页面完全加载
    await page.waitForLoadState('networkidle');
    // 等待登录表单出现
    await page.waitForSelector('form[name="login"]');
    // 填写登录表单
    await page.fill('form[name="login"] input[name="username"]', 'admin');
    await page.fill('form[name="login"] input[name="password"]', '123456');
    // 点击登录按钮
    await page.click('form[name="login"] button[type="submit"]');
    // 导航到概览页面
    await page.click('text=Overview');
    await page.waitForURL('/overview');
  });

  test('Overview page loads successfully', async ({ page }) => {
    await expect(page).toHaveTitle(/Overview/);
    await expect(page.locator('h1')).toContainText('Overview');
  });

  test('Overview shows system summary', async ({ page }) => {
    await expect(page.locator('.system-summary')).toBeVisible();
  });

  test('Overview shows key metrics', async ({ page }) => {
    await expect(page.locator('.key-metrics')).toBeVisible();
  });

  test('Overview shows recent events', async ({ page }) => {
    await expect(page.locator('.recent-events')).toBeVisible();
  });

  test('Overview shows site status', async ({ page }) => {
    await expect(page.locator('.site-status')).toBeVisible();
  });

  test('Overview allows navigating to detailed pages', async ({ page }) => {
    // 测试导航到站点管理
    await page.click('text=View All Sites');
    await page.waitForURL('/sites');
    await expect(page).toHaveTitle(/Sites/);

    // 测试导航到监控
    await page.click('text=View Monitoring');
    await page.waitForURL('/monitoring');
    await expect(page).toHaveTitle(/Monitoring/);
  });

  test('Overview shows performance overview', async ({ page }) => {
    await expect(page.locator('.performance-overview')).toBeVisible();
  });

  test('Overview shows security status', async ({ page }) => {
    await expect(page.locator('.security-status')).toBeVisible();
  });
});
