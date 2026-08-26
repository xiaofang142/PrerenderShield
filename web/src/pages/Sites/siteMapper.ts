/**
 * Sites 页面纯数据转换逻辑（自 Sites.tsx 抽离，便于单元测试锁定行为）
 */

/** 后端站点响应 → 前端列表行模型（兼容 snake_case / camelCase 双命名） */
export function mapSiteResponse(site: any): any {
  return {
    id: site.id || site.ID,
    name: site.name || site.Name || '未知站点',
    domain: site.domains?.[0] || site.domain || '127.0.0.1',
    domains: site.domains || [],
    port: site.port || 80,
    mode: site.mode || 'proxy',
    firewallEnabled: Boolean(site.firewall?.enabled),
    prerenderEnabled: Boolean(site.prerender?.enabled),

    // 映射完整的配置对象，确保编辑时表单能回填数据
    proxy: site.proxy || {},
    redirect: site.redirect || {},
    firewall: site.firewall || {},
    file_integrity: site.file_integrity || {},
    routing: site.routing || {},

    // 映射完整的渲染预热配置对象
    prerender: {
      enabled: site.prerender?.enabled || false,
      poolSize: site.prerender?.pool_size || site.prerender?.poolSize || 5,
      minPoolSize: site.prerender?.min_pool_size || site.prerender?.minPoolSize || 2,
      maxPoolSize: site.prerender?.max_pool_size || site.prerender?.maxPoolSize || 20,
      timeout: site.prerender?.timeout || 30,
      cacheTTL: site.prerender?.cache_ttl || site.prerender?.cacheTTL || 3600,
      idleTimeout: site.prerender?.idle_timeout || site.prerender?.idleTimeout || 300,
      dynamicScaling: site.prerender?.dynamic_scaling !== false && site.prerender?.dynamicScaling !== false,
      scalingFactor: site.prerender?.scaling_factor || site.prerender?.scalingFactor || 0.5,
      scalingInterval: site.prerender?.scaling_interval || site.prerender?.scalingInterval || 60,
      useDefaultHeaders: site.prerender?.use_default_headers || site.prerender?.useDefaultHeaders || false,
      crawlerHeaders: site.prerender?.crawler_headers || site.prerender?.crawlerHeaders || [],
      preheat: {
        enabled: site.prerender?.preheat?.enabled || false,
        sitemapURL: site.prerender?.preheat?.sitemap_url || site.prerender?.preheat?.sitemapURL || '',
        schedule: site.prerender?.preheat?.schedule || '0 0 * * *',
        concurrency: site.prerender?.preheat?.concurrency || 5,
        defaultPriority: site.prerender?.preheat?.default_priority || site.prerender?.preheat?.defaultPriority || 0,
        maxDepth: site.prerender?.preheat?.max_depth || site.prerender?.preheat?.maxDepth || 3
      },
      push: {
        enabled: site.prerender?.push?.enabled || false,
        baiduAPI: site.prerender?.push?.baidu_api || site.prerender?.push?.baiduAPI || 'http://data.zz.baidu.com/urls',
        baiduToken: site.prerender?.push?.baidu_token || site.prerender?.push?.baiduToken || '',
        bingAPI: site.prerender?.push?.bing_api || site.prerender?.push?.bingAPI || 'https://ssl.bing.com/webmaster/api.svc/json/SubmitUrl',
        bingToken: site.prerender?.push?.bing_token || site.prerender?.push?.bingToken || '',
        baiduDailyLimit: site.prerender?.push?.baidu_daily_limit || site.prerender?.push?.baiduDailyLimit || 1000,
        bingDailyLimit: site.prerender?.push?.bing_daily_limit || site.prerender?.push?.bingDailyLimit || 1000,
        pushDomain: site.prerender?.push?.push_domain || site.prerender?.push?.pushDomain || '',
        schedule: site.prerender?.push?.schedule || '0 1 * * *'
      }
    }
  }
}

/** 编辑弹窗表单回填值构造：端口转字符串 + 下划线命名转驼峰 + 默认值兜底 */
export function buildEditFormValues(site: any): any {
  return {
    ...site,
    port: String(site.port),
    // 转换firewall配置
    firewall: {
      ...site.firewall,
      action: {
        ...site.firewall?.action,
        defaultAction: site.firewall?.action?.default_action || 'block',
        blockMessage: site.firewall?.action?.block_message || 'Request blocked by firewall'
      },
      geoip: {
        ...site.firewall?.geoip,
        allowList: site.firewall?.geoip?.allow_list || [],
        blockList: site.firewall?.geoip?.block_list || []
      },
      rate_limit: site.firewall?.rate_limit ? {
        ...site.firewall.rate_limit,
        requests: site.firewall.rate_limit.requests || 100,
        window: site.firewall.rate_limit.window || 60,
        ban_time: site.firewall.rate_limit.ban_time || 3600
      } : {
        enabled: false,
        requests: 100,
        window: 60,
        ban_time: 3600
      }
    },
    // 转换file_integrity配置
    file_integrity: site.file_integrity ? {
      ...site.file_integrity,
      check_interval: site.file_integrity.check_interval || 300,
      hash_algorithm: site.file_integrity.hash_algorithm || 'sha256'
    } : {
      enabled: false,
      check_interval: 300,
      hash_algorithm: 'sha256'
    },
    // 转换prerender配置
    prerender: {
      ...site.prerender,
      poolSize: site.prerender?.pool_size || 5,
      minPoolSize: site.prerender?.min_pool_size || 2,
      maxPoolSize: site.prerender?.max_pool_size || 20,
      cacheTTL: site.prerender?.cache_ttl || 3600,
      idleTimeout: site.prerender?.idle_timeout || 300,
      dynamicScaling: site.prerender?.dynamic_scaling !== false,
      scalingFactor: site.prerender?.scaling_factor || 0.5,
      scalingInterval: site.prerender?.scaling_interval || 60,
      useDefaultHeaders: site.prerender?.use_default_headers || false,
      crawlerHeaders: site.prerender?.crawler_headers || [],
      preheat: {
        ...site.prerender?.preheat,
        sitemapURL: site.prerender?.preheat?.sitemap_url || '',
        defaultPriority: site.prerender?.preheat?.default_priority || 0,
        maxDepth: site.prerender?.preheat?.max_depth || 3
      },
      push: {
        ...site.prerender?.push,
        baiduAPI: site.prerender?.push?.baidu_api || 'http://data.zz.baidu.com/urls',
        baiduToken: site.prerender?.push?.baidu_token || '',
        baiduDailyLimit: site.prerender?.push?.baidu_daily_limit || 1000,
        bingAPI: site.prerender?.push?.bing_api || 'https://ssl.bing.com/webmaster/api.svc/json/SubmitUrl',
        bingToken: site.prerender?.push?.bing_token || '',
        bingDailyLimit: site.prerender?.push?.bing_daily_limit || 1000,
        pushDomain: site.prerender?.push?.push_domain || ''
      }
    }
  }
}
