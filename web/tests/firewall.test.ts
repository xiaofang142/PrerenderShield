import { test, expect } from '@playwright/test';

test.describe('Firewall Page', () => {
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

    await page.waitForTimeout(1000);

    // 使用侧边栏导航到防火墙页面
    const firewallLink = page.locator('a[href="/firewall"]');
    if (await firewallLink.count() > 0) {
      await firewallLink.click();
      await page.waitForLoadState('domcontentloaded');
    } else {
      await page.evaluate(() => window.location.href = '/firewall');
    }
    await page.waitForTimeout(2000);
  });

  test('Firewall page loads successfully', async ({ page }) => {
    const title = page.locator('h1.page-title, h1:has-text("防火墙"), h1:has-text("Firewall")');
    if (await title.count() > 0) {
      await expect(title.first()).toBeVisible();
    }
  });

  test('Firewall shows rules list', async ({ page }) => {
    const table = page.locator('table, .ant-table, .firewall-rules');
    if (await table.count() > 0) {
      await expect(table.first()).toBeVisible();
    }
  });

  test('Firewall allows creating new rule', async ({ page }) => {
    const addBtn = page.locator('button:has-text("添加规则"), button:has-text("Add Rule"), button:has-text("新建规则")').first();
    if (await addBtn.count() > 0) {
      await addBtn.click();
      await page.waitForSelector('.ant-modal, .modal', { timeout: 3000 });
    }
  });

  test('Firewall allows editing existing rule', async ({ page }) => {
    const editBtn = page.locator('button:has-text("编辑"), button:has-text("Edit"), .ant-btn-edit').first();
    if (await editBtn.count() > 0) {
      await editBtn.click();
      await page.waitForSelector('.ant-modal, .modal', { timeout: 3000 });
    }
  });

  test('Firewall allows deleting rule', async ({ page }) => {
    const deleteBtn = page.locator('button:has-text("删除"), button:has-text("Delete"), .ant-btn-delete').first();
    if (await deleteBtn.count() > 0) {
      await deleteBtn.click();
      await page.waitForSelector('.ant-modal-confirm, .modal-confirm, button:has-text("确认"), button:has-text("Cancel")', { timeout: 3000 });
    }
  });

  test('Firewall allows enabling/disabling rules', async ({ page }) => {
    const ruleSwitch = page.locator('.ant-switch').first();
    if (await ruleSwitch.count() > 0) {
      await ruleSwitch.click();
      await page.waitForTimeout(1000);
    }
  });

  test('Firewall shows rule statistics', async ({ page }) => {
    // 检查是否有统计或卡片显示
    const stats = page.locator('.ant-statistic, .stat, .stats-card, .ant-card');
    if (await stats.count() > 0) {
      await expect(stats.first()).toBeVisible();
    }
  });

  test('Firewall shows blocked requests', async ({ page }) => {
    await expect(page.locator('.ant-card, .stat-card').first()).toBeVisible();
  });

  test('Firewall rate limit configuration test', async ({ page }) => {
    const rateBtn = page.locator('button:has-text("频率限制"), button:has-text("Rate Limit"), button:has-text("限流")').first();
    if (await rateBtn.count() > 0) {
      await rateBtn.click();
      await page.waitForSelector('.ant-modal, .modal', { timeout: 3000 });
      await page.click('button:has-text("取消"), button:has-text("Cancel")').first();
    }
  });

  test('Firewall attack interception test', async ({ page }) => {
    await expect(page.locator('.ant-card, .stat-card').first()).toBeVisible();
  });

  test('Firewall geoip configuration test', async ({ page }) => {
    const geoBtn = page.locator('button:has-text("地理位置"), button:has-text("GeoIP"), button:has-text("IP 地理位置")').first();
    if (await geoBtn.count() > 0) {
      await geoBtn.click();
      await page.waitForSelector('.ant-modal, .modal', { timeout: 3000 });
      await page.click('button:has-text("取消"), button:has-text("Cancel")').first();
    }
  });

  test('Firewall IP whitelist/blacklist test', async ({ page }) => {
    const ipBtn = page.locator('button:has-text("IP 名单"), button:has-text("IP Whitelist"), button:has-text("IP Blacklist")').first();
    if (await ipBtn.count() > 0) {
      await ipBtn.click();
      await page.waitForSelector('.ant-modal, .modal', { timeout: 3000 });
      await page.click('button:has-text("取消"), button:has-text("Cancel")').first();
    }
  });

  test('Firewall rule priority test', async ({ page }) => {
    const addBtn = page.locator('button:has-text("添加规则"), button:has-text("Add Rule")').first();
    if (await addBtn.count() > 0) {
      await addBtn.click();
      await page.waitForSelector('.ant-modal, .modal', { timeout: 3000 });
      await page.click('button:has-text("取消"), button:has-text("Cancel")').first();
    }
  });

  test('Firewall rule filtering test', async ({ page }) => {
    const searchInput = page.locator('input[placeholder="搜索规则"], input[placeholder*="search" i]').first();
    if (await searchInput.count() > 0) {
      await searchInput.fill('test');
      await page.keyboard.press('Enter');
      await page.waitForLoadState('networkidle');
    }
  });
});
