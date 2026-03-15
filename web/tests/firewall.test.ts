import { test, expect } from '@playwright/test';

test.describe('Firewall Page', () => {
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
      await page.waitForSelector('input[placeholder="Username"]');
      // 填写登录表单
      await page.fill('input[placeholder="Username"]', 'admin');
      await page.fill('input[placeholder="Password"]', '123456');
      // 点击登录按钮
      await page.click('button[type="submit"]');
      // 等待导航到首页
      await page.waitForURL('/');
    }
    
    // 直接导航到防火墙页面
    await page.goto('/firewall');
    await page.waitForURL('/firewall');
  });

  test('Firewall page loads successfully', async ({ page }) => {
    await expect(page.locator('h1.page-title')).toContainText('防火墙');
  });

  test('Firewall shows rules list', async ({ page }) => {
    await expect(page.locator('table')).toBeVisible();
    await expect(page.locator('table thead')).toBeVisible();
    await expect(page.locator('table tbody')).toBeVisible();
  });

  test('Firewall allows creating new rule', async ({ page }) => {
    // 点击添加规则按钮
    await page.click('button:has-text("添加规则")');
    
    // 等待规则表单出现
    await page.waitForSelector('.ant-modal');
    await expect(page.locator('.ant-modal-title')).toContainText('添加规则');
    
    // 填写规则信息
    await page.fill('input[name="name"]', 'test-rule');
    await page.selectOption('select[name="action"]', 'block');
    await page.fill('input[name="pattern"]', 'test-pattern');
    
    // 点击保存按钮
    await page.click('button[type="primary"]:has-text("保存")');
    
    // 等待成功提示
    await page.waitForSelector('.ant-message-success');
    await expect(page.locator('.ant-message-success')).toBeVisible();
  });

  test('Firewall allows editing existing rule', async ({ page }) => {
    // 找到测试规则并点击编辑按钮
    const ruleRow = page.locator('tr:has-text("test-rule")');
    await ruleRow.locator('button').nth(0).click();
    
    // 等待编辑表单出现
    await page.waitForSelector('.ant-modal');
    await expect(page.locator('.ant-modal-title')).toContainText('编辑规则');
    
    // 修改规则名称
    await page.fill('input[name="name"]', 'test-rule-updated');
    
    // 点击保存按钮
    await page.click('button[type="primary"]:has-text("保存")');
    
    // 等待成功提示
    await page.waitForSelector('.ant-message-success');
    await expect(page.locator('.ant-message-success')).toBeVisible();
  });

  test('Firewall allows deleting rule', async ({ page }) => {
    // 找到测试规则并点击删除按钮
    const ruleRow = page.locator('tr:has-text("test-rule-updated")');
    await ruleRow.locator('button').nth(1).click();
    
    // 等待确认对话框出现
    await page.waitForSelector('.ant-modal-confirm');
    
    // 点击确认删除
    await page.click('button[type="danger"]:has-text("确定")');
    
    // 等待成功提示
    await page.waitForSelector('.ant-message-success');
    await expect(page.locator('.ant-message-success')).toBeVisible();
  });

  test('Firewall allows enabling/disabling rules', async ({ page }) => {
    // 验证开关按钮存在
    await expect(page.locator('.ant-switch')).toBeVisible();
    
    // 找到一个规则并切换状态
    const ruleRow = page.locator('tr').first();
    const ruleSwitch = ruleRow.locator('.ant-switch');
    await ruleSwitch.click();
    
    // 等待成功提示
    await page.waitForSelector('.ant-message-success');
    await expect(page.locator('.ant-message-success')).toBeVisible();
  });

  test('Firewall shows rule statistics', async ({ page }) => {
    await expect(page.locator('.ant-statistic')).toBeVisible();
  });

  test('Firewall shows blocked requests', async ({ page }) => {
    await expect(page.locator('.ant-card:has-text("拦截请求")')).toBeVisible();
  });



  test('Firewall rate limit configuration test', async ({ page }) => {
    // 点击频率限制配置按钮
    await page.click('button:has-text("频率限制")');
    
    // 等待配置模态框出现
    await page.waitForSelector('.ant-modal');
    await expect(page.locator('.ant-modal-title')).toContainText('频率限制');
    
    // 关闭模态框
    await page.click('button:has-text("取消")');
  });

  test('Firewall attack interception test', async ({ page }) => {
    // 导航到攻击拦截页面
    await page.goto('/firewall/attack');
    await page.waitForURL('/firewall/attack');
    
    // 验证页面加载成功
    await expect(page.locator('h1.page-title')).toContainText('攻击拦截');
    
    // 验证攻击拦截数据显示
    await expect(page.locator('.ant-card')).toBeVisible();
  });

  test('Firewall geoip configuration test', async ({ page }) => {
    // 点击地理位置配置按钮
    await page.click('button:has-text("地理位置配置")');
    
    // 等待配置模态框出现
    await page.waitForSelector('.ant-modal');
    await expect(page.locator('.ant-modal-title')).toContainText('地理位置配置');
    
    // 测试添加地理位置规则
    await page.fill('input[name="country"]', 'CN');
    await page.selectOption('select[name="action"]', 'allow');
    await page.click('button[type="primary"]:has-text("保存")');
    await page.waitForSelector('.ant-message-success');
    await expect(page.locator('.ant-message-success')).toBeVisible();
  });

  test('Firewall IP whitelist/blacklist test', async ({ page }) => {
    // 点击IP黑白名单配置按钮
    await page.click('button:has-text("IP黑白名单")');
    
    // 等待配置模态框出现
    await page.waitForSelector('.ant-modal');
    await expect(page.locator('.ant-modal-title')).toContainText('IP黑白名单');
    
    // 测试添加IP白名单
    await page.fill('input[name="ip"]', '192.168.1.1');
    await page.selectOption('select[name="type"]', 'whitelist');
    await page.click('button[type="primary"]:has-text("保存")');
    await page.waitForSelector('.ant-message-success');
    await expect(page.locator('.ant-message-success')).toBeVisible();
  });

  test('Firewall rule priority test', async ({ page }) => {
    // 测试规则优先级调整
    await page.click('button:has-text("添加规则")');
    await page.waitForSelector('.ant-modal');
    await page.fill('input[name="name"]', 'high-priority-rule');
    await page.selectOption('select[name="action"]', 'block');
    await page.fill('input[name="pattern"]', 'high-priority');
    await page.fill('input[name="priority"]', '1');
    await page.click('button[type="primary"]:has-text("保存")');
    await page.waitForSelector('.ant-message-success');
    await expect(page.locator('.ant-message-success')).toBeVisible();
  });

  test('Firewall rule filtering test', async ({ page }) => {
    // 测试规则过滤功能
    await page.fill('input[placeholder="搜索规则"]', 'test');
    await page.keyboard.press('Enter');
    await page.waitForLoadState('networkidle');
  });
});
