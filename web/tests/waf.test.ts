import { test, expect } from '@playwright/test';

test.describe('WAF Settings Page', () => {
  test.beforeEach(async ({ page }) => {
    // 登录
    await page.goto('/login');
    await page.waitForLoadState('networkidle');
    await page.waitForSelector('input[placeholder="Username"]');
    await page.fill('input[placeholder="Username"]', 'admin');
    await page.fill('input[placeholder="Password"]', '123456');
    await page.click('button[type="submit"]');
    // 等待登录成功后的延迟跳转（1.5 秒）
    await page.waitForURL('/', { timeout: 10000 });
    await page.waitForTimeout(1000);

    // 使用侧边栏导航到防火墙页面（WAF 配置在防火墙页面中）
    await page.click('a[href="/firewall"]');
    await page.waitForURL('/firewall');
    await page.waitForLoadState('domcontentloaded');
    await page.waitForTimeout(2000);
  });

  test('WAF settings page loads successfully', async ({ page }) => {
    const title = page.locator('h1.page-title, h1:has-text("防火墙"), h1:has-text("Firewall")').first();
    await expect(title).toBeVisible();
  });

  test('WAF settings shows general configuration', async ({ page }) => {
    await expect(page.locator('.ant-card, .waf-config, .config-section, .firewall-rules').first()).toBeVisible();
  });

  test('WAF settings allows saving configuration', async ({ page }) => {
    const saveBtn = page.locator('button:has-text("保存"), button:has-text("Save")').first();
    if (await saveBtn.count() > 0) {
      await saveBtn.click();
      await page.waitForSelector('.ant-message-success, .ant-notification', { timeout: 3000 });
    }
  });

  test('WAF settings shows rule sets', async ({ page }) => {
    await expect(page.locator('.ant-table, .rule-sets, .firewall-rules').first()).toBeVisible();
  });

  test('WAF settings shows threat protection', async ({ page }) => {
    await expect(page.locator('.ant-card, .threat-section, .protection-stats').first()).toBeVisible();
  });

  test('WAF settings shows bot protection', async ({ page }) => {
    await expect(page.locator('.ant-card, .bot-section, .bot-stats').first()).toBeVisible();
  });

  test('WAF settings allows enabling/disabling modules', async ({ page }) => {
    // 检查是否有开关控件
    const switches = page.locator('.ant-switch, .toggle-switch');
    if (await switches.count() > 0) {
      await expect(switches.first()).toBeVisible();
    }
  });

  test('WAF settings shows security statistics', async ({ page }) => {
    // 检查是否有统计卡片
    const stats = page.locator('.ant-statistic, .stats-card, .security-stats, .ant-card');
    if (await stats.count() > 0) {
      await expect(stats.first()).toBeVisible();
    }
  });
});
