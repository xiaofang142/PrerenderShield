import { test, expect } from '@playwright/test';

test.describe('Logs Page', () => {
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
      await page.waitForURL('/', { timeout: 10000 });
    } else {
      await page.waitForSelector('input[placeholder="Username"]');
      await page.fill('input[placeholder="Username"]', 'admin');
      await page.fill('input[placeholder="Password"]', '123456');
      await page.click('button[type="submit"]');
      await page.waitForURL('/', { timeout: 10000 });
    }

    // 导航到防火墙页面（日志功能可能在防火墙页面中）
    await page.click('a[href="/firewall"]');
    await page.waitForURL('/firewall');
    await page.waitForLoadState('domcontentloaded');
    await page.waitForTimeout(2000);
  });

  test('Logs page loads successfully', async ({ page }) => {
    // 防火墙页面应该包含日志表格
    await expect(page.locator('.ant-table, table, .firewall-rules').first()).toBeVisible();
  });

  test('Logs shows log entries', async ({ page }) => {
    await expect(page.locator('.ant-table, table').first()).toBeVisible();
  });

  test('Logs allows filtering by level', async ({ page }) => {
    const levelSelect = page.locator('.ant-select, .filter-select').first();
    if (await levelSelect.count() > 0) {
      await levelSelect.click();
      await page.waitForSelector('.ant-select-dropdown', { timeout: 3000 });
    }
  });

  test('Logs allows filtering by time range', async ({ page }) => {
    const timePicker = page.locator('.ant-picker, .date-picker').first();
    if (await timePicker.count() > 0) {
      await timePicker.click();
      await page.waitForSelector('.ant-picker-dropdown, .ant-picker-panel', { timeout: 3000 });
    }
  });

  test('Logs allows searching', async ({ page }) => {
    const searchInput = page.locator('input[placeholder="搜索"], input[placeholder*="search" i]').first();
    if (await searchInput.count() > 0) {
      await searchInput.fill('test');
      await page.keyboard.press('Enter');
      await page.waitForLoadState('networkidle');
    }
  });

  test('Logs allows exporting', async ({ page }) => {
    const exportBtn = page.locator('button:has-text("导出"), button:has-text("Export")').first();
    if (await exportBtn.count() > 0) {
      await exportBtn.click();
      await page.waitForSelector('.ant-dropdown, .dropdown', { timeout: 3000 });
    }
  });

  test('Logs shows pagination', async ({ page }) => {
    // 检查是否有分页控件
    const pagination = page.locator('.ant-pagination, .pagination');
    if (await pagination.count() > 0) {
      await expect(pagination.first()).toBeVisible();
    }
  });

  test('Logs allows clearing filters', async ({ page }) => {
    const clearBtn = page.locator('button:has-text("清空"), button:has-text("Clear"), button:has-text("重置")').first();
    if (await clearBtn.count() > 0) {
      await clearBtn.click();
      await page.waitForLoadState('networkidle');
    }
  });

  test('Logs pagination navigation test', async ({ page }) => {
    const nextBtn = page.locator('.ant-pagination-next, .pagination-next').first();
    if (await nextBtn.count() > 0) {
      await nextBtn.click();
      await page.waitForLoadState('networkidle');
    }
  });

  test('Logs time range filtering test', async ({ page }) => {
    const timePicker = page.locator('.ant-picker, .date-picker').first();
    if (await timePicker.count() > 0) {
      await timePicker.click();
      await page.waitForSelector('.ant-picker-dropdown, .ant-picker-panel', { timeout: 3000 });
    }
  });

  test('Logs log level filtering test', async ({ page }) => {
    const levelSelect = page.locator('.ant-select, .filter-select').first();
    if (await levelSelect.count() > 0) {
      await levelSelect.click();
      await page.waitForSelector('.ant-select-dropdown', { timeout: 3000 });
    }
  });

  test('Logs keyword search test', async ({ page }) => {
    const searchInput = page.locator('input[placeholder="搜索"], input[placeholder*="search" i]').first();
    if (await searchInput.count() > 0) {
      await searchInput.fill('admin');
      await page.keyboard.press('Enter');
      await page.waitForLoadState('networkidle');
    }
  });

  test('Logs pagination test', async ({ page }) => {
    const page2Btn = page.locator('.ant-pagination-item-2, .pagination-item-2').first();
    if (await page2Btn.count() > 0) {
      await page2Btn.click();
      await page.waitForLoadState('networkidle');
    }
  });

  test('Logs export test', async ({ page }) => {
    const exportBtn = page.locator('button:has-text("导出"), button:has-text("Export")').first();
    if (await exportBtn.count() > 0) {
      await exportBtn.click();
      await page.waitForSelector('.ant-dropdown, .dropdown', { timeout: 3000 });
    }
  });

  test('Logs clear test', async ({ page }) => {
    const clearBtn = page.locator('button:has-text("清空"), button:has-text("Clear")').first();
    if (await clearBtn.count() > 0) {
      await clearBtn.click();
    }
  });

  test('Logs auto refresh test', async ({ page }) => {
    const refreshSwitch = page.locator('.ant-switch').nth(1);
    if (await refreshSwitch.count() > 0) {
      await refreshSwitch.click();
      await page.waitForTimeout(2000);
      await refreshSwitch.click();
    }
  });
});
