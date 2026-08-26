// 全站模拟点击巡检: 官网(全部路由) + 后台控制台(登录+导航)
// 用法: node scripts/browser_audit.js <siteBaseURL> <consoleBaseURL>
const { chromium } = require('playwright');

const SITE = process.argv[2] || 'http://127.0.0.1:4173';
const CONSOLE = process.argv[3] || 'http://127.0.0.1:19597';

const siteRoutes = [
  '/', '/features', '/pain-points', '/tech-principle',
  '/competitor-comparison', '/pricing', '/installation', '/tech-doc',
  '/article/spa-seo', '/article/prerender-tech', '/article/security-threats',
  '/article/waf-firewall', '/article/crawler-identification', '/article/smart-routing',
  '/article/rendering-warming', '/article/headless-chromium', '/article/redis-cache',
  '/article/open-source', '/article/system-requirements', '/article/config-management',
];

(async () => {
  const browser = await chromium.launch();
  const issues = [];
  let checked = 0;

  // ── 官网巡检 ──
  const ctx = await browser.newContext();
  const page = await ctx.newPage();
  page.on('pageerror', (e) => issues.push(`[SITE][JS-ERROR] ${page.url()} :: ${e.message}`));
  page.on('console', (m) => {
    if (m.type() === 'error') issues.push(`[SITE][CONSOLE] ${page.url()} :: ${m.text().slice(0, 150)}`);
  });

  for (const route of siteRoutes) {
    try {
      const resp = await page.goto(SITE + route, { waitUntil: 'networkidle', timeout: 20000 });
      if (!resp.ok()) issues.push(`[SITE][HTTP] ${route} -> ${resp.status()}`);
      // 检查空白页(渲染失败时 #app 为空)
      const htmlLen = await page.evaluate(() => document.getElementById('app')?.innerHTML.length || 0);
      if (htmlLen < 200) issues.push(`[SITE][EMPTY] ${route} app content ${htmlLen}b`);
      // 检查 h1 唯一性(SEO)
      const h1Count = await page.locator('h1').count();
      if (h1Count !== 1) issues.push(`[SITE][SEO] ${route} h1 count=${h1Count}`);
      // 点击所有站内链接, 只收集不跳转(验证无死链)
      const brokenLinks = await page.evaluate(() => {
        const out = [];
        document.querySelectorAll('a[href]').forEach((a) => {
          const href = a.getAttribute('href');
          if (!href || href.startsWith('http') || href.startsWith('mailto')) return;
          if (href.includes('yourusername') || href === '#' || href.endsWith('/undefined')) {
            out.push(href);
          }
        });
        return out;
      });
      brokenLinks.forEach((h) => issues.push(`[SITE][LINK] ${route} suspicious href: ${h}`));
      checked++;
    } catch (e) {
      issues.push(`[SITE][NAV-FAIL] ${route} :: ${e.message.slice(0, 120)}`);
    }
  }

  // 语言切换冒烟
  try {
    await page.goto(SITE + '/', { waitUntil: 'networkidle' });
    const btn = page.locator('.lang-switch');
    if (await btn.count() > 0) {
      await btn.click();
      await page.waitForTimeout(300);
      const navText = await page.locator('.nav-links a').first().textContent();
      if (navText.trim() !== 'Home' && navText.trim() !== '首页') {
        issues.push(`[SITE][I18N] switch failed, first nav link = "${navText}"`);
      }
      await btn.click(); // 切回
    } else {
      issues.push('[SITE][I18N] language switcher not found');
    }
    checked++;
  } catch (e) {
    issues.push(`[SITE][I18N-FAIL] ${e.message.slice(0, 120)}`);
  }

  // ── 后台控制台巡检 ──
  const cctx = await browser.newContext();
  const cpage = await cctx.newPage();
  cpage.on('pageerror', (e) => issues.push(`[ADMIN][JS-ERROR] ${cpage.url()} :: ${e.message.slice(0, 150)}`));

  try {
    await cpage.goto(CONSOLE + '/', { waitUntil: 'networkidle', timeout: 20000 });
    checked++;
    // 登录页应出现
    const hasLogin = await cpage.locator('input[type="password"]').count();
    if (hasLogin === 0) issues.push('[ADMIN][LOGIN] no password input on landing');

    // 通过 API 登录拿 token 写入 localStorage 后进入控制台
    const loginResp = await cpage.request.post(CONSOLE + '/api/v1/auth/login', {
      data: { username: 'apitest', password: 'ApiTest#2026' },
    });
    if (loginResp.ok()) {
      const j = await loginResp.json();
      const token = j?.data?.token || j?.data?.access_token;
      if (token) {
        await cpage.evaluate((t) => localStorage.setItem('token', t), token);
        const adminRoutes = [
          '/', '/sites', '/firewall', '/firewall/rules', '/dashboard',
          '/prerender', '/prerender/preheat', '/prerender/push',
          '/monitoring', '/monitoring/alerts', '/logs', '/crawler',
          '/system', '/ssl', '/settings',
        ];
        for (const r of adminRoutes) {
          try {
            await cpage.goto(CONSOLE + r, { waitUntil: 'networkidle', timeout: 20000 });
            const len = await cpage.evaluate(() => document.getElementById('root')?.innerHTML.length || 0);
            if (len < 200) issues.push(`[ADMIN][EMPTY] ${r} root content ${len}b`);
            checked++;
          } catch (e) {
            issues.push(`[ADMIN][NAV-FAIL] ${r} :: ${e.message.slice(0, 100)}`);
          }
        }
      } else {
        issues.push('[ADMIN][LOGIN] no token in response');
      }
    } else {
      issues.push(`[ADMIN][LOGIN] api status ${loginResp.status()}`);
    }
  } catch (e) {
    issues.push(`[ADMIN][FAIL] ${e.message.slice(0, 150)}`);
  }

  await browser.close();
  console.log(`\n===== BROWSER AUDIT: ${checked} checks, ${issues.length} issues =====`);
  issues.forEach((i) => console.log('  ' + i));
  process.exit(issues.length ? 1 : 0);
})();
