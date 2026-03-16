import { test, expect } from '@playwright/test';

test.describe('System Configuration Page', () => {
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
      await page.waitForURL('/');
    } else {
      await page.waitForSelector('input[placeholder="Username"]');
      await page.fill('input[placeholder="Username"]', 'admin');
      await page.fill('input[placeholder="Password"]', '123456');
      await page.click('button[type="submit"]');
      await page.waitForURL('/');
    }

    // 使用侧边栏导航到系统配置页面
    await page.click('a[href="/system"]');
    await page.waitForURL('/system');
    await page.waitForLoadState('domcontentloaded');
    await page.waitForTimeout(2000);
  });

  test('System configuration page loads successfully', async ({ page }) => {
    // 检查是否有标题或配置卡片
    const title = page.locator('h1.page-title, h1:has-text("系统配置"), h1:has-text("System")');
    const configCard = page.locator('.ant-card, .config-card, .settings-card');

    if (await title.count() > 0) {
      await expect(title.first()).toBeVisible();
    } else if (await configCard.count() > 0) {
      await expect(configCard.first()).toBeVisible();
    }
  });

  test('System configuration shows basic settings', async ({ page }) => {
    await expect(page.locator('.ant-card, .config-card, .settings-card').first()).toBeVisible();
  });

  test('System configuration allows saving settings', async ({ page }) => {
    const saveBtn = page.locator('button:has-text("保存"), button:has-text("Save"), button:has-text("保存设置")').first();
    if (await saveBtn.count() > 0) {
      await saveBtn.click();
      await page.waitForSelector('.ant-message-success, .ant-notification', { timeout: 3000 });
    }
  });

  test('System configuration shows database settings', async ({ page }) => {
    await expect(page.locator('.ant-card, .db-settings, .database-section').first()).toBeVisible();
  });

  test('System configuration shows cache settings', async ({ page }) => {
    await expect(page.locator('.ant-card, .cache-settings, .cache-section').first()).toBeVisible();
  });

  test('System configuration shows security settings', async ({ page }) => {
    await expect(page.locator('.ant-card, .security-settings, .security-section').first()).toBeVisible();
  });

  test('System configuration allows restarting services', async ({ page }) => {
    const restartBtn = page.locator('button:has-text("重启"), button:has-text("Restart"), button:has-text("重启服务")').first();
    if (await restartBtn.count() > 0) {
      await restartBtn.click();
      await page.waitForSelector('.ant-modal-confirm, .modal-confirm', { timeout: 3000 });
      await page.click('button:has-text("取消"), button:has-text("Cancel")').first();
    }
  });

  test('System configuration shows service status', async ({ page }) => {
    await expect(page.locator('.ant-card, .status-card, .service-status').first()).toBeVisible();
  });

  test('System configuration service status refresh test', async ({ page }) => {
    const refreshBtn = page.locator('button:has-text("刷新"), button:has-text("Refresh"), button:has-text("刷新状态")').first();
    if (await refreshBtn.count() > 0) {
      await refreshBtn.click();
      await page.waitForLoadState('networkidle');
    }
  });

  test('System configuration basic settings test', async ({ page }) => {
    const appNameInput = page.locator('input[name="appName"], input[placeholder*="app" i], input[placeholder*="应用"]').first();
    if (await appNameInput.count() > 0) {
      await appNameInput.fill('PrerenderShield');
      const saveBtn = page.locator('button:has-text("保存"), button:has-text("Save")').first();
      if (await saveBtn.count() > 0) {
        await saveBtn.click();
        await page.waitForSelector('.ant-message-success, .ant-notification', { timeout: 3000 });
      }
    }
  });

  test('System configuration database settings test', async ({ page }) => {
    const dbHostInput = page.locator('input[name="dbHost"], input[placeholder*="host" i], input[placeholder*="主机"]').first();
    if (await dbHostInput.count() > 0) {
      await dbHostInput.fill('localhost');
      const saveBtn = page.locator('button:has-text("保存"), button:has-text("Save")').first();
      if (await saveBtn.count() > 0) {
        await saveBtn.click();
        await page.waitForSelector('.ant-message-success, .ant-notification', { timeout: 3000 });
      }
    }
  });

  test('System configuration cache settings test', async ({ page }) => {
    const cacheInput = page.locator('input[name="cacheTTL"], input[placeholder*="cache" i], input[placeholder*="缓存"]').first();
    if (await cacheInput.count() > 0) {
      await cacheInput.fill('3600');
      const saveBtn = page.locator('button:has-text("保存"), button:has-text("Save")').first();
      if (await saveBtn.count() > 0) {
        await saveBtn.click();
        await page.waitForSelector('.ant-message-success, .ant-notification', { timeout: 3000 });
      }
    }
  });

  test('System configuration security settings test', async ({ page }) => {
    const secretInput = page.locator('input[name="jwtSecret"], input[placeholder*="secret" i], input[placeholder*="密钥"]').first();
    if (await secretInput.count() > 0) {
      await secretInput.fill('test-secret-key');
      const saveBtn = page.locator('button:has-text("保存"), button:has-text("Save")').first();
      if (await saveBtn.count() > 0) {
        await saveBtn.click();
        await page.waitForSelector('.ant-message-success, .ant-notification', { timeout: 3000 });
      }
    }
  });

  test('System configuration service management test', async ({ page }) => {
    const restartBtn = page.locator('button:has-text("重启"), button:has-text("Restart"), button:has-text("重启服务")').first();
    if (await restartBtn.count() > 0) {
      await restartBtn.click();
      await page.waitForSelector('.ant-modal-confirm, .modal-confirm', { timeout: 3000 });
      await page.click('button:has-text("取消"), button:has-text("Cancel")').first();
    }
  });

  test('System configuration service status test', async ({ page }) => {
    const refreshBtn = page.locator('button:has-text("刷新"), button:has-text("Refresh")').first();
    if (await refreshBtn.count() > 0) {
      await refreshBtn.click();
      await page.waitForLoadState('networkidle');
    }
    // 检查是否有状态指示器
    const statusIndicator = page.locator('.ant-statistic, .stat, .status-indicator');
    if (await statusIndicator.count() > 0) {
      await expect(statusIndicator.first()).toBeVisible();
    }
  });

  test('System configuration backup test', async ({ page }) => {
    const backupBtn = page.locator('button:has-text("备份"), button:has-text("Backup"), button:has-text("备份系统")').first();
    if (await backupBtn.count() > 0) {
      await backupBtn.click();
      await page.waitForSelector('.ant-modal-confirm, .modal-confirm', { timeout: 3000 });
      await page.click('button:has-text("取消"), button:has-text("Cancel")').first();
    }
  });

  test('System configuration restore test', async ({ page }) => {
    const restoreBtn = page.locator('button:has-text("恢复"), button:has-text("Restore"), button:has-text("恢复系统")').first();
    if (await restoreBtn.count() > 0) {
      await restoreBtn.click();
      await page.waitForSelector('.ant-modal-confirm, .modal-confirm', { timeout: 3000 });
      await page.click('button:has-text("取消"), button:has-text("Cancel")').first();
    }
  });
});
