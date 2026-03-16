import { test, expect } from '@playwright/test';

test.describe('Crawler Page', () => {
  test.beforeEach(async ({ page }) => {
    // 登录
    await page.goto('/login');
    // 等待页面完全加载
    await page.waitForLoadState('domcontentloaded');
    await page.waitForTimeout(2000);

    // 检查是否显示系统初始化向导
    const firstRunTitle = page.locator('h1:has-text("系统初始化向导"), h2:has-text("系统初始化向导"), div:has-text("系统初始化向导")');
    if (await firstRunTitle.count() > 0) {
      // 处理系统初始化向导
      // 点击同意使用声明复选框
      await page.click('input[type="checkbox"]');

      // 点击下一步按钮
      await page.click('button:has-text("下一步")');

      // 等待设置管理员页面
      await page.waitForSelector('input[placeholder="Username"]');

      // 填写管理员信息
      await page.fill('input[placeholder="Username"]', 'admin');
      await page.fill('input[placeholder="Password"]', '123456');
      await page.fill('input[placeholder="Confirm Password"]', '123456');
      await page.fill('input[placeholder="Email"]', 'admin@example.com');
      await page.fill('input[placeholder="Company"]', 'Test Company');

      // 点击下一步按钮
      await page.click('button:has-text("下一步")');

      // 等待完成页面
      await page.waitForSelector('button:has-text("完成")');

      // 点击完成按钮
      await page.click('button:has-text("完成")');

      // 等待页面跳转到首页
      await page.waitForURL('/');
      // 等待首页元素出现
      await page.waitForSelector('h1.page-title, h1:has-text("概览")', { timeout: 5000 });
    } else {
      // 等待登录表单出现
      await page.waitForSelector('input[placeholder="Username"]');
      // 填写登录表单
      await page.fill('input[placeholder="Username"]', 'admin');
      await page.fill('input[placeholder="Password"]', '123456');
      // 点击登录按钮
      await page.click('button[type="submit"]');
      // 等待导航到首页
      await page.waitForURL('/');
      // 等待首页元素出现
      await page.waitForSelector('h1.page-title, h1:has-text("概览")', { timeout: 5000 });
    }

    // 等待侧边栏加载完成
    await page.waitForTimeout(1000);
    // 点击侧边栏导航到爬虫页面（通过 URL 匹配）
    await page.click('a[href="/crawler"]');
    await page.waitForTimeout(2000);
  });

  test('Crawler page loads successfully', async ({ page }) => {
    await expect(page.locator('h1.page-title')).toBeVisible();
    await expect(page.locator('h1.page-title')).toContainText('爬虫');
  });

  test('Crawler shows site selector', async ({ page }) => {
    await expect(page.locator('.ant-select')).toBeVisible();
  });

  test('Crawler shows logs table', async ({ page }) => {
    // Click the "Access Logs" tab first
    await page.click('text="访问记录"');
    await page.waitForTimeout(1000);
    await expect(page.locator('.ant-table').first()).toBeVisible();
  });

  test('Crawler shows statistics', async ({ page }) => {
    await expect(page.locator('.ant-statistic').first()).toBeVisible();
  });

  test('Crawler granularity selector test', async ({ page }) => {
    // 测试粒度选择器
    await expect(page.locator('.ant-radio-group')).toBeVisible();
  });

  test('Crawler site selection test', async ({ page }) => {
    // 测试站点选择
    await expect(page.locator('.ant-select')).toBeVisible();
  });
});
