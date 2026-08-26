import { describe, it, expect } from 'vitest'
import { mapSiteResponse, buildEditFormValues } from '../siteMapper'

describe('mapSiteResponse', () => {
  it('snake_case 后端响应完整映射并应用默认值', () => {
    const mapped = mapSiteResponse({
      id: 's1',
      name: 'Site One',
      domains: ['a.com', 'b.com'],
      port: 8080,
      mode: 'static',
      firewall: { enabled: true },
      prerender: {
        enabled: true,
        pool_size: 8,
        preheat: { enabled: true, sitemap_url: 'https://a.com/sitemap.xml' },
        push: { enabled: true, baidu_token: 'tok' }
      }
    })

    expect(mapped.domain).toBe('a.com')
    expect(mapped.firewallEnabled).toBe(true)
    expect(mapped.prerender.poolSize).toBe(8)
    expect(mapped.prerender.minPoolSize).toBe(2) // 默认值
    expect(mapped.prerender.preheat.sitemapURL).toBe('https://a.com/sitemap.xml')
    expect(mapped.prerender.push.baiduAPI).toBe('http://data.zz.baidu.com/urls') // 默认 API
  })

  it('camelCase 响应同样兼容', () => {
    const mapped = mapSiteResponse({
      ID: 's2',
      Name: 'Camel Site',
      prerender: { poolSize: 12, preheat: { sitemapURL: 'u' }, push: { baiduToken: 't' } }
    })
    expect(mapped.id).toBe('s2')
    expect(mapped.prerender.poolSize).toBe(12)
    expect(mapped.prerender.preheat.sitemapURL).toBe('u')
    expect(mapped.prerender.push.bingDailyLimit).toBe(1000)
  })

  it('空对象全默认值且不抛异常', () => {
    const mapped = mapSiteResponse({})
    expect(mapped.name).toBe('未知站点')
    expect(mapped.port).toBe(80)
    expect(mapped.mode).toBe('proxy')
    expect(mapped.prerender.dynamicScaling).toBe(true) // undefined !== false
    expect(mapped.prerender.crawlerHeaders).toEqual([])
    expect(mapped.domains).toEqual([])
  })

  it('dynamic_scaling 显式 false 时保持 false', () => {
    const mapped = mapSiteResponse({ prerender: { dynamic_scaling: false } })
    expect(mapped.prerender.dynamicScaling).toBe(false)
  })
})

describe('buildEditFormValues', () => {
  it('端口转字符串 + 下划线转驼峰 + rate_limit 兜底', () => {
    const v = buildEditFormValues({
      port: 3000,
      firewall: {
        action: { default_action: 'allow' },
        geoip: { allow_list: ['CN'] }
      }
    })
    expect(v.port).toBe('3000')
    expect(v.firewall.action.defaultAction).toBe('allow')
    expect(v.firewall.geoip.allowList).toEqual(['CN'])
    expect(v.firewall.rate_limit).toEqual({ enabled: false, requests: 100, window: 60, ban_time: 3600 })
  })

  it('已有 rate_limit 保留原值，缺失字段补默认', () => {
    const v = buildEditFormValues({
      port: 80,
      firewall: { rate_limit: { enabled: true, requests: 50 } },
      file_integrity: { hash_algorithm: 'md5' }
    })
    expect(v.firewall.rate_limit.requests).toBe(50)
    expect(v.firewall.rate_limit.ban_time).toBe(3600)
    expect(v.file_integrity.check_interval).toBe(300)
    expect(v.file_integrity.hash_algorithm).toBe('md5')
  })

  it('prerender 空配置补齐全部默认值', () => {
    const v = buildEditFormValues({ port: 1 })
    expect(v.prerender.poolSize).toBe(5)
    expect(v.prerender.maxPoolSize).toBe(20)
    expect(v.prerender.cacheTTL).toBe(3600)
    expect(v.prerender.preheat.maxDepth).toBe(3)
    expect(v.prerender.push.bingAPI).toContain('bing.com')
  })
})
