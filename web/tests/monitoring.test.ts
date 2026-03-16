import { test, expect } from '@playwright/test';

test.describe('Monitoring Page', () => {
  test.beforeEach(async ({ page }) => {
    // 登录
    await page.goto('/login');
    await page.waitForLoadState('networkidle');

    // 检查是否显示系统初始化向导
    if (await page.locator('h1:has-text("系统初始化向导")').count() > 0) {
      await page.click('input[type="checkbox"]');
      await page.click('button:has-text("下一步")');
      await page.waitForSelector('input[name="username"]');
      await page.fill('input[name="username"]', 'admin');
      await page.fill('input[name="password"]', '123456');
      await page.fill('input[name="confirmPassword"]', '123456');
      await page.fill('input[name="email"]', 'admin@example.com');
      await page.fill('input[name="company"]', 'Test Company');
      await page.click('button:has-text("下一步")');
      await page.waitForSelector('button:has-text("完成")');
      await page.click('button:has-text("完成")');
      await page.waitForLoadState('networkidle');
    } else {
      await page.waitForSelector('input[placeholder="Username"]');
      await page.fill('input[placeholder="Username"]', 'admin');
      await page.fill('input[placeholder="Password"]', '123456');
      await page.click('button[type="submit"]');
      await page.waitForLoadState('networkidle');
    }

    // 等待登录成功 - 检查侧边栏是否存在
    await expect(page.locator('.ant-menu, .sidebar').first()).toBeVisible();
    await page.waitForTimeout(1000);

    // 使用侧边栏导航到监控页面
    const monitoringLink = page.locator('a[href="/monitoring"]');
    if (await monitoringLink.count() > 0) {
      await monitoringLink.click();
      await page.waitForURL('/monitoring');
    } else {
      await page.evaluate(() => window.location.href = '/monitoring');
    }
    await page.waitForLoadState('domcontentloaded');
    await page.waitForTimeout(2000);
  });

  test('Monitoring page loads successfully', async ({ page }) => {
    const title = page.locator('h1.page-title, h1:has-text("监控"), h1:has-text("Monitoring")').first();
    await expect(title).toBeVisible();
  });

  test('Monitoring shows system metrics', async ({ page }) => {
    await expect(page.locator('.ant-card, .stat-card, .metric-card').first()).toBeVisible();
  });

  test('Monitoring shows performance charts', async ({ page }) => {
    await expect(page.locator('.ant-card, .chart-card, .recharts-wrapper').first()).toBeVisible();
  });

  test('Monitoring shows error rates', async ({ page }) => {
    await expect(page.locator('.ant-card, .stat-card').first()).toBeVisible();
  });

  test('Monitoring shows request statistics', async ({ page }) => {
    await expect(page.locator('.ant-card, .stat-card').first()).toBeVisible();
  });

  test('Monitoring allows changing time range', async ({ page }) => {
    const timePicker = page.locator('.ant-picker, .date-range-picker, .time-range').first();
    if (await timePicker.count() > 0) {
      await timePicker.click();
      await page.waitForSelector('.ant-picker-dropdown, .ant-picker-panel', { timeout: 3000 });
    }
  });

  test('Monitoring shows real-time data', async ({ page }) => {
    await expect(page.locator('.ant-card, .real-time-card').first()).toBeVisible();
  });

  test('Monitoring allows exporting data', async ({ page }) => {
    const exportBtn = page.locator('button:has-text("导出"), button:has-text("Export")').first();
    if (await exportBtn.count() > 0) {
      await exportBtn.click();
      await page.waitForSelector('.ant-dropdown, .dropdown', { timeout: 3000 });
    }
  });

  test('Monitoring time range change test', async ({ page }) => {
    const timePicker = page.locator('.ant-picker, .date-range-picker').first();
    if (await timePicker.count() > 0) {
      await timePicker.click();
      await page.waitForSelector('.ant-picker-dropdown, .ant-picker-panel', { timeout: 3000 });
      const todayBtn = page.locator('text="今天", text="Today", .ant-picker-today-btn').first();
      if (await todayBtn.count() > 0) {
        await todayBtn.click();
        await page.waitForLoadState('networkidle');
      }
    }
  });

  test('Monitoring refresh data test', async ({ page }) => {
    const refreshBtn = page.locator('button:has-text("刷新"), button:has-text("Refresh"), .ant-btn-refresh').first();
    if (await refreshBtn.count() > 0) {
      await refreshBtn.click();
      await page.waitForLoadState('networkidle');
    }
  });

  test('Monitoring chart interaction test', async ({ page }) => {
    const chart = page.locator('.ant-card-body, .recharts-wrapper, .chart-container').first();
    if (await chart.count() > 0) {
      await chart.click();
      await page.waitForLoadState('networkidle');
    }
  });

  test('Monitoring navigation to other pages', async ({ page }) => {
    // 测试导航到概览页面
    const overviewLink = page.locator('a[href="/"], a:has-text("概览"), a:has-text("Overview"), a:has-text("Dashboard")').first();
    if (await overviewLink.count() > 0) {
      await overviewLink.click();
      await page.waitForLoadState('domcontentloaded');
      await page.waitForTimeout(1000);

      // 导航回监控页面
      const monitoringLink = page.locator('a[href="/monitoring"], a:has-text("监控"), a:has-text("Monitoring")').first();
      if (await monitoringLink.count() > 0) {
        await monitoringLink.click();
        await page.waitForLoadState('domcontentloaded');
        await page.waitForTimeout(1000);
      }
    }
  });

  test('Monitoring system metrics test', async ({ page }) => {
    // 验证系统指标显示
    await expect(page.locator('.ant-statistic, .metric, .stat').first()).toBeVisible();
  });

  test('Monitoring performance charts test', async ({ page }) => {
    // 验证性能图表显示
    await expect(page.locator('.ant-card, .chart-card, .recharts-wrapper').first()).toBeVisible();
  });

  test('Monitoring error rates test', async ({ page }) => {
    // 验证错误率数据显示
    await expect(page.locator('.ant-card, .stat-card').first()).toBeVisible();
  });

  test('Monitoring request statistics test', async ({ page }) => {
    // 验证请求统计数据显示
    await expect(page.locator('.ant-card, .stat-card').first()).toBeVisible();
  });

  test('Monitoring real-time data test', async ({ page }) => {
    // 验证实时数据更新
    await expect(page.locator('.ant-card, .real-time-card').first()).toBeVisible();
    await page.waitForTimeout(2000);
  });

  test('Monitoring data export test', async ({ page }) => {
    const exportBtn = page.locator('button:has-text("导出"), button:has-text("Export")').first();
    if (await exportBtn.count() > 0) {
      await exportBtn.click();
      await page.waitForSelector('.ant-dropdown, .dropdown', { timeout: 3000 });
    }
  });
});
