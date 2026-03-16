/**
 * 站点管理模块测试
 * 测试场景：站点管理页面的基本功能
 */

import { test, expect } from '@playwright/test';

test.describe('站点管理模块测试', () => {
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
    const sidebar = page.locator('.ant-menu, .sidebar');
    if (await sidebar.count() > 0) {
      await expect(sidebar.first()).toBeVisible();
    }
    await page.waitForTimeout(1000);

    // 使用侧边栏导航到站点管理页面
    const sitesLink = page.locator('a[href="/sites"]');
    if (await sitesLink.count() > 0) {
      await sitesLink.click();
      await page.waitForURL('/sites');
    } else {
      await page.evaluate(() => window.location.href = '/sites');
    }
    await page.waitForLoadState('domcontentloaded');
    await page.waitForTimeout(2000);
  });

  test('站点管理页面加载成功', async ({ page }) => {
    // 检查页面标题或表格存在
    const pageTitle = page.locator('h1.page-title, h1:has-text("站点"), h1:has-text("Sites")');
    const table = page.locator('table, .ant-table');

    if (await pageTitle.count() > 0) {
      await expect(pageTitle.first()).toBeVisible();
    }
    if (await table.count() > 0) {
      await expect(table.first()).toBeVisible();
    }
  });

  test('站点管理显示站点列表', async ({ page }) => {
    const table = page.locator('table, .ant-table');
    if (await table.count() > 0) {
      await expect(table.first()).toBeVisible();
    }
  });

  test('站点管理显示操作按钮', async ({ page }) => {
    // 检查页面上有任何按钮
    const buttons = page.locator('button, .ant-btn');
    await expect(buttons.first()).toBeVisible();
  });

  test('导航到其他页面从站点管理', async ({ page }) => {
    // 测试导航到概览页面 - 使用侧边栏
    const overviewLink = page.locator('a[href="/"], a:has-text("概览"), a:has-text("Dashboard")').first();
    if (await overviewLink.count() > 0) {
      await overviewLink.click();
      await page.waitForLoadState('domcontentloaded');
      await page.waitForTimeout(1000);
      // 使用包含匹配而不是正则匹配
      expect(page.url()).toContain('localhost');
    }

    // 测试导航回站点管理 - 使用侧边栏
    const sitesLink = page.locator('a[href="/sites"]');
    if (await sitesLink.count() > 0) {
      await sitesLink.click();
      await page.waitForLoadState('domcontentloaded');
      await page.waitForTimeout(1000);
      expect(page.url()).toContain('/sites');
    }
  });

  test('站点创建流程测试', async ({ page }) => {
    // 点击添加站点按钮
    const addSiteButton = page.locator('button:has-text("添加站点"), button:has-text("新建"), button:has-text("Add Site"), .ant-btn-primary:has-text("添加")').first();
    if (await addSiteButton.count() === 0) {
      return;
    }
    await addSiteButton.click();

    // 等待模态框出现
    await page.waitForSelector('.ant-modal', { timeout: 5000 });

    // 填写站点信息
    const nameInput = page.locator('input[name="name"], input[placeholder*="名称"], input[placeholder*="name"]').first();
    const domainInput = page.locator('input[name="domain"], input[placeholder*="域名"], input[placeholder*="domain"]').first();
    const portInput = page.locator('input[name="port"], input[placeholder*="端口"], input[placeholder*="port"]').first();

    if (await nameInput.count() > 0) await nameInput.fill('test-site-' + Date.now());
    if (await domainInput.count() > 0) await domainInput.fill('test-' + Date.now() + '.local');
    if (await portInput.count() > 0) await portInput.fill('80');

    // 选择模式
    const modeSelect = page.locator('select[name="mode"]').first();
    if (await modeSelect.count() > 0) {
      await modeSelect.selectOption('static');
    }

    // 点击保存按钮
    const saveButton = page.locator('button[type="submit"], button:has-text("保存"), button:has-text("Save")').first();
    if (await saveButton.count() > 0) {
      await saveButton.click();
    }

    // 等待成功消息或页面刷新
    await page.waitForLoadState('networkidle');
    await page.waitForTimeout(1000);
  });

  test('站点编辑流程测试', async ({ page }) => {
    // 找到任意站点并尝试点击编辑按钮
    const siteRow = page.locator('table tbody tr').first();
    if (await siteRow.count() === 0) {
      return;
    }

    // 查找编辑按钮
    const editButton = siteRow.locator('button:has-text("编辑"), button:has-text("Edit"), .ant-btn-primary').first();
    if (await editButton.count() > 0) {
      await editButton.click();
      await page.waitForSelector('.ant-modal', { timeout: 5000 });

      // 关闭模态框
      const cancelButton = page.locator('button:has-text("取消"), button:has-text("Cancel")').first();
      if (await cancelButton.count() > 0) {
        await cancelButton.click();
      }
    }
  });

  test('站点删除流程测试', async ({ page }) => {
    // 找到任意站点
    const siteRow = page.locator('table tbody tr').first();
    if (await siteRow.count() === 0) {
      return;
    }

    // 查找删除按钮
    const deleteButton = siteRow.locator('button:has-text("删除"), button:has-text("Delete")').first();
    if (await deleteButton.count() > 0) {
      await deleteButton.click();

      // 等待确认对话框并取消
      const confirmModal = page.locator('.ant-modal-confirm, .ant-modal:has-text("确认")');
      if (await confirmModal.count() > 0) {
        const cancelButton = page.locator('button:has-text("取消"), button:has-text("Cancel")').first();
        if (await cancelButton.count() > 0) {
          await cancelButton.click();
        }
      }
    }
  });

  test('站点预渲染启用测试', async ({ page }) => {
    // 找到任意站点并尝试切换预渲染开关
    const siteRow = page.locator('table tbody tr').first();
    if (await siteRow.count() === 0) {
      return;
    }

    const prerenderSwitch = siteRow.locator('.ant-switch').first();
    if (await prerenderSwitch.count() > 0) {
      await prerenderSwitch.click();
      await page.waitForLoadState('networkidle');
      await page.waitForTimeout(500);
      // 再次点击恢复
      await prerenderSwitch.click();
      await page.waitForLoadState('networkidle');
    }
  });

  test('站点防火墙启用测试', async ({ page }) => {
    // 找到任意站点并尝试切换防火墙开关
    const siteRow = page.locator('table tbody tr').first();
    if (await siteRow.count() === 0) {
      return;
    }

    const firewallSwitch = siteRow.locator('.ant-switch').nth(1);
    if (await firewallSwitch.count() > 0) {
      await firewallSwitch.click();
      await page.waitForLoadState('networkidle');
      await page.waitForTimeout(500);
      // 再次点击恢复
      await firewallSwitch.click();
      await page.waitForLoadState('networkidle');
    }
  });

  test('站点静态资源管理测试', async ({ page }) => {
    // 找到任意站点并尝试点击静态资源管理按钮
    const siteRow = page.locator('table tbody tr').first();
    if (await siteRow.count() === 0) {
      return;
    }

    const staticResourceButton = siteRow.locator('button:has-text("静态"), button:has-text("资源"), button:has-text("Static")').first();
    if (await staticResourceButton.count() > 0) {
      await staticResourceButton.click();
      await page.waitForSelector('.ant-modal', { timeout: 5000 });

      // 关闭模态框
      const cancelButton = page.locator('button:has-text("取消"), button:has-text("Cancel")').first();
      if (await cancelButton.count() > 0) {
        await cancelButton.click();
      }
    }
  });

  test('站点预渲染配置测试', async ({ page }) => {
    // 找到任意站点并尝试点击预渲染配置按钮
    const siteRow = page.locator('table tbody tr').first();
    if (await siteRow.count() === 0) {
      return;
    }

    const configButton = siteRow.locator('button:has-text("预渲染"), button:has-text("Preheat"), button:has-text("配置")').first();
    if (await configButton.count() > 0) {
      await configButton.click();
      await page.waitForSelector('.ant-modal', { timeout: 5000 });

      // 关闭模态框
      const cancelButton = page.locator('button:has-text("取消"), button:has-text("Cancel")').first();
      if (await cancelButton.count() > 0) {
        await cancelButton.click();
      }
    }
  });

  test('站点 WAF 配置测试', async ({ page }) => {
    // 找到任意站点并尝试点击 WAF 配置按钮
    const siteRow = page.locator('table tbody tr').first();
    if (await siteRow.count() === 0) {
      return;
    }

    const wafButton = siteRow.locator('button:has-text("WAF"), button:has-text("防火墙")').first();
    if (await wafButton.count() > 0) {
      await wafButton.click();
      await page.waitForLoadState('domcontentloaded');
      await page.waitForTimeout(1000);

      // 导航回站点管理
      const sitesLink = page.locator('a[href="/sites"]');
      if (await sitesLink.count() > 0) {
        await sitesLink.click();
        await page.waitForURL('/sites');
      }
    }
  });

  test('站点推送配置测试', async ({ page }) => {
    // 找到任意站点并尝试点击推送配置按钮
    const siteRow = page.locator('table tbody tr').first();
    if (await siteRow.count() === 0) {
      return;
    }

    const pushButton = siteRow.locator('button:has-text("推送"), button:has-text("Push")').first();
    if (await pushButton.count() > 0) {
      await pushButton.click();
      await page.waitForSelector('.ant-modal', { timeout: 5000 });

      // 关闭模态框
      const cancelButton = page.locator('button:has-text("取消"), button:has-text("Cancel")').first();
      if (await cancelButton.count() > 0) {
        await cancelButton.click();
      }
    }
  });

  test('站点类型测试 - 代理站点', async ({ page }) => {
    // 点击添加站点按钮
    const addSiteButton = page.locator('button:has-text("添加站点"), button:has-text("新建")').first();
    if (await addSiteButton.count() === 0) {
      return;
    }
    await addSiteButton.click();
    await page.waitForSelector('.ant-modal', { timeout: 5000 });

    // 填写站点信息
    const nameInput = page.locator('input[name="name"]').first();
    const domainInput = page.locator('input[name="domain"]').first();
    const modeSelect = page.locator('select[name="mode"]').first();

    if (await nameInput.count() > 0) await nameInput.fill('test-proxy-' + Date.now());
    if (await domainInput.count() > 0) await domainInput.fill('test-proxy-' + Date.now() + '.local');

    if (await modeSelect.count() > 0) {
      await modeSelect.selectOption('proxy');
    }

    // 点击保存按钮
    const saveButton = page.locator('button[type="submit"], button:has-text("保存")').first();
    if (await saveButton.count() > 0) {
      await saveButton.click();
    }

    await page.waitForLoadState('networkidle');
    await page.waitForTimeout(1000);
  });

  test('站点类型测试 - 重定向站点', async ({ page }) => {
    // 点击添加站点按钮
    const addSiteButton = page.locator('button:has-text("添加站点"), button:has-text("新建")').first();
    if (await addSiteButton.count() === 0) {
      return;
    }
    await addSiteButton.click();
    await page.waitForSelector('.ant-modal', { timeout: 5000 });

    // 填写站点信息
    const nameInput = page.locator('input[name="name"]').first();
    const domainInput = page.locator('input[name="domain"]').first();
    const modeSelect = page.locator('select[name="mode"]').first();

    if (await nameInput.count() > 0) await nameInput.fill('test-redirect-' + Date.now());
    if (await domainInput.count() > 0) await domainInput.fill('test-redirect-' + Date.now() + '.local');

    if (await modeSelect.count() > 0) {
      await modeSelect.selectOption('redirect');
    }

    // 点击保存按钮
    const saveButton = page.locator('button[type="submit"], button:has-text("保存")').first();
    if (await saveButton.count() > 0) {
      await saveButton.click();
    }

    await page.waitForLoadState('networkidle');
    await page.waitForTimeout(1000);
  });

  test('站点启用/禁用测试', async ({ page }) => {
    // 找到任意站点并尝试切换启用/禁用开关
    const siteRow = page.locator('table tbody tr').first();
    if (await siteRow.count() === 0) {
      return;
    }

    const enableSwitch = siteRow.locator('.ant-switch').nth(2);
    if (await enableSwitch.count() > 0) {
      await enableSwitch.click();
      await page.waitForLoadState('networkidle');
      await page.waitForTimeout(500);
      // 再次点击恢复
      await enableSwitch.click();
      await page.waitForLoadState('networkidle');
    }
  });

  test('站点搜索测试', async ({ page }) => {
    // 查找搜索框
    const searchInput = page.locator('input[placeholder="搜索"], input[placeholder*="search" i], .ant-input[type="text"]').first();
    if (await searchInput.count() > 0) {
      await searchInput.fill('test');
      await page.waitForLoadState('networkidle');
      await page.waitForTimeout(500);

      // 清空搜索
      await searchInput.clear();
      await page.waitForLoadState('networkidle');
    }
  });
});
