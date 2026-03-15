import { test, expect } from '@playwright/test';

test.describe('Overview Page', () => {
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
    // 导航到概览页面
    await page.click('text=概览');
    await page.waitForURL('/');
  });

  test('Overview page loads successfully', async ({ page }) => {
    await expect(page.locator('h1')).toContainText('概览');
  });

  test('Overview shows system summary', async ({ page }) => {
    await expect(page.locator('.ant-card:has-text("系统")')).toBeVisible();
  });

  test('Overview shows key metrics', async ({ page }) => {
    await expect(page.locator('.ant-statistic')).toBeVisible();
  });

  test('Overview shows site status', async ({ page }) => {
    await expect(page.locator('.ant-card:has-text("站点")')).toBeVisible();
  });
});
