import { test, expect } from '@playwright/test';

test.describe('WAF Settings Page', () => {
  test.beforeEach(async ({ page }) => {
    // 登录
    await page.goto('/login');
    // 等待页面完全加载
    await page.waitForLoadState('networkidle');
    // 等待登录表单出现
    await page.waitForSelector('form[name="login"]');
    // 填写登录表单
    await page.fill('form[name="login"] input[name="username"]', 'admin');
    await page.fill('form[name="login"] input[name="password"]', '123456');
    // 点击登录按钮
    await page.click('form[name="login"] button[type="submit"]');
    // 导航到WAF设置页面
    await page.click('text=WAF Settings');
    await page.waitForURL('/waf-settings');
  });

  test('WAF settings page loads successfully', async ({ page }) => {
    await expect(page).toHaveTitle(/WAF Settings/);
    await expect(page.locator('h1')).toContainText('WAF Settings');
  });

  test('WAF settings shows general configuration', async ({ page }) => {
    await expect(page.locator('.general-config')).toBeVisible();
  });

  test('WAF settings allows saving configuration', async ({ page }) => {
    await page.click('button:has-text("Save Settings")');
    await expect(page.locator('.ant-message-success')).toBeVisible();
  });

  test('WAF settings shows rule sets', async ({ page }) => {
    await expect(page.locator('.rule-sets')).toBeVisible();
  });

  test('WAF settings shows threat protection', async ({ page }) => {
    await expect(page.locator('.threat-protection')).toBeVisible();
  });

  test('WAF settings shows bot protection', async ({ page }) => {
    await expect(page.locator('.bot-protection')).toBeVisible();
  });

  test('WAF settings allows enabling/disabling modules', async ({ page }) => {
    await expect(page.locator('.ant-switch')).toBeVisible();
  });

  test('WAF settings shows security statistics', async ({ page }) => {
    await expect(page.locator('.security-statistics')).toBeVisible();
  });
});
