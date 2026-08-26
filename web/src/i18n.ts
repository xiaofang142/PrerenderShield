import i18n, { type Resource } from 'i18next';
import { initReactI18next } from 'react-i18next';
import LanguageDetector from 'i18next-browser-languagedetector';

import zh from './locales/zh.json';
import en from './locales/en.json';
import ja from './locales/ja.json';
import ko from './locales/ko.json';

// 支持的语言：中文、英文、日文、韩文
// 注意 i18next 资源结构为 { 语言: { 命名空间: 词条 } }，此处命名空间固定为 translation
const resources: Record<string, Record<string, unknown>> = {
  zh: { translation: { ...(zh as Record<string, unknown>) } },
  en: { translation: { ...(en as Record<string, unknown>) } },
  ja: { translation: { ...(ja as Record<string, unknown>) } },
  ko: { translation: { ...(ko as Record<string, unknown>) } }
};

// ─── 页面级命名空间 locale（静态导入，确定性打包）───
import alertConfig_en from './locales/pages/alertConfig.en.json'
import alertConfig_ja from './locales/pages/alertConfig.ja.json'
import alertConfig_ko from './locales/pages/alertConfig.ko.json'
import alertConfig_zh from './locales/pages/alertConfig.zh.json'
import crawlerPage_en from './locales/pages/crawlerPage.en.json'
import crawlerPage_ja from './locales/pages/crawlerPage.ja.json'
import crawlerPage_ko from './locales/pages/crawlerPage.ko.json'
import crawlerPage_zh from './locales/pages/crawlerPage.zh.json'
import dashboard_en from './locales/pages/dashboard.en.json'
import dashboard_ja from './locales/pages/dashboard.ja.json'
import dashboard_ko from './locales/pages/dashboard.ko.json'
import dashboard_zh from './locales/pages/dashboard.zh.json'
import firewallPage_en from './locales/pages/firewallPage.en.json'
import firewallPage_ja from './locales/pages/firewallPage.ja.json'
import firewallPage_ko from './locales/pages/firewallPage.ko.json'
import firewallPage_zh from './locales/pages/firewallPage.zh.json'
import firewallRules_en from './locales/pages/firewallRules.en.json'
import firewallRules_ja from './locales/pages/firewallRules.ja.json'
import firewallRules_ko from './locales/pages/firewallRules.ko.json'
import firewallRules_zh from './locales/pages/firewallRules.zh.json'
import logs_en from './locales/pages/logs.en.json'
import logs_ja from './locales/pages/logs.ja.json'
import logs_ko from './locales/pages/logs.ko.json'
import logs_zh from './locales/pages/logs.zh.json'
import monitoringPage_en from './locales/pages/monitoringPage.en.json'
import monitoringPage_ja from './locales/pages/monitoringPage.ja.json'
import monitoringPage_ko from './locales/pages/monitoringPage.ko.json'
import monitoringPage_zh from './locales/pages/monitoringPage.zh.json'
import overview_en from './locales/pages/overview.en.json'
import overview_ja from './locales/pages/overview.ja.json'
import overview_ko from './locales/pages/overview.ko.json'
import overview_zh from './locales/pages/overview.zh.json'
import preheat_en from './locales/pages/preheat.en.json'
import preheat_ja from './locales/pages/preheat.ja.json'
import preheat_ko from './locales/pages/preheat.ko.json'
import preheat_zh from './locales/pages/preheat.zh.json'
import prerender_en from './locales/pages/prerender.en.json'
import prerender_ja from './locales/pages/prerender.ja.json'
import prerender_ko from './locales/pages/prerender.ko.json'
import prerender_zh from './locales/pages/prerender.zh.json'
import settings_en from './locales/pages/settings.en.json'
import settings_ja from './locales/pages/settings.ja.json'
import settings_ko from './locales/pages/settings.ko.json'
import settings_zh from './locales/pages/settings.zh.json'
import sites_en from './locales/pages/sites.en.json'
import sites_ja from './locales/pages/sites.ja.json'
import sites_ko from './locales/pages/sites.ko.json'
import sites_zh from './locales/pages/sites.zh.json'
import ssl_en from './locales/pages/ssl.en.json'
import ssl_ja from './locales/pages/ssl.ja.json'
import ssl_ko from './locales/pages/ssl.ko.json'
import ssl_zh from './locales/pages/ssl.zh.json'
import system_en from './locales/pages/system.en.json'
import system_ja from './locales/pages/system.ja.json'
import system_ko from './locales/pages/system.ko.json'
import system_zh from './locales/pages/system.zh.json'
import wafSettings_en from './locales/pages/wafSettings.en.json'
import wafSettings_ja from './locales/pages/wafSettings.ja.json'
import wafSettings_ko from './locales/pages/wafSettings.ko.json'
import wafSettings_zh from './locales/pages/wafSettings.zh.json'

