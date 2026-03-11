import { test, expect } from '@playwright/test';

test.describe('Logs Page', () => {
  test.beforeEach(async ({ page }) => {
    // 登录
    await page.goto('/login');
    // 等待页面完全加载
    await page.waitForLoadState('networkidle');
    
    // 检查是否显示系统初始化向导
    if (await page.locator('h1:has-text("系统初始化向导")').count() > 0) {
      // 处理系统初始化向导
      // 点击同意使用声明复选框
      await page.click('input[type="checkbox"]');
      
      // 点击下一步按钮
      await page.click('button:has-text("下一步")');
      
      // 等待设置管理员页面
      await page.waitForSelector('input[name="username"]');
      
      // 填写管理员信息
      await page.fill('input[name="username"]', 'admin');
      await page.fill('input[name="password"]', '123456');
      await page.fill('input[name="confirmPassword"]', '123456');
      await page.fill('input[name="email"]', 'admin@example.com');
      await page.fill('input[name="company"]', 'Test Company');
      
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
      await page.waitForSelector('form[name="login"]');
      // 填写登录表单
      await page.fill('form[name="login"] input[name="username"]', 'admin');
      await page.fill('form[name="login"] input[name="password"]', '123456');
      // 点击登录按钮
      await page.click('form[name="login"] button[type="submit"]');
      // 等待导航到首页
      await page.waitForURL('/');
    }
    
    // 直接导航到日志页面
    await page.goto('/logs');
    await page.waitForURL('/logs');
  });

  test('Logs page loads successfully', async ({ page }) => {
    await expect(page.locator('h1.page-title')).toContainText('日志');
  });

  test('Logs shows log entries', async ({ page }) => {
    await expect(page.locator('table')).toBeVisible();
    await expect(page.locator('table thead')).toBeVisible();
    await expect(page.locator('table tbody')).toBeVisible();
  });

  test('Logs allows filtering by level', async ({ page }) => {
    // 点击级别过滤下拉框
    await page.click('.ant-select:has-text("全部级别")');
    
    // 等待下拉选项出现
    await page.waitForSelector('.ant-select-dropdown');
    
    // 选择ERROR级别
    await page.click('text="ERROR"');
    
    // 等待数据更新
    await page.waitForLoadState('networkidle');
  });

  test('Logs allows filtering by time range', async ({ page }) => {
    // 点击时间范围选择器
    await page.click('.ant-picker');
    
    // 等待时间选择器出现
    await page.waitForSelector('.ant-picker-dropdown');
    
    // 选择时间范围
    await page.click('text="今天"');
    
    // 等待数据更新
    await page.waitForLoadState('networkidle');
  });

  test('Logs allows searching', async ({ page }) => {
    // 填写搜索关键词
    await page.fill('input[placeholder="搜索关键词"]', 'test');
    
    // 点击搜索按钮
    await page.click('button:has-icon("SearchOutlined")');
    
    // 等待数据更新
    await page.waitForLoadState('networkidle');
  });

  test('Logs allows exporting', async ({ page }) => {
    // 点击导出按钮
    await page.click('button:has-text("导出")');
    await expect(page.locator('.ant-dropdown')).toBeVisible();
    
    // 选择导出格式
    await page.click('text="导出为CSV"');
    
    // 等待导出完成
    await page.waitForLoadState('networkidle');
  });

  test('Logs shows pagination', async ({ page }) => {
    await expect(page.locator('.ant-pagination')).toBeVisible();
  });

  test('Logs allows clearing filters', async ({ page }) => {
    // 点击清空过滤按钮
    await page.click('button:has-text("清空过滤")');
    
    // 等待数据更新
    await page.waitForLoadState('networkidle');
    await expect(page.locator('table')).toBeVisible();
  });

  test('Logs pagination navigation test', async ({ page }) => {
    // 点击下一页
    await page.click('.ant-pagination-next');
    
    // 等待数据更新
    await page.waitForLoadState('networkidle');
    
    // 点击上一页
    await page.click('.ant-pagination-prev');
    
    // 等待数据更新
    await page.waitForLoadState('networkidle');
  });



  test('Logs time range filtering test', async ({ page }) => {
    // 测试不同时间范围的过滤
    const timeRanges = ['今天', '昨天', '最近7天', '最近30天'];
    
    for (const range of timeRanges) {
      // 点击时间范围选择器
      await page.click('.ant-picker');
      await page.waitForSelector('.ant-picker-dropdown');
      await page.click(`text="${range}"`);
      await page.waitForLoadState('networkidle');
    }
  });

  test('Logs log level filtering test', async ({ page }) => {
    // 测试不同级别的过滤
    const levels = ['INFO', 'WARN', 'ERROR', 'DEBUG'];
    
    for (const level of levels) {
      // 点击级别过滤下拉框
      await page.click('.ant-select:has-text("全部级别")');
      await page.waitForSelector('.ant-select-dropdown');
      await page.click(`text="${level}"`);
      await page.waitForLoadState('networkidle');
    }
  });

  test('Logs keyword search test', async ({ page }) => {
    // 测试不同关键词的搜索
    const keywords = ['admin', 'login', 'error', 'success'];
    
    for (const keyword of keywords) {
      // 填写搜索关键词
      await page.fill('input[placeholder="搜索关键词"]', keyword);
      await page.keyboard.press('Enter');
      await page.waitForLoadState('networkidle');
    }
    
    // 清空搜索
    await page.fill('input[placeholder="搜索关键词"]', '');
    await page.keyboard.press('Enter');
    await page.waitForLoadState('networkidle');
  });

  test('Logs pagination test', async ({ page }) => {
    // 测试分页导航
    await page.click('.ant-pagination-next');
    await page.waitForLoadState('networkidle');
    
    await page.click('.ant-pagination-prev');
    await page.waitForLoadState('networkidle');
    
    // 测试跳转到指定页
    await page.click('.ant-pagination-item-2');
    await page.waitForLoadState('networkidle');
  });

  test('Logs export test', async ({ page }) => {
    // 测试导出为CSV
    await page.click('button:has-text("导出")');
    await page.click('text="导出为CSV"');
    await page.waitForLoadState('networkidle');
    
    // 测试导出为Excel
    await page.click('button:has-text("导出")');
    await page.click('text="导出为Excel"');
    await page.waitForLoadState('networkidle');
    
    // 测试导出为JSON
    await page.click('button:has-text("导出")');
    await page.click('text="导出为JSON"');
    await page.waitForLoadState('networkidle');
  });

  test('Logs clear test', async ({ page }) => {
    // 测试清空日志
    await page.click('button:has-text("清空日志")');
    await page.waitForSelector('.ant-modal-confirm');
    await page.click('button[type="danger"]:has-text("确定")');
    await page.waitForSelector('.ant-message-success');
    await expect(page.locator('.ant-message-success')).toBeVisible();
  });

  test('Logs auto refresh test', async ({ page }) => {
    // 测试自动刷新功能
    const autoRefreshSwitch = page.locator('.ant-switch');
    await autoRefreshSwitch.click();
    await page.waitForTimeout(2000);
    await autoRefreshSwitch.click();
  });
});
