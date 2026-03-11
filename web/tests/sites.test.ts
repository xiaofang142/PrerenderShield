import { test, expect } from '@playwright/test';

test.describe('站点管理模块测试', () => {
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
      await page.waitForURL('/');
    }
    
    // 直接导航到站点管理页面
    await page.goto('/sites');
    await page.waitForURL('/sites');
  });

  test('站点管理页面加载成功', async ({ page }) => {
    await expect(page.locator('h1.page-title')).toContainText('站点管理');
    await expect(page.locator('table')).toBeVisible();
  });

  test('站点管理显示站点列表', async ({ page }) => {
    await expect(page.locator('table')).toBeVisible();
    await expect(page.locator('table thead')).toBeVisible();
    await expect(page.locator('table tbody')).toBeVisible();
  });

  test('站点管理显示操作按钮', async ({ page }) => {
    await expect(page.locator('button')).toBeVisible();
  });

  test('导航到其他页面从站点管理', async ({ page }) => {
    // 测试导航到概览页面
    await page.click('text="概览"');
    await page.waitForURL('/');
    await expect(page).toHaveURL('/');

    // 测试导航回站点管理
    await page.click('text="站点管理"');
    await page.waitForURL('/sites');
    await expect(page).toHaveURL('/sites');
  });

  test('站点创建流程测试', async ({ page }) => {
    // 点击添加站点按钮
    await page.click('button:has-text("添加站点")');
    
    // 等待模态框出现
    await page.waitForSelector('.ant-modal');
    await expect(page.locator('.ant-modal-title')).toContainText('添加站点');
    
    // 填写站点信息
    await page.fill('input[name="name"]', 'test-static-site');
    await page.fill('input[name="domain"]', 'test-static.local');
    await page.fill('input[name="port"]', '80');
    await page.selectOption('select[name="mode"]', 'static');
    
    // 点击保存按钮
    await page.click('button[type="primary"]:has-text("保存")');
    
    // 等待成功提示
    await page.waitForSelector('.ant-message-success');
    
    // 验证站点创建成功
    await expect(page.locator('.ant-message-success')).toBeVisible();
    await expect(page.locator('table')).toContainText('test-static-site');
  });

  test('站点编辑流程测试', async ({ page }) => {
    // 找到测试站点并点击编辑按钮
    const siteRow = page.locator('tr:has-text("test-static-site")');
    await siteRow.locator('button').nth(0).click();
    
    // 等待编辑模态框出现
    await page.waitForSelector('.ant-modal');
    await expect(page.locator('.ant-modal-title')).toContainText('编辑站点');
    
    // 修改站点名称
    await page.fill('input[name="name"]', 'test-static-site-updated');
    
    // 点击保存按钮
    await page.click('button[type="primary"]:has-text("保存")');
    
    // 等待成功提示
    await page.waitForSelector('.ant-message-success');
    
    // 验证站点更新成功
    await expect(page.locator('.ant-message-success')).toBeVisible();
    await expect(page.locator('table')).toContainText('test-static-site-updated');
  });

  test('站点删除流程测试', async ({ page }) => {
    // 找到测试站点并点击删除按钮
    const siteRow = page.locator('tr:has-text("test-static-site-updated")');
    await siteRow.locator('button').nth(1).click();
    
    // 等待确认模态框出现
    await page.waitForSelector('.ant-modal-confirm');
    
    // 点击确认删除
    await page.click('button[type="danger"]:has-text("确定")');
    
    // 等待成功提示
    await page.waitForSelector('.ant-message-success');
    
    // 验证站点删除成功
    await expect(page.locator('.ant-message-success')).toBeVisible();
    await expect(page.locator('table')).not.toContainText('test-static-site-updated');
  });

  test('站点预渲染启用测试', async ({ page }) => {
    // 先创建一个测试站点
    await page.click('button:has-text("添加站点")');
    await page.waitForSelector('.ant-modal');
    await page.fill('input[name="name"]', 'test-prerender-site');
    await page.fill('input[name="domain"]', 'test-prerender.local');
    await page.fill('input[name="port"]', '80');
    await page.selectOption('select[name="mode"]', 'static');
    await page.click('button[type="primary"]:has-text("保存")');
    await page.waitForSelector('.ant-message-success');
    
    // 找到测试站点并启用预渲染
    const siteRow = page.locator('tr:has-text("test-prerender-site")');
    const prerenderSwitch = siteRow.locator('.ant-switch');
    await prerenderSwitch.click();
    
    // 等待成功提示
    await page.waitForSelector('.ant-message-success');
    
    // 验证预渲染启用成功
    await expect(page.locator('.ant-message-success')).toBeVisible();
  });

  test('站点防火墙启用测试', async ({ page }) => {
    // 找到测试站点并启用防火墙
    const siteRow = page.locator('tr:has-text("test-prerender-site")');
    const firewallSwitch = siteRow.locator('.ant-switch').nth(1);
    await firewallSwitch.click();
    
    // 等待成功提示
    await page.waitForSelector('.ant-message-success');
    
    // 验证防火墙启用成功
    await expect(page.locator('.ant-message-success')).toBeVisible();
  });

  test('站点静态资源管理测试', async ({ page }) => {
    // 找到测试站点并点击静态资源管理按钮
    const siteRow = page.locator('tr:has-text("test-prerender-site")');
    await siteRow.locator('button').nth(2).click();
    
    // 等待静态资源管理模态框出现
    await page.waitForSelector('.ant-modal');
    await expect(page.locator('.ant-modal-title')).toContainText('静态资源管理');
    
    // 关闭模态框
    await page.click('button:has-text("取消")');
  });

  test('站点预渲染配置测试', async ({ page }) => {
    // 找到测试站点并点击预渲染配置按钮
    const siteRow = page.locator('tr:has-text("test-prerender-site")');
    await siteRow.locator('button').nth(3).click();
    
    // 等待预渲染配置模态框出现
    await page.waitForSelector('.ant-modal');
    await expect(page.locator('.ant-modal-title')).toContainText('预渲染配置');
    
    // 关闭模态框
    await page.click('button:has-text("取消")');
  });

  test('站点WAF配置测试', async ({ page }) => {
    // 找到测试站点并点击WAF配置按钮
    const siteRow = page.locator('tr:has-text("test-prerender-site")');
    await siteRow.locator('button').nth(4).click();
    
    // 等待WAF配置页面加载
    await page.waitForURL('/sites/*/waf');
    await expect(page.locator('h1.page-title')).toContainText('WAF配置');
    
    // 导航回站点管理
    await page.click('text="站点管理"');
    await page.waitForURL('/sites');
  });

  test('站点推送配置测试', async ({ page }) => {
    // 找到测试站点并点击推送配置按钮
    const siteRow = page.locator('tr:has-text("test-prerender-site")');
    await siteRow.locator('button').nth(5).click();
    
    // 等待推送配置模态框出现
    await page.waitForSelector('.ant-modal');
    await expect(page.locator('.ant-modal-title')).toContainText('推送配置');
    
    // 关闭模态框
    await page.click('button:has-text("取消")');
  });

  test('清理测试数据', async ({ page }) => {
    // 删除测试站点
    const siteRow = page.locator('tr:has-text("test-prerender-site")');
    if (await siteRow.count() > 0) {
      await siteRow.locator('button').nth(1).click();
      await page.waitForSelector('.ant-modal-confirm');
      await page.click('button[type="danger"]:has-text("确定")');
      await page.waitForSelector('.ant-message-success');
    }
  });

  test('站点类型测试 - 代理站点', async ({ page }) => {
    // 点击添加站点按钮
    await page.click('button:has-text("添加站点")');
    
    // 等待模态框出现
    await page.waitForSelector('.ant-modal');
    await expect(page.locator('.ant-modal-title')).toContainText('添加站点');
    
    // 填写站点信息
    await page.fill('input[name="name"]', 'test-proxy-site');
    await page.fill('input[name="domain"]', 'test-proxy.local');
    await page.fill('input[name="port"]', '80');
    await page.selectOption('select[name="mode"]', 'proxy');
    await page.fill('input[name="proxyPass"]', 'http://example.com');
    
    // 点击保存按钮
    await page.click('button[type="primary"]:has-text("保存")');
    
    // 等待成功提示
    await page.waitForSelector('.ant-message-success');
    
    // 验证站点创建成功
    await expect(page.locator('.ant-message-success')).toBeVisible();
    await expect(page.locator('table')).toContainText('test-proxy-site');
    
    // 清理测试数据
    const siteRow = page.locator('tr:has-text("test-proxy-site")');
    if (await siteRow.count() > 0) {
      await siteRow.locator('button').nth(1).click();
      await page.waitForSelector('.ant-modal-confirm');
      await page.click('button[type="danger"]:has-text("确定")');
      await page.waitForSelector('.ant-message-success');
    }
  });

  test('站点类型测试 - 重定向站点', async ({ page }) => {
    // 点击添加站点按钮
    await page.click('button:has-text("添加站点")');
    
    // 等待模态框出现
    await page.waitForSelector('.ant-modal');
    await expect(page.locator('.ant-modal-title')).toContainText('添加站点');
    
    // 填写站点信息
    await page.fill('input[name="name"]', 'test-redirect-site');
    await page.fill('input[name="domain"]', 'test-redirect.local');
    await page.fill('input[name="port"]', '80');
    await page.selectOption('select[name="mode"]', 'redirect');
    await page.fill('input[name="redirectTarget"]', 'http://example.com');
    
    // 点击保存按钮
    await page.click('button[type="primary"]:has-text("保存")');
    
    // 等待成功提示
    await page.waitForSelector('.ant-message-success');
    
    // 验证站点创建成功
    await expect(page.locator('.ant-message-success')).toBeVisible();
    await expect(page.locator('table')).toContainText('test-redirect-site');
    
    // 清理测试数据
    const siteRow = page.locator('tr:has-text("test-redirect-site")');
    if (await siteRow.count() > 0) {
      await siteRow.locator('button').nth(1).click();
      await page.waitForSelector('.ant-modal-confirm');
      await page.click('button[type="danger"]:has-text("确定")');
      await page.waitForSelector('.ant-message-success');
    }
  });

  test('站点启用/禁用测试', async ({ page }) => {
    // 先创建一个测试站点
    await page.click('button:has-text("添加站点")');
    await page.waitForSelector('.ant-modal');
    await page.fill('input[name="name"]', 'test-enable-site');
    await page.fill('input[name="domain"]', 'test-enable.local');
    await page.fill('input[name="port"]', '80');
    await page.selectOption('select[name="mode"]', 'static');
    await page.click('button[type="primary"]:has-text("保存")');
    await page.waitForSelector('.ant-message-success');
    
    // 找到测试站点并禁用
    const siteRow = page.locator('tr:has-text("test-enable-site")');
    const enableSwitch = siteRow.locator('.ant-switch').nth(2);
    await enableSwitch.click();
    
    // 等待成功提示
    await page.waitForSelector('.ant-message-success');
    await expect(page.locator('.ant-message-success')).toBeVisible();
    
    // 重新启用站点
    await enableSwitch.click();
    await page.waitForSelector('.ant-message-success');
    await expect(page.locator('.ant-message-success')).toBeVisible();
    
    // 清理测试数据
    await siteRow.locator('button').nth(1).click();
    await page.waitForSelector('.ant-modal-confirm');
    await page.click('button[type="danger"]:has-text("确定")');
    await page.waitForSelector('.ant-message-success');
  });

  test('站点搜索测试', async ({ page }) => {
    // 先创建几个测试站点
    const testSites = [
      { name: 'site-1', domain: 'site-1.local' },
      { name: 'site-2', domain: 'site-2.local' },
      { name: 'site-3', domain: 'site-3.local' }
    ];
    
    for (const site of testSites) {
      await page.click('button:has-text("添加站点")');
      await page.waitForSelector('.ant-modal');
      await page.fill('input[name="name"]', site.name);
      await page.fill('input[name="domain"]', site.domain);
      await page.fill('input[name="port"]', '80');
      await page.selectOption('select[name="mode"]', 'static');
      await page.click('button[type="primary"]:has-text("保存")');
      await page.waitForSelector('.ant-message-success');
    }
    
    // 测试搜索功能
    await page.fill('input[placeholder="搜索站点"]', 'site-1');
    await page.keyboard.press('Enter');
    await page.waitForLoadState('networkidle');
    await expect(page.locator('table')).toContainText('site-1');
    await expect(page.locator('table')).not.toContainText('site-2');
    
    // 清空搜索
    await page.fill('input[placeholder="搜索站点"]', '');
    await page.keyboard.press('Enter');
    await page.waitForLoadState('networkidle');
    
    // 清理测试数据
    for (const site of testSites) {
      const siteRow = page.locator(`tr:has-text("${site.name}")`);
      if (await siteRow.count() > 0) {
        await siteRow.locator('button').nth(1).click();
        await page.waitForSelector('.ant-modal-confirm');
        await page.click('button[type="danger"]:has-text("确定")');
        await page.waitForSelector('.ant-message-success');
      }
    }
  });
});
