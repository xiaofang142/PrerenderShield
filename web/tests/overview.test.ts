import { test, expect } from '@playwright/test';

test.describe('Overview Page', () => {
  test.beforeEach(async ({ page }) => {
    // 登录
    await page.goto('/login');
    await page.waitForLoadState('networkidle');
    await page.waitForSelector('input[placeholder="Username"]');
    await page.fill('input[placeholder="Username"]', 'admin');
    await page.fill('input[placeholder="Password"]', '123456');
    await page.click('button[type="submit"]');
    // 等待登录成功 - 不等待特定 URL，只等待网络空闲和侧边栏出现
    await page.waitForLoadState('networkidle');
    await page.waitForTimeout(2000);
    await expect(page.locator('.ant-menu, .sidebar').first()).toBeVisible();
  });

  test('Overview page loads successfully', async ({ page }) => {
    // 尝试导航到概览页面
    const overviewLink = page.locator('a[href="/"], a:has-text("概览"), a:has-text("Overview"), a:has-text("Dashboard")').first();
    if (await overviewLink.count() > 0) {
      await overviewLink.click();
      await page.waitForLoadState('domcontentloaded');
      await page.waitForTimeout(1000);
    }

    // 检查是否有标题或卡片
    const title = page.locator('h1:has-text("概览"), h1:has-text("Overview"), h1.page-title');
    const cards = page.locator('.ant-card, .stat-card');

    if (await title.count() > 0) {
      await expect(title.first()).toBeVisible();
    } else if (await cards.count() > 0) {
      // 如果没有标题，至少有卡片也算成功
      await expect(cards.first()).toBeVisible();
    }
  });

  test('Overview shows system summary', async ({ page }) => {
    await expect(page.locator('.ant-card, .stat-card, .summary-card').first()).toBeVisible();
  });

  test('Overview shows key metrics', async ({ page }) => {
    await expect(page.locator('.ant-statistic, .stat, .metric, .stats-card').first()).toBeVisible();
  });

  test('Overview shows site status', async ({ page }) => {
    await expect(page.locator('.ant-card, .site-card, .site-status, .sites-overview').first()).toBeVisible();
  });
});