// ─── 页面级命名空间 locale 深度合并 ───
// 文件命名: locales/pages/<namespace>.<lang>.json（如 sites.zh.json / sites.en.json）
// 页面各自的 key 挂到 translation 根下，与基础文件同名 key 时页面文件优先。
type Json = Record<string, unknown>;

function isPlainObject(v: unknown): v is Json {
  return typeof v === 'object' && v !== null && !Array.isArray(v);
}

function deepMerge(base: Json, override: Json): Json {
  const out: Json = { ...base };
  for (const [k, v] of Object.entries(override)) {
    out[k] = isPlainObject(v) && isPlainObject(out[k]) ? deepMerge(out[k] as Json, v) : v;
  }
  return out;
}



// 将页面命名空间合并进各语言的 translation 根
{
  const pageFiles: Array<Json> = []
pageFiles.push({ zh: alertConfig_zh, en: alertConfig_en, ja: alertConfig_ja, ko: alertConfig_ko })
pageFiles.push({ zh: crawlerPage_zh, en: crawlerPage_en, ja: crawlerPage_ja, ko: crawlerPage_ko })
pageFiles.push({ zh: dashboard_zh, en: dashboard_en, ja: dashboard_ja, ko: dashboard_ko })
pageFiles.push({ zh: firewallPage_zh, en: firewallPage_en, ja: firewallPage_ja, ko: firewallPage_ko })
pageFiles.push({ zh: firewallRules_zh, en: firewallRules_en, ja: firewallRules_ja, ko: firewallRules_ko })
pageFiles.push({ zh: logs_zh, en: logs_en, ja: logs_ja, ko: logs_ko })
pageFiles.push({ zh: monitoringPage_zh, en: monitoringPage_en, ja: monitoringPage_ja, ko: monitoringPage_ko })
pageFiles.push({ zh: overview_zh, en: overview_en, ja: overview_ja, ko: overview_ko })
pageFiles.push({ zh: preheat_zh, en: preheat_en, ja: preheat_ja, ko: preheat_ko })
pageFiles.push({ zh: prerender_zh, en: prerender_en, ja: prerender_ja, ko: prerender_ko })
pageFiles.push({ zh: settings_zh, en: settings_en, ja: settings_ja, ko: settings_ko })
pageFiles.push({ zh: sites_zh, en: sites_en, ja: sites_ja, ko: sites_ko })
pageFiles.push({ zh: ssl_zh, en: ssl_en, ja: ssl_ja, ko: ssl_ko })
pageFiles.push({ zh: system_zh, en: system_en, ja: system_ja, ko: system_ko })
pageFiles.push({ zh: wafSettings_zh, en: wafSettings_en, ja: wafSettings_ja, ko: wafSettings_ko })
  // 页面文件自带命名空间顶层键（如 { "sites": {...} }），直接深度合并进 translation 根
  for (const byLang of pageFiles) {
    for (const lang of Object.keys(byLang)) {
      const langFile = resources[lang].translation as Json
      langFile[lang] === undefined
      const value = byLang[lang]
      resources[lang].translation = isPlainObject(value)
        ? deepMerge(langFile, value)
        : langFile
    }
  }
}

i18n
  .use(LanguageDetector)
  .use(initReactI18next)
  .init({
    resources: resources as unknown as Resource,
    fallbackLng: 'zh',
    interpolation: {
      escapeValue: false,
    },
    react: {
      useSuspense: false, // 资源为同步打包注入，无需 Suspense 边界
    },
  });

export default i18n;
