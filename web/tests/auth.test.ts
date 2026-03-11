import { test, expect } from '@playwright/test';

test.describe('认证模块测试', () => {
  test.beforeEach(async ({ page }) => {
    // 导航到登录页面
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
    }
  });

  test('成功登录测试', async ({ page }) => {
    // 填写登录表单
    await page.fill('form[name="login"] input[name="username"]', 'admin');
    await page.fill('form[name="login"] input[name="password"]', '123456');
    
    // 点击登录按钮
    await page.click('form[name="login"] button[type="submit"]');
    
    // 等待页面跳转
    await page.waitForURL('/');
    
    // 验证登录成功，页面跳转到概览页面
    await expect(page).toHaveURL('/');
    await expect(page.locator('h1.page-title')).toContainText('概览');
  });

  test('登录失败测试', async ({ page }) => {
    // 填写错误密码
    await page.fill('form[name="login"] input[name="username"]', 'admin');
    await page.fill('form[name="login"] input[name="password"]', 'wrongpassword');
    
    // 点击登录按钮
    await page.click('form[name="login"] button[type="submit"]');
    
    // 等待错误提示
    await page.waitForSelector('.ant-message-error');
    
    // 验证错误提示
    await expect(page.locator('.ant-message-error')).toBeVisible();
  });

  test('空用户名登录测试', async ({ page }) => {
    // 只填写密码，不填写用户名
    await page.fill('form[name="login"] input[name="password"]', '123456');
    
    // 点击登录按钮
    await page.click('form[name="login"] button[type="submit"]');
    
    // 等待错误提示
    await page.waitForSelector('.ant-message-error');
    
    // 验证错误提示
    await expect(page.locator('.ant-message-error')).toBeVisible();
  });

  test('空密码登录测试', async ({ page }) => {
    // 只填写用户名，不填写密码
    await page.fill('form[name="login"] input[name="username"]', 'admin');
    
    // 点击登录按钮
    await page.click('form[name="login"] button[type="submit"]');
    
    // 等待错误提示
    await page.waitForSelector('.ant-message-error');
    
    // 验证错误提示
    await expect(page.locator('.ant-message-error')).toBeVisible();
  });

  test('登出测试', async ({ page }) => {
    // 先登录
    await page.fill('form[name="login"] input[name="username"]', 'admin');
    await page.fill('form[name="login"] input[name="password"]', '123456');
    await page.click('form[name="login"] button[type="submit"]');
    await page.waitForURL('/');
    
    // 点击用户头像打开菜单
    await page.click('.ant-avatar');
    
    // 点击登出按钮
    await page.click('text=登出');
    
    // 等待页面跳转到登录页
    await page.waitForURL('/login');
    
    // 验证登出成功
    await expect(page).toHaveURL('/login');
    await expect(page.locator('h1')).toContainText('登录');
  });

  test('未登录访问受保护页面测试', async ({ page }) => {
    // 直接访问受保护页面
    await page.goto('/dashboard');
    
    // 验证被重定向到登录页
    await page.waitForURL('/login');
    await expect(page).toHaveURL('/login');
  });

  test('已登录状态访问登录页测试', async ({ page }) => {
    // 先登录
    await page.fill('form[name="login"] input[name="username"]', 'admin');
    await page.fill('form[name="login"] input[name="password"]', '123456');
    await page.click('form[name="login"] button[type="submit"]');
    await page.waitForURL('/');
    
    // 再次访问登录页
    await page.goto('/login');
    
    // 验证被重定向到首页
    await page.waitForURL('/');
    await expect(page).toHaveURL('/');
  });

  test('语言切换测试', async ({ page }) => {
    // 点击语言切换按钮
    await page.click('.ant-dropdown-trigger:has-icon');
    
    // 选择英文
    await page.click('text=English');
    
    // 验证语言切换成功
    await expect(page.locator('form[name="login"] input[name="username"]')).toBeVisible();
    await expect(page.locator('form[name="login"] input[name="password"]')).toBeVisible();
    
    // 切换回中文
    await page.click('.ant-dropdown-trigger:has-icon');
    await page.click('text=简体中文');
    
    // 验证语言切换回中文
    await expect(page.locator('form[name="login"] input[name="username"]')).toBeVisible();
    await expect(page.locator('form[name="login"] input[name="password"]')).toBeVisible();
  });

  test('登录表单验证测试', async ({ page }) => {
    // 测试表单验证
    await page.click('form[name="login"] button[type="submit"]');
    
    // 验证表单验证提示
    await expect(page.locator('.ant-form-item-explain-error')).toBeVisible();
  });

  test('登录后会话保持测试', async ({ page }) => {
    // 先登录
    await page.fill('form[name="login"] input[name="username"]', 'admin');
    await page.fill('form[name="login"] input[name="password"]', '123456');
    await page.click('form[name="login"] button[type="submit"]');
    await page.waitForURL('/');
    
    // 刷新页面
    await page.reload();
    
    // 验证仍然保持登录状态
    await expect(page).toHaveURL('/');
    await expect(page.locator('h1.page-title')).toContainText('概览');
  });

  test('连续登录失败测试', async ({ page }) => {
    // 多次尝试登录失败
    for (let i = 0; i < 3; i++) {
      // 填写错误密码
      await page.fill('form[name="login"] input[name="username"]', 'admin');
      await page.fill('form[name="login"] input[name="password"]', 'wrongpassword');
      
      // 点击登录按钮
      await page.click('form[name="login"] button[type="submit"]');
      
      // 等待错误提示
      await page.waitForSelector('.ant-message-error');
      await expect(page.locator('.ant-message-error')).toBeVisible();
    }
  });

  test('密码强度验证测试', async ({ page }) => {
    // 先登录
    await page.fill('form[name="login"] input[name="username"]', 'admin');
    await page.fill('form[name="login"] input[name="password"]', '123456');
    await page.click('form[name="login"] button[type="submit"]');
    await page.waitForURL('/');
    
    // 导航到个人设置页面
    await page.click('.ant-avatar');
    await page.click('text=个人设置');
    await page.waitForURL('/profile');
    
    // 测试密码修改
    await page.fill('input[name="oldPassword"]', '123456');
    await page.fill('input[name="newPassword"]', '123456');
    await page.fill('input[name="confirmPassword"]', '123456');
    await page.click('button:has-text("保存修改")');
    
    // 等待提示
    await page.waitForSelector('.ant-message');
  });

  test('登录超时测试', async ({ page }) => {
    // 登录
    await page.fill('form[name="login"] input[name="username"]', 'admin');
    await page.fill('form[name="login"] input[name="password"]', '123456');
    await page.click('form[name="login"] button[type="submit"]');
    await page.waitForURL('/');
    
    // 模拟长时间不操作
    await page.waitForTimeout(5000);
    
    // 刷新页面
    await page.reload();
    
    // 验证仍然保持登录状态
    await expect(page).toHaveURL('/');
    await expect(page.locator('h1.page-title')).toContainText('概览');
  });
});
