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

      // 等待页面跳转
      await page.waitForURL('/');
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
    }

    // 导航到爬虫页面
    await page.goto('/crawler');
    await page.waitForURL('/crawler');
  });

  test('Crawler page loads successfully', async ({ page }) => {
    await expect(page).toHaveTitle(/Crawler/);
    await expect(page.locator('h1')).toContainText('Crawler');
  });

  test('Crawler shows configuration options', async ({ page }) => {
    await expect(page.locator('.crawler-config')).toBeVisible();
  });

  test('Crawler allows saving configuration', async ({ page }) => {
    await page.click('button:has-text("Save Configuration")');
    await expect(page.locator('.ant-message-success')).toBeVisible();
  });

  test('Crawler shows crawling status', async ({ page }) => {
    await expect(page.locator('.crawling-status')).toBeVisible();
  });

  test('Crawler allows starting crawl', async ({ page }) => {
    await page.click('button:has-text("Start Crawl")');
    await expect(page.locator('.ant-message-success')).toBeVisible();
  });

  test('Crawler allows stopping crawl', async ({ page }) => {
    await page.click('button:has-text("Stop Crawl")');
    await expect(page.locator('.ant-message-success')).toBeVisible();
  });

  test('Crawler shows crawl history', async ({ page }) => {
    await expect(page.locator('.crawl-history')).toBeVisible();
  });

  test('Crawler shows crawl statistics', async ({ page }) => {
    await expect(page.locator('.crawl-statistics')).toBeVisible();
  });

  test('Crawler configuration test', async ({ page }) => {
    // 测试爬虫配置更新
    await page.fill('input[name="crawlDepth"]', '3');
    await page.fill('input[name="crawlDelay"]', '1000');
    await page.fill('input[name="maxConcurrent"]', '5');
    await page.fill('input[name="userAgent"]', 'Mozilla/5.0 (compatible; PrerenderShield Crawler)');
    await page.click('button:has-text("Save Configuration")');
    await page.waitForSelector('.ant-message-success');
    await expect(page.locator('.ant-message-success')).toBeVisible();
  });

  test('Crawler start/stop test', async ({ page }) => {
    // 测试启动爬虫
    await page.click('button:has-text("Start Crawl")');
    await page.waitForSelector('.ant-message-success');
    await expect(page.locator('.ant-message-success')).toBeVisible();
    
    // 测试停止爬虫
    await page.click('button:has-text("Stop Crawl")');
    await page.waitForSelector('.ant-message-success');
    await expect(page.locator('.ant-message-success')).toBeVisible();
  });

  test('Crawler history test', async ({ page }) => {
    // 验证爬取历史显示
    await expect(page.locator('.crawl-history')).toBeVisible();
    await expect(page.locator('table')).toBeVisible();
  });

  test('Crawler statistics test', async ({ page }) => {
    // 验证爬取统计显示
    await expect(page.locator('.crawl-statistics')).toBeVisible();
    await expect(page.locator('.ant-statistic')).toBeVisible();
  });

  test('Crawler site selection test', async ({ page }) => {
    // 测试站点选择
    await page.click('.ant-select:has-text("Select Site")');
    await page.waitForSelector('.ant-select-dropdown');
    await page.click('text="All Sites"');
    await page.waitForLoadState('networkidle');
  });

  test('Crawler schedule test', async ({ page }) => {
    // 测试爬虫调度设置
    await page.click('button:has-text("Schedule Crawl")');
    await page.waitForSelector('.ant-modal');
    await page.fill('input[name="schedule"]', '0 0 * * *'); // 每天午夜执行
    await page.click('button[type="primary"]:has-text("Save")');
    await page.waitForSelector('.ant-message-success');
    await expect(page.locator('.ant-message-success')).toBeVisible();
  });
});
